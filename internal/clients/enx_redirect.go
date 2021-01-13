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

package clients

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
)

// ENXRedirectClient is a client that talks to the enx-redirect service.
type ENXRedirectClient struct {
	*client
}

// NewENXRedirectClient creates a new enx-redirect service http client.
func NewENXRedirectClient(base string, opts ...Option) (*ENXRedirectClient, error) {
	client, err := newClient(base, "", opts...)
	if err != nil {
		return nil, err
	}

	return &ENXRedirectClient{
		client: client,
	}, nil
}

// AppleSiteAssociation calls and parses the Apple site association file.
func (c *ENXRedirectClient) AppleSiteAssociation(ctx context.Context) (*api.IOSDataResponse, error) {
	req, err := c.newRequest(ctx, "GET", "/.well-known/apple-app-site-association", nil)
	if err != nil {
		return nil, err
	}

	var out api.IOSDataResponse
	resp, err := c.doOK(req, &out)
	if err != nil {
		return &out, err
	}
	defer resp.Body.Close()

	return &out, nil
}

// AndroidAssetLinks calls and parses the Android assetlinks file.
func (c *ENXRedirectClient) AndroidAssetLinks(ctx context.Context) ([]*api.AndroidDataResponse, error) {
	req, err := c.newRequest(ctx, "GET", "/.well-known/assetlinks.json", nil)
	if err != nil {
		return nil, err
	}

	var out []*api.AndroidDataResponse
	resp, err := c.doOK(req, &out)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	return out, nil
}

// CheckRedirect processes the redirect. It returns the http response. It does
// not follow any redirects or check the response status.
func (c *ENXRedirectClient) CheckRedirect(ctx context.Context, userAgent string) (*http.Response, error) {
	// Copy the http client - we need one that doesn't follow redirects.
	httpClient := &http.Client{
		Transport: c.client.httpClient.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Jar:     c.client.httpClient.Jar,
		Timeout: c.client.httpClient.Timeout,
	}

	req, err := c.newRequest(ctx, "GET", "/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
