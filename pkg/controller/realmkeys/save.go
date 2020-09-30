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

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleSave() http.Handler {
	type FormData struct {
		Issuer         string `form:"certificateIssuer"`
		Audience       string `form:"certificateAudience"`
		DuratingString string `form:"certificateDuration"`
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

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderShow(ctx, w, r, realm)
			return
		}

		// Update settings.
		realm.CertificateIssuer = form.Issuer
		realm.CertificateAudience = form.Audience
		// AsString delgates the duration parsing and validation to the model.
		realm.CertificateDuration.AsString = form.DuratingString

		if err := c.db.SaveRealm(realm, currentUser); err != nil {
			flash.Error("Failed to update realm: %v", err)
			c.renderShow(ctx, w, r, realm)
		}

		flash.Alert("Updated realm certificate settings.")
		c.redirectShow(ctx, w, r)
	})
}
