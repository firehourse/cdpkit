package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/firehourse/cdpkit/browser"
	"github.com/firehourse/cdpkit/config"
	"github.com/firehourse/cdpkit/tab"
)

func main() {
	// Step 1: 初始化 Manager（共用一個 Chromium 實例）
	cfg := config.Config{
		TabLimit: 10,
	}
	manager, err := browser.NewManagerFromConfig(cfg)
	if err != nil {
		panic(err)
	}
	defer manager.Shutdown()

	// Step 2: 同時開啟多個 tab，每個 goroutine 控制一個
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			ctx, cancel, _ := manager.NewPageContext()
			defer cancel()

			t := tab.New(ctx, cancel)

			url := fmt.Sprintf("https://httpbin.org/html?tab=%d", idx)
			if err := t.Navigate(url, 5*time.Second); err != nil {
				fmt.Println("Navigate error:", err)
				return
			}

			if err := t.WaitVisible("h1", 3*time.Second); err != nil {
				fmt.Println("WaitVisible error:", err)
				return
			}

			result, err := t.RunJS("document.querySelector('h1').textContent", 2*time.Second)
			if err != nil {
				fmt.Println("RunJS error:", err)
				return
			}

			fmt.Printf("Tab %d h1: %v\n", idx, result)
		}(i)
	}

	wg.Wait()
	fmt.Println("✅ 所有 tab 已執行完畢")
}
