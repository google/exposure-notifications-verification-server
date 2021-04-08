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
	"net/url"
	"strings"
)

// ENXRedirectWebClient is a client that talks to the enx-redirect web components (user-report).
type ENXRedirectWebClient struct {
	*client
}

// NewENXRedirectWebClient creates a new enx-redirect service http client for user-report.
func NewENXRedirectWebClient(base string, apiKey string, opts ...Option) (*ENXRedirectWebClient, error) {
	client, err := newClient(base, apiKey, opts...)
	if err != nil {
		return nil, err
	}

	return &ENXRedirectWebClient{
		client: client,
	}, nil
}

// SendUserReportIndex request "/report" on the ENX Redirect server which is the landing page
// for a client embedded webview. This requires a client with an installed cookiejar to work correctly
// since this will create a session cookie that embeds the nonce provided in the header.
func (c *ENXRedirectWebClient) SendUserReportIndex(ctx context.Context, nonce string) error {
	req, err := c.newRequest(ctx, http.MethodPost, "/report", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Nonce", nonce)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making initial load request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code from load request: %v", res.StatusCode)
	}
	return nil
}

// SendUserReportIssue issues a user-report verification code by posting the web form. Must be called from the same
// client
func (c *ENXRedirectWebClient) SendUserReportIssue(ctx context.Context, testDate string, phone string, agree string) error {
	// Send the issue code request
	values := &url.Values{
		"testDate":  []string{testDate},
		"phone":     []string{phone},
		"agreement": []string{agree},
	}

	u := c.baseURL.ResolveReference(&url.URL{Path: "report/issue"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error posting report form: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code from load request: %v", res.StatusCode)
	}
	return nil
}
