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
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
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

// scrubPhoneNumbers checks for phone numbers in known Twilio error strings that contains
// user phone numbers.
func scrubPhoneNumbers(s string) string {
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

func (il *Controller) sendSMS(ctx context.Context, request *api.IssueCodeRequest, result *IssueResult, realm *database.Realm) error {
	if request.Phone == "" {
		return nil
	}
	smsProvider, err := realm.SMSProvider(il.db)
	if smsProvider == nil {
		return nil
	}
	if err != nil {
		return err
	}

	logger := logging.FromContext(ctx).Named("issueapi.sendSMS")

	if err := func() error {
		defer observability.RecordLatency(ctx, time.Now(), mSMSLatencyMs, &result.ObsBlame, &result.ObsResult)

		message, err := realm.BuildSMSText(result.verCode.Code, result.verCode.LongCode, il.config.GetENXRedirectDomain(), request.SMSTemplateLabel)
		if err != nil {
			return err
		}

		if err := smsProvider.SendSMS(ctx, request.Phone, message); err != nil {
			// Delete the token
			if err := il.db.DeleteVerificationCode(result.verCode.Code); err != nil {
				logger.Errorw("failed to delete verification code", "error", err)
				// fallthrough to the error
			}

			logger.Infow("failed to send sms", "error", scrubPhoneNumbers(err.Error()))
			result.ObsBlame = observability.BlameClient
			result.ObsResult = observability.ResultError("FAILED_TO_SEND_SMS")
			return err
		}
		return nil
	}(); err != nil {
		result.HTTPCode = http.StatusBadRequest
		result.errorReturn = api.Errorf("failed to send sms: %s", err)
		return err
	}
	return nil
}
