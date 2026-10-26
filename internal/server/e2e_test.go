// e2e 測試：以真實的 config / vylo client / cache / server / embed 前端
// 串接整條鏈路（僅 Vylo API 本身以 httptest mock），驗證模組間接線。
package server_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vylo/internal/cache"
	"vylo/internal/config"
	"vylo/internal/server"
	"vylo/internal/vylo"
	"vylo/web"
)

// e2eAdapter 把 cache.Cache 接上 server.Shortener（與 main.go 相同邏輯）。
type e2eAdapter struct{ c *cache.Cache }

func (a e2eAdapter) Shorten(ctx context.Context, longURL string, steps int) (string, error) {
	return a.c.GetOrShorten(ctx, longURL, steps)
}

// newE2EServer 組裝全真依賴；回傳測試站與 Vylo mock 呼叫計數器指標。
func newE2EServer(t *testing.T, vyloHandler http.HandlerFunc) (*httptest.Server, *int) {
	t.Helper()

	calls := 0
	counting := func(w http.ResponseWriter, r *http.Request) {
		calls++
		vyloHandler(w, r)
	}
	mockVylo := httptest.NewServer(http.HandlerFunc(counting))
	t.Cleanup(mockVylo.Close)

	t.Setenv("VYLO_API_TOKEN", "E2E_SECRET_TOKEN")
	t.Setenv("VYLO_BASE_URL", mockVylo.URL)
	t.Setenv("VYLO_STEPS", "2")
	t.Setenv("TARGET_URL", "https://e2e.example/thanks")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	client := vylo.New(cfg.BaseURL, cfg.APIToken, mockVylo.Client())
	shortCache := cache.New(client, cache.DefaultTTL)

	handler := server.New(server.Config{
		Shortener: e2eAdapter{shortCache},
		TargetURL: cfg.TargetURL,
		Steps:     cfg.Steps,
		Static:    web.FS, // 真 embed 靜態檔
	})

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, &calls
}

// noRedirectClient 不跟隨 302，讓測試能直接斷言 Location。
var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func get(t *testing.T, rawURL string) (*http.Response, string) {
	t.Helper()
	resp, err := noRedirectClient.Get(rawURL)
	if err != nil {
		t.Fatalf("GET %s: %v", rawURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func TestE2EHappyPath(t *testing.T) {
	ts, calls := newE2EServer(t, func(w http.ResponseWriter, r *http.Request) {
		// 嚴格檢查 API 參數（AdLinkFly 格式：api / url / steps 全在 query）
		if r.URL.Query().Get("api") != "E2E_SECRET_TOKEN" ||
			r.URL.Query().Get("url") != "https://e2e.example/thanks" ||
			r.URL.Query().Get("steps") != "2" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"status":"error","message":"e2e: bad params"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","shortenedUrl":"https://vylo.cc/e2e123"}`))
	})

	// 首頁：真 embed 的 index.html
	resp, body := get(t, ts.URL+"/")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "點擊幫我賺錢") {
		t.Errorf("首頁異常: code=%d", resp.StatusCode)
	}

	// /go 兩次：302 到短鏈，且第二次命中快取（Vylo 只被調一次）
	for i := 0; i < 2; i++ {
		resp, _ = get(t, ts.URL+"/go")
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "https://vylo.cc/e2e123" {
			t.Errorf("/go 第%d次應 302 到短鏈，得到 code=%d Location=%q",
				i+1, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
	if *calls != 1 {
		t.Errorf("快取應讓兩次 /go 只打一次 Vylo API，實際 %d 次", *calls)
	}

	// /thanks：真 embed 的 thanks.html
	resp, body = get(t, ts.URL+"/thanks")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "再幫點一次") {
		t.Errorf("感謝頁異常: code=%d", resp.StatusCode)
	}

	// /healthz
	resp, body = get(t, ts.URL+"/healthz")
	if resp.StatusCode != http.StatusOK || body != "ok" {
		t.Errorf("healthz 異常: code=%d body=%q", resp.StatusCode, body)
	}

	// 洩漏檢查：頁面內容不得含 token
	for _, page := range []string{"/", "/thanks", "/healthz"} {
		_, b := get(t, ts.URL+page)
		if strings.Contains(b, "E2E_SECRET_TOKEN") {
			t.Errorf("頁面 %s 洩漏 token", page)
		}
	}
}

func TestE2EFallbackWhenVyloDown(t *testing.T) {
	ts, _ := newE2EServer(t, func(w http.ResponseWriter, r *http.Request) {
		// 401（不可重試）且錯誤訊息裡植入 token，驗證細節不外洩
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":"error","message":"E2E_SECRET_TOKEN invalid api token"}`))
	})

	resp, body := get(t, ts.URL+"/go")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "https://e2e.example/thanks" {
		t.Errorf("/go fallback 應 302 到 TargetURL，得到 code=%d Location=%q",
			resp.StatusCode, resp.Header.Get("Location"))
	}
	if strings.Contains(body, "E2E_SECRET_TOKEN") {
		t.Error("fallback 回應不得洩漏 token 或錯誤細節")
	}
}
