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

package testsuite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	urlpkg "net/url"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
)

// AdminClient is a test client for admin API.
type AdminClient struct {
	client        *http.Client
	urlBase       string
	key           string
	retry         bool
	retryTimes    uint64
	retryInterval time.Duration
}

// BatchIssueCode calls the issue-batch API call.
// returns the http status code, response.
// The caller may get a non-200 code even if the response contains some successful codes.
func (c *AdminClient) BatchIssueCode(req api.BatchIssueCodeRequest) (int, *api.BatchIssueCodeResponse, error) {
	url := c.urlBase + "/api/batch-issue"

	j, err := json.Marshal(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(j))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq.Header.Add("content-type", "application/json")
	httpReq.Header.Add("X-API-Key", c.key)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to POST %s: %w", url, err)
	}

	defer httpResp.Body.Close()
	body, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var pubResponse api.BatchIssueCodeResponse
	if err := json.Unmarshal(body, &pubResponse); err != nil {
		return 0, nil, fmt.Errorf("bad publish response")
	}

	return httpResp.StatusCode, &pubResponse, nil
}

// IssueCode wraps the IssueCode API call.
// returns the http status code, response.
// The caller may get a non-200 code even if the response contains some successful codes.
func (c *AdminClient) IssueCode(req api.IssueCodeRequest) (int, *api.IssueCodeResponse, error) {
	var resp *api.IssueCodeResponse
	var err error
	httpCode := 0
	if c.retry {
		finalErr := Eventually(c.retryTimes, c.retryInterval, func() error {
			httpCode, resp, err = c.issueCode(req)
			return err
		})
		if finalErr != nil {
			return httpCode, nil, finalErr
		}
		return httpCode, resp, nil
	}
	return c.issueCode(req)
}

func (c *AdminClient) issueCode(req api.IssueCodeRequest) (int, *api.IssueCodeResponse, error) {
	url := c.urlBase + "/api/issue"

	j, err := json.Marshal(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(j))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	httpReq.Header.Add("content-type", "application/json")
	httpReq.Header.Add("X-API-Key", c.key)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to POST %s: %w", url, err)
	}

	defer httpResp.Body.Close()
	body, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var pubResponse api.IssueCodeResponse
	if err := json.Unmarshal(body, &pubResponse); err != nil {
		return 0, nil, fmt.Errorf("bad publish response")
	}

	return httpResp.StatusCode, &pubResponse, nil
}

// APIClient is a test client for verification API.
type APIClient struct {
	urlBase string
	client  *http.Client
	key     string
}

// GetToken wraps the VerifyCode API call.
func (c *APIClient) GetToken(req api.VerifyCodeRequest) (*api.VerifyCodeResponse, error) {
	url := c.urlBase + "/api/verify"

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
		return nil, fmt.Errorf("failed to POST %s: %w", url, err)
	}

	body, err := checkResp(httpResp)
	if err != nil {
		return nil, fmt.Errorf("failed to POST %s: %w: %s", url, err, body)
	}

	var pubResponse api.VerifyCodeResponse
	if err := json.Unmarshal(body, &pubResponse); err != nil {
		return nil, fmt.Errorf("bad publish response")
	}

	return &pubResponse, nil
}

// GetCertificate wraps the VerificationCertificate API call.
func (c *APIClient) GetCertificate(req api.VerificationCertificateRequest) (*api.VerificationCertificateResponse, error) {
	url := c.urlBase + "/api/certificate"

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

// NewAdminClient creates an Admin API test client.
func NewAdminClient(addr, key string) (*AdminClient, error) {
	url, err := urlpkg.Parse(addr)
	if err != nil {
		return nil, err
	}
	prt := newPrefixRoundTripper(url.Host, url.Scheme)
	httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: prt,
	}
	return &AdminClient{
		client: httpClient,
		key:    key,
	}, nil
}

// NewAPIClient creates an API server test client.
func NewAPIClient(addr, key string) (*APIClient, error) {
	url, err := urlpkg.Parse(addr)
	if err != nil {
		return nil, err
	}
	prt := newPrefixRoundTripper(url.Host, url.Scheme)
	httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: prt,
	}
	return &APIClient{
		client: httpClient,
		key:    key,
	}, nil
}
