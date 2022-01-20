// Copyright 2022 the Exposure Notifications Verification Server authors
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

package emailer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
)

// HandleSMSErrors handles a request to send emails about code ratio anomalies.
func (c *Controller) HandleSMSErrors() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("emailer.HandleSMSErrors")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		ok, err := c.db.TryLock(ctx, emailerSMSErrorsLock, c.config.MinTTL)
		if err != nil {
			logger.Errorw("failed to acquire lock", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}
		if !ok {
			logger.Debugw("skipping (too early)")
			c.h.RenderJSON(w, http.StatusOK, fmt.Errorf("too early"))
			return
		}

		// Get the list of realms.
		realms, _, err := c.db.ListRealms(pagination.UnlimitedResults)
		if err != nil {
			logger.Errorw("failed to list realms", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		var merr *multierror.Error
		for _, realm := range realms {
			if err := c.sendSMSErrorsEmails(ctx, realm); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to send emails for realm %d: %w", realm.ID, err))
				continue
			}
		}

		if err := merr.ErrorOrNil(); err != nil {
			logger.Errorw("failed to send sms errors emails", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, mSMSErrorsSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}

// sendSMSErrorsEmails sends emails to all email contacts in the realm if SMS
// errors are above the acceptable thresholds.
func (c *Controller) sendSMSErrorsEmails(ctx context.Context, realm *database.Realm) error {
	logger := logging.FromContext(ctx).Named("emailer.sendSMSErrorsEmails").
		With("realm_id", realm.ID)

	from := c.config.FromAddress
	tos := realm.ContactEmailAddresses
	ccs := c.config.CCAddresses
	bccs := c.config.BCCAddresses

	if len(tos) == 0 {
		logger.Warnw("no contact email addresses registered")

		if len(ccs) == 0 && len(bccs) == 0 {
			logger.Warnw("no cc or bcc emails registered either, skipping")
			return nil
		}
	}
	var addresses []string
	addresses = append(addresses, tos...)
	addresses = append(addresses, ccs...)
	addresses = append(addresses, bccs...)

	count, err := realm.RecentSMSErrorsCount(c.db, c.config.SMSIgnoredErrorCodes)
	if err != nil {
		return fmt.Errorf("failed to get recent sms errors count: %w", err)
	}
	if minimum := c.config.SMSErrorsEmailThreshold; count < minimum {
		logger.Debugw("sms errors is less than minimum value, skipping",
			"count", count,
			"minimum", minimum)
		return nil
	}

	msg, err := c.h.RenderEmail("email/sms_errors", map[string]interface{}{
		"FromAddress":  from,
		"ToAddresses":  tos,
		"CCAddresses":  ccs,
		"BCCAddresses": bccs,
		"Realm":        realm,
		"RootURL":      c.config.ServerEndpoint,
		"NumSMSErrors": count,
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	logger.Debugw("sending email",
		"tos", realm.ContactEmailAddresses,
		"ccs", c.config.CCAddresses,
		"bccs", c.config.BCCAddresses)
	if err := c.sendMail(ctx, addresses, msg); err != nil {
		return fmt.Errorf("failed to send: %w", err)
	}
	return nil
}
