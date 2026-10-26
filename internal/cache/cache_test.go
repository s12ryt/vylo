package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockShortener 模擬 vylo.Client，統計 API 呼叫次數。
type mockShortener struct {
	mu     sync.Mutex
	calls  int
	result string
	err    error
}

func (m *mockShortener) Shorten(ctx context.Context, longURL string, steps int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return "", m.err
	}
	return m.result, nil
}

func (m *mockShortener) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// newTestCache 建立可注入時間的快取。
func newTestCache(s Shortener, ttl time.Duration) (*Cache, *fakeClock) {
	fc := &fakeClock{t: time.Now()}
	c := New(s, ttl)
	c.now = fc.Now
	return c, fc
}

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

func TestGetOrShortenCachesResult(t *testing.T) {
	mock := &mockShortener{result: "https://vylo.cc/abc123"}
	c, _ := newTestCache(mock, 12*time.Hour)
	ctx := context.Background()

	first, err := c.GetOrShorten(ctx, "https://example.com/long", 2)
	if err != nil {
		t.Fatalf("第一次 GetOrShorten 錯誤: %v", err)
	}
	second, err := c.GetOrShorten(ctx, "https://example.com/long", 2)
	if err != nil {
		t.Fatalf("第二次 GetOrShorten 錯誤: %v", err)
	}

	if first != "https://vylo.cc/abc123" || second != first {
		t.Errorf("結果應一致，first=%q second=%q", first, second)
	}
	if got := mock.callCount(); got != 1 {
		t.Errorf("同 (url, steps) 應只調 API 一次，實際調了 %d 次", got)
	}
}

func TestGetOrShortenDifferentKeys(t *testing.T) {
	mock := &mockShortener{result: "https://vylo.cc/x"}
	c, _ := newTestCache(mock, 12*time.Hour)
	ctx := context.Background()

	_, _ = c.GetOrShorten(ctx, "https://a.com", 2)
	_, _ = c.GetOrShorten(ctx, "https://b.com", 2)
	_, _ = c.GetOrShorten(ctx, "https://a.com", 3) // 同 URL 不同 steps → 不同 key

	if got := mock.callCount(); got != 3 {
		t.Errorf("不同 (url, steps) 各自調 API，期望 3 次，實際 %d 次", got)
	}
}

func TestGetOrShortenExpiresAfterTTL(t *testing.T) {
	mock := &mockShortener{result: "https://vylo.cc/ttl"}
	c, fc := newTestCache(mock, 12*time.Hour)
	ctx := context.Background()

	if _, err := c.GetOrShorten(ctx, "https://example.com", 2); err != nil {
		t.Fatalf("第一次呼叫錯誤: %v", err)
	}

	fc.Advance(12*time.Hour + time.Second) // 越過 TTL

	if _, err := c.GetOrShorten(ctx, "https://example.com", 2); err != nil {
		t.Fatalf("過期後呼叫錯誤: %v", err)
	}
	if got := mock.callCount(); got != 2 {
		t.Errorf("TTL 過期後應重新調 API，期望 2 次，實際 %d 次", got)
	}
}

func TestGetOrShortenErrorNotCached(t *testing.T) {
	mock := &mockShortener{err: errors.New("vylo: some error")}
	c, _ := newTestCache(mock, 12*time.Hour)
	ctx := context.Background()

	if _, err := c.GetOrShorten(ctx, "https://example.com", 2); err == nil {
		t.Fatal("第一次應回傳錯誤")
	}

	// 修復上游後，第二次應重新調 API 並成功
	mock.mu.Lock()
	mock.err = nil
	mock.result = "https://vylo.cc/recovered"
	mock.mu.Unlock()

	got, err := c.GetOrShorten(ctx, "https://example.com", 2)
	if err != nil {
		t.Fatalf("第二次不應快取錯誤: %v", err)
	}
	if got != "https://vylo.cc/recovered" {
		t.Errorf("應拿到恢復後的結果，得到 %q", got)
	}
	if n := mock.callCount(); n != 2 {
		t.Errorf("錯誤不應被快取，期望調 2 次，實際 %d 次", n)
	}
}

func TestConcurrentAccess(t *testing.T) {
	mock := &mockShortener{result: "https://vylo.cc/concurrent"}
	c, _ := newTestCache(mock, 12*time.Hour)
	ctx := context.Background()

	const n = 50
	var wg sync.WaitGroup
	results := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			url, err := c.GetOrShorten(ctx, "https://example.com/race", 2)
			if err != nil {
				t.Errorf("併發呼叫錯誤: %v", err)
				return
			}
			results[i] = url
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r != "https://vylo.cc/concurrent" {
			t.Errorf("結果[%d]不一致: %q", i, r)
		}
	}
}

func TestDefaultTTL(t *testing.T) {
	if DefaultTTL != 12*time.Hour {
		t.Errorf("DefaultTTL 應為 12 小時，實際 %v", DefaultTTL)
	}
}
