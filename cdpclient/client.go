// === cdpclient/client.go ===
package cdpclient

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
)

// Client 是單純的 CDP WebSocket 連線控制器
type Client struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan Response
}

// Response 是接收 CDP 回應
type Response struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorObj       `json:"error,omitempty"`
}

type ErrorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewClient 建立一個新的連線
func NewClient(wsURL string) (*Client, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, err
	}
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	client := &Client{
		conn:    conn,
		nextID:  rand.Int63n(1000) + 1,
		pending: make(map[int64]chan Response),
	}

	// 開始接收 loop
	go client.readLoop()

	return client, nil
}

// readLoop 不斷接收 server 回傳
func (c *Client) readLoop() {
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[cdpclient] Read error: %v", err)
			return
		}

		var resp Response
		if err := json.Unmarshal(data, &resp); err != nil {
			log.Printf("[cdpclient] Unmarshal error: %v", err)
			continue
		}

		if resp.ID != 0 {
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				ch <- resp
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()
		}
	}
}

// Send 傳送一個指令，並等待回應
func (c *Client) Send(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	// 準備 payload
	payload := map[string]interface{}{
		"id":     id,
		"method": method,
	}
	if params != nil {
		payload["params"] = params
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	respCh := make(chan Response, 1)

	c.mu.Lock()
	c.pending[id] = respCh
	c.mu.Unlock()

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, errors.New(resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// Close 關閉連線
func (c *Client) Close() {
	c.conn.Close()
}
