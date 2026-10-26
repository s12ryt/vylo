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
		"favicon.svg": {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
		"icon.png":    {Data: []byte("ICON-PNG-MARKER")},
		"og.png":      {Data: []byte("OG-PNG-MARKER")},
		"robots.txt":  {Data: []byte("User-agent: *\nAllow: /\nDisallow: /go\n")},
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

// 圖示、社群預覽圖、robots.txt 都要有內容且帶正確型別，否則瀏覽器會噴 404 並失去分享預覽。
func TestStaticAssets(t *testing.T) {
	srv := newTestServer(&mockShortener{})
	for _, tt := range []struct{ path, contentType, marker string }{
		{"/favicon.svg", "image/svg+xml", "svg"},
		{"/favicon.ico", "image/png", "ICON-PNG-MARKER"},
		{"/icon.png", "image/png", "ICON-PNG-MARKER"},
		{"/og.png", "image/png", "OG-PNG-MARKER"},
		{"/robots.txt", "text/plain", "Disallow: /go"},
	} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s 應為 200，得到 %d", tt.path, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, tt.contentType) {
			t.Errorf("GET %s Content-Type 應含 %s，得到 %q", tt.path, tt.contentType, ct)
		}
		if !strings.Contains(rec.Body.String(), tt.marker) {
			t.Errorf("GET %s 回應內容不含 %q，得到 %q", tt.path, tt.marker, rec.Body.String())
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age") {
			t.Errorf("GET %s 應可快取，Cache-Control 得到 %q", tt.path, cc)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	srv := newTestServer(&mockShortener{})
	for _, path := range []string{"/", "/thanks", "/go", "/healthz"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s X-Content-Type-Options 應為 nosniff，得到 %q", path, got)
		}
		if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s X-Frame-Options 應為 DENY，得到 %q", path, got)
		}
		if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("%s Referrer-Policy 應為 no-referrer，得到 %q", path, got)
		}
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Errorf("%s CSP 應含 frame-ancestors 'none'，得到 %q", path, csp)
		}
	}
}

func TestGoIsNotCached(t *testing.T) {
	srv := newTestServer(&mockShortener{result: "https://vylo.cc/abc123"})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/go", nil))

	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("/go 的 Cache-Control 應為 no-store，得到 %q", cc)
	}
}
