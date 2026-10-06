// Package hkapi contains clients for Hong Kong government real-time open
// data APIs (weather + transport).
package hkapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const userAgent = "hk-gov-rt-mcp-go/1.0 (+https://data.gov.hk open data consumer)"

// UpstreamError reports a non-2xx or unparseable upstream response.
type UpstreamError struct {
	URL    string
	Status int
	Detail string
}

func (e *UpstreamError) Error() string {
	host := e.URL
	if u, err := url.Parse(e.URL); err == nil {
		host = u.Host
	}
	return fmt.Sprintf("Upstream %d from %s: %s", e.Status, host, e.Detail)
}

// UpstreamUnavailableError reports timeouts or invalid payloads.
type UpstreamUnavailableError struct {
	URL    string
	Detail string
}

func (e *UpstreamUnavailableError) Error() string {
	host := e.URL
	if u, err := url.Parse(e.URL); err == nil {
		host = u.Host
	}
	return fmt.Sprintf("Upstream unavailable (%s): %s", host, e.Detail)
}

var client = &http.Client{Timeout: 10 * time.Second}

func do(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/xml, text/plain;q=0.8")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &UpstreamUnavailableError{URL: url, Detail: err.Error()}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if resp.StatusCode >= 300 {
		return nil, &UpstreamError{URL: url, Status: resp.StatusCode, Detail: string(data[:min(len(data), 200)])}
	}
	return data, nil
}

// GetJSON fetches and decodes a JSON API response.
func GetJSON(ctx context.Context, url string, timeout time.Duration, out any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	data, err := do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // TD feeds carry a BOM
	if err := json.Unmarshal(data, out); err != nil {
		return &UpstreamUnavailableError{URL: url, Detail: "invalid JSON: " + err.Error()}
	}
	return nil
}

// GetText fetches a text/XML response.
func GetText(ctx context.Context, url string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	data, err := do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// PostJSON posts a JSON body and decodes the JSON response.
func PostJSON(ctx context.Context, url string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, err := do(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // TD feeds carry a BOM
	if err := json.Unmarshal(data, out); err != nil {
		return &UpstreamUnavailableError{URL: url, Detail: "invalid JSON: " + err.Error()}
	}
	return nil
}
