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

package userreport

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleSend() http.Handler {
	type FormData struct {
		TestDate  string `form:"testDate"`
		Phone     string `form:"phone"`
		Agreement bool   `form:"agreement"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("userreport.HandleSend")

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		region := controller.RegionFromSession(session)
		realm, err := c.db.FindRealmByRegion(region)
		if err != nil {
			if database.IsNotFound(err) {
				controller.NotFound(w, r, c.h)
				return
			}

			logger.Warnw("region doesn't exist", "region", region, "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}
		ctx = controller.WithRealm(ctx, realm)

		if !realm.AllowsUserReport() {
			controller.NotFound(w, r, c.h)
			return
		}

		locale := controller.LocaleFromContext(ctx)
		if locale == nil {
			controller.InternalError(w, r, c.h, fmt.Errorf(locale.Get("user-report.invalid-request")))
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			logger.Warn("error binding form", "error", err)
			m["error"] = []string{locale.Get("user-report.invalid-request")}
			c.renderIndex(w, realm, m)
			return
		}
		m["date"] = form.TestDate
		m["phoneNumber"] = form.Phone
		m["agreement"] = form.Agreement

		// Pull the nonce from the session.
		nonceStr := controller.NonceFromSession(session)
		if nonceStr == "" {
			controller.NotFound(w, r, c.h)
			return
		}
		nonce, err := base64util.DecodeString(nonceStr)
		if err != nil {
			logger.Warnw("nonce cannot be decoded", "error", err)
			m["error"] = []string{locale.Get("user-report.invalid-request")}
			c.renderIndex(w, realm, m)
			return
		}

		// Check agreement.
		if !form.Agreement {
			msg := locale.Get("user-report.missing-agreement")
			m["error"] = []string{msg}
			m["termsError"] = msg
			c.renderIndex(w, realm, m)
			return
		}

		// Attempt to send the code.
		issueRequest := &issueapi.IssueRequestInternal{
			IssueRequest: &api.IssueCodeRequest{
				TestDate:         form.TestDate,
				TestType:         api.TestTypeUserReport, // Always test type of user report.
				Phone:            form.Phone,
				SMSTemplateLabel: database.UserReportTemplateLabel,
			},
			UserRequested: true,
			Nonce:         nonce,
		}

		// If the realm has configured a custom webhook URL, do not send the message
		// even if an SMS provider was given. Instead, generate the SMS message and
		// post the payload to their endpoint.
		if realm.UserReportWebhookURL != "" {
			issueRequest.IssueRequest.OnlyGenerateSMS = true
		}

		result := c.issueController.IssueOne(ctx, issueRequest)
		suppressError := false
		if result.HTTPCode != http.StatusOK {
			// Handle errors that the user can fix.
			if result.ErrorReturn.ErrorCode == api.ErrInvalidDate {
				// This shows a localized error without specifics and an English error string w/ specific dates.
				m["error"] = []string{
					locale.Get("user-report.error-invalid-date"),
					result.ErrorReturn.Error,
				}
				m["dateError"] = locale.Get("user-report.error-invalid-date")
				c.renderIndex(w, realm, m)
				return
			}
			if result.ErrorReturn.ErrorCode == api.ErrMissingPhone {
				msg := locale.Get("user-report.error-missing-phone")
				m["error"] = []string{msg}
				m["phoneError"] = msg
				c.renderIndex(w, realm, m)
				return
			}
			if result.ErrorReturn.ErrorCode == api.ErrQuotaExceeded {
				m["error"] = []string{locale.Get("user-report.quota-exceeded")}
				c.renderIndex(w, realm, m)
				return
			}
			if result.ErrorReturn.ErrorCode == api.ErrUserReportTryLater {
				// This error counts as success. It prevents a user from probing for
				// phone numbers that have already been used to self report.
				suppressError = true
			}

			if !suppressError {
				logger.Errorw("unable to issue user-report code", "status", result.HTTPCode, "error", result.ErrorReturn.Error)
				// The error returned isn't something the user can easily fix, show internal error, but hide form.
				m["error"] = []string{locale.Get("user-report.internal-error")}
				m["skipForm"] = true
				c.renderIndex(w, realm, m)
				return
			}
		}

		// Compile and send the payload to the webhook URL.
		if realm.UserReportWebhookURL != "" {
			if err := sendWebhookRequest(ctx, c.httpClient, realm, result); err != nil {
				logger.Errorw("failed to send webhook request", "error", err)
				m["error"] = []string{locale.Get("user-report.internal-error")}
				c.renderIndex(w, realm, m)
				return
			}
		}

		controller.ClearNonceFromSession(session)

		// If this is being accessed from an iOS device, send the close signal.
		if controller.OperatingSystemFromContext(ctx) == database.OSTypeIOS {
			m["webkitClose"] = true
		}

		m["realm"] = realm
		c.h.RenderHTML(w, "report/issue", m)
	})
}

// sendWebhookRequest builds and sends the webhook request to the realm's
// webhook URL.
func sendWebhookRequest(ctx context.Context, client *http.Client, realm *database.Realm, result *issueapi.IssueResult) error {
	logger := logging.FromContext(ctx).Named("userreport.sendWebhookRequest").
		With("realm", realm.ID).
		With("webhook_url", realm.UserReportWebhookURL)

	b, mac, err := buildAndSignPayloadForWebhook(realm.UserReportWebhookSecret, result)
	if err != nil {
		return fmt.Errorf("failed to build and sign payload for webhook: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, realm.UserReportWebhookURL, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("failed to build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", mac)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to issue payload request: %w", err)
	}
	defer resp.Body.Close()

	if code := resp.StatusCode; code != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		logger.Errorw("unsuccessful response from webhook",
			"code", code,
			"headers", resp.Header,
			"body", body)
		return fmt.Errorf("unsuccessful response from webhook (%d)", code)
	}

	return nil
}

// buildAndSignPayloaForWebhook encodes the response as JSON, generates an HMAC
// of the generated JSON, and returns the generated JSON and computed HMAC to
// the caller.
func buildAndSignPayloadForWebhook(secret string, body interface{}) ([]byte, string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal json for webhook: %w", err)
	}

	mac := hmac.New(sha512.New, []byte(secret))
	if _, err := mac.Write(b); err != nil {
		return nil, "", fmt.Errorf("failed to write hmac for webhook: %w", err)
	}
	result := hex.EncodeToString(mac.Sum(nil))

	return b, result, nil
}
