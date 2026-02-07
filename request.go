package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type BodyType string

const (
	BodyJSON  BodyType = "json"
	BodyXML   BodyType = "xml"
	BodyPlain BodyType = "plain"
)

const DefaultTimeout = 15 * time.Second

type Request struct {
	URL    string
	Method string
	Type   BodyType

	// Retry enables a small built-in retry strategy (network timeouts/temporary + selected status codes).
	Retry bool

	// Client allows injecting a custom http.Client (transport, proxy, etc.).
	// If provided and timeout override is passed to Perform, a shallow copy is used
	// to apply the timeout without mutating the original client.
	Client *http.Client
}

// Perform executes the request.
// - ctx: cancellation/deadlines
// - headers: map values can be string, []string, numbers, bool, etc.
// - body: nil for no body; any value for JSON/XML; string/[]byte for plain
// - timeout: optional override, defaults to DefaultTimeout
func (r Request) Perform(
	ctx context.Context,
	headers map[string]interface{},
	body any,
	timeout ...time.Duration,
) (*http.Response, []byte, error) {

	if ctx == nil {
		ctx = context.Background()
	}

	if strings.TrimSpace(r.URL) == "" {
		return nil, nil, fmt.Errorf("URL is empty")
	}
	if strings.TrimSpace(r.Method) == "" {
		return nil, nil, fmt.Errorf("Method is empty")
	}
	if r.Type == "" {
		r.Type = BodyJSON
	}

	t := DefaultTimeout
	if len(timeout) > 0 && timeout[0] > 0 {
		t = timeout[0]
	}

	// Encode body once so retries don't suffer from consumed readers.
	var payload []byte
	var contentType string
	var err error

	if body != nil {
		payload, contentType, err = encodeBody(r.Type, body)
		if err != nil {
			return nil, nil, err
		}
	}

	attempts := 1
	if r.Retry && isIdempotentMethod(r.Method) {
		attempts = DefaultRetryAttempts
	}

	var client *http.Client
	if r.Client == nil {
		client = &http.Client{Timeout: t}
	} else if len(timeout) > 0 && timeout[0] > 0 {
		c := *r.Client
		c.Timeout = t
		client = &c
	} else {
		client = r.Client
	}

	var lastResp *http.Response
	var lastBody []byte

	for i := 1; i <= attempts; i++ {
		if err := ctx.Err(); err != nil {
			return lastResp, lastBody, err
		}

		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, reader)
		if err != nil {
			return nil, nil, fmt.Errorf("create request: %w", err)
		}

		applyHeaders(req.Header, headers)

		// Content-Type if we have a body and user didn't override it.
		if body != nil && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", contentType)
		}

		// Default Accept if not set.
		if req.Header.Get("Accept") == "" {
			switch r.Type {
			case BodyXML:
				req.Header.Set("Accept", "application/xml")
			case BodyPlain:
				req.Header.Set("Accept", "text/plain")
			default:
				req.Header.Set("Accept", "application/json")
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			if r.Retry && i < attempts && isRetryableError(err) {
				sleepBackoff(ctx, i)
				continue
			}
			return lastResp, lastBody, fmt.Errorf("do request: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			if r.Retry && i < attempts {
				lastResp, lastBody = resp, respBody
				sleepBackoff(ctx, i)
				continue
			}
			return resp, nil, fmt.Errorf("read response: %w", readErr)
		}

		lastResp, lastBody = resp, respBody

		// Retry on selected HTTP status codes
		if r.Retry && i < attempts && isRetryableStatus(resp.StatusCode) {
			sleepBackoff(ctx, i)
			continue
		}

		// Return HTTP error as error (but include resp + body)
		if resp.StatusCode >= 400 {
			return resp, respBody, fmt.Errorf("http error %d: %s", resp.StatusCode, truncate(string(respBody), 800))
		}

		return resp, respBody, nil
	}

	// Should not normally reach here.
	if lastResp != nil && lastResp.StatusCode >= 400 {
		return lastResp, lastBody, fmt.Errorf("http error %d: %s", lastResp.StatusCode, truncate(string(lastBody), 800))
	}
	return lastResp, lastBody, fmt.Errorf("request failed after %d attempts", attempts)
}
