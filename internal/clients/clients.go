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
	"io/ioutil"
	"net/http"
	"net/url"
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

// WithMaxBodySize sets a custom max body sixe for each request. The default is
// 64kib.
func WithMaxBodySize(max int64) Option {
	return func(c *client) *client {
		c.maxBodySize = max
		return c
	}
}

// client is a private client that handles the heavy lifting.
type client struct {
	httpClient  *http.Client
	baseURL     *url.URL
	apiKey      string
	maxBodySize int64
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

	return req, nil
}

// doOK is like do, but expects a 200 response.
func (c *client) doOK(req *http.Request, out interface{}) (*http.Response, error) {
	resp, err := c.do(req, out)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("expected 200 response, got %d", resp.StatusCode)
	}
	return resp, nil
}

// do executes the request and decodes the result into out. It returns the http
// response. It does NOT do error checking on the response code.
func (c *client) do(req *http.Request, out interface{}) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	r := io.LimitReader(resp.Body, c.maxBodySize)

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		bodyBytes, rawErr := ioutil.ReadAll(r)
		if rawErr != nil {
			return nil, fmt.Errorf("failed to read %s response body %w", ct, rawErr)
		}
		return nil, fmt.Errorf("response content type: %s. Raw response: %s", ct, string(bodyBytes))
	}

	if err := json.NewDecoder(r).Decode(out); err != nil {
		bodyBytes, rawErr := ioutil.ReadAll(r)
		if rawErr != nil {
			return nil, fmt.Errorf("failed to read JSON response body %w", rawErr)
		}
		return nil, fmt.Errorf("failed to decode JSON response: %w. Raw response: %s", err, string(bodyBytes))
	}
	return resp, nil
}
