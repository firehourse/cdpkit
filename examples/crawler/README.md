# cdpkit 高併發爬蟲範例

此範例展示如何使用 cdpkit 庫建立高併發爬蟲系統，支援以下功能：

- ✅ 多 Tab 併發處理，提高爬取效率
- ✅ 自訂 JS 腳本注入，提取結構化資料
- ✅ HTTP/SOCKS 代理設定
- ✅ 自訂 Chrome 調試埠
- ✅ 優雅關閉和錯誤處理
- ✅ JSON 格式結果輸出

## 編譯與安裝

```bash
cd go/cdpkit/examples/crawler
go build -o crawler
```

## 基本用法

```bash
# 爬取單一網站
./crawler https://example.org

# 爬取多個網站 (併發處理)
./crawler https://example.org https://example.com https://example.net

# 指定併發數量 (預設為 5)
./crawler -concurrency 10 https://example.org https://example.com https://example.net

# 使用代理
./crawler -proxy http://user:pass@proxy.example.com:8080 https://example.org

# 指定 Chrome 調試埠
./crawler -port 9333 https://example.org

# 自訂結果輸出路徑
./crawler -output results.json https://example.org

# 設定操作超時
./crawler -timeout 90s https://example.org
```

## 使用自訂 JS 腳本

提供 `-js` 參數指定 JavaScript 腳本文件路徑：

```bash
# 使用自訂腳本提取網頁資料
./crawler -js extract_products.js https://example.org/product/123
```

### 腳本格式

腳本必須返回一個物件或值，例如：

```javascript
// 簡單腳本範例
return {
  title: document.title,
  heading: document.querySelector('h1')?.textContent,
  links: Array.from(document.querySelectorAll('a')).map(a => a.href)
};
```

詳見 `extract_products.js` 範例腳本。

## 幕後實現原理

此爬蟲工具使用了多個技術：

1. **Browser Manager**: 管理 Chrome 瀏覽器實例
2. **Worker Pool**: 使用 goroutines 和通道實現併發控制
3. **Tab Per Worker**: 每個工作線程使用獨立分頁
4. **JS 注入**: 使用 JS 腳本從頁面提取結構化資料
5. **Signal Handling**: 支援 Ctrl+C 優雅終止

## 自訂與擴展

您可以自訂此爬蟲：

- 修改 `CrawlerConfig` 結構體新增更多選項
- 在 `scrapePage` 方法中新增更多頁面處理邏輯
- 客製化結果處理，例如儲存到資料庫

## 常見問題

**Q: 我需要預先啟動 Chrome 嗎？**  
A: 不需要，程式會自動啟動 Chrome。但如果您想要使用已有的 Chrome 實例，可以先啟動 Chrome 並指定相同的調試埠。

**Q: 如何處理動態載入內容？**  
A: 在 JS 腳本中使用 `waitForElement` 函數等待特定元素出現。

**Q: 我可以從多少個網站同時爬取資料？**  
A: 理論上沒有限制，但實際受到機器資源和網路條件限制。建議根據可用記憶體調整 `-concurrency` 參數。 