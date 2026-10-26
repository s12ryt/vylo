// Package server 提供 HTTP 路由與頁面處理：
// 首頁、跳轉（/go）、感謝頁、健康檢查與靜態資源。
package server

import (
	"context"
	"io/fs"
	"log"
	"net/http"
)

// Shortener 是 /go 背後的短網址來源（由 cache.Cache 實作）。
type Shortener interface {
	Shorten(ctx context.Context, longURL string, steps int) (string, error)
}

// Config 注入 server 所需依賴。
type Config struct {
	Shortener Shortener // 短網址來源（帶快取）
	TargetURL string    // 要被縮短、也是 Vylo 故障時的 fallback 目標
	Steps     int       // 廣告層級
	Static    fs.FS     // 靜態 HTML（根層為 index.html / thanks.html）
}

// securityHeaders 統一加上安全標頭。
// 頁面沒有任何外部資源與腳本，CSP 可以收得很緊。
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; "+
				"script-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; "+
				"frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// New 回傳掛好所有路由的 http.Handler。
func New(cfg Config) http.Handler {
	mux := http.NewServeMux()

	serveAsset := func(name, contentType, cacheControl string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			data, err := fs.ReadFile(cfg.Static, name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Cache-Control", cacheControl)
			_, _ = w.Write(data)
		}
	}

	serveHTML := func(name string) http.HandlerFunc {
		return serveAsset(name, "text/html; charset=utf-8", "no-store")
	}

	mux.HandleFunc("GET /{$}", serveHTML("index.html"))
	mux.HandleFunc("GET /thanks", serveHTML("thanks.html"))

	// 圖示與社群預覽圖：長快取，瀏覽器與 CDN 都不用每次回來問。
	const assetCache = "public, max-age=86400"
	mux.HandleFunc("GET /favicon.svg", serveAsset("favicon.svg", "image/svg+xml", assetCache))
	mux.HandleFunc("GET /favicon.ico", serveAsset("icon.png", "image/png", assetCache))
	mux.HandleFunc("GET /icon.png", serveAsset("icon.png", "image/png", assetCache))
	mux.HandleFunc("GET /og.png", serveAsset("og.png", "image/png", assetCache))
	mux.HandleFunc("GET /robots.txt", serveAsset("robots.txt", "text/plain; charset=utf-8", "public, max-age=3600"))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /go", func(w http.ResponseWriter, r *http.Request) {
		shortURL, err := cfg.Shortener.Shorten(r.Context(), cfg.TargetURL, cfg.Steps)
		if err != nil {
			// Vylo 故障時降級：直接跳 TargetURL，錯誤細節絕不外洩。
			log.Printf("vylo shorten failed, falling back to target: %v", err)
			shortURL = cfg.TargetURL
		}
		// 每次都要拿最新短鏈，不讓中間層快取這個跳轉。
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, shortURL, http.StatusFound)
	})

	var handler http.Handler = mux
	return securityHeaders(handler)
}
