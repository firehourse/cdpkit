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

// BrowserManager 是頂層瀏覽器控制器，負責建立 tab、控制最大分頁數、重啟 Chrome 實例等。
// 內部調用 config + allocator 層，不持有具體啟動邏輯，而是由 Config 注入控制。
type BrowserManager struct {
	allocCtx context.Context    // Chrome 全域執行 context
	cancel   context.CancelFunc // 關閉用 cancel func

	tabLimit int        // 最大允許 tab 數量
	tabCount int        // 當前活躍 tab 數
	mu       sync.Mutex // tab 計數保護鎖

	config config.Config // 使用者原始配置，作為重啟 fallback 使用
}

// NewManagerFromConfig 是建立整個瀏覽器控制流程的唯一入口。
// 你可以自定義 flags、mergeFn、tab 限制等控制行為。
// 若某些欄位為零值（如 flags），將 fallback 到底層預設。
func NewManagerFromConfig(cfg config.Config) (*BrowserManager, error) {
	allocCtx, allocCancel := cdp.NewAllocator(cfg)

	tabLimit := cfg.TabLimit
	if tabLimit <= 0 {
		tabLimit = 50
	}

	return &BrowserManager{
		allocCtx: allocCtx,
		cancel:   allocCancel,
		tabLimit: tabLimit,
		tabCount: 0,
		config:   cfg,
	}, nil
}

// NewPageContext 開啟一個新的 Chrome tab。當 tab 數超過上限時，將自動重啟瀏覽器。
// 回傳新的 context（每個 tab 擁有自己的 context）。
func (bm *BrowserManager) NewPageContext() (context.Context, context.CancelFunc, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.tabCount >= bm.tabLimit {
		log.Printf("[BrowserManager] tab 達上限 %d，正在重啟 Chrome", bm.tabLimit)
		bm.restart()
	}

	ctx, cancel := chromedp.NewContext(bm.allocCtx)
	bm.tabCount++
	return ctx, cancel, nil
}

// Shutdown 結束整個瀏覽器實例，釋放 Chrome 所有資源。
func (bm *BrowserManager) Shutdown() {
	bm.cancel()
	log.Println("[BrowserManager] Chrome 已關閉")
}

// restart 根據原始 Config 重新啟動瀏覽器，並清空 tab 計數。
func (bm *BrowserManager) restart() {
	bm.cancel()
	time.Sleep(1 * time.Second)

	newCtx, newCancel := cdp.NewAllocator(bm.config)
	bm.allocCtx = newCtx
	bm.cancel = newCancel
	bm.tabCount = 0
}
