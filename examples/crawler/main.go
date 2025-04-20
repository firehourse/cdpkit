package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/firehourse/cdpkit/browser"
	"github.com/firehourse/cdpkit/config"
	"github.com/firehourse/cdpkit/tab"
)

// 爬蟲配置
type CrawlerConfig struct {
	// 最大併發連接數
	Concurrency int
	// 爬取的網址列表
	URLs []string
	// 代理 URL
	ProxyURL string
	// Chrome 調試埠
	Port int
	// 自定義腳本
	CustomJS string
	// 結果輸出路徑
	OutputPath string
	// 超時設置
	Timeout time.Duration
}

// 爬取結果
type ScrapeResult struct {
	URL         string      `json:"url"`
	Title       string      `json:"title"`
	Content     string      `json:"content,omitempty"`
	ScriptData  interface{} `json:"script_data,omitempty"`
	Error       string      `json:"error,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
	ElapsedTime string      `json:"elapsed_time"`
}

func main() {
	// 解析命令行參數
	var cfg CrawlerConfig
	flag.IntVar(&cfg.Concurrency, "concurrency", 5, "最大併發連接數")
	flag.StringVar(&cfg.ProxyURL, "proxy", "", "代理 URL，例如 http://user:pass@proxy.example.com:8080")
	flag.IntVar(&cfg.Port, "port", 9222, "Chrome 調試埠")
	flag.StringVar(&cfg.CustomJS, "js", "", "自定義 JS 腳本文件路徑")
	flag.StringVar(&cfg.OutputPath, "output", "results.json", "結果輸出路徑")
	flag.DurationVar(&cfg.Timeout, "timeout", 60*time.Second, "操作超時時間")
	flag.Parse()

	// 獲取要爬取的 URL 列表
	if flag.NArg() == 0 {
		log.Fatal("請指定至少一個要爬取的 URL")
	}
	cfg.URLs = flag.Args()

	// 讀取自定義 JS 腳本
	var customScript string
	if cfg.CustomJS != "" {
		scriptBytes, err := os.ReadFile(cfg.CustomJS)
		if err != nil {
			log.Fatalf("無法讀取腳本文件 %s: %v", cfg.CustomJS, err)
		}
		customScript = string(scriptBytes)
	} else {
		// 默認的提取腳本
		customScript = `
			// 默認提取頁面標題和主要內容
			(function() {
				return {
					title: document.title,
					metaDesc: document.querySelector('meta[name="description"]')?.content || '',
					text: Array.from(document.body.querySelectorAll('h1, h2, h3, p'))
						.map(el => el.textContent.trim())
						.filter(text => text.length > 0)
						.join('\n')
				};
			})();
		`
	}

	// 創建爬蟲實例並執行
	crawler := NewCrawler(cfg, customScript)
	crawler.Run()
}

// Crawler 爬蟲實例
type Crawler struct {
	config       CrawlerConfig
	customScript string
	bm           *browser.BrowserManager
	results      []ScrapeResult
	mu           sync.Mutex
	wg           sync.WaitGroup
}

// NewCrawler 創建新的爬蟲實例
func NewCrawler(cfg CrawlerConfig, script string) *Crawler {
	return &Crawler{
		config:       cfg,
		customScript: script,
		results:      make([]ScrapeResult, 0, len(cfg.URLs)),
	}
}

// Run 啟動爬取流程
func (c *Crawler) Run() {
	// 捕獲 Ctrl+C 信號以優雅關閉
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		<-signalCh
		log.Println("接收到中斷信號，正在優雅退出...")
		cancel()
	}()

	// 初始化瀏覽器
	log.Println("初始化瀏覽器...")
	browserCfg := config.Config{
		RemotePort: c.config.Port,
		Timeout:    c.config.Timeout,
		WindowSize: [2]int{1280, 720},
		Flags: map[string]interface{}{
			"headless":              true,
			"no-sandbox":            true,
			"disable-gpu":           true,
			"disable-dev-shm-usage": true,
		},
	}

	// 如果設置了代理
	if c.config.ProxyURL != "" {
		// 驗證代理URL格式
		if isValidProxyURL(c.config.ProxyURL) {
			log.Printf("使用代理: %s", c.config.ProxyURL)
			browserCfg.Proxy = c.config.ProxyURL
		} else {
			log.Printf("警告: 代理URL格式不正確 '%s'，將不使用代理", c.config.ProxyURL)
		}
	}

	bm, err := browser.NewManagerFromConfig(browserCfg)
	if err != nil {
		log.Fatalf("初始化瀏覽器失敗: %v", err)
	}
	c.bm = bm
	defer bm.Shutdown()

	// 創建工作通道
	urlCh := make(chan string, c.config.Concurrency)

	// 啟動工作者
	for i := 0; i < c.config.Concurrency; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i+1, urlCh)
	}

	// 發送工作
	go func() {
		for _, url := range c.config.URLs {
			select {
			case <-ctx.Done():
				return
			case urlCh <- url:
			}
		}
		close(urlCh)
	}()

	// 等待所有工作完成
	c.wg.Wait()

	// 保存結果
	c.saveResults()
}

// worker goroutine 處理每個 URL
func (c *Crawler) worker(ctx context.Context, workerID int, urlCh <-chan string) {
	defer c.wg.Done()

	// 創建新分頁
	tabCtx, tabCancel, err := c.bm.NewPageContext()
	if err != nil {
		log.Printf("工作者 %d: 創建分頁失敗: %v", workerID, err)
		return
	}
	pageTab := tab.NewTab(tabCtx, tabCancel, config.Config{Timeout: c.config.Timeout})
	defer pageTab.Close(c.bm)

	// 處理每個 URL
	for {
		select {
		case <-ctx.Done():
			return
		case url, ok := <-urlCh:
			if !ok {
				return // 通道已關閉
			}

			result := ScrapeResult{
				URL:       url,
				Timestamp: time.Now(),
			}
			startTime := time.Now()

			log.Printf("工作者 %d: 開始處理 %s", workerID, url)
			if err := c.scrapePage(pageTab, url, &result); err != nil {
				result.Error = err.Error()
				log.Printf("工作者 %d: 爬取 %s 失敗: %v", workerID, url, err)
			} else {
				log.Printf("工作者 %d: 成功爬取 %s", workerID, url)
			}

			result.ElapsedTime = time.Since(startTime).String()

			// 儲存結果
			c.mu.Lock()
			c.results = append(c.results, result)
			c.mu.Unlock()
		}
	}
}

// scrapePage 爬取單個頁面
func (c *Crawler) scrapePage(pageTab *tab.Tab, url string, result *ScrapeResult) error {
	// 1. 導航到頁面
	if err := pageTab.Navigate(url, c.config.Timeout); err != nil {
		return fmt.Errorf("導航失敗: %w", err)
	}

	// 等待頁面加載完成
	time.Sleep(2 * time.Second)

	// 2. 適配異步腳本：在腳本中添加Promise處理邏輯
	scriptWrapper := `
		(function() {
			const result = %s;
			// 如果結果是Promise，等待它解析
			if (result && typeof result.then === 'function') {
				return new Promise((resolve) => {
					result.then(data => {
						resolve(data);
					}).catch(err => {
						resolve({error: err.toString(), fallback: document.title});
					});
				});
			}
			return result;
		})()
	`

	finalScript := fmt.Sprintf(scriptWrapper, c.customScript)
	scriptResult, err := pageTab.RunJS(finalScript, c.config.Timeout)
	if err != nil {
		return fmt.Errorf("執行腳本失敗: %w", err)
	}
	result.ScriptData = scriptResult

	// 3. 提取標題
	title, err := pageTab.RunJS("document.title", c.config.Timeout)
	if err == nil && title != nil {
		result.Title = fmt.Sprintf("%v", title)
	}

	// 4. 獲取 HTML（可選，取決於是否需要保存完整 HTML）
	// html, err := pageTab.HTML(c.config.Timeout)
	// if err == nil {
	// 	result.Content = html
	// }

	return nil
}

// saveResults 保存結果到文件
func (c *Crawler) saveResults() {
	log.Printf("正在保存 %d 個結果到 %s", len(c.results), c.config.OutputPath)

	// 將結果序列化為 JSON
	jsonData, err := json.MarshalIndent(c.results, "", "  ")
	if err != nil {
		log.Fatalf("序列化結果失敗: %v", err)
	}

	// 寫入文件
	if err := os.WriteFile(c.config.OutputPath, jsonData, 0644); err != nil {
		log.Fatalf("寫入結果文件失敗: %v", err)
	}

	log.Printf("結果已保存到 %s", c.config.OutputPath)
}

// isValidProxyURL 驗證代理URL格式是否正確
func isValidProxyURL(proxyURL string) bool {
	// 檢查常見的代理URL前綴
	validPrefixes := []string{
		"http://", "https://", "socks5://", "socks4://", "socks4a://", "socks5h://",
	}

	// 檢查是否以有效前綴開頭
	hasValidPrefix := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(proxyURL, prefix) {
			hasValidPrefix = true
			break
		}
	}

	if !hasValidPrefix {
		return false
	}

	// 檢查是否包含主機部分
	parts := strings.Split(strings.TrimPrefix(proxyURL, "http://"), ":")
	if len(parts) < 2 {
		parts = strings.Split(strings.TrimPrefix(proxyURL, "https://"), ":")
	}
	if len(parts) < 2 {
		parts = strings.Split(strings.TrimPrefix(proxyURL, "socks5://"), ":")
	}

	// 至少需要主機名和端口
	if len(parts) < 2 {
		return false
	}

	// 檢查端口是否為數字
	_, err := strconv.Atoi(parts[len(parts)-1])
	return err == nil
}
