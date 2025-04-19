package tab

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

// Tab 代表一個 Chrome 分頁（Tab），獨立執行任務
type Tab struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

// New 建立一個新的 Tab 實例，綁定 context 與 cancel
func New(ctx context.Context, cancel context.CancelFunc) *Tab {
	return &Tab{
		Ctx:    ctx,
		Cancel: cancel,
	}
}

// Navigate 導航至指定 URL，並等待載入完成
func (t *Tab) Navigate(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.Navigate(url),
	)
}

// RunJS 執行 JavaScript 腳本，並回傳結果（未轉型）
func (t *Tab) RunJS(script string, timeout time.Duration) (interface{}, error) {
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()

	var result interface{}
	err := chromedp.Run(ctx,
		chromedp.Evaluate(script, &result),
	)
	return result, err
}

// HTML 取得整頁 HTML，可設定超時時間
func (t *Tab) HTML(timeout time.Duration) (string, error) {
	var html string
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.OuterHTML("html", &html),
	)
	return html, err
}

// WaitVisible 等待 selector 對應的元素在畫面中可見
func (t *Tab) WaitVisible(selector string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
	)
}

// Close 關閉當前分頁（釋放資源）
func (t *Tab) Close() {
	if t.Cancel != nil {
		t.Cancel()
	}
}
