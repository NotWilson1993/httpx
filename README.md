# httpx

Do you hate writing the same HTTP boilerplate code again and again?

Creating requests, setting headers, handling timeouts, retries,
encoding JSON or XML -- just to perform a simple HTTP call?

**httpx** is a tiny helper built on top of Go's standard `net/http` package.
It removes repetitive code while staying explicit, lightweight, and easy to reason about.

No heavy framework.  
No hidden magic.  
Just less boilerplate.

---

## Features

- Minimal `Request` struct:
- `URL`
- `Method`
- `Type` (`json`, `xml`, `plain`)
- `Retry` (bool)
- `Client` (*http.Client)
- Single `Perform(...)` call
- Context support (`context.Context`)
- Default timeout (15s) with optional override
- Optional retry logic for transient failures (idempotent methods only)
- Automatic JSON / XML / plain body encoding
- Full access to `*http.Response` and raw response body

---

## Installation

```bash
go get github.com/NotWilson1993/httpx
```

---

## Quick Start

Simple JSON POST with headers and a timeout:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/NotWilson1993/httpx"
)

func main() {
	req := httpx.Request{
		URL:    "https://api.example.com/v1/items",
		Method: http.MethodPost,
		Type:   httpx.BodyJSON,
		Retry:  true,
	}

	headers := map[string]interface{}{
		"Authorization": "Bearer <token>",
		"X-Request-Id":  "abc-123",
	}

	body := map[string]any{
		"name":  "demo",
		"count": 3,
	}

	resp, respBody, err := req.Perform(context.TODO(), headers, body, 10*time.Second)
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("status=%d body=%s\n", resp.StatusCode, string(respBody))
}
```

---

## Request Fields

| Field   | Type         | Meaning |
|--------|--------------|---------|
| URL    | string       | Full URL to call |
| Method | string       | HTTP method (`GET`, `POST`, ...) |
| Type   | BodyType     | `json`, `xml`, or `plain` |
| Retry  | bool         | Enables retry for idempotent methods only |
| Client | *http.Client | Optional custom client (transport, proxy, etc.) |

---

## Perform Signature

```go
func (r Request) Perform(
	ctx context.Context,
	headers map[string]interface{},
	body any,
	timeout ...time.Duration,
) (*http.Response, []byte, error)
```

Inputs:
- `ctx` is used for cancellation/deadlines.
- `headers` accepts `string`, `[]string`, numbers, bools; values are converted to strings.
- `body` can be:
- `nil` for no body
- `string` or `[]byte` for `plain`
- any Go value for `json` or `xml`
- `timeout` is optional; when provided, it overrides the default 15s.

Outputs:
- `*http.Response` and raw response body as `[]byte`.
- If status is `>= 400`, `error` is returned but `resp` and `body` are still provided.

---

## Examples

GET JSON:
```go
req := httpx.Request{
	URL:    "https://api.example.com/v1/items",
	Method: http.MethodGet,
	Type:   httpx.BodyJSON,
	Retry:  true,
}

resp, body, err := req.Perform(context.TODO(), nil, nil)
```

Plain text POST:
```go
req := httpx.Request{
	URL:    "https://api.example.com/v1/echo",
	Method: http.MethodPost,
	Type:   httpx.BodyPlain,
}

resp, body, err := req.Perform(context.TODO(), nil, "hello world")
```

XML POST:
```go
type Payload struct {
	Name string `xml:"name"`
}

req := httpx.Request{
	URL:    "https://api.example.com/v1/xml",
	Method: http.MethodPost,
	Type:   httpx.BodyXML,
}

resp, body, err := req.Perform(context.TODO(), nil, Payload{Name: "demo"})
```

---

## Retry Behavior

- Retries are only enabled when `Retry: true` and the method is idempotent.
- Retryable status codes: `429`, `500`, `502`, `503`, `504`.
- Retryable errors are network timeouts/temporary errors.
- Backoff uses exponential delays: 200ms, 400ms, 800ms.

---

## Using a Custom http.Client

You can inject your own client if you need custom transport settings, proxies, or TLS config:

```go
client := &http.Client{
	Timeout: 5 * time.Second,
}

req := httpx.Request{
	URL:    "https://api.example.com/v1/items",
	Method: http.MethodGet,
	Type:   httpx.BodyJSON,
	Retry:  true,
	Client: client,
}

resp, body, err := req.Perform(context.TODO(), nil, nil)
```

If you pass a timeout override to `Perform`, a shallow copy of the client is used so the original is not mutated.
