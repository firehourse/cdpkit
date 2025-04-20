package config

import (
	"encoding/json"
	"fmt"
	"os" // Replaced io/ioutil with os
	"time"
)

// FlagMergeFunc 允許外部自訂 flags 合併策略
type FlagMergeFunc func(
	defaultFlags map[string]interface{},
	userFlags map[string]interface{},
) map[string]interface{}

type Config struct {
	// WebSocketURL 指定本地 Chrome 的遠程調試 WebSocket 地址，例如 ws://localhost:9222/devtools/browser/<ID>。
	// 必須提供有效的 WebSocket URL，否則無法連接到 Chrome。
	WebSocketURL string
	// DefaultFlags 內建旗標（若自行啟動 Chrome 才會用到）
	DefaultFlags map[string]interface{}
	// Flags 由使用者指定、用於覆寫 DefaultFlags
	Flags map[string]interface{}
	// MergeFn 合併策略；nil 時採用 collectFlags 的預設行為
	MergeFn FlagMergeFunc
	// TabLimit 單個 BrowserManager 允許的最大分頁數；<=0 則退回 50
	TabLimit int
	// Timeout 全域預設操作超時
	Timeout time.Duration
	// UserAgent 自定義 User-Agent，若為空則隨機選擇
	UserAgent string
	// WindowSize 瀏覽器窗口大小 [寬, 高]，若為 [0, 0] 則隨機生成
	WindowSize [2]int
	// Proxy HTTP/SOCKS5 代理地址，例如 http://proxy.example.com:8080
	Proxy      string
	ChromePath string // (可選) 指定 chrome 二進位路徑
	RemotePort int
}

// SafeDefaults 提供穩定可用的旗標集合
func SafeDefaults() map[string]interface{} {
	return map[string]interface{}{
		"disable-blink-features": "AutomationControlled",
		"headless":               true,
		"no-sandbox":             true,
	}
}

// LoadFromFile 從 JSON 文件加載配置
func LoadFromFile(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("無法讀取配置文件 %s: %w", filePath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("無法解析 JSON 配置: %w", err)
	}

	// 設置默認值
	if cfg.DefaultFlags == nil {
		cfg.DefaultFlags = SafeDefaults()
	}
	if cfg.TabLimit <= 0 {
		cfg.TabLimit = 50
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	// 可選：為 WindowSize 設置常見預設值
	if cfg.WindowSize == [2]int{0, 0} {
		cfg.WindowSize = [2]int{1280, 720}
	}

	return &cfg, nil
}
