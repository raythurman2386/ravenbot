package agent

import (
	"log/slog"
	"net/http"
	"sync"
)

// KeyRotatingTransport is an http.RoundTripper that rotates API keys on 429 errors.
type KeyRotatingTransport struct {
	keys            []string
	base            http.RoundTripper
	mu              sync.Mutex
	currentKeyIndex int
}

// NewKeyRotatingTransport creates a new transport with the given keys.
func NewKeyRotatingTransport(keys []string) *KeyRotatingTransport {
	return &KeyRotatingTransport{
		keys: keys,
		base: http.DefaultTransport,
	}
}

// RoundTrip executes the HTTP request, rotating keys if a 429 occurs.
func (t *KeyRotatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// If no keys, just pass through
	if len(t.keys) == 0 {
		return t.base.RoundTrip(req)
	}

	// Try each key at least once
	attempts := len(t.keys)
	var resp *http.Response
	var err error

	for i := 0; i < attempts; i++ {
		t.mu.Lock()
		key := t.keys[t.currentKeyIndex]
		t.mu.Unlock()

		// Clone the request to modify headers safely
		clonedReq := req.Clone(req.Context())

		// Rewind the body if necessary (critical for retries)
		if req.Body != nil && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			clonedReq.Body = body
		}

		// Remove existing key if any and set the current one
		clonedReq.Header.Del("x-goog-api-key")
		clonedReq.Header.Set("x-goog-api-key", key)

		resp, err = t.base.RoundTrip(clonedReq)
		if err != nil {
			// Network error, return immediately as rotation might not help (or could implement retry)
			return nil, err
		}

		// Check for Rate Limit (429)
		if resp.StatusCode == http.StatusTooManyRequests {
			slog.Warn("Gemini API Rate Limit (429) hit, rotating key...", "key_index", t.currentKeyIndex)
			resp.Body.Close() // Important: close the body of the failed response

			t.mu.Lock()
			t.currentKeyIndex = (t.currentKeyIndex + 1) % len(t.keys)
			t.mu.Unlock()
			continue
		}

		// Success or other error, return the response
		return resp, nil
	}

	// If all keys failed with 429, return the last response
	return resp, err
}
