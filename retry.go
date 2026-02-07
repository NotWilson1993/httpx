package httpx

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultRetryAttempts = 3
	DefaultBaseBackoff   = 200 * time.Millisecond
)

// Retryable statuses: common transient ones.
func isRetryableStatus(code int) bool {
	switch code {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

// Retryable methods should be idempotent.
func isIdempotentMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// Network-ish errors that are often transient.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout() || ne.Temporary()
	}
	return false
}

func sleepBackoff(ctx context.Context, attempt int) {
	// backoff: base * 2^(attempt-1)
	d := DefaultBaseBackoff
	for i := 1; i < attempt; i++ {
		d *= 2
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		return
	}
}
