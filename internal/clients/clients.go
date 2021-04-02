// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package clients defines API clients for interacting with select APIs.
package clients

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

// Option is a customization option for the client.
type Option func(c *client) *client

// WithTimeout sets a custom timeout for each request. The default is 5s.
func WithTimeout(d time.Duration) Option {
	return func(c *client) *client {
		c.httpClient.Timeout = d
		return c
	}
}

// WithMaxBodySize sets a custom max body size for each request. The default is
// 64kib.
func WithMaxBodySize(max int64) Option {
	return func(c *client) *client {
		c.maxBodySize = max
		return c
	}
}

// WithHostOverride creates a new client that overrides the Host header.
func WithHostOverride(host string) Option {
	return func(c *client) *client {
		ot := c.httpClient.Transport
		c.httpClient.Transport = &hostOverrideRoundTripper{
			host: host,
			def:  ot,
		}
		return c
	}
}

// WithUserAgent sets a custom User-Agent header
func WithUserAgent(userAgent string) Option {
	return func(c *client) *client {
		c.userAgent = userAgent
		return c
	}
}

type hostOverrideRoundTripper struct {
	host string
	def  http.RoundTripper
}

func (h *hostOverrideRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if h.host != "" {
		r = r.Clone(context.Background())
		r.Host = h.host
	}

	def := h.def
	if def == nil {
		def = http.DefaultTransport
	}
	return def.RoundTrip(r)
}

// client is a private client that handles the heavy lifting.
type client struct {
	httpClient  *http.Client
	baseURL     *url.URL
	apiKey      string
	maxBodySize int64
	userAgent   string
}

// newClient creates a new client.
func newClient(base, apiKey string, opts ...Option) (*client, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}

	client := &client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:     u,
		apiKey:      apiKey,
		maxBodySize: 65536, // 64 KiB
	}

	for _, opt := range opts {
		client = opt(client)
	}
	return client, nil
}

// newRequest creates a new request with the given method, path (relative to the
// baseURL), and optional body. If the body is given, it's encoded as json.
func (c *client) newRequest(ctx context.Context, method, pth string, body interface{}) (*http.Request, error) {
	pth = strings.TrimPrefix(pth, "/")
	u := c.baseURL.ResolveReference(&url.URL{Path: pth})

	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			return nil, fmt.Errorf("failed to encode JSON: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), &b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	return req, nil
}

// doOK is like do, but expects a 200 response.
func (c *client) doOK(req *http.Request, out interface{}) error {
	resp, err := c.do(req, out)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return fmt.Errorf("expected 200 response, got %d", resp.StatusCode)
	}
	return nil
}

// errorResponse is used to extract an error from the response, if it exists.
// This is a fallback for when all else fails.
type errorResponse struct {
	Error1 string `json:"error"`
	Error2 string `json:"Error"`
}

// Error returns the error string, if any.
func (e *errorResponse) Error() string {
	if e.Error1 != "" {
		return e.Error1
	}
	return e.Error2
}

// do executes the request and decodes the result into out. It returns the http
// response. It does NOT do error checking on the response code.
func (c *client) do(req *http.Request, out interface{}) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	errPrefix := fmt.Sprintf("%s %s - %d", strings.ToUpper(req.Method), req.URL.String(), resp.StatusCode)

	r := io.LimitReader(resp.Body, c.maxBodySize)
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read body: %w", errPrefix, err)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		return nil, fmt.Errorf("%s: response content-type is not application/json (got %s): body: %s",
			errPrefix, ct, body)
	}

	var errResp errorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error() != "" {
		return nil, fmt.Errorf("%s: error response from API: %s, err: %w, body: %s",
			errPrefix, errResp.Error(), err, body)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("%s: failed to decode JSON response: %w: body: %s",
			errPrefix, err, body)
	}
	return resp, nil
}
