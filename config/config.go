package config

import (
	"time"

	"github.com/chromedp/chromedp"
)

// FlagMergeFunc 定義 flags 合併邏輯的函數類型
// 可注入自定義行為來取代預設的合併策略
type FlagMergeFunc func(
	defaultFlags []chromedp.ExecAllocatorOption,
	userFlags []chromedp.ExecAllocatorOption,
) []chromedp.ExecAllocatorOption

// Config 是啟動 Chrome 實例的所有可注入參數
// 此結構作為 browser.NewManagerFromConfig 的唯一輸入
type Config struct {
	// Chrome 啟動參數（如 headless, no-sandbox 等）
	Flags []chromedp.ExecAllocatorOption

	// 合併策略：將 defaultFlags + userFlags 做合併（可選）
	MergeFn FlagMergeFunc

	// 分頁上限：超過後會重啟 Chrome
	TabLimit int

	// 其他預留欄位：例如超時、UA、proxy、CDP 路徑等
	Timeout time.Duration
}

// DefaultConfig 回傳一份具有預設值的 Config，可供外部擴充或複製使用
func DefaultConfig() Config {
	return Config{
		Flags: []chromedp.ExecAllocatorOption{
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("headless", true),
			chromedp.Flag("no-sandbox", true),
		},
		MergeFn:  nil, // 若為 nil，將由 cdp 層處理 fallback
		TabLimit: 50,
		Timeout:  60 * time.Second,
	}
}
