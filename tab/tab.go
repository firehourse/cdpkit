// === tab/tab.go ===
package tab

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/firehourse/cdpkit/browser"
	"github.com/firehourse/cdpkit/config"
)

// Go 1.20+ 不需要手動設置種子，但為了兼容性保留初始化
func init() {
	// 檢查 Go 版本，較舊版本需要設置種子
	var r1 int
	// 產生兩個隨機數，若相同則極可能是舊版 Go 需要設置種子
	r1 = rand.Intn(100)
	time.Sleep(1 * time.Nanosecond)
	r2 := rand.Intn(100)
	if r1 == r2 {
		rand.Seed(time.Now().UnixNano())
	}
}

// Tab 包裹單一 chromedp Context 與輔助方法
type Tab struct {
	Ctx     context.Context
	Cancel  context.CancelFunc
	Timeout time.Duration
	// 追踪分頁狀態
	IsNavigating bool
	CurrentURL   string
}

// New 由 BrowserManager 建立完 Context 後包裝成 Tab
// 推薦使用 NewTab 代替，它會自動套用配置
func New(ctx context.Context, cancel context.CancelFunc, timeout time.Duration) *Tab {
	return &Tab{
		Ctx:     ctx,
		Cancel:  cancel,
		Timeout: timeout,
	}
}

// NewTab 創建一個新分頁，並自動套用配置（UA、viewport、反檢測等）
func NewTab(ctx context.Context, cancel context.CancelFunc, cfg config.Config) *Tab {
	t := &Tab{
		Ctx:     ctx,
		Cancel:  cancel,
		Timeout: cfg.Timeout,
	}

	// 1. 準備 UA 和視窗尺寸
	ua := cfg.UserAgent
	if ua == "" {
		ua = randomUA()
	}

	w, h := cfg.WindowSize[0], cfg.WindowSize[1]
	if w == 0 || h == 0 {
		w = 1280
		h = 720
	}

	// 2. 一次註冊所有腳本，在每個新頁面載入時自動執行
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(int64(w), int64(h)),

		// 設置 UA
		chromedp.ActionFunc(func(ctx context.Context) error {
			return emulation.SetUserAgentOverride(ua).Do(ctx)
		}),

		// 註冊全局腳本：反檢測和其他注入
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 主要反檢測腳本
			script := `
				// 隱藏 webdriver
				Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
				
				// 模擬正常用戶特徵
				Object.defineProperty(navigator, 'plugins', {get: () => [1, 2, 3, 4, 5]});
				Object.defineProperty(navigator, 'languages', {get: () => ['zh-TW', 'zh', 'en-US', 'en']});
				
				// 防止自動化檢測
				const originalQuery = window.navigator.permissions.query;
				window.navigator.permissions.query = (parameters) => (
					parameters.name === 'notifications' || 
					parameters.name === 'clipboard-read' || 
					parameters.name === 'clipboard-write' ? 
					Promise.resolve({state: 'prompt', onchange: null}) : 
					originalQuery(parameters)
				);
				
				// 常見的反機器人檢測對象
				delete window.cdc_adoQpoasnfa76pfcZLmcfl_Array;
				delete window.cdc_adoQpoasnfa76pfcZLmcfl_Promise;
				delete window.cdc_adoQpoasnfa76pfcZLmcfl_Symbol;
			`
			// 忽略 ScriptIdentifier 返回值，只關注錯誤
			_, err := page.AddScriptToEvaluateOnNewDocument(script).Do(ctx)
			return err
		}),
	)

	if err != nil {
		log.Printf("[cdpkit] 警告：初始化分頁時設置失敗：%v", err)
	} else {
		log.Printf("[cdpkit] 分頁創建成功，已套用 UA 和反檢測設置")
	}

	return t
}

// DefaultTimeout 取預設逾時 (fallback 30 s)
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

	// 設置狀態
	t.IsNavigating = true
	defer func() { t.IsNavigating = false }()

	log.Printf("[cdpkit] 正在導航到: %s", url)
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()

	err := chromedp.Run(ctx, chromedp.Navigate(url))
	if err != nil {
		log.Printf("[cdpkit] 導航失敗: %v", err)
		return err
	}

	// 更新當前 URL
	t.CurrentURL = url
	log.Printf("[cdpkit] 導航成功: %s", url)
	return nil
}

// RunJS 執行 JS
func (t *Tab) RunJS(script string, timeout time.Duration) (interface{}, error) {
	if timeout <= 0 {
		timeout = t.DefaultTimeout()
	}
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()

	log.Printf("[cdpkit] 執行 JS 腳本 (長度: %d 字符)", len(script))
	var res interface{}
	err := chromedp.Run(ctx, chromedp.Evaluate(script, &res))
	if err != nil {
		log.Printf("[cdpkit] JS 執行失敗: %v", err)
	}
	return res, err
}

// HTML 取得整頁 HTML
func (t *Tab) HTML(timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = t.DefaultTimeout()
	}
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()

	log.Printf("[cdpkit] 獲取頁面 HTML")
	var html string
	err := chromedp.Run(ctx, chromedp.OuterHTML("html", &html))
	if err != nil {
		log.Printf("[cdpkit] 獲取 HTML 失敗: %v", err)
	} else {
		log.Printf("[cdpkit] 獲取 HTML 成功 (長度: %d 字符)", len(html))
	}
	return html, err
}

// WaitVisible 等待元素出現
func (t *Tab) WaitVisible(sel string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = t.DefaultTimeout()
	}
	ctx, cancel := context.WithTimeout(t.Ctx, timeout)
	defer cancel()

	log.Printf("[cdpkit] 等待元素出現: %s", sel)
	err := chromedp.Run(ctx, chromedp.WaitVisible(sel, chromedp.ByQuery))
	if err != nil {
		log.Printf("[cdpkit] 等待元素超時: %v", err)
	} else {
		log.Printf("[cdpkit] 元素已出現: %s", sel)
	}
	return err
}

// Close 關閉分頁
func (t *Tab) Close(mgr *browser.BrowserManager) {
	log.Printf("[cdpkit] 關閉分頁")
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
// 注意：如果使用 NewTab 創建分頁，這個方法是多餘的
// 因為 NewTab 已經在頁面加載時自動注入了反檢測腳本
func (t *Tab) Spoof() error {
	log.Printf("[cdpkit] 執行反檢測腳本")
	_, err := t.RunJS(
		`Object.defineProperty(navigator, 'webdriver', {get: () => undefined})`,
		t.DefaultTimeout(),
	)
	if err != nil {
		log.Printf("[cdpkit] 反檢測腳本執行失敗: %v", err)
	}
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
// 注意：如果使用 NewTab 創建分頁，這個方法是多餘的
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

	log.Printf("[cdpkit] 套用配置 (UA 長度: %d, 窗口: %dx%d)", len(ua), w, h)
	ctx, cancel := context.WithTimeout(t.Ctx, t.DefaultTimeout())
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(int64(w), int64(h)),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return emulation.SetUserAgentOverride(ua).Do(ctx)
		}),
		chromedp.Evaluate(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined})`, nil),
	)

	if err != nil {
		log.Printf("[cdpkit] 套用配置失敗: %v", err)
	}
	return err
}
