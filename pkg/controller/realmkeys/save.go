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

package realmkeys

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

// HandleSave handles saving certificate settings to the current realm.
func (c *Controller) HandleSave() http.Handler {
	type FormData struct {
		Issuer         string `form:"certificateIssuer"`
		Audience       string `form:"certificateAudience"`
		DurationString string `form:"certificateDuration"`
	}

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

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderShow(ctx, w, r, currentRealm)
			return
		}

		// Update settings.
		if !currentRealm.UseRealmCertificateKey {
			// Once upgraded to realm specific, these values cannot change.
			currentRealm.CertificateIssuer = form.Issuer
			currentRealm.CertificateAudience = form.Audience
		}
		// AsString delgates the duration parsing and validation to the model.
		currentRealm.CertificateDuration.AsString = form.DurationString

		if err := c.db.SaveRealm(currentRealm, currentUser); err != nil {
			flash.Error("Failed to update realm: %v", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderShow(ctx, w, r, currentRealm)
		}

		flash.Alert("Updated realm certificate settings.")
		c.redirectShow(ctx, w, r)
	})
}
