package httpx

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestRetryIdempotentGET(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	req := Request{
		URL:    srv.URL,
		Method: http.MethodGet,
		Type:   BodyJSON,
		Retry:  true,
	}

	resp, body, err := req.Perform(context.TODO(), nil, nil)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != "ok" {
		t.Fatalf("unexpected body: %q", string(body))
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestRetrySkippedForPOST(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	req := Request{
		URL:    srv.URL,
		Method: http.MethodPost,
		Type:   BodyJSON,
		Retry:  true,
	}

	_, _, err := req.Perform(context.TODO(), nil, map[string]any{"x": 1})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 attempt, got %d", got)
	}
}

func TestClientInjectionAndTimeoutOverride(t *testing.T) {
	originalTimeout := 1 * time.Second
	client := &http.Client{Timeout: originalTimeout}

	var used int32
	client.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&used, 1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("ok")),
			Header:     make(http.Header),
		}, nil
	})

	req := Request{
		URL:    "http://example.com",
		Method: http.MethodGet,
		Type:   BodyJSON,
		Retry:  false,
		Client: client,
	}

	_, body, err := req.Perform(context.TODO(), nil, nil, 2*time.Second)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("unexpected body: %q", string(body))
	}
	if got := atomic.LoadInt32(&used); got != 1 {
		t.Fatalf("expected transport to be used once, got %d", got)
	}
	if client.Timeout != originalTimeout {
		t.Fatalf("expected client timeout unchanged (%v), got %v", originalTimeout, client.Timeout)
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestRetryOnNetworkTimeout(t *testing.T) {
	var hits int32
	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			n := atomic.AddInt32(&hits, 1)
			if n < 3 {
				return nil, timeoutErr{}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	req := Request{
		URL:    "http://example.com",
		Method: http.MethodGet,
		Type:   BodyJSON,
		Retry:  true,
		Client: client,
	}

	_, body, err := req.Perform(context.TODO(), nil, nil)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("unexpected body: %q", string(body))
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestNoRetryOnNetworkTimeoutForPOST(t *testing.T) {
	var hits int32
	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			atomic.AddInt32(&hits, 1)
			return nil, timeoutErr{}
		}),
	}

	req := Request{
		URL:    "http://example.com",
		Method: http.MethodPost,
		Type:   BodyJSON,
		Retry:  true,
		Client: client,
	}

	_, _, err := req.Perform(context.TODO(), nil, map[string]any{"x": 1})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 attempt, got %d", got)
	}
}

func TestRetryOnStatus503(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	req := Request{
		URL:    srv.URL,
		Method: http.MethodGet,
		Type:   BodyJSON,
		Retry:  true,
	}

	_, body, err := req.Perform(context.TODO(), nil, nil)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("unexpected body: %q", string(body))
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestNoRetryOnStatus400(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	req := Request{
		URL:    srv.URL,
		Method: http.MethodGet,
		Type:   BodyJSON,
		Retry:  true,
	}

	_, _, err := req.Perform(context.TODO(), nil, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 attempt, got %d", got)
	}
}
