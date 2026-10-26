// Package config 從環境變數載入本服務所需的全部配置。
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config 為服務運行配置。
type Config struct {
	APIToken  string // Vylo API token（僅存在後端記憶體，絕不外洩）
	BaseURL   string // Vylo 服務地址，測試時可指向 mock
	Steps     int    // 廣告層級 1–3
	TargetURL string // 廣告跳完後的落地頁
	Port      string // 本服務監聽埠
}

// Load 從環境變數載入配置並套用預設值。
func Load() (*Config, error) {
	token := os.Getenv("VYLO_API_TOKEN")
	if token == "" {
		return nil, errors.New("環境變數 VYLO_API_TOKEN 未設定（於 Dashboard → Tools & API 取得）")
	}

	baseURL := getEnvDefault("VYLO_BASE_URL", "https://vylo.cc")

	steps := 2
	if s := os.Getenv("VYLO_STEPS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= 3 {
			steps = n
		}
	}

	port := getEnvDefault("PORT", "8080")

	targetURL := getEnvDefault("TARGET_URL", fmt.Sprintf("http://localhost:%s/thanks", port))

	return &Config{
		APIToken:  token,
		BaseURL:   baseURL,
		Steps:     steps,
		TargetURL: targetURL,
		Port:      port,
	}, nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
