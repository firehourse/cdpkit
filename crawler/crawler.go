package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/firehourse/cdpkit/browser"
	"github.com/firehourse/cdpkit/config"
	"github.com/firehourse/cdpkit/tab"
)

// Result 表示單個頁面的爬取結果
type Result struct {
	URL           string                 `json:"url"`
	Title         string                 `json:"title,omitempty"`
	HTML          string                 `json:"html,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Error         string                 `json:"error,omitempty"`
	ResponseCode  int                    `json:"response_code,omitempty"`
	ElapsedTime   time.Duration          `json:"elapsed_time,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	RawJSResponse interface{}            `json:"-"` // 原始JS返回值，不序列化
}

// Options 爬蟲配置選項
type Options struct {
	// 最大並發數
	Concurrency int
	// 超時設置
	Timeout time.Duration
	// 代理URL
	ProxyURL string
	// 用戶代理
	UserAgent string
	// 窗口大小 [寬,高]
	WindowSize [2]int
	// 是否無頭模式
	Headless bool
	// 是否禁用JavaScript
	DisableJS bool
	// 瀏覽器啟動標誌
	BrowserFlags map[string]interface{}
	// 調試端口
	DebugPort int
	// 是否保存完整HTML
	SaveHTML bool
	// 日誌級別 (0=無, 1=錯誤, 2=警告, 3=信息, 4=調試)
	LogLevel int
}

// DefaultOptions 返回默認配置選項
func DefaultOptions() Options {
	return Options{
		Concurrency: 5,
		Timeout:     60 * time.Second,
		WindowSize:  [2]int{1280, 720},
		Headless:    true,
		DebugPort:   9222,
		LogLevel:    3, // 默認信息級別
		BrowserFlags: map[string]interface{}{
			"no-sandbox":            true,
			"disable-gpu":           true,
			"disable-dev-shm-usage": true,
		},
	}
}

// Crawler 爬蟲客戶端
type Crawler struct {
	options Options
	bm      *browser.BrowserManager
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
}

