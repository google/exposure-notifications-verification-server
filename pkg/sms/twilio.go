// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/sethvargo/go-retry"
)

// TwilioMessagingServiceSidPrefix is the prefix for a 34 character messaging service identifier
const TwilioMessagingServiceSidPrefix = "MG"

var _ Provider = (*Twilio)(nil)

// Twilio sends messages via the Twilio API.
type Twilio struct {
	client *http.Client
	from   string
}

// NewTwilio creates a new Twilio SMS sender with the given auth.
func NewTwilio(ctx context.Context, accountSid, authToken, from string) (Provider, error) {
	transport := project.DefaultHTTPTransport()
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &twilioAuthRoundTripper{transport, accountSid, authToken},
	}

	return &Twilio{
		client: client,
		from:   from,
	}, nil
}

// SendSMS sends a message using the Twilio API.
func (p *Twilio) SendSMS(ctx context.Context, to, message string) error {
	b := retry.NewFibonacci(250 * time.Millisecond)
	b = retry.WithMaxRetries(4, b)

	return retry.Do(ctx, b, func(ctx context.Context) error {
		params := url.Values{}
		params.Set("To", to)
		if strings.HasPrefix(p.from, TwilioMessagingServiceSidPrefix) {
			params.Set("MessagingServiceSid", p.from)
		} else {
			params.Set("From", p.from)
		}

		params.Set("Body", message)
		body := strings.NewReader(params.Encode())

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/Messages.json", body)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Close = true

		resp, err := p.client.Do(req)
		if err != nil {
			return retry.RetryableError(fmt.Errorf("failed to make request: %w", err))
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if code := resp.StatusCode; code < http.StatusOK || code >= http.StatusMultipleChoices {
			var terr TwilioError
			if err := json.Unmarshal(respBody, &terr); err != nil {
				return fmt.Errorf("twilio error %d: %s", code, respBody)
			}
			return &terr
		}

		return nil
	})
}

// twilioAuthRoundTripper is an http.RoundTripper that updates the
// authentication and headers to match Twilio's API.
type twilioAuthRoundTripper struct {
	transport             *http.Transport
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

	return rt.transport.RoundTrip(r)
}

// TwilioError represents an error returned from the Twilio API.
type TwilioError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *TwilioError) Error() string {
	return e.Message
}

// IsSMSQueueFull returns if the given error is Twilio's message queue full error.
func IsSMSQueueFull(err error) bool {
	return IsTwilioCode(err, 21611) // https://www.twilio.com/docs/api/errors/21611
}

// IsTwilioCode returns if the given error matches a Twilio error code.
func IsTwilioCode(err error, code int) bool {
	var tErr *TwilioError
	if errors.As(err, &tErr) {
		return tErr.Code == code
	}
	return false
}
