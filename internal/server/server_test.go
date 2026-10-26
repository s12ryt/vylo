package server

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const fakeToken = "SECRET_TOKEN_123"

// mockShortener 模擬快取/上游，記錄收到的參數。
type mockShortener struct {
	calls    int
	lastURL  string
	lastStep int
	result   string
	err      error
}

func (m *mockShortener) Shorten(ctx context.Context, longURL string, steps int) (string, error) {
	m.calls++
	m.lastURL = longURL
	m.lastStep = steps
	if m.err != nil {
		return "", m.err
	}
	return m.result, nil
}

func testStaticFS() fs.FS {
	return fstest.MapFS{
		"index.html":  {Data: []byte("<html><body>INDEX-PAGE-MARKER</body></html>")},
		"thanks.html": {Data: []byte("<html><body>THANKS-PAGE-MARKER</body></html>")},
	}
}

func newTestServer(m *mockShortener) http.Handler {
	return New(Config{
		Shortener: m,
		TargetURL: "https://example.com/target",
		Steps:     2,
		Static:    testStaticFS(),
	})
}

func TestIndexServesHTML(t *testing.T) {
	srv := newTestServer(&mockShortener{})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET / 應為 200，得到 %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type 應含 text/html，得到 %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "INDEX-PAGE-MARKER") {
		t.Errorf("首頁應回 index.html 內容，得到 %q", rec.Body.String())
	}
}

func TestGoRedirectsToShortURL(t *testing.T) {
	m := &mockShortener{result: "https://vylo.cc/abc123"}
	srv := newTestServer(m)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/go", nil))

	if rec.Code != http.StatusFound {
		t.Errorf("GET /go 應為 302，得到 %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://vylo.cc/abc123" {
		t.Errorf("Location 應為短鏈，得到 %q", loc)
	}
	if m.calls != 1 {
		t.Errorf("應調用 Shortener 一次，實際 %d 次", m.calls)
	}
	if m.lastURL != "https://example.com/target" {
		t.Errorf("應縮短 TargetURL，實際收到 %q", m.lastURL)
	}
	if m.lastStep != 2 {
		t.Errorf("steps 應為 2，實際 %d", m.lastStep)
	}
}

func TestGoFallsBackWhenVyloFails(t *testing.T) {
	m := &mockShortener{err: errors.New("vylo: " + fakeToken + " boom")}
	srv := newTestServer(m)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/go", nil))

	if rec.Code != http.StatusFound {
		t.Errorf("Vylo 失敗時 /go 仍應 302 fallback，得到 %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://example.com/target" {
		t.Errorf("fallback Location 應為 TargetURL，得到 %q", loc)
	}
	if strings.Contains(rec.Body.String(), fakeToken) {
		t.Error("回應 body 不得洩漏 token")
	}
}

func TestThanksPage(t *testing.T) {
	srv := newTestServer(&mockShortener{})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/thanks", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /thanks 應為 200，得到 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "THANKS-PAGE-MARKER") {
		t.Errorf("應回 thanks.html 內容，得到 %q", rec.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(&mockShortener{})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /healthz 應為 200，得到 %d", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("healthz body 應為 ok，得到 %q", body)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv := newTestServer(&mockShortener{})
	for _, tt := range []struct{ method, path string }{
		{http.MethodPost, "/go"},
		{http.MethodPost, "/"},
		{http.MethodPost, "/thanks"},
	} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s 應為 405，得到 %d", tt.method, tt.path, rec.Code)
		}
	}
}

func TestNotFound(t *testing.T) {
	srv := newTestServer(&mockShortener{})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/no-such-page", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("未知路徑應為 404，得到 %d", rec.Code)
	}
}
