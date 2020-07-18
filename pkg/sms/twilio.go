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

package sms

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var _ Provider = (*Twilio)(nil)

// Twilio sends messages via the Twilio API.
type Twilio struct {
	client *http.Client
}

// NewTwilio creates a new Twilio SMS sender with the given auth.
func NewTwilio(ctx context.Context, accountSid, authToken string) (Provider, error) {
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &twilioAuthRoundTripper{accountSid, authToken},
	}

	return &Twilio{
		client: client,
	}, nil
}

// SendSMS sends a message using the Twilio API.
func (p *Twilio) SendSMS(ctx context.Context, from, to, message string) error {
	params := url.Values{}
	params.Set("To", to)
	params.Set("From", from)
	params.Set("Body", message)
	body := strings.NewReader(params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/Messages.json", body)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if code := resp.StatusCode; code < 200 || code > 299 {
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("twilio error %d, but failed to read body: %w", code, err)
		}
		return fmt.Errorf("twilio error %d: %s", code, respBody)
	}

	return nil
}

// twilioAuthRoundTripper is an http.RoundTripper that updates the
// authentication and headers to match Twilio's API.
type twilioAuthRoundTripper struct {
	accountSid, authToken string
}

// RoundTrip implements http.RoundTripper.
func (rt *twilioAuthRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL = &url.URL{
		Scheme: "https",
		Host:   "api.twilio.com",
		Path:   "/2010-04-01/Accounts/" + rt.accountSid + "/" + strings.Trim(r.URL.Path, "/"),
	}

	r.SetBasicAuth(rt.accountSid, rt.authToken)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return http.DefaultTransport.RoundTrip(r)
}
