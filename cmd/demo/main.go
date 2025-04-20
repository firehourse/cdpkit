package main

import (
	"fmt"
	"log"
	"time"

	"github.com/firehourse/cdpkit/browser"
	"github.com/firehourse/cdpkit/config"
	"github.com/firehourse/cdpkit/tab"
)

func main() {
	// 設定詳細的日誌輸出
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("cdpkit 示例程序啟動")

	// 創建配置
	cfg := config.Config{
		// 留空時會自動查找或啟動 Chrome
		WebSocketURL: "",

		// 如果找不到系統 Chrome，可以指定路徑
		// ChromePath: "/usr/bin/google-chrome",

		// 遠程調試埠
		RemotePort: 9222,

		// 操作超時設置
		Timeout: 60 * time.Second,

		// 瀏覽器視窗大小 (若設為 [0,0] 則使用默認值)
		WindowSize: [2]int{1280, 720},

		// 用戶代理字符串 (若留空則隨機選擇)
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",

		// Chrome 啟動選項
		Flags: map[string]interface{}{
			"headless":              true, // 無頭模式
			"no-sandbox":            true, // 沙箱限制
			"disable-gpu":           true, // 禁用 GPU 加速
			"disable-dev-shm-usage": true, // 禁用 /dev/shm (低內存環境)
		},
	}

	// 步驟 1: 建立瀏覽器管理器
	log.Println("步驟 1: 初始化瀏覽器管理器")
	bm, err := browser.NewManagerFromConfig(cfg)
	if err != nil {
		log.Fatalf("初始化失敗: %v", err)
	}
	defer func() {
		log.Println("關閉瀏覽器")
		bm.Shutdown()
	}()

	// 步驟 2: 創建一個新分頁 (使用 NewTab，會自動套用配置)
	log.Println("步驟 2: 創建新分頁")
	ctx, cancel, err := bm.NewPageContext()
	if err != nil {
		log.Fatalf("創建分頁失敗: %v", err)
	}

	// 使用新的 NewTab 方法自動設置 UA、視窗和注入腳本
	pageTab := tab.NewTab(ctx, cancel, cfg)
	defer func() {
		log.Println("關閉分頁")
		pageTab.Close(bm)
	}()

	// 確保有足夠時間初始化
	time.Sleep(1 * time.Second)

	// 步驟 3: 瀏覽網頁並獲取 HTML
	url := "https://example.org"
	log.Printf("步驟 3: 瀏覽 %s", url)

	if err := pageTab.Navigate(url, 30*time.Second); err != nil {
		log.Fatalf("導航失敗: %v", err)
	}

	// 等待頁面加載完成
	time.Sleep(2 * time.Second)

	// 步驟 4: 獲取頁面 HTML
	log.Println("步驟 4: 獲取頁面 HTML")
	html, err := pageTab.HTML(30 * time.Second)
	if err != nil {
		log.Fatalf("獲取 HTML 失敗: %v", err)
	}

	// 顯示 HTML 摘要
	if len(html) > 200 {
		fmt.Printf("HTML 摘要 (前 200 字符):\n%s...\n", html[:200])
	} else {
		fmt.Printf("HTML:\n%s\n", html)
	}

	// 示範 JS 執行
	log.Println("步驟 5: 執行 JavaScript")
	result, err := pageTab.RunJS(`
		// 獲取頁面標題
		document.title;
	`, 5*time.Second)

	if err != nil {
		log.Printf("JS 執行失敗: %v", err)
	} else {
		log.Printf("頁面標題: %v", result)
	}

	log.Println("示例程序執行完成")
}
