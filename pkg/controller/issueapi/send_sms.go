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

package issueapi

import (
	"context"
	"crypto"
	"fmt"
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

// BuildSMS builds and signs (if configured) the SMS message. It returns the
// complete and compiled message.
func (c *Controller) BuildSMS(ctx context.Context, realm *database.Realm, signer crypto.Signer, keyID string, request *api.IssueCodeRequest, vercode *database.VerificationCode) (string, error) {
	now := time.Now()

	logger := logging.FromContext(ctx).Named("issueapi.BuildSMS")
	redirectDomain := c.config.IssueConfig().ENExpressRedirectDomain

	message, err := realm.BuildSMSText(vercode.Code, vercode.LongCode, redirectDomain, request.SMSTemplateLabel)
	if err != nil {
		logger.Errorw("failed to build sms text for realm",
			"template", request.SMSTemplateLabel,
			"error", err)
		return "", fmt.Errorf("failed to build sms message: %w", err)
	}

	// A signer will only be provided if the realm has configured and enabled
	// SMS signing.
	if signer == nil {
		return message, nil
	}

	purpose := signatures.SMSPurposeENReport
	if request.TestType == api.TestTypeUserReport {
		purpose = signatures.SMSPurposeUserReport
	}

	message, err = signatures.SignSMS(signer, keyID, now, purpose, request.Phone, message)
	if err != nil {
		logger.Errorw("failed to sign sms", "error", err)
		if c.config.GetAuthenticatedSMSFailClosed() {
			return "", fmt.Errorf("failed to sign sms: %w", err)
		}
	}
	return message, nil
}

func (c *Controller) doSend(ctx context.Context, realm *database.Realm, smsProvider sms.Provider, signer crypto.Signer, keyID string, request *api.IssueCodeRequest, result *IssueResult) error {
	defer enobs.RecordLatency(ctx, time.Now(), mSMSLatencyMs, &result.obsResult)

	logger := logging.FromContext(ctx).Named("issueapi.sendSMS")

	// Build the message
	message, err := c.BuildSMS(ctx, realm, signer, keyID, request, result.VerCode)
	if err != nil {
		logger.Errorw("failed to build sms", "error", err)
		result.obsResult = enobs.ResultError("FAILED_TO_BUILD_SMS")
		return err
	}

	// Send the message
	if err := smsProvider.SendSMS(ctx, request.Phone, message); err != nil {
		// Delete the user report record.
		if result.VerCode.UserReportID != nil {
			if err := c.db.DeleteUserReport(request.Phone); err != nil {
				logger.Errorw("failed to delete the user report record", "error", err)
			}
		}

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
