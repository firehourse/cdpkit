package cdp

import (
	"context"

	"github.com/firehourse/cdpkit/config"

	"github.com/chromedp/chromedp"
)

// NewAllocator 根據使用者提供的 config，建立 Chrome allocator（執行環境）
// 支援注入 defaultFlags、flags、合併策略
func NewAllocator(cfg config.Config) (context.Context, context.CancelFunc) {
	var defaultFlags []chromedp.ExecAllocatorOption
	if cfg.DefaultFlags != nil {
		defaultFlags = cfg.DefaultFlags
	} else {
		defaultFlags = config.SafeDefaults()
	}

	var merged []chromedp.ExecAllocatorOption
	if cfg.MergeFn != nil {
		merged = cfg.MergeFn(defaultFlags, cfg.Flags)
	} else {
		merged = collectFlags(defaultFlags, cfg.Flags)
	}

	var allocCtx context.Context
	var allocCancel context.CancelFunc
	allocCtx, allocCancel = chromedp.NewExecAllocator(context.Background(), merged...)

	return allocCtx, allocCancel
}

// collectFlags 是預設合併策略（可被覆寫）
// 會將 defaultFlags + userFlags 合併，後者同名會覆蓋前者
func collectFlags(defaultFlags []chromedp.ExecAllocatorOption, userFlags []chromedp.ExecAllocatorOption) []chromedp.ExecAllocatorOption {
	var flagMap map[string]interface{} = make(map[string]interface{})

	for _, opt := range defaultFlags {
		if f, ok := opt.(chromedp.Flag); ok {
			flagMap[f.Name] = f.Value
		}
	}

	for _, opt := range userFlags {
		if f, ok := opt.(chromedp.Flag); ok {
			flagMap[f.Name] = f.Value
		}
	}

	var merged []chromedp.ExecAllocatorOption
	for k, v := range flagMap {
		merged = append(merged, chromedp.Flag(k, v))
	}

	return merged
}
