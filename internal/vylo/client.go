// Package vylo 封裝 Vylo（AdLinkFly 兼容）縮短連結 API。
// 端點文件：https://vylo.cc/developers
package vylo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	maxAttempts = 3 // 含首次請求與重試，共 3 次
)

// Client 為 Vylo API 客戶端介面（便於測試替換）。
type Client interface {
	// Shorten 將 longURL 縮短為帶 steps 廣告層級的短鏈。
	Shorten(ctx context.Context, longURL string, steps int) (string, error)
}

type client struct {
	baseURL string
	token   string
	http    *http.Client
	backoff time.Duration // 429 重試起始退避間隔（指數遞增）
}

// New 建立用戶端；httpClient 為 nil 時使用預設。
func New(baseURL, token string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &client{
		baseURL: baseURL,
		token:   token,
		http:    httpClient,
		backoff: 500 * time.Millisecond,
	}
}

type apiResponse struct {
	Status       string `json:"status"`
	ShortenedURL string `json:"shortenedUrl"`
	Message      string `json:"message"`
}

func (c *client) Shorten(ctx context.Context, longURL string, steps int) (string, error) {
	q := url.Values{}
	q.Set("api", c.token)
	q.Set("url", longURL)
	q.Set("steps", strconv.Itoa(steps))
	reqURL := c.baseURL + "/api?" + q.Encode()

	var lastErr error
	backoff := c.backoff
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-time.After(backoff):
				backoff *= 2
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		short, err := c.doOnce(ctx, reqURL)
		if err == nil {
			return short, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("vylo: 重試 %d 次後仍失敗: %w", maxAttempts, lastErr)
}

func (c *client) doOnce(ctx context.Context, reqURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", &retryableError{err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("vylo: 讀取回應失敗: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return "", &retryableError{fmt.Errorf("vylo: HTTP 429 限流")}
	}

	// Vylo 錯誤（400/401/403…）時 body 仍為 JSON，message 更具診斷價值；
	// 解析失敗才退回純狀態碼。
	var ar apiResponse
	jsonErr := json.Unmarshal(body, &ar)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if jsonErr == nil && ar.Status != "success" && ar.Message != "" {
			return "", fmt.Errorf("vylo: %s", ar.Message)
		}
		return "", fmt.Errorf("vylo: HTTP %d", resp.StatusCode)
	}
	if jsonErr != nil {
		return "", fmt.Errorf("vylo: 解析 JSON 失敗: %w", jsonErr)
	}
	if ar.Status != "success" {
		msg := ar.Message
		if msg == "" {
			msg = "未知錯誤"
		}
		return "", fmt.Errorf("vylo: %s", msg)
	}
	if ar.ShortenedURL == "" {
		return "", fmt.Errorf("vylo: 回報 success 但無 shortenedUrl")
	}
	return ar.ShortenedURL, nil
}

// retryableError 標記可重試錯誤（429、網路層錯誤）。
type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

func isRetryable(err error) bool {
	var re *retryableError
	return errors.As(err, &re)
}
