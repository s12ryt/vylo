// Package web 以 embed 崁入靜態頁面，編譯出單一執行檔即可部署。
package web

import "embed"

// FS 內含 index.html 與 thanks.html。
//
//go:embed index.html thanks.html
var FS embed.FS
