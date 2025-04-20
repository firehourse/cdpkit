// === browser/manager.go ===
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/firehourse/cdpkit/cdp"
	"github.com/firehourse/cdpkit/config"
)

// BrowserManager 可連線既有 Chrome (RemoteAllocator)
// 亦可自行啟動 Chrome (ExecAllocator)；取決於 cfg.WebSocketURL 是否為空。
type BrowserManager struct {
	allocCtx context.Context
	cancel   context.CancelFunc

	tabLimit int
	tabCount int
	mu       sync.Mutex

	cfg config.Config
}

// ---------------- 新增：依設定初始化 ----------------

func NewManagerFromConfig(cfg config.Config) (*BrowserManager, error) {
	// 優先使用明確的 WebSocketURL
	if cfg.WebSocketURL != "" {
		return newRemoteManager(cfg)
	}

	// 若未指定 WebSocketURL，嘗試探測現有 Chrome
	if ws, err := probeWebSocket(cfg.RemotePort); err == nil && ws != "" {
		log.Printf("[cdpkit] 發現現有 Chrome：%s", ws)
		cfg.WebSocketURL = ws
		return newRemoteManager(cfg)
	}

	// 若沒有現有 Chrome，則啟動新的
	log.Printf("[cdpkit] 未發現現有 Chrome，嘗試啟動新實例，Port=%d", cfg.RemotePort)
	bm, err := newExecManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("無法啟動 Chrome: %w", err)
	}
	return bm, nil
}

// ---------- Remote 模式 (連接現有 Chrome) ----------

func newRemoteManager(cfg config.Config) (*BrowserManager, error) {
	allocCtx, allocCancel, err := cdp.NewRemoteAllocator(cfg.WebSocketURL)
	if err != nil {
		return nil, fmt.Errorf("連接 Chrome 失敗: %w", err)
	}
	log.Printf("[cdpkit] 成功連接到 Chrome: %s", cfg.WebSocketURL)
	return &BrowserManager{
		allocCtx: allocCtx,
		cancel:   allocCancel,
		tabLimit: defaultTabLimit(cfg.TabLimit),
		cfg:      cfg,
	}, nil
}

// ---------- Exec 模式 (自啟 Chrome) ----------

func newExecManager(cfg config.Config) (*BrowserManager, error) {
	// 1. 準備啟動選項
	opts := prepareExecOptions(cfg)
	log.Printf("[cdpkit] 使用以下選項啟動 Chrome:")
	for _, opt := range opts {
		if strings.Contains(fmt.Sprintf("%v", opt), "--remote-debugging-port") {
			log.Printf("[cdpkit]   - %v", opt)
		}
	}

	// 2. 啟動 Chrome
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	// 3. 等待 debug 埠可連接
	var wsURL string
	var err error
	for i := 0; i < 5; i++ { // 最多重試 5 次
		wsURL, err = waitForDebugger(cfg.RemotePort, 3*time.Second)
		if err == nil {
			break
		}
		log.Printf("[cdpkit] 等待 Chrome 調試埠就緒 (嘗試 %d/5): %v", i+1, err)
		time.Sleep(1 * time.Second)
	}

	if wsURL == "" {
		allocCancel()
		return nil, fmt.Errorf("啟動 Chrome 後無法連接調試埠: %v", err)
	}

	log.Printf("[cdpkit] Chrome 已啟動並就緒: %s", wsURL)
	return &BrowserManager{
		allocCtx: allocCtx,
		cancel:   allocCancel,
		tabLimit: defaultTabLimit(cfg.TabLimit),
		cfg:      cfg,
	}, nil
}

