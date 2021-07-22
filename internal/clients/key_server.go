// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"fmt"
	"net/http"
	"strings"

	keyserver "github.com/google/exposure-notifications-server/pkg/api/v1"
)

// KeyServerClient is a client that talks to the key-server
type KeyServerClient struct {
	*client
}

// NewKeyServerClient creates a new key-server http client.
func NewKeyServerClient(base string, opts ...Option) (*KeyServerClient, error) {
	client, err := newClient(base, "", opts...)
	if err != nil {
		return nil, err
	}

	// To maintain backwards-compatibility with older implementations that wanted
	// KEY_SERVER as the full URL to the publish endpoint, strip off anything
	// after /v1.
	if idx := strings.Index(client.baseURL.Path, "/v1"); idx != -1 {
		client.baseURL.Path = client.baseURL.Path[0:idx]
	}

	return &KeyServerClient{
		client: client,
	}, nil
}

// Publish uploads TEKs to the key server
func (c *KeyServerClient) Publish(ctx context.Context, in *keyserver.Publish) (*keyserver.PublishResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/publish", in)
	if err != nil {
		return nil, err
	}

	var out keyserver.PublishResponse
	if err := c.doOK(req, &out); err != nil {
		return &out, err
	}
	return &out, nil
}

// Stats calls the /v1/stats endpoint to get key-server statistics.
func (c *KeyServerClient) Stats(ctx context.Context, in *keyserver.StatsRequest, authToken string) (*keyserver.StatsResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/stats", in)
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	}

	var out keyserver.StatsResponse
	if err := c.doOK(req, &out); err != nil {
		return &out, err
	}
	return &out, nil
}
