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

// Package firebase is common logic and handling around firebase.
package firebase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type verifyPasswordResetCodeRequest struct {
	Code        string `json:"oobCode"`
	NewPassword string `json:"newPassword,omitempty"`
}

// VerifyPasswordResetCode is called with the one-time-code given from a reset email to the user.
// It can be used to check that the code is valid without making changes to the user.
//
// See: https://firebase.google.com/docs/reference/rest/auth#section-send-password-reset-email
func (c *Client) VerifyPasswordResetCode(ctx context.Context, code string) (string, error) {
	return c.ChangePasswordWithCode(ctx, code, "")
}

// ChangePasswordWithCode is called with the one-time-code given from a reset email to the user.
// When called with newPassword, it updates the user's password.
//
// See: https://firebase.google.com/docs/reference/rest/auth#section-send-password-reset-email
func (c *Client) ChangePasswordWithCode(ctx context.Context, code, newPassword string) (string, error) {
	r := &verifyPasswordResetCodeRequest{Code: code}
	if newPassword != "" {
		r.NewPassword = newPassword
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(r); err != nil {
		return "", fmt.Errorf("failed to create json body: %w", err)
	}

	u := c.buildURL("/v1/accounts:resetPassword")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &body)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send password reset email: %w", err)
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("response was %d, but failed to read body: %w", status, err)
	}

	if status != http.StatusOK {
		// Try to unmarshal the error message. Firebase uses these as enum values to expand on the code.
		var m map[string]ErrorDetails
		if err := json.Unmarshal(b, &m); err == nil {
			d := m["error"]
			return "", &d
		}

		return "", fmt.Errorf("failure %d: %s", status, string(b))
	}

	var m map[string]string
	if err := json.Unmarshal(b, &m); err == nil {
		return m["email"], nil
	}

	return "", nil
}
