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

// HandleAnomalies handles a request to send emails about code ratio anomalies.
func (c *Controller) HandleAnomalies() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("emailer.HandleAnomalies")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		ok, err := c.db.TryLock(ctx, emailerAnomaliesLock, c.config.MinTTL)
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
			if err := c.sendAnomaliesEmails(ctx, realm); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to send emails for realm %d: %w", realm.ID, err))
				continue
			}
		}

		if err := merr.ErrorOrNil(); err != nil {
			logger.Errorw("failed to send anomalies emails", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, mAnomaliesSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}

// sendAnomaliesEmails sends anomalies emails to all contacts configured in the
// realm if the current values are anomalous.
func (c *Controller) sendAnomaliesEmails(ctx context.Context, realm *database.Realm) error {
	logger := logging.FromContext(ctx).Named("emailer.sendAnomaliesEmails").
		With("realm_id", realm.ID)

	if len(realm.ContactEmailAddresses) == 0 {
		logger.Debugw("no contact email addresses registered, skipping")
		return nil
	}

	if !realm.CodesClaimedRatioAnomalous() {
		logger.Debugw("codes claimed ratio is not anomalous, skipping")
		return nil
	}

	var merr *multierror.Error
	for _, addr := range realm.ContactEmailAddresses {
		msg, err := c.h.RenderEmail("email/anomalies", map[string]interface{}{
			"ToEmail":   addr,
			"FromEmail": c.config.FromAddress,
			"Realm":     realm,
			"RootURL":   c.config.ServerEndpoint,
		})
		if err != nil {
			return fmt.Errorf("failed to render template: %w", err)
		}

		logger.Debugw("sending email", "email", addr)
		if err := c.sendMail(addr, msg); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to send to %q: %w", addr, err))
		}
	}

	return merr.ErrorOrNil()
}
