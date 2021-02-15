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

// APIServerClient is a client that talks to a device API server.
type APIServerClient struct {
	*client
}

// NewAPIServerClient creates a new API server http client.
func NewAPIServerClient(base, apiKey string, opts ...Option) (*APIServerClient, error) {
	client, err := newClient(base, apiKey, opts...)
	if err != nil {
		return nil, err
	}

	return &APIServerClient{
		client: client,
	}, nil
}

// Verify calls the /verify endpoint to convert a code into a token.
func (c *APIServerClient) Verify(ctx context.Context, in *api.VerifyCodeRequest) (*api.VerifyCodeResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/verify", in)
	if err != nil {
		return nil, err
	}

	var out api.VerifyCodeResponse
	if err := c.doOK(req, &out); err != nil {
		return &out, err
	}
	return &out, nil
}

// Certificate calls the /certificate endpoint to exchange a token for a certificate.
func (c *APIServerClient) Certificate(ctx context.Context, in *api.VerificationCertificateRequest) (*api.VerificationCertificateResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/certificate", in)
	if err != nil {
		return nil, err
	}

	var out api.VerificationCertificateResponse
	if err := c.doOK(req, &out); err != nil {
		return &out, err
	}
	return &out, nil
}
