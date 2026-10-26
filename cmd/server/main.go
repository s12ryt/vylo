// 執行檔入口：載入配置、組裝依賴並啟動 HTTP 服務。
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"vylo/internal/cache"
	"vylo/internal/config"
	"vylo/internal/server"
	"vylo/internal/vylo"
	"vylo/web"
)

// shortenerAdapter 將 cache.Cache 適配成 server.Shortener 介面。
type shortenerAdapter struct{ c *cache.Cache }

func (s shortenerAdapter) Shorten(ctx context.Context, longURL string, steps int) (string, error) {
	return s.c.GetOrShorten(ctx, longURL, steps)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	client := vylo.New(cfg.BaseURL, cfg.APIToken, &http.Client{Timeout: 15 * time.Second})
	shortCache := cache.New(client, cache.DefaultTTL)

	handler := server.New(server.Config{
		Shortener: shortenerAdapter{shortCache},
		TargetURL: cfg.TargetURL,
		Steps:     cfg.Steps,
		Static:    web.FS,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("vylo-earn server listening on :%s (steps=%d)", cfg.Port, cfg.Steps)
	log.Fatal(srv.ListenAndServe())
}
