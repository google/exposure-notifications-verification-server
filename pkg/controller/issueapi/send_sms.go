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

package issueapi

import (
	"context"
	"crypto"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/signatures"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"go.opencensus.io/stats"
)

// scrubbers is a list of known Twilio error messages that contain the send to phone number.
var scrubbers = []struct {
	prefix string
	suffix string
}{
	{
		prefix: "phone number: ",
		suffix: ", ",
	},
	{
		prefix: "'To' number ",
		suffix: " is not",
	},
}

// ScrubPhoneNumbers checks for phone numbers in known Twilio error strings that contains
// user phone numbers.
func ScrubPhoneNumbers(s string) string {
	noScrubs := s
	for _, scrub := range scrubbers {
		pi := strings.Index(noScrubs, scrub.prefix)
		si := strings.Index(noScrubs, scrub.suffix)

		// if prefix is in the string and suffix is in the sting after the prefix
		if pi >= 0 && si > pi+len(scrub.prefix) {
			noScrubs = strings.Join([]string{
				noScrubs[0 : pi+len(scrub.prefix)],
				noScrubs[si:],
			}, "REDACTED")
		}
	}
	return noScrubs
}

// SendSMS sends the sms mesage with the given provider and wraps any seen errors into the IssueResult
func (c *Controller) SendSMS(ctx context.Context, realm *database.Realm, smsProvider sms.Provider, signer crypto.Signer, keyID string, request *api.IssueCodeRequest, result *IssueResult) {
	if request.Phone == "" {
		return
	}

	if err := c.doSend(ctx, realm, smsProvider, signer, keyID, request, result); err != nil {
		result.HTTPCode = http.StatusBadRequest
		if sms.IsSMSQueueFull(err) {
			result.ErrorReturn = result.ErrorReturn.WithCode(api.ErrSMSQueueFull)
		} else {
			result.ErrorReturn = api.Errorf("failed to send sms: %s", err).WithCode(api.ErrSMSFailure)
		}
	}
}

func (c *Controller) doSend(ctx context.Context, realm *database.Realm, smsProvider sms.Provider, signer crypto.Signer, keyID string, request *api.IssueCodeRequest, result *IssueResult) error {
	smsStart := time.Now()
	defer enobs.RecordLatency(ctx, smsStart, mSMSLatencyMs, &result.obsResult)

	logger := logging.FromContext(ctx).Named("issueapi.sendSMS")

	message, err := realm.BuildSMSText(result.VerCode.Code, result.VerCode.LongCode, c.config.GetENXRedirectDomain(), request.SMSTemplateLabel)
	if err != nil {
		result.obsResult = enobs.ResultError("FAILED_TO_BUILD_SMS")
		return err
	}

	// A signer will only be provided if the realm has configured and enabled
	// SMS signing.
	if signer != nil {
		var err error
		message, err = signatures.SignSMS(signer, keyID, smsStart, signatures.SMSPurposeENReport, request.Phone, message)
		if err != nil {
			defer func() {
				if err := stats.RecordWithOptions(ctx,
					stats.WithMeasurements(mAuthenticatedSMSFailure.M(1)),
					stats.WithTags(enobs.ResultError("FAILED_TO_SIGN_SMS"))); err != nil {
					logger.Errorw("failed to record stats", "error", err)
				}
			}()

			if c.config.GetAuthenticatedSMSFailClosed() {
				result.obsResult = enobs.ResultError("FAILED_TO_SIGN_SMS")
				return err
			}

			// Fail open, but still log the error and record the metric.
			logger.Errorw("failed to sign sms", "error", err)
		}
	}

	if err := smsProvider.SendSMS(ctx, request.Phone, message); err != nil {
		// Delete the token
		if err := c.db.DeleteVerificationCode(result.VerCode.Code); err != nil {
			logger.Errorw("failed to delete verification code", "error", err)
			// fallthrough to the error
		}

		logger.Infow("failed to send sms", "error", ScrubPhoneNumbers(err.Error()))
		result.obsResult = enobs.ResultError("FAILED_TO_SEND_SMS")
		return err
	}
	return nil
}
