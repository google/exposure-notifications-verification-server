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

package realmadmin

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func (c *Controller) HandleDisableExpress() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.SettingsWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}
		currentRealm := membership.Realm
		currentUser := membership.User

		if !currentRealm.EnableENExpress {
			flash.Error("Realm is not currently enrolled in EN Express.")
			c.renderSettings(ctx, w, r, currentRealm, nil, nil, 0, 0)
			return
		}

		defaultSettings := database.NewRealmWithDefaults("--")
		currentRealm.EnableENExpress = false
		currentRealm.SMSTextTemplate = defaultSettings.SMSTextTemplate
		if err := c.db.SaveRealm(currentRealm, currentUser); err != nil {
			flash.Error("Failed to disable EN Express: %v", err)

			c.renderSettings(ctx, w, r, currentRealm, nil, nil, 0, 0)
			return
		}

		flash.Alert("Successfully disabled EN Express")
		http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
	})
}

func (c *Controller) HandleEnableExpress() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.SettingsWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}
		currentRealm := membership.Realm
		currentUser := membership.User

		if currentRealm.EnableENExpress {
			flash.Error("Realm already has EN Express Enabled.")
			c.renderSettings(ctx, w, r, currentRealm, nil, nil, 0, 0)
			return
		}

		// Enable EN Express by setting default settings.
		enxSettings := database.NewRealmWithDefaults("--")
		currentRealm.EnableENExpress = true
		currentRealm.CodeLength = enxSettings.CodeLength
		currentRealm.CodeDuration = enxSettings.CodeDuration
		currentRealm.LongCodeLength = enxSettings.LongCodeLength
		currentRealm.LongCodeDuration = enxSettings.LongCodeDuration
		currentRealm.SMSTextTemplate = "Your Exposure Notifications verification link: [enslink] Expires in [longexpires] hours (click for mobile device only)"
		// Confirmed is the only allowed test type for EN Express.
		currentRealm.AllowedTestTypes = database.TestTypeConfirmed

		if err := c.db.SaveRealm(currentRealm, currentUser); err != nil {
			flash.Error("Failed to enable EN Express: %v", err)
			// This will allow the user to correct other validation errors and then click "uprade" again.
			currentRealm.EnableENExpress = false
			currentRealm.SMSTextTemplate = enxSettings.SMSTextTemplate
			c.renderSettings(ctx, w, r, currentRealm, nil, nil, 0, 0)
			return
		}

		flash.Alert("Successfully enabled EN Express!")
		http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
	})
}
