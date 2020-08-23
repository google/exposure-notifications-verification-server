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

package realmkeys

import (
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleSave() http.Handler {
	type FormData struct {
		Issuer         string `form:"iss"`
		Audience       string `form:"aud"`
		DuratingString string `form:"certDuration"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderShow(ctx, w, r, realm)
			return
		}

		errors := false
		if realm.UseRealmCertificateKey {
			if form.Issuer == "" {
				flash.Error("Issuer cannot be blank")
				errors = true
			}
			if form.Audience == "" {
				flash.Error("Audience cannot be blank")
				errors = true
			}
		}
		dur, err := time.ParseDuration(form.DuratingString)
		if err != nil {
			flash.Error("Certificate duration is invalid: %v", err)
			errors = true
		}
		if errors {
			c.renderShow(ctx, w, r, realm)
		} else {
			// Update settings.
			realm.CertificateIssuer = form.Issuer
			realm.CertificateAudience = form.Audience
			realm.CertificateDuration.Duration = dur

			if err := c.db.SaveRealm(realm); err != nil {
				flash.Error("Failed to update realm: %v", err)
			} else {
				flash.Alert("Updated realm certificate settings.")
			}
		}
		c.renderShow(ctx, w, r, realm)
	})
}
