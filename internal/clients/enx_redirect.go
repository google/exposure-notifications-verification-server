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
	"log"
	"net/http"
	"net/url"
	"strings"

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

func (c *ENXRedirectClient) SendUserReportIndex(ctx context.Context, apikey string, nonce string) error {
	req, err := c.newRequest(ctx, http.MethodPost, "/report", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", apikey)
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

func (c *ENXRedirectClient) SendUserReportIssue(ctx context.Context, testDate string, symptomDate string, phone string, agree string) error {
	// Send the issue code request
	values := url.Values{
		"testDate":    []string{testDate},
		"symptomDate": []string{symptomDate},
		"phone":       []string{phone},
		"agreement":   []string{agree},
	}

	log.Printf("VAULES: %+v", values)

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

// AppleSiteAssociation calls and parses the Apple site association file.
func (c *ENXRedirectClient) AppleSiteAssociation(ctx context.Context) (*api.IOSDataResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/.well-known/apple-app-site-association", nil)
	if err != nil {
		return nil, err
	}

	var out api.IOSDataResponse
	if err := c.doOK(req, &out); err != nil {
		return &out, err
	}
	return &out, nil
}

// AndroidAssetLinks calls and parses the Android assetlinks file.
func (c *ENXRedirectClient) AndroidAssetLinks(ctx context.Context) ([]*api.AndroidDataResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/.well-known/assetlinks.json", nil)
	if err != nil {
		return nil, err
	}

	var out []*api.AndroidDataResponse
	if err := c.doOK(req, &out); err != nil {
		return out, err
	}
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

	req, err := c.newRequest(ctx, http.MethodGet, "/", nil)
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

// RunE2E uses the client to exercise an end-to-end test of the ENX redirector.
func (c *ENXRedirectClient) RunE2E(ctx context.Context) error {
	// Android
	androidResp, err := c.AndroidAssetLinks(ctx)
	if err != nil {
		return fmt.Errorf("failed to get android asset links: %w", err)
	}
	if len(androidResp) == 0 || androidResp[0].Target.PackageName == "" {
		return fmt.Errorf("expected android assetlinks, got %#v", androidResp)
	}

	// iOS
	iosResp, err := c.AppleSiteAssociation(ctx)
	if err != nil {
		return fmt.Errorf("failed to get apple site association: %w", err)
	}
	if iosResp == nil || len(iosResp.Applinks.Details) == 0 {
		return fmt.Errorf("expected apple site association, got %#v", iosResp)
	}

	// Android redirect
	androidHTTPResp, err := c.CheckRedirect(ctx, "android")
	if err != nil {
		return fmt.Errorf("failed to check android redirect: %w", err)
	}
	defer androidHTTPResp.Body.Close()
	if got, want := androidHTTPResp.StatusCode, http.StatusSeeOther; got != want {
		return fmt.Errorf("expected android redirect code %d to be %d", got, want)
	}
	if got, want := androidHTTPResp.Header.Get("Location"), "android.test.app"; !strings.Contains(got, want) {
		return fmt.Errorf("expected android redirect location %q to contain %q", got, want)
	}

	// iOS redirect
	iosHTTPResp, err := c.CheckRedirect(ctx, "iphone")
	if err != nil {
		return fmt.Errorf("failed to check apple redirect: %w", err)
	}
	defer iosHTTPResp.Body.Close()
	if got, want := iosHTTPResp.StatusCode, http.StatusSeeOther; got != want {
		return fmt.Errorf("expected apple redirect code %d to be %d", got, want)
	}
	if got, want := iosHTTPResp.Header.Get("Location"), "ios.test.app"; !strings.Contains(got, want) {
		return fmt.Errorf("expected apple redirect location %q to contain %q", got, want)
	}

	// unknown redirect
	unknownHTTPResp, err := c.CheckRedirect(ctx, "unknown")
	if err != nil {
		return fmt.Errorf("failed to check unknown redirect: %w", err)
	}
	defer unknownHTTPResp.Body.Close()
	if got, want := unknownHTTPResp.StatusCode, http.StatusSeeOther; got != want {
		return fmt.Errorf("expected unknown redirect code %d to be %d", got, want)
	}
	// expecting generic landing redirect
	if got, want := unknownHTTPResp.Header.Get("Location"), "https://www.google.com/covid19/exposurenotifications"; !strings.Contains(got, want) {
		return fmt.Errorf("expected unknown redirect location %q to contain %q", got, want)
	}

	return nil
}
