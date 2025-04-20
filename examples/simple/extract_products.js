/**
 * 電商產品頁面提取腳本範例
 * 兼容新的 cdpkit 爬蟲接口
 */

// 使用立即執行函數來避免語法錯誤
(function () {
  // 返回提取的對象
  return {
    // 頁面資訊
    url: window.location.href,
    title: document.title,

    // 產品相關信息
    product: {
      // 標題
      title: document.querySelector('h1') ?
        document.querySelector('h1').textContent.trim() :
        document.title.split('|')[0].trim(),

      // 價格（嘗試多種選擇器）
      price: (function () {
        var selectors = [
          '.price',
          '.product-price',
          '[data-price]',
          '.offer-price',
          '.sale-price',
          '.current-price'
        ];

        for (var i = 0; i < selectors.length; i++) {
          var el = document.querySelector(selectors[i]);
          if (el) return el.textContent.trim();
        }
        return null;
      })(),

      // 描述
      description: (function () {
        var selectors = [
          '.product-description',
          '.description',
          '#description',
          '[itemprop="description"]'
        ];

        for (var i = 0; i < selectors.length; i++) {
          var el = document.querySelector(selectors[i]);
          if (el) return el.textContent.trim();
        }
        return null;
      })(),

      // 圖片（最多 5 張）
      images: Array.from(document.querySelectorAll('.product-image img, .gallery img, [itemprop="image"]'))
        .map(function (img) { return img.src || img.getAttribute('data-src'); })
        .filter(function (src) { return src && !src.includes('placeholder'); })
        .slice(0, 5),

      // 規格
      specs: Array.from(document.querySelectorAll('.specs li, .product-specs li, .features li'))
        .map(function (li) { return li.textContent.trim(); }),

      // 品牌
      brand: (function () {
        var brandEl = document.querySelector('[itemprop="brand"], .brand, .product-brand');
        return brandEl ? brandEl.textContent.trim() : null;
      })(),

      // 可用性
      availability: (function () {
        var availEl = document.querySelector('[itemprop="availability"], .availability, .stock-status');
        return availEl ? availEl.textContent.trim() : 'unknown';
      })()
    },

    // 頁面類型和分類
    page_info: {
      type: (function () {
        var path = window.location.pathname;

        if (path.includes('product') || path.match(/\/[pP]\d+/)) {
          return 'product';
        } else if (path.includes('category') || path.includes('collection')) {
          return 'category';
        } else if (path.includes('cart') || path.includes('checkout')) {
          return 'cart';
        } else if (document.querySelector('.price, .product-price, [data-price]')) {
          return 'likely_product';
        }
        return 'unknown';
      })(),

      // 頁面分類
      categories: Array.from(document.querySelectorAll('.breadcrumb li, .breadcrumbs li, nav.breadcrumb a'))
        .map(function (el) { return el.textContent.trim(); })
        .filter(function (text) { return text && text !== 'Home' && text !== '>' && text.length < 50; })
    },

    // 結構化數據（如果有）
    structured_data: (function () {
      var data = [];
      var scripts = document.querySelectorAll('script[type="application/ld+json"]');

      for (var i = 0; i < scripts.length; i++) {
        try {
          var parsed = JSON.parse(scripts[i].textContent);
          data.push(parsed);
        } catch (e) {
          // 解析失敗時忽略
        }
      }

      return data;
    })(),

    // 提取時間戳
    timestamp: new Date().toISOString()
  };
})(); 