package config

import (
	"time"

	"github.com/chromedp/chromedp"
)

// FlagMergeFunc 是 flags 合併策略
type FlagMergeFunc func(
	defaultFlags []chromedp.ExecAllocatorOption,
	userFlags []chromedp.ExecAllocatorOption,
) []chromedp.ExecAllocatorOption

// Config 定義所有可注入的啟動參數
type Config struct {
	// 預設 flags，可被覆蓋
	DefaultFlags []chromedp.ExecAllocatorOption

	// 使用者 flags，會被合併（或覆蓋）進 defaultFlags 中
	Flags []chromedp.ExecAllocatorOption

	// 合併邏輯，可自定義，若為 nil 則 fallback 為內建合併策略
	MergeFn FlagMergeFunc

	// 分頁數限制
	TabLimit int

	// 啟動瀏覽器後的全域 context timeout（預留給未來擴充）
	Timeout time.Duration
}

// SafeDefaults 提供一份「安全但可擴充」的預設 flags，可作為 defaultFlags 的 fallback 用
func SafeDefaults() []chromedp.ExecAllocatorOption {
	return []chromedp.ExecAllocatorOption{
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
	}
}
