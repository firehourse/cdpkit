package browser

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/firehourse/cdpkit/cdp"
	"github.com/firehourse/cdpkit/config"

	"github.com/chromedp/chromedp"
)

// BrowserManager 是一個統一的頂層控制器，負責調度 Chrome 分頁與實例
// 通常不直接控制 Chrome，而是透過注入的 Config 與下層 allocator 進行組裝
// 可支援 tab 數量限制、自動重啟與資源回收等功能

type BrowserManager struct {
	allocCtx context.Context    // Chromium 全域執行環境 context
	cancel   context.CancelFunc // 用於關閉整個 allocCtx
	tabLimit int                // 允許的最大分頁數
	tabCount int                // 當前已使用的分頁數
	mu       sync.Mutex         // 保護 tabCount 操作
}

// NewManagerFromConfig 是整個 cdpkit 的入口，根據注入的 config 建立 browser 實例
// - config.Flags 可自定義 flags
// - config.TabLimit 控制同時打開的分頁上限
// - config.MergeFn 可注入 flags 合併策略（預設為內建 collectFlags）
// - 所有參數皆可為零值，將套用預設邏輯
func NewManagerFromConfig(cfg config.Config) (*BrowserManager, error) {
	// 建立 allocator context（即 Chrome 啟動參數 + 配置）
	var allocCtx context.Context
	var allocCancel context.CancelFunc
	allocCtx, allocCancel = cdp.NewAllocator(cfg)

	var tabLimit int = cfg.TabLimit
	if tabLimit <= 0 {
		tabLimit = 50
	}

	return &BrowserManager{
		allocCtx: allocCtx,
		cancel:   allocCancel,
		tabLimit: tabLimit,
		tabCount: 0,
	}, nil
}

// NewPageContext 建立新的 Chrome tab，並限制 tab 數量避免資源溢出
// 若超過上限，會觸發重啟 Chrome 實例（重建 allocCtx）
func (bm *BrowserManager) NewPageContext() (context.Context, context.CancelFunc, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.tabCount >= bm.tabLimit {
		log.Printf("[BrowserManager] tab 達上限 %d，正在重啟 Chrome", bm.tabLimit)
		bm.restart()
	}

	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = chromedp.NewContext(bm.allocCtx)
	bm.tabCount++
	return ctx, cancel, nil
}

// Shutdown 完整關閉 browser 實例
func (bm *BrowserManager) Shutdown() {
	bm.cancel()
	log.Println("[BrowserManager] Chrome 已關閉")
}

// restart 強制重啟 Chrome 實例（分頁數歸零）
func (bm *BrowserManager) restart() {
	bm.cancel()
	time.Sleep(1 * time.Second)
	bm.allocCtx, bm.cancel = chromedp.NewExecAllocator(context.Background(), chromedp.DefaultExecAllocatorOptions[:]...)
	bm.tabCount = 0
}
