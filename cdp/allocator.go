// === cdp/allocator.go ===
package cdp

import (
	"context"

	"github.com/chromedp/chromedp"
)

// NewRemoteAllocator 連線至已啟動的 Chrome Remote Debugger
func NewRemoteAllocator(wsURL string) (context.Context, context.CancelFunc, error) {
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	return ctx, cancel, nil
}
