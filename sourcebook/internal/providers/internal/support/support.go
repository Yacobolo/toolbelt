package support

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultUserAgent = "sourcebook/1.0 (+https://github.com/Yacobolo/toolbelt)"
	DefaultMaxBytes  = 32 << 20
)

type Fetcher struct {
	client    *http.Client
	userAgent string
	retries   int
	maxBytes  int64
	delay     time.Duration
	requestMu sync.Mutex
	next      time.Time
}

func NewFetcher(client *http.Client, userAgent string, retries int, maxBytes int64, delay time.Duration) *Fetcher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	if retries < 0 {
		retries = 0
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return &Fetcher{
		client:    client,
		userAgent: userAgent,
		retries:   retries,
		maxBytes:  maxBytes,
		delay:     delay,
	}
}

func (f *Fetcher) Get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= f.retries; attempt++ {
		if attempt > 0 {
			if err := wait(ctx, time.Duration(1<<(attempt-1))*250*time.Millisecond); err != nil {
				return nil, err
			}
		}
		if err := f.waitForTurn(ctx); err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("User-Agent", f.userAgent)
		response, err := f.client.Do(request)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, f.maxBytes+1))
		closeErr := response.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		if int64(len(body)) > f.maxBytes {
			return nil, fmt.Errorf("response exceeds %d bytes", f.maxBytes)
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return body, nil
		}
		lastErr = fmt.Errorf("HTTP %d", response.StatusCode)
		if response.StatusCode != http.StatusTooManyRequests && response.StatusCode < 500 {
			break
		}
		if retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), time.Now()); retryAfter > 0 {
			if err := wait(ctx, retryAfter); err != nil {
				return nil, err
			}
		}
	}
	return nil, lastErr
}

func (f *Fetcher) waitForTurn(ctx context.Context) error {
	if f.delay <= 0 {
		return nil
	}
	f.requestMu.Lock()
	now := time.Now()
	start := now
	if f.next.After(start) {
		start = f.next
	}
	f.next = start.Add(f.delay)
	f.requestMu.Unlock()
	return wait(ctx, start.Sub(now))
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		return when.Sub(now)
	}
	return 0
}

func wait(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func WriteFileAtomically(filename string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".sourcebook-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, filename)
}
