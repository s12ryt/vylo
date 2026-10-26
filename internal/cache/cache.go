// Package cache 提供短網址結果的記憶體快取，
// 避免對同一個 (長網址, 廣告層級) 組合重複呼叫 Vylo API。
package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DefaultTTL 是快取項目的預設存活時間。
const DefaultTTL = 12 * time.Hour

// Shortener 是快取背後的縮網址來源（由 vylo.Client 實作）。
type Shortener interface {
	Shorten(ctx context.Context, longURL string, steps int) (string, error)
}

type entry struct {
	shortURL  string
	expiresAt time.Time
}

// Cache 是併發安全的記憶體快取。
type Cache struct {
	mu        sync.RWMutex
	entries   map[string]entry
	shortener Shortener
	ttl       time.Duration
	now       func() time.Time // 可注入，便於測試 TTL
}

// New 建立快取；ttl <= 0 時使用 DefaultTTL。
func New(s Shortener, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{
		entries:   make(map[string]entry),
		shortener: s,
		ttl:       ttl,
		now:       time.Now,
	}
}

func key(longURL string, steps int) string {
	return fmt.Sprintf("%d|%s", steps, longURL)
}

// GetOrShorten 回傳快取中的短網址；快取未命中或已過期時
// 呼叫 Shortener 取得並存入快取。錯誤結果不會被快取。
func (c *Cache) GetOrShorten(ctx context.Context, longURL string, steps int) (string, error) {
	k := key(longURL, steps)

	c.mu.RLock()
	if e, ok := c.entries[k]; ok && c.now().Before(e.expiresAt) {
		c.mu.RUnlock()
		return e.shortURL, nil
	}
	c.mu.RUnlock()

	shortURL, err := c.shortener.Shorten(ctx, longURL, steps)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.entries[k] = entry{
		shortURL:  shortURL,
		expiresAt: c.now().Add(c.ttl),
	}
	c.mu.Unlock()

	return shortURL, nil
}
