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
	"net/http"

	"github.com/google/exposure-notifications-verification-server/internal/appsync"
)

// AppSyncClient is a client that talks to the appsync service.
type AppSyncClient struct {
	*client
}

// NewAppSyncClient creates a new app sync service http client.
func NewAppSyncClient(base string, opts ...Option) (*AppSyncClient, error) {
	client, err := newClient(base, "", opts...)
	if err != nil {
		return nil, err
	}

	return &AppSyncClient{
		client: client,
	}, nil
}

// AppSync triggers an application sync.
func (c *AppSyncClient) AppSync(ctx context.Context) (*appsync.AppsResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return nil, err
	}

	var out appsync.AppsResponse
	if err := c.doOK(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
