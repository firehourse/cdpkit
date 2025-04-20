package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/firehourse/cdpkit/crawler"
)

func main() {
	// 解析命令行參數
	var opts crawler.Options

	// 基本選項
	flag.IntVar(&opts.Concurrency, "concurrency", 3, "最大併發數")
	flag.DurationVar(&opts.Timeout, "timeout", 60*time.Second, "操作超時時間")
	flag.StringVar(&opts.ProxyURL, "proxy", "", "代理URL (例如 http://user:pass@proxy.example.com:8080)")
	flag.BoolVar(&opts.Headless, "headless", true, "是否使用無頭模式")
	flag.BoolVar(&opts.SaveHTML, "save-html", false, "是否保存完整HTML")
	flag.IntVar(&opts.LogLevel, "log-level", 3, "日誌級別 (0=無, 1=錯誤, 2=警告, 3=信息, 4=調試)")

	// 自定義腳本
	scriptPath := flag.String("js", "", "自定義JS腳本文件路徑")
	outputPath := flag.String("output", "results.json", "結果輸出路徑")

	flag.Parse()

	// 獲取要爬取的URL列表
	urls := flag.Args()
	if len(urls) == 0 {
		log.Fatal("請指定至少一個要爬取的URL")
	}

	// 讀取自定義JS腳本
	var jsScript string
	if *scriptPath != "" {
		scriptBytes, err := os.ReadFile(*scriptPath)
		if err != nil {
			log.Fatalf("無法讀取腳本文件 %s: %v", *scriptPath, err)
		}
		jsScript = string(scriptBytes)
	} else {
		// 默認的提取腳本
		jsScript = `
			// 默認提取頁面標題和基本信息
			{
				title: document.title,
				metaDescription: document.querySelector('meta[name="description"]')?.content || '',
				h1: Array.from(document.querySelectorAll('h1')).map(el => el.textContent.trim()),
				links: Array.from(document.querySelectorAll('a[href]'))
					.slice(0, 10)  // 僅取前10個鏈接
					.map(el => ({
						text: el.textContent.trim(),
						href: el.getAttribute('href')
					}))
			}
		`
	}

	log.Println("正在初始化爬蟲...")

	// 創建爬蟲實例
	c, err := crawler.New(opts)
	if err != nil {
		log.Fatalf("創建爬蟲失敗: %v", err)
	}
	defer c.Close()

	log.Printf("開始爬取 %d 個URL...", len(urls))

	// 執行爬取
	startTime := time.Now()
	results, err := c.FetchAll(urls, jsScript)
	if err != nil {
		log.Fatalf("爬取失敗: %v", err)
	}

	// 輸出統計信息
	elapsedTime := time.Since(startTime)
	log.Printf("爬取完成，共 %d 個頁面，耗時: %v", len(results), elapsedTime)

	// 將結果保存為JSON
	jsonData, err := crawler.ResultsToJSON(results)
	if err != nil {
		log.Fatalf("序列化結果失敗: %v", err)
	}

	// 寫入文件
	if err := os.WriteFile(*outputPath, jsonData, 0644); err != nil {
		log.Fatalf("寫入結果文件失敗: %v", err)
	}
	log.Printf("結果已保存到 %s", *outputPath)

	// 簡單展示部分結果
	for i, result := range results {
		if i >= 3 {
			fmt.Printf("... 以及 %d 個其他結果\n", len(results)-3)
			break
		}

		fmt.Printf("\n--- 結果 #%d ---\n", i+1)
		fmt.Printf("URL: %s\n", result.URL)
		fmt.Printf("標題: %s\n", result.Title)
		if result.Error != "" {
			fmt.Printf("錯誤: %s\n", result.Error)
		} else if result.Data != nil {
			// 展示部分數據
			if title, ok := result.Data["title"]; ok {
				fmt.Printf("頁面標題: %v\n", title)
			}

			if links, ok := result.Data["links"].([]interface{}); ok {
				fmt.Printf("發現 %d 個鏈接\n", len(links))
			}
		}
	}
}
