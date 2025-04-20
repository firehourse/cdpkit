# cdpkit - Chrome DevTools Protocol 爬蟲工具包

`cdpkit` 是一個基於 Chrome DevTools Protocol 的 Go 爬蟲工具包，專為網頁爬蟲和自動化任務設計。它提供了簡潔易用的接口，可以輕鬆實現多並發網頁爬取、JavaScript 腳本執行、反爬蟲處理等功能。

## 特點

- 簡潔的 API 設計，易於使用
- 支持多併發爬取
- 內置反爬蟲機制
- 支持 JavaScript 腳本執行和頁面數據提取
- 完善的代理支持
- 穩定的瀏覽器管理
- 自動處理超時和錯誤

## 安裝

```bash
go get github.com/firehourse/cdpkit
```

## 快速入門

### 基本用法

```go
package main

import (
	"fmt"
	"github.com/firehourse/cdpkit/crawler"
	"time"
)

func main() {
	// 創建爬蟲客戶端
	c, err := crawler.New(crawler.DefaultOptions())
	if err != nil {
		panic(err)
	}
	defer c.Close()
	
	// 設置 JavaScript 腳本
	script := `
		// 返回頁面標題和所有鏈接
		{
			title: document.title,
			links: Array.from(document.querySelectorAll('a[href]'))
				.map(link => link.href)
				.slice(0, 10)
		}
	`
	
	// 爬取單個頁面
	result, err := c.Fetch("https://example.com", script)
	if err != nil {
		panic(err)
	}
	
	// 輸出結果
	fmt.Printf("標題: %s\n", result.Title)
	fmt.Printf("數據: %v\n", result.Data)
}
```

### 批量爬取

```go
// 批量爬取多個頁面
urls := []string{
	"https://example.com",
	"https://example.org",
	"https://example.net",
}

results, err := c.FetchAll(urls, script)
if err != nil {
	panic(err)
}

// 處理結果
for _, result := range results {
	fmt.Printf("URL: %s, 標題: %s\n", result.URL, result.Title)
}
```

### 自定義配置

```go
// 創建自定義配置
options := crawler.Options{
	Concurrency: 5,               // 最大並發數
	Timeout:     30 * time.Second, // 操作超時
	ProxyURL:    "http://user:pass@proxy.example.com:8080", // 代理
	Headless:    true,            // 無頭模式
	UserAgent:   "Custom User Agent", // 自定義 UA
	WindowSize:  [2]int{1280, 800}, // 視窗大小
	SaveHTML:    true,           // 保存完整 HTML
}

c, err := crawler.New(options)
```

## 範例

請參考 `examples` 目錄中的範例程序：

- `examples/simple` - 基本爬蟲範例
- `examples/crawler` - 多並發爬蟲範例

## 自定義 JavaScript 腳本

`cdpkit` 支持使用 JavaScript 腳本提取頁面數據。腳本需要返回一個 JavaScript 對象，它將被自動轉換為 Go 中的 `map[string]interface{}`。

範例腳本：

```javascript
// 返回頁面信息
(function() {
  return {
    title: document.title,
    description: document.querySelector('meta[name="description"]')?.content,
    h1: Array.from(document.querySelectorAll('h1')).map(h => h.textContent),
    links: Array.from(document.querySelectorAll('a[href]'))
      .map(a => ({ text: a.textContent, href: a.href }))
      .slice(0, 10)
  };
})();
```

## 代理支持

`cdpkit` 支持常見的代理類型：

- HTTP 代理: `http://user:pass@host:port`
- HTTPS 代理: `https://user:pass@host:port`
- SOCKS5 代理: `socks5://user:pass@host:port`

## 貢獻

歡迎提交 Pull Request 和 Issue! 