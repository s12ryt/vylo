// Package server 提供 HTTP 路由與頁面處理：
// 首頁、跳轉（/go）、感謝頁與健康檢查。
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

// New 回傳掛好所有路由的 http.Handler。
func New(cfg Config) http.Handler {
	mux := http.NewServeMux()

	serveHTML := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			data, err := fs.ReadFile(cfg.Static, name)
			if err != nil {
				http.Error(w, "page not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(data)
		}
	}

	mux.HandleFunc("GET /{$}", serveHTML("index.html"))
	mux.HandleFunc("GET /thanks", serveHTML("thanks.html"))

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
		http.Redirect(w, r, shortURL, http.StatusFound)
	})

	var handler http.Handler = mux
	return handler
}
