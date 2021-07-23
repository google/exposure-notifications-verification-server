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

	"github.com/google/exposure-notifications-verification-server/pkg/api"
)

// AdminAPIServerClient is a client that talks to an admin API server.
type AdminAPIServerClient struct {
	*client
}

// NewAdminAPIServerClient creates a new admin API server http client.
func NewAdminAPIServerClient(base, apiKey string, opts ...Option) (*AdminAPIServerClient, error) {
	client, err := newClient(base, apiKey, opts...)
	if err != nil {
		return nil, err
	}

	return &AdminAPIServerClient{
		client: client,
	}, nil
}

// BatchIssueCode calls the /batch-issue endpoint. Callers must check the HTTP
// response code.
func (c *AdminAPIServerClient) BatchIssueCode(ctx context.Context, in *api.BatchIssueCodeRequest) (*api.BatchIssueCodeResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/batch-issue", in)
	if err != nil {
		return nil, err
	}

	var out api.BatchIssueCodeResponse
	if err := c.doOK(req, &out); err != nil {
		return &out, err
	}
	return &out, nil
}

// CheckCodeStatus uses the Admin API to retrieve the status of an OTP code.
func (c *AdminAPIServerClient) CheckCodeStatus(ctx context.Context, in *api.CheckCodeStatusRequest) (*api.CheckCodeStatusResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/checkcodestatus", in)
	if err != nil {
		return nil, err
	}

	var out api.CheckCodeStatusResponse
	if err := c.doOK(req, &out); err != nil {
		return &out, err
	}
	return &out, nil
}

// IssueCode calls the /issue endpoint. Callers must check the HTTP response
// code.
func (c *AdminAPIServerClient) IssueCode(ctx context.Context, in *api.IssueCodeRequest) (*api.IssueCodeResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/issue", in)
	if err != nil {
		return nil, err
	}

	var out api.IssueCodeResponse
	if err := c.doOK(req, &out); err != nil {
		return &out, err
	}
	return &out, nil
}
