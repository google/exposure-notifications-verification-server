// Copyright 2020 Google LLC
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

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
)

// AdminClient is a test client for admin API.
type AdminClient struct {
	client *http.Client
	key    string
}

// IssueCode wraps the IssueCode API call.
func (c *AdminClient) IssueCode(req api.IssueCodeRequest) (*api.IssueCodeResponse, error) {
	url := "/api/issue"

	j, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(j))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq.Header.Add("content-type", "application/json")
	httpReq.Header.Add("X-API-Key", c.key)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to POST /api/issue: %w", err)
	}

	body, err := checkResp(httpResp)
	if err != nil {
		return nil, fmt.Errorf("failed to POST /api/issue: %w: %s", err, body)
	}

	var pubResponse api.IssueCodeResponse
	if err := json.Unmarshal(body, &pubResponse); err != nil {
		return nil, fmt.Errorf("bad publish response")
	}

	return &pubResponse, nil
}

// APIClient is a test client for verification API.
type APIClient struct {
	client *http.Client
	key    string
}

// GetToken wraps the VerifyCode API call.
func (c *APIClient) GetToken(req api.VerifyCodeRequest) (*api.VerifyCodeResponse, error) {
	url := "/api/verify"

	j, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(j))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq.Header.Add("content-type", "application/json")
	httpReq.Header.Add("X-API-Key", c.key)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to POST /api/issue: %w", err)
	}

	body, err := checkResp(httpResp)
	if err != nil {
		return nil, fmt.Errorf("failed to POST /api/issue: %w: %s", err, body)
	}

	var pubResponse api.VerifyCodeResponse
	if err := json.Unmarshal(body, &pubResponse); err != nil {
		return nil, fmt.Errorf("bad publish response")
	}

	return &pubResponse, nil
}

// GetCertificate wraps the VerificationCertificate API call.
func (c *APIClient) GetCertificate(req api.VerificationCertificateRequest) (*api.VerificationCertificateResponse, error) {
	url := "/api/certificate"

	j, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(j))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq.Header.Add("content-type", "application/json")
	httpReq.Header.Add("X-API-Key", c.key)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to POST /api/certificate: %w", err)
	}

	body, err := checkResp(httpResp)
	if err != nil {
		return nil, fmt.Errorf("failed to POST /api/certificate: %w: %s", err, body)
	}

	var pubResponse api.VerificationCertificateResponse
	if err := json.Unmarshal(body, &pubResponse); err != nil {
		return nil, fmt.Errorf("bad publish response")
	}

	return &pubResponse, nil
}

func checkResp(r *http.Response) ([]byte, error) {
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if r.StatusCode != 200 {
		return nil, fmt.Errorf("response was not 200 OK: %s", body)
	}

	return body, nil
}
