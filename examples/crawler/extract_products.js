/**
 * 電商產品頁面提取腳本範例
 * 此腳本嘗試從常見電商網站提取產品信息
 */

// 簡化版本：直接提取可見資訊，不使用異步操作
// 檢查頁面是否有產品特徵
var hasProductFeatures = !!document.querySelector('.price, .product-price, [data-price]') ||
  document.querySelector('h1');

// 嘗試多種選擇器模式提取產品信息
var productInfo = {
  // 產品標題
  title: document.querySelector('h1') ? document.querySelector('h1').textContent.trim() :
    document.title.split('|')[0].trim(),

  // 產品價格
  price: document.querySelector('.price, .product-price, [data-price]') ?
    document.querySelector('.price, .product-price, [data-price]').textContent.trim() : "",

  // 產品描述
  description: document.querySelector('.product-description, .description, #description') ?
    document.querySelector('.product-description, .description, #description').textContent.trim() : "",

  // 產品圖片
  images: Array.from(document.querySelectorAll('.product-image img, .gallery img'))
    .map(function (img) { return img.src; })
    .filter(function (src) { return src && !src.includes('placeholder'); }),

  // 規格
  specs: Array.from(document.querySelectorAll('.specs li, .product-specs li, .features li'))
    .map(function (li) { return li.textContent.trim(); }),

  // 提取結構化資料
  structuredData: Array.from(document.querySelectorAll('script[type="application/ld+json"]'))
    .map(function (script) {
      try {
        return JSON.parse(script.textContent);
      } catch (e) {
        return null;
      }
    })
    .filter(Boolean),

  // 頁面類型
  pageType: 'unknown',

  // 頁面摘要
  contentSummary: Array.from(document.querySelectorAll('p, h2, h3'))
    .map(function (el) { return el.textContent.trim(); })
    .filter(function (text) { return text.length > 10 && text.length < 200; })
    .slice(0, 5)
    .join(' | ')
};

// 偵測頁面類型
var url = window.location.href;
var path = window.location.pathname;

if (path.includes('product') || path.match(/\/[pP]\d+/)) {
  productInfo.pageType = 'product';
} else if (path.includes('category') || path.includes('collection')) {
  productInfo.pageType = 'category';
} else if (path.includes('cart') || path.includes('checkout')) {
  productInfo.pageType = 'cart';
} else if (hasProductFeatures) {
  productInfo.pageType = 'likely_product';
}

// 返回結果
productInfo; 