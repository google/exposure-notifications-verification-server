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
	"fmt"
	"net/http"

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

	return &KeyServerClient{
		client: client,
	}, nil
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
