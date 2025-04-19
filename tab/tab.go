// === tab/tab.go ===
package tab

import (
	"context"
	"math/rand"
	"time"

	"github.com/chromedp/cdproto/emulation" // ← UA 設定改用 emulation domain
	"github.com/chromedp/chromedp"
	"github.com/firehourse/cdpkit/browser"
	"github.com/firehourse/cdpkit/config"
)

func init() { rand.Seed(time.Now().UnixNano()) }

// Tab 包裹單一 chromedp Context 與輔助方法
type Tab struct {
	Ctx     context.Context
	Cancel  context.CancelFunc
	Timeout time.Duration
}

// New 由 BrowserManager 建立完 Context 後包裝成 Tab
func New(ctx context.Context, cancel context.CancelFunc, timeout time.Duration) *Tab {
	return &Tab{Ctx: ctx, Cancel: cancel, Timeout: timeout}
}

// DefaultTimeout 取預設逾時 (fallback 30 s)
func (t *Tab) DefaultTimeout() time.Duration {
	if t.Timeout <= 0 {
		return 30 * time.Second
	}
	return t.Timeout
}

// Navigate 前往 URL
func (t *Tab) Navigate(url string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = t.DefaultTimeout()
	}
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()
	return chromedp.Run(ctx, chromedp.Navigate(url))
}

// RunJS 執行 JS
func (t *Tab) RunJS(script string, timeout time.Duration) (interface{}, error) {
	if timeout <= 0 {
		timeout = t.DefaultTimeout()
	}
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()
	var res interface{}
	err := chromedp.Run(ctx, chromedp.Evaluate(script, &res))
	return res, err
}

// HTML 取得整頁 HTML
func (t *Tab) HTML(timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = t.DefaultTimeout()
	}
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()
	var html string
	err := chromedp.Run(ctx, chromedp.OuterHTML("html", &html))
	return html, err
}

// WaitVisible 等待元素出現
func (t *Tab) WaitVisible(sel string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = t.DefaultTimeout()
	}
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()
	return chromedp.Run(ctx, chromedp.WaitVisible(sel, chromedp.ByQuery))
}

// Close 關閉分頁
func (t *Tab) Close(mgr *browser.BrowserManager) {
	if t.Cancel != nil {
		t.Cancel()
		t.Cancel = nil
	}
	t.Ctx = nil
	if mgr != nil {
		mgr.DecrementTabCount()
	}
}

// Spoof 移除 navigator.webdriver
func (t *Tab) Spoof() error {
	_, err := t.RunJS(
		`Object.defineProperty(navigator, 'webdriver', {get: () => undefined})`,
		t.DefaultTimeout(),
	)
	return err
}

// -------------------- 附加工具 --------------------

func randomUA() string {
	ua := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	}
	return ua[rand.Intn(len(ua))]
}

// ApplyConfig 套用 UA、視窗尺寸、隱蔽 JS
func (t *Tab) ApplyConfig(cfg config.Config) error {
	// ---- UA ----
	ua := cfg.UserAgent
	if ua == "" {
		ua = randomUA()
	}

	// ---- 視窗尺寸 ----
	w, h := cfg.WindowSize[0], cfg.WindowSize[1]
	if w == 0 || h == 0 {
		w = 1280 + rand.Intn(201) - 100 // 1180‑1380
		h = 720 + rand.Intn(201) - 100  // 620‑820
	}

	ctx, cancel := context.WithTimeout(t.Ctx, t.DefaultTimeout())
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.EmulateViewport(int64(w), int64(h)),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return emulation.SetUserAgentOverride(ua).Do(ctx)
		}),
		chromedp.Evaluate(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined})`, nil),
	)
}
