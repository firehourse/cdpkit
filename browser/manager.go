// === browser/manager.go ===
package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/firehourse/cdpkit/cdp"
	"github.com/firehourse/cdpkit/config"
)

// BrowserManager 管控與單一 Chrome Session 的所有分頁
type BrowserManager struct {
	allocCtx context.Context
	cancel   context.CancelFunc

	tabLimit int
	tabCount int
	mu       sync.Mutex

	config config.Config
}

// NewManagerFromConfig 建立一個連線到既有 Chrome 的管理器
func NewManagerFromConfig(cfg config.Config) (*BrowserManager, error) {
	if cfg.WebSocketURL == "" {
		return nil, errors.New("WebSocketURL 不可為空")
	}
	if !strings.HasPrefix(cfg.WebSocketURL, "ws://") && !strings.HasPrefix(cfg.WebSocketURL, "wss://") {
		return nil, fmt.Errorf("無效的 WebSocketURL: %s，必須以 ws:// 或 wss:// 開頭", cfg.WebSocketURL)
	}

	allocCtx, allocCancel, err := cdp.NewRemoteAllocator(cfg.WebSocketURL)
	if err != nil {
		return nil, fmt.Errorf("remote allocator 連線失敗: %w", err)
	}
	if cfg.WebSocketURL == "" {
		return nil, errors.New("WebSocketURL 不可為空")
	}

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

// NewPageContext 申請新的 Tab Context，超過上限時自動重啟瀏覽器
func (bm *BrowserManager) NewPageContext() (context.Context, context.CancelFunc, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.tabCount >= bm.tabLimit {
		log.Printf("[BrowserManager] 分頁達上限 %d，嘗試重啟 Chrome", bm.tabLimit)
		if err := bm.restart(); err != nil {
			return nil, nil, err
		}
	}

	ctx, cancel := chromedp.NewContext(bm.allocCtx)
	bm.tabCount++
	return ctx, cancel, nil
}

// Shutdown 關閉整個 Remote Allocator
func (bm *BrowserManager) Shutdown() {
	if bm.cancel != nil {
		bm.cancel()
		bm.cancel = nil
	}
	log.Println("[BrowserManager] Chrome 已正常關閉")
}

// restart 在同一個 WebSocketURL 上重新建立 Allocator
func (bm *BrowserManager) restart() error {
	if bm.cancel != nil {
		bm.cancel()
	}
	time.Sleep(time.Second) // 避免馬上重連失敗

	newCtx, newCancel, err := cdp.NewRemoteAllocator(bm.config.WebSocketURL)
	if err != nil {
		return fmt.Errorf("failed to create new allocator context: %w", err)
	}

	bm.allocCtx = newCtx
	bm.cancel = newCancel
	bm.tabCount = 0
	return nil
}

// DecrementTabCount 關閉分頁後回收計數
func (bm *BrowserManager) DecrementTabCount() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.tabCount > 0 {
		bm.tabCount--
	}
}