// New 創建新的爬蟲客戶端
func New(options Options) (*Crawler, error) {
	// 套用默認值
	opts := DefaultOptions()

	// 覆蓋用戶提供的選項
	if options.Concurrency > 0 {
		opts.Concurrency = options.Concurrency
	}
	if options.Timeout > 0 {
		opts.Timeout = options.Timeout
	}
	if options.ProxyURL != "" {
		opts.ProxyURL = options.ProxyURL
	}
	if options.UserAgent != "" {
		opts.UserAgent = options.UserAgent
	}
	if options.WindowSize[0] > 0 && options.WindowSize[1] > 0 {
		opts.WindowSize = options.WindowSize
	}
	if options.DebugPort > 0 {
		opts.DebugPort = options.DebugPort
	}
	opts.Headless = options.Headless
	opts.DisableJS = options.DisableJS
	opts.SaveHTML = options.SaveHTML
	if options.LogLevel > 0 {
		opts.LogLevel = options.LogLevel
	}

	// 合併瀏覽器標誌
	if options.BrowserFlags != nil {
		for k, v := range options.BrowserFlags {
			opts.BrowserFlags[k] = v
		}
	}

	// 設置是否無頭模式
	opts.BrowserFlags["headless"] = opts.Headless

	// 創建上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 初始化瀏覽器
	browserCfg := config.Config{
		RemotePort: opts.DebugPort,
		Timeout:    opts.Timeout,
		WindowSize: opts.WindowSize,
		UserAgent:  opts.UserAgent,
		Flags:      opts.BrowserFlags,
	}

	// 設置代理
	if opts.ProxyURL != "" {
		if isValidProxyURL(opts.ProxyURL) {
			logf(opts.LogLevel, 3, "使用代理: %s", opts.ProxyURL)
			browserCfg.Proxy = opts.ProxyURL
		} else {
			logf(opts.LogLevel, 2, "警告: 代理URL格式不正確 '%s'，將不使用代理", opts.ProxyURL)
		}
	}

	// 初始化瀏覽器管理器
	bm, err := browser.NewManagerFromConfig(browserCfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("初始化瀏覽器失敗: %w", err)
	}

	return &Crawler{
		options: opts,
		bm:      bm,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Close 關閉爬蟲客戶端和瀏覽器
func (c *Crawler) Close() {
	c.cancel()
	if c.bm != nil {
		c.bm.Shutdown()
		c.bm = nil
	}
}

// Fetch 爬取單個頁面
func (c *Crawler) Fetch(url string, jsScript string) (Result, error) {
	result := Result{
		URL:       url,
		Timestamp: time.Now(),
	}

	// 創建新分頁
	tabCtx, tabCancel, err := c.bm.NewPageContext()
	if err != nil {
		return result, fmt.Errorf("創建分頁失敗: %w", err)
	}

	pageTab := tab.NewTab(tabCtx, tabCancel, config.Config{Timeout: c.options.Timeout})
	defer pageTab.Close(c.bm)

	startTime := time.Now()

	// 導航到頁面
	if err := pageTab.Navigate(url, c.options.Timeout); err != nil {
		result.Error = fmt.Sprintf("導航失敗: %v", err)
		return result, fmt.Errorf("導航失敗: %w", err)
	}

	// 等待頁面加載
	time.Sleep(2 * time.Second)

	// 獲取頁面標題
	title, err := pageTab.RunJS("document.title", c.options.Timeout)
	if err == nil && title != nil {
		result.Title = fmt.Sprintf("%v", title)
	}

	// 執行自定義腳本
	if jsScript != "" {
		// 包裝腳本處理異步情況
		scriptWrapper := `
			(function() {
				const result = %s;
				// 如果結果是Promise，等待它解析
				if (result && typeof result.then === 'function') {
					return new Promise((resolve) => {
						result.then(data => {
							resolve(data);
						}).catch(err => {
							resolve({error: err.toString()});
						});
					});
				}
				return result;
			})()
		`

		finalScript := fmt.Sprintf(scriptWrapper, jsScript)
		scriptResult, err := pageTab.RunJS(finalScript, c.options.Timeout)
		if err != nil {
			result.Error = fmt.Sprintf("執行腳本失敗: %v", err)
		} else {
			result.RawJSResponse = scriptResult

			// 嘗試轉換為map
			if m, ok := scriptResult.(map[string]interface{}); ok {
				result.Data = m
			} else {
				// 如果不是map，放入特殊鍵
				result.Data = map[string]interface{}{
					"result": scriptResult,
				}
			}
		}
	}

	// 獲取HTML（如果需要）
	if c.options.SaveHTML {
		html, err := pageTab.HTML(c.options.Timeout)
		if err == nil {
			result.HTML = html
		}
	}

	result.ElapsedTime = time.Since(startTime)
	return result, nil
}

// FetchAll 批量爬取多個頁面
func (c *Crawler) FetchAll(urls []string, jsScript string) ([]Result, error) {
	results := make([]Result, 0, len(urls))
	resultCh := make(chan Result, len(urls))

	// 創建URL通道
	urlCh := make(chan string, c.options.Concurrency)

	// 啟動工作協程
	var wg sync.WaitGroup
	for i := 0; i < c.options.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for url := range urlCh {
				logf(c.options.LogLevel, 3, "工作者 %d: 開始處理 %s", workerID, url)
				result, err := c.Fetch(url, jsScript)
				if err != nil {
					logf(c.options.LogLevel, 2, "工作者 %d: 爬取 %s 失敗: %v", workerID, url, err)
				} else {
					logf(c.options.LogLevel, 3, "工作者 %d: 成功爬取 %s", workerID, url)
				}
				resultCh <- result
			}
		}(i + 1)
	}

	// 發送URL到通道
	go func() {
		for _, url := range urls {
			select {
			case <-c.ctx.Done():
				break
			case urlCh <- url:
				// URL已發送
			}
		}
		close(urlCh)
	}()

	// 等待所有工作完成
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集結果
	for result := range resultCh {
		results = append(results, result)
	}

	return results, nil
}

// ToJSON 將結果轉換為JSON
func (r Result) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ToJSON 將結果數組轉換為JSON
func ResultsToJSON(results []Result) ([]byte, error) {
	return json.MarshalIndent(results, "", "  ")
}

// Helper functions

// isValidProxyURL 驗證代理URL格式是否正確
func isValidProxyURL(proxyURL string) bool {
	// 檢查是否以常見代理前綴開頭
	validPrefixes := []string{"http://", "https://", "socks5://", "socks4://"}

	for _, prefix := range validPrefixes {
		if len(proxyURL) > len(prefix) && proxyURL[:len(prefix)] == prefix {
			// 簡單檢查是否包含主機和端口
			rest := proxyURL[len(prefix):]
			if !containsPort(rest) {
				return false
			}
			return true
		}
	}

	return false
}

// containsPort 檢查字符串是否包含端口
func containsPort(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			// 確保冒號後有數字
			for j := i + 1; j < len(s); j++ {
				if s[j] >= '0' && s[j] <= '9' {
					return true
				}
			}
		}
	}
	return false
}

// logf 根據日誌級別打印日誌
func logf(configLevel, msgLevel int, format string, args ...interface{}) {
	if configLevel >= msgLevel {
		log.Printf(format, args...)
	}
}
