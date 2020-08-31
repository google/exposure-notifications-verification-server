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

package admin

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleCreateRealm() http.Handler {
	type FormData struct {
		Name                   string `form:"name"`
		RegionCode             string `form:"regionCode"`
		UseRealmCertificateKey bool   `form:"useRealmCertificateKey"`
		CertificateIssuer      string `form:"certificateIssuer"`
		CertificateAudience    string `form:"certificateAudiance"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		user := controller.UserFromContext(ctx)
		if user == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		flash := controller.Flash(session)

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var realm database.Realm
			realm.UseRealmCertificateKey = true
			c.renderNew(ctx, w, &realm)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			var realm database.Realm
			realm.UseRealmCertificateKey = true
			flash.Error("Failed to process form: %v", err)
			c.renderNew(ctx, w, &realm)
			return
		}

		realm := database.NewRealmWithDefaults(form.Name)
		realm.RegionCode = form.RegionCode
		realm.UseRealmCertificateKey = form.UseRealmCertificateKey
		realm.CertificateIssuer = form.CertificateIssuer
		realm.CertificateAudience = form.CertificateAudience

		user.Realms = append(user.Realms, realm)
		user.AdminRealms = append(user.AdminRealms, realm)

		if err := c.db.SaveUser(user); err != nil {
			flash.Error("Failed to create realm: %v", err)
			c.renderNew(ctx, w, realm)
			return
		}
		flash.Alert("Created realm: %q. You have been made an admin of the realm.", realm.Name)

		// Remove this user from the cache so that the allowed realms will be reloaded.
		c.cacher.Delete(ctx, user.CacheKey())

		if realm.UseRealmCertificateKey {
			// If we are using realm specific keys - we need to create the first one.
			keyID, err := realm.CreateSigningKeyVersion(ctx, c.db)
			if err != nil {
				flash.Error("Failed to create signing keys for realm. This can be done from the realm's admin screens.")
				http.Redirect(w, r, "/admin/realms", http.StatusSeeOther)
				return
			}
			flash.Alert("Created initial signing key for realm, id: %q", keyID)
		}

		http.Redirect(w, r, "/admin/realms", http.StatusSeeOther)
	})
}

func (c *Controller) renderNew(ctx context.Context, w http.ResponseWriter, realm *database.Realm) {
	m := controller.TemplateMapFromContext(ctx)
	m["realm"] = realm
	m["supportsPerRealmSigning"] = c.db.SupportsPerRealmSigning()
	c.h.RenderHTML(w, "admin/newrealm", m)
}
