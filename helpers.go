package httpx

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
)

func encodeBody(t BodyType, v any) ([]byte, string, error) {
	switch t {
	case BodyXML:
		b, err := xml.Marshal(v)
		if err != nil {
			return nil, "", fmt.Errorf("xml marshal: %w", err)
		}
		return b, "application/xml", nil

	case BodyPlain:
		switch x := v.(type) {
		case string:
			return []byte(x), "text/plain", nil
		case []byte:
			return x, "text/plain", nil
		default:
			return nil, "", fmt.Errorf("plain body expects string or []byte, got %T", v)
		}

	case BodyJSON:
		fallthrough
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, "", fmt.Errorf("json marshal: %w", err)
		}
		return b, "application/json", nil
	}
}

func applyHeaders(h http.Header, m map[string]interface{}) {
	if m == nil {
		return
	}
	for k, v := range m {
		switch x := v.(type) {
		case string:
			h.Set(k, x)
		case []string:
			for _, s := range x {
				h.Add(k, s)
			}
		default:
			h.Set(k, fmt.Sprint(v))
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
