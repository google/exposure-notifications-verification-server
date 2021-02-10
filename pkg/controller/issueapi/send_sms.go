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
)

var (
	// scrubbers is a list of known Twilio error messages that contain the send to phone number.
	scrubbers = []struct {
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
)

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

func (c *Controller) SendSMS(ctx context.Context, realm *database.Realm, smsProvider sms.Provider, signer crypto.Signer, keyID string, request *api.IssueCodeRequest, result *IssueResult) error {
	if request.Phone == "" {
		return nil
	}

	logger := logging.FromContext(ctx).Named("issueapi.sendSMS")
	smsStart := time.Now()
	err := func() error {
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
				result.obsResult = enobs.ResultError("FAILED_TO_SIGN_SMS")
				return err
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
	}()
	enobs.RecordLatency(ctx, smsStart, mSMSLatencyMs, &result.obsResult)
	if err != nil {
		result.HTTPCode = http.StatusBadRequest
		result.ErrorReturn = api.Errorf("failed to send sms: %s", err).WithCode(api.ErrSMSFailure)

		if sms.IsSMSQueueFull(err) {
			result.ErrorReturn = result.ErrorReturn.WithCode(api.ErrSMSQueueFull)
		}
		return err
	}
	return nil
}