func prepareExecOptions(cfg config.Config) []chromedp.ExecAllocatorOption {
	// 1. 濾掉內建 options 中的 --remote-debugging-port
	var opts []chromedp.ExecAllocatorOption
	for _, opt := range chromedp.DefaultExecAllocatorOptions {
		// 將 option 轉為字串，比對是否為 remote-debugging-port
		optStr := fmt.Sprintf("%v", opt)
		if strings.Contains(optStr, "--remote-debugging-port=") {
			continue
		}
		opts = append(opts, opt)
	}

	// 2. 加入你想設定的 remote port
	opts = append(opts, chromedp.Flag("remote-debugging-port", fmt.Sprintf("%d", cfg.RemotePort)))

	// 3. 加入常見反指紋 UA 欺騙
	opts = append(opts, chromedp.Flag("disable-blink-features", "AutomationControlled"))

	// 4. 如果未指定 headless，預設使用舊版 headless 模式
	hasHeadless := false
	for k := range cfg.Flags {
		if k == "headless" {
			hasHeadless = true
			break
		}
	}
	if !hasHeadless {
		opts = append(opts, chromedp.Flag("headless", true))
	}

	// 5. 加入穩定性建議選項（除非使用者已覆蓋）
	stabilityOpts := map[string]interface{}{
		"no-sandbox":             true,
		"disable-gpu":            true,
		"disable-dev-shm-usage":  true,
		"disable-setuid-sandbox": true,
	}
	for k, v := range stabilityOpts {
		if _, exists := cfg.Flags[k]; !exists {
			opts = append(opts, chromedp.Flag(k, v))
		}
	}

	// 6. 用戶自定 flags（最高優先）
	for k, v := range cfg.Flags {
		opts = append(opts, chromedp.Flag(k, v))
	}

	// 7. Chrome 執行檔路徑
	if cfg.ChromePath != "" {
		opts = append(opts, chromedp.ExecPath(cfg.ChromePath))
	} else {
		// 若沒指定則自動探測
		if path := findChromePath(); path != "" {
			log.Printf("[cdpkit] 找到系統 Chrome: %s", path)
			opts = append(opts, chromedp.ExecPath(path))
		}
	}

	return opts
}

// findChromePath 嘗試在系統中找到 Chrome 路徑
func findChromePath() string {
	possibleNames := []string{"google-chrome", "chrome", "chromium", "chromium-browser"}

	// Windows 上可能的路徑
	if runtime.GOOS == "windows" {
		paths := []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		}
		for _, path := range paths {
			if _, err := exec.Command("cmd", "/c", "if exist", path, "echo", "1").Output(); err == nil {
				return path
			}
		}
	}

	// Linux/Mac 嘗試 which
	for _, name := range possibleNames {
		if path, err := exec.Command("which", name).Output(); err == nil {
			return strings.TrimSpace(string(path))
		}
	}

	return ""
}

func waitForDebugger(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ws, err := probeWebSocket(port); err == nil && ws != "" {
			return ws, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return "", fmt.Errorf("在 %s 內未偵測到 Chrome 調試埠", timeout)
}

// ---------- 公共方法 ----------

func (bm *BrowserManager) NewPageContext() (context.Context, context.CancelFunc, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.tabCount >= bm.tabLimit {
		log.Printf("[cdpkit] 分頁達到上限 (%d)，嘗試重置...", bm.tabLimit)
		if err := bm.restart(); err != nil {
			return nil, nil, fmt.Errorf("無法重置瀏覽器: %w", err)
		}
	}

	ctx, cancel := chromedp.NewContext(
		bm.allocCtx,
		chromedp.WithLogf(log.Printf),
	)
	bm.tabCount++
	log.Printf("[cdpkit] 創建新分頁 (目前總數: %d)", bm.tabCount)
	return ctx, cancel, nil
}

func (bm *BrowserManager) Shutdown() {
	log.Printf("[cdpkit] 關閉瀏覽器管理器")
	if bm.cancel != nil {
		bm.cancel()
	}
}

func (bm *BrowserManager) DecrementTabCount() {
	bm.mu.Lock()
	if bm.tabCount > 0 {
		bm.tabCount--
		log.Printf("[cdpkit] 關閉分頁 (剩餘: %d)", bm.tabCount)
	}
	bm.mu.Unlock()
}

// restart：Remote 模式 → 重新連線；Exec 模式 → 整個重啟 Chrome
func (bm *BrowserManager) restart() error {
	log.Printf("[cdpkit] 重置瀏覽器開始...")
	bm.cancel()
	time.Sleep(time.Second)

	if bm.cfg.WebSocketURL == "" {
		// Exec 模式重建
		log.Printf("[cdpkit] 重新啟動 Chrome...")
		m, err := newExecManager(bm.cfg)
		if err != nil {
			return err
		}
		*bm = *m
	} else {
		// Remote 模式重連
		log.Printf("[cdpkit] 重新連接 Chrome: %s", bm.cfg.WebSocketURL)
		m, err := newRemoteManager(bm.cfg)
		if err != nil {
			return err
		}
		*bm = *m
	}
	bm.tabCount = 0
	log.Printf("[cdpkit] 瀏覽器重置完成")
	return nil
}

// ----------------- 內部輔助 -----------------

func defaultTabLimit(n int) int {
	if n <= 0 {
		return 50
	}
	return n
}

// probeWebSocket 探測指定 port 的 Chrome 是否已啟動
func probeWebSocket(port int) (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var v struct {
		WS string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	return v.WS, nil
}
