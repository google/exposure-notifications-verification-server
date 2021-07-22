// Copyright 2020 the Exposure Notifications Verification Server authors
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

// Package firebase is common logic and handling around firebase.
package firebase

import (
	"context"
	"net/http"
	"net/url"

	ghttp "google.golang.org/api/transport/http"
)

const (
	identityToolkitBaseURL = "https://identitytoolkit.googleapis.com/"
)

type Client struct {
	baseURL *url.URL
	client  *http.Client
}

func New(ctx context.Context) (*Client, error) {
	u, err := url.Parse(identityToolkitBaseURL)
	if err != nil {
		return nil, err
	}

	client, _, err := ghttp.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &Client{
		baseURL: u,
		client:  client,
	}, nil
}

func (c *Client) buildURL(path string) string {
	return c.baseURL.ResolveReference(&url.URL{Path: path}).String()
}
