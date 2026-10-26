package vylo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestShortenBuildsCorrectRequest(t *testing.T) {
	var gotPath string
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","shortenedUrl":"https://vylo.cc/Ab3x9K"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok123", srv.Client())
	got, err := c.Shorten(context.Background(), "https://example.com/page?t=1 z", 2)
	if err != nil {
		t.Fatalf("Shorten() 不應報錯: %v", err)
	}

	if gotPath != "/api" {
		t.Errorf("請求路徑 = %q, want /api", gotPath)
	}
	want := map[string]string{
		"api":   "tok123",
		"url":   "https://example.com/page?t=1 z", // 由 url.Values 正確編碼
		"steps": "2",
	}
	for k, v := range want {
		if got := gotQuery.Get(k); got != v {
			t.Errorf("查詢參數 %s = %q, want %q", k, got, v)
		}
	}
	if got != "https://vylo.cc/Ab3x9K" {
		t.Errorf("Shorten() = %q, want https://vylo.cc/Ab3x9K", got)
	}
}

func TestShortenReturnsAPIErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"status":"error","message":"That alias is already taken.","shortenedUrl":""}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", srv.Client())
	_, err := c.Shorten(context.Background(), "https://example.com", 2)
	if err == nil {
		t.Fatal("status=error 時應回傳錯誤")
	}
	want := "vylo: That alias is already taken."
	if err.Error() != want {
		t.Errorf("錯誤訊息 = %q, want %q", err.Error(), want)
	}
}

func TestShortenNon200WithoutBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", srv.Client())
	_, err := c.Shorten(context.Background(), "https://example.com", 2)
	if err == nil {
		t.Fatal("HTTP 401 應回傳錯誤")
	}
}

func TestShortenRetriesOn429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"status":"success","shortenedUrl":"https://vylo.cc/ok"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", srv.Client())
	got, err := c.Shorten(context.Background(), "https://example.com", 2)
	if err != nil {
		t.Fatalf("首次 429 後重試應成功: %v", err)
	}
	if got != "https://vylo.cc/ok" {
		t.Errorf("Shorten() = %q, want https://vylo.cc/ok", got)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("應呼叫 2 次, got %d", n)
	}
}

func TestShortenGivesUpAfterMaxRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	// 重試間隔設為極小以加速測試
	c := New(srv.URL, "tok", srv.Client())
	c.(*client).backoff = time.Millisecond
	if _, err := c.Shorten(context.Background(), "https://example.com", 2); err == nil {
		t.Fatal("連續 429 應最終報錯")
	}
	if n := calls.Load(); n != maxAttempts {
		t.Errorf("含重試應共呼叫 %d 次, got %d", maxAttempts, n)
	}
}
