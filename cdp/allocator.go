package cdp

import (
	"context"

	"github.com/firehourse/cdpkit/config"

	"github.com/chromedp/chromedp"
)

// NewAllocator 建立 chromedp 的執行環境（ExecAllocator）
// 會根據 Config 的 Flags / MergeFn 注入 Chrome 啟動參數
// 此為最底層構建，實際返回 context 與其取消函數
func NewAllocator(cfg config.Config) (context.Context, context.CancelFunc) {
	var mergedFlags []chromedp.ExecAllocatorOption

	if cfg.MergeFn != nil {
		// 使用外部注入的 flags 合併策略
		mergedFlags = cfg.MergeFn(config.DefaultConfig().Flags, cfg.Flags)
	} else {
		// 使用預設 flags 合併策略
		mergedFlags = collectFlags(config.DefaultConfig().Flags, cfg.Flags)
	}

	var allocCtx context.Context
	var allocCancel context.CancelFunc
	allocCtx, allocCancel = chromedp.NewExecAllocator(context.Background(), mergedFlags...)

	return allocCtx, allocCancel
}

// collectFlags 是內建 flags 合併策略（default + user override）
// 若 key 重複，userFlags 會覆蓋 defaultFlags
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
