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
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"go.opencensus.io/stats"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("userreport.HandleIndex")

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			controller.NotFound(w, r, c.h)
			return
		}

		realm := controller.RealmFromContext(ctx)
		if !realm.AllowsUserReport() {
			controller.NotFound(w, r, c.h)
			return
		}

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		locale := controller.LocaleFromContext(ctx)
		if locale == nil {
			logger.Errorw("no locale in context")
			controller.InternalError(w, r, c.h, fmt.Errorf("internal error, please try again"))
			return
		}

		m := controller.TemplateMapFromContext(ctx)

		now := time.Now().UTC()
		pastDaysDuration := -1 * c.config.IssueConfig().AllowedSymptomAge
		displayAllowedDays := fmt.Sprintf("%.0f", c.config.IssueConfig().AllowedSymptomAge.Hours()/24.0)
		m["maxDate"] = now.Format(project.RFC3339Date)
		m["minDate"] = now.Add(pastDaysDuration).Format(project.RFC3339Date)
		m["maxSymptomDays"] = displayAllowedDays
		m["duration"] = realm.CodeDuration.Duration.String()

		var errorMessages []string
		// Check the nonce - if it isn't valid, show an error page, but with branding since we know the app.
		nonce := controller.NonceFromContext(ctx)
		if decoded, err := base64util.DecodeString(nonce); err != nil || len(decoded) != database.NonceLength {
			stats.Record(ctx, mInvalidNonce.M(1))
			logger.Warnw("invalid nonce on webview load", "error", err, "nonce-length", len(decoded))
			errorMessages = addError(locale.Get("user-report.invalid-request"), errorMessages)
			m["skipForm"] = true
		}

		// This could get triggered in a pause.
		if !realm.AllowsUserReport() {
			stats.Record(ctx, mUserReportNotAllowed.M(1))
			errorMessages = addError(locale.Get("user-report.not-available"), errorMessages)
			m["skipForm"] = true
		}

		m["error"] = errorMessages
		// stash the nonce value in the session
		controller.StoreSessionNonce(session, nonce)
		controller.StoreSessionRegion(session, realm.RegionCode)
		c.renderIndex(w, realm, m)
	})
}

func (c *Controller) renderIndex(w http.ResponseWriter, realm *database.Realm, m controller.TemplateMap) {
	m["realm"] = realm
	c.h.RenderHTML(w, "report/index", m)
}
