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
	"encoding/json"
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

	// accountSid and authToken are the auth information.
	accountSid, authToken string

	// from is the from number.
	from string
}

// NewTwilio creates a new Twilio SMS sender with the given auth.
func NewTwilio(ctx context.Context, accountSid, authToken, from string) (Provider, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	return &Twilio{
		client:     client,
		accountSid: accountSid,
		authToken:  authToken,
		from:       from,
	}, nil
}

type lookupResponse struct {
	Carrier *carrierResponse `json:"carrier"`
}

type carrierResponse struct {
	ErrorCode string `json:"error_code"`
	Name      string `json:"name"`
	Type      string `json:"type"`
}

// ValidateSMSNumber validates that the given phone number is capable of
// receiving SMS text messages. It returns an error if it's not.
func (p *Twilio) ValidateSMSNumber(ctx context.Context, number string) error {
	u := fmt.Sprintf("https://lookups.twilio.com/v1/PhoneNumbers/%s?Type=carrier", number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.SetBasicAuth(p.accountSid, p.authToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate SMS number: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("phone number is invalid")
	}

	var r lookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("failed to process response: %w", err)
	}

	if r.Carrier == nil || r.Carrier.Type != "mobile" {
		return fmt.Errorf("phone number is incapable of receiving SMS messages")
	}

	return nil
}

// SendSMS sends a message using the Twilio API.
func (p *Twilio) SendSMS(ctx context.Context, to, message string) error {
	params := url.Values{}
	params.Set("To", to)
	params.Set("From", p.from)
	params.Set("Body", message)
	body := strings.NewReader(params.Encode())

	u := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", p.accountSid)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.SetBasicAuth(p.accountSid, p.authToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if code := resp.StatusCode; code < 200 || code > 299 {
		var terr TwilioError
		if err := json.Unmarshal(respBody, &terr); err != nil {
			return fmt.Errorf("twilio error %d: %s", code, respBody)
		}
		return &terr
	}

	return nil
}

// TwilioError represents an error returned from the Twilio API.
type TwilioError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *TwilioError) Error() string {
	return e.Message
}
