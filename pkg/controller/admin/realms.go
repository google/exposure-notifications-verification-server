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
	"github.com/gorilla/mux"
)

func (c *Controller) HandleRealmsIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		realms, err := c.db.GetRealms()
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		m["realms"] = realms
		c.h.RenderHTML(w, "admin/realms/index", m)
	})
}

func (c *Controller) HandleRealmsCreate() http.Handler {
	type FormData struct {
		Name                   string `form:"name"`
		RegionCode             string `form:"regionCode"`
		UseRealmCertificateKey bool   `form:"useRealmCertificateKey"`
		CertificateIssuer      string `form:"certificateIssuer"`
		CertificateAudience    string `form:"certificateAudiance"`
		CanUseSystemSMSConfig  bool   `form:"can_use_system_sms_config"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		smsConfig, err := c.db.SystemSMSConfig()
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var realm database.Realm
			realm.UseRealmCertificateKey = true
			c.renderNewRealm(ctx, w, &realm, smsConfig)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			var realm database.Realm
			realm.UseRealmCertificateKey = true
			flash.Error("Failed to process form: %v", err)
			c.renderNewRealm(ctx, w, &realm, smsConfig)
			return
		}

		realm := database.NewRealmWithDefaults(form.Name)
		realm.RegionCode = form.RegionCode
		realm.UseRealmCertificateKey = form.UseRealmCertificateKey
		realm.CertificateIssuer = form.CertificateIssuer
		realm.CertificateAudience = form.CertificateAudience
		realm.CanUseSystemSMSConfig = form.CanUseSystemSMSConfig
		if err := c.db.SaveRealm(realm, currentUser); err != nil {
			flash.Error("Failed to create realm: %v", err)
			c.renderNewRealm(ctx, w, realm, smsConfig)
			return
		}
		flash.Alert("Created realm: %q.", realm.Name)

		currentUser.Realms = append(currentUser.Realms, realm)
		currentUser.AdminRealms = append(currentUser.AdminRealms, realm)
		if err := c.db.SaveUser(currentUser, currentUser); err != nil {
			flash.Error("Failed to add you as an admin to the realm: %v", err)
			c.renderNewRealm(ctx, w, realm, smsConfig)
			return
		}
		flash.Alert("Added you as a user and admin to the realm.")

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

func (c *Controller) renderNewRealm(ctx context.Context, w http.ResponseWriter, realm *database.Realm, smsConfig *database.SMSConfig) {
	m := controller.TemplateMapFromContext(ctx)
	m["realm"] = realm
	m["systemSMSConfig"] = smsConfig
	m["supportsPerRealmSigning"] = c.db.SupportsPerRealmSigning()
	c.h.RenderHTML(w, "admin/realms/new", m)
}

func (c *Controller) HandleRealmsUpdate() http.Handler {
	type FormData struct {
		CanUseSystemSMSConfig bool `form:"can_use_system_sms_config"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		realm, err := c.db.FindRealm(vars["id"])
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		smsConfig, err := c.db.SystemSMSConfig()
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			c.renderEditRealm(ctx, w, realm, smsConfig)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderEditRealm(ctx, w, realm, smsConfig)
			return
		}

		realm.CanUseSystemSMSConfig = form.CanUseSystemSMSConfig
		if err := c.db.SaveRealm(realm, currentUser); err != nil {
			flash.Error("Failed to create realm: %v", err)
			c.renderEditRealm(ctx, w, realm, smsConfig)
			return
		}

		flash.Alert("Successfully updated realm %q", realm.Name)
		http.Redirect(w, r, "/admin/realms", http.StatusSeeOther)
	})
}

func (c *Controller) renderEditRealm(ctx context.Context, w http.ResponseWriter, realm *database.Realm, smsConfig *database.SMSConfig) {
	m := controller.TemplateMapFromContext(ctx)
	m["realm"] = realm
	m["systemSMSConfig"] = smsConfig
	m["supportsPerRealmSigning"] = c.db.SupportsPerRealmSigning()
	c.h.RenderHTML(w, "admin/realms/edit", m)
}

func (c *Controller) HandleRealmsJoin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		realm, err := c.db.FindRealm(vars["id"])
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		currentUser.Realms = append(currentUser.Realms, realm)
		currentUser.AdminRealms = append(currentUser.AdminRealms, realm)

		// Save the user
		if err := c.db.SaveUser(currentUser, currentUser); err != nil {
			flash.Error("Failed to join %q: %v", realm.Name, err)
			controller.Back(w, r, c.h)
			return
		}

		// Store the current realm on the session.
		controller.StoreSessionRealm(session, realm)

		flash.Alert("Successfully joined %q", realm.Name)
		controller.Back(w, r, c.h)
	})
}

func (c *Controller) HandleRealmsLeave() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		realm, err := c.db.FindRealm(vars["id"])
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		currentUser.RemoveRealm(realm)

		// Save the user
		if err := c.db.SaveUser(currentUser, currentUser); err != nil {
			flash.Error("Failed to leave %q: %v", realm.Name, err)
			controller.Back(w, r, c.h)
			return
		}

		// If the currently-selected realm is the realm the admin just left, clear
		// it.
		if controller.RealmIDFromSession(session) == realm.ID {
			controller.ClearSessionRealm(session)
		}
		flash.Alert("Successfully left %q", realm.Name)
		controller.Back(w, r, c.h)
	})
}
