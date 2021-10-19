// Copyright 2021 the Exposure Notifications Verification Server authors
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

package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/gorilla/mux"
	"go.opencensus.io/stats"
)

type TwilioWebhookPayload struct {
	ErrorCode string `json:"error_code"`
}

// HandleTwilio handles secrets rotation.
func (c *Controller) HandleTwilio() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("webhooks.HandleTwilio")
		vars := mux.Vars(r)

		// Ensure the header is present. We'll validate it later, but validating it
		// requires us to lookup the realm and decrypt the authToken, which is an
		// expensive operation. So we do all the checks we can in advance of that
		// lookup to minimize I/O.
		givenSignature := r.Header.Get("X-Twilio-Signature")
		if givenSignature == "" {
			logger.Debug("request is missing signature header")
			controller.BadRequest(w, r, c.h)
			return
		}

		if err := r.ParseForm(); err != nil {
			logger.Errorw("failed to parse form", "error", err)
			controller.BadRequest(w, r, c.h)
			return
		}

		// Ensure there's a Level field with a value of "ERROR".
		level := strings.TrimSpace(r.Form.Get("Level"))
		if got, want := level, "ERROR"; !strings.EqualFold(got, want) {
			logger.Debugw("request Level is not ERROR", "value", got)
			c.h.RenderJSON(w, http.StatusOK, nil)
			return
		}

		// Ensure there's a PayloadType field with a value of "application/json".
		payloadType := strings.TrimSpace(r.Form.Get("PayloadType"))
		if got, want := payloadType, "application/json"; !strings.EqualFold(got, want) {
			logger.Debugw("request PayloadType is not application/json", "value", got)
			c.h.RenderJSON(w, http.StatusOK, nil)
			return
		}

		// Parse up to 64 KiB of the payload as JSON (it should be bytes normally).
		lr := io.LimitReader(strings.NewReader(r.Form.Get("Payload")), 64*1024) // 64 KiB
		var payload TwilioWebhookPayload
		if err := json.NewDecoder(lr).Decode(&payload); err != nil {
			logger.Warnw("failed to decode Payload", "error", err)
			controller.BadRequest(w, r, c.h)
			return
		}

		// If there's no error code, continue.
		if payload.ErrorCode == "" {
			logger.Debugw("got payload, but error_code is empty")
			c.h.RenderJSON(w, http.StatusOK, nil)
			return
		}

		// If we got this far, this is a webhook request for which we should
		// increment a metric. Find the realm based on the URL param.
		logger = logger.With("realm_id", vars["realm_id"])
		realm, err := c.db.FindRealm(vars["realm_id"])
		if err != nil {
			logger.Warnw("failed to lookup realm", "error", err)

			if database.IsNotFound(err) {
				controller.BadRequest(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Look up the sms configuration for the realm. This is necessary because
		// Twilio uses the auth token as the HMAC key.
		smsConfig, err := realm.SMSConfig(c.db)
		if err != nil {
			logger.Warnw("failed to lookup realm sms config", "error", err)

			if database.IsNotFound(err) {
				controller.BadRequest(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Sanity check account sids.
		if got, want := r.Form.Get("AccountSid"), smsConfig.TwilioAccountSid; got != want {
			logger.Warnw("twilio account sid mismatch",
				"got", got,
				"want", want)
			controller.BadRequest(w, r, c.h)
			return
		}

		// Calculate the expected signature.
		expSignature, err := ComputeSignature(r, smsConfig.TwilioAuthToken)
		if err != nil {
			logger.Errorw("failed to compute twilio signature", "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Compare the expected signature with the given signature.
		if subtle.ConstantTimeCompare([]byte(givenSignature), []byte(expSignature)) != 1 {
			logger.Debugw("signature mismatch",
				"given", givenSignature,
				"expected", expSignature)
			controller.BadRequest(w, r, c.h)
			return
		}

		// If we got this far, the message passed the signature check.
		ctx = observability.WithRealmID(ctx, uint64(realm.ID))
		ctx = observability.WithErrorCode(ctx, payload.ErrorCode)
		defer stats.Record(ctx, mTwilioErrors.M(1))

		if err := c.db.InsertSMSErrorStat(realm.ID, payload.ErrorCode); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, nil)
		return
	})
}

// ComputeSignature builds the expected webhook signature from a Twilio request
// as described in
// https://www.twilio.com/docs/usage/security#validating-requests.
func ComputeSignature(r *http.Request, authToken string) (string, error) {
	if r.Method != http.MethodPost {
		return "", fmt.Errorf("request is not POST")
	}

	if err := r.ParseForm(); err != nil {
		return "", fmt.Errorf("failed to parse form: %w", err)
	}

	// Twilio takes all POST fields, sorts them alphabetically by their name, and
	// concatenates the parameter name and value to the end of the URL (with no
	// delimiter).
	keys := make([]string, 0, len(r.Form))
	for k := range r.Form {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	buf.WriteString(r.URL.String())
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString(r.Form.Get(k))
	}

	h := hmac.New(sha1.New, []byte(authToken))
	if _, err := h.Write(buf.Bytes()); err != nil {
		return "", fmt.Errorf("failed to generate hash: %w", err)
	}
	sig := h.Sum(nil)

	return base64.StdEncoding.EncodeToString(sig), nil
}
