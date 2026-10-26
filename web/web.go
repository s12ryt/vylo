// Package web 以 embed 崁入靜態頁面，編譯出單一執行檔即可部署。
package web

import "embed"

// FS 內含頁面與附屬靜態資源（圖示、社群預覽圖、robots.txt）。
//
//go:embed index.html thanks.html favicon.svg icon.png og.png robots.txt
var FS embed.FS
