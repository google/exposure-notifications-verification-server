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
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
)

func (c *Controller) HandleRealmsIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		memberships := controller.MembershipsFromContext(ctx)
		membershipsMap := make(map[uint]bool, len(memberships))
		for _, m := range memberships {
			membershipsMap[m.RealmID] = true
		}

		pageParams, err := pagination.FromRequest(r)
		if err != nil {
			controller.BadRequest(w, r, c.h)
			return
		}

		q := r.FormValue(QueryKeySearch)

		realms, paginator, err := c.db.ListRealms(pageParams, database.WithRealmSearch(q))
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		m.Title("Realms - System Admin")
		m["realms"] = realms
		m["memberships"] = membershipsMap
		m["query"] = q
		m["paginator"] = paginator
		c.h.RenderHTML(w, "admin/realms/index", m)
	})
}

func (c *Controller) HandleRealmsCreate() http.Handler {
	type FormData struct {
		Name                    string `form:"name"`
		RegionCode              string `form:"regionCode"`
		UseRealmCertificateKey  bool   `form:"useRealmCertificateKey"`
		CertificateIssuer       string `form:"certificateIssuer"`
		CertificateAudience     string `form:"certificateAudience"`
		CanUseSystemSMSConfig   bool   `form:"can_use_system_sms_config"`
		CanUseSystemEmailConfig bool   `form:"can_use_system_email_config"`
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

		emailConfig, err := c.db.SystemEmailConfig()
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var realm database.Realm
			realm.UseRealmCertificateKey = true
			c.renderNewRealm(ctx, w, &realm, smsConfig, emailConfig)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			var realm database.Realm
			realm.UseRealmCertificateKey = true
			flash.Error("Failed to process form: %v", err)
			c.renderNewRealm(ctx, w, &realm, smsConfig, emailConfig)
			return
		}

		realm := database.NewRealmWithDefaults(form.Name)
		realm.RegionCode = form.RegionCode
		realm.UseRealmCertificateKey = form.UseRealmCertificateKey
		realm.CertificateIssuer = form.CertificateIssuer
		realm.CertificateAudience = form.CertificateAudience
		realm.CanUseSystemSMSConfig = form.CanUseSystemSMSConfig
		realm.CanUseSystemEmailConfig = form.CanUseSystemEmailConfig
		if err := c.db.SaveRealm(realm, currentUser); err != nil {
			if database.IsValidationError(err) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderNewRealm(ctx, w, realm, smsConfig, emailConfig)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}
		flash.Alert("Created realm %q", realm.Name)

		// Make the current user an admin of the realm they just created.
		if err := currentUser.AddToRealm(c.db, realm, rbac.LegacyRealmAdmin, currentUser); err != nil {
			flash.Error("Failed to add you as an admin to the realm: %v", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderNewRealm(ctx, w, realm, smsConfig, emailConfig)
			return
		}
		flash.Alert("Added you as an administrator of %q", realm.Name)

		if realm.UseRealmCertificateKey {
			// If we are using realm specific keys - we need to create the first one.
			keyID, err := realm.CreateSigningKeyVersion(ctx, c.db, currentUser)
			if err != nil {
				flash.Error("Failed to create signing keys for realm. This can be done from the realm's admin screens.")
				http.Redirect(w, r, "/admin/realms", http.StatusSeeOther)
				return
			}
			flash.Alert("Created initial signing key %q", keyID)
		}

		http.Redirect(w, r, fmt.Sprintf("/admin/realms/%d/edit", realm.ID), http.StatusSeeOther)
	})
}

func (c *Controller) renderNewRealm(ctx context.Context, w http.ResponseWriter,
	realm *database.Realm, smsConfig *database.SMSConfig, emailConfig *database.EmailConfig) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("New Realm - System Admin")
	m["realm"] = realm
	m["systemSMSConfig"] = smsConfig
	m["systemEmailConfig"] = emailConfig
	m["supportsPerRealmSigning"] = c.db.SupportsPerRealmSigning()
	c.h.RenderHTML(w, "admin/realms/new", m)
}

func (c *Controller) HandleRealmsUpdate() http.Handler {
	type FormData struct {
		CanUseSystemSMSConfig   bool `form:"can_use_system_sms_config"`
		CanUseSystemEmailConfig bool `form:"can_use_system_email_config"`
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

		membership, err := currentUser.FindMembership(c.db, realm.ID)
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}

		smsConfig, err := c.db.SystemSMSConfig()
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}

		emailConfig, err := c.db.SystemEmailConfig()
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}

		var quotaLimit, quotaRemaining uint64
		if realm.AbusePreventionEnabled {
			key, err := realm.QuotaKey(c.config.RateLimit.HMACKey)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			quotaLimit, quotaRemaining, err = c.limiter.Get(ctx, key)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			c.renderEditRealm(ctx, w, realm, membership, smsConfig, emailConfig, quotaLimit, quotaRemaining)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderEditRealm(ctx, w, realm, membership, smsConfig, emailConfig, quotaLimit, quotaRemaining)
			return
		}

		realm.CanUseSystemSMSConfig = form.CanUseSystemSMSConfig
		realm.CanUseSystemEmailConfig = form.CanUseSystemEmailConfig
		if err := c.db.SaveRealm(realm, currentUser); err != nil {
			if database.IsValidationError(err) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderEditRealm(ctx, w, realm, membership, smsConfig, emailConfig, quotaLimit, quotaRemaining)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		flash.Alert("Successfully updated realm %q", realm.Name)
		http.Redirect(w, r, fmt.Sprintf("/admin/realms/%d/edit", realm.ID), http.StatusSeeOther)
	})
}

func (c *Controller) renderEditRealm(ctx context.Context, w http.ResponseWriter,
	realm *database.Realm, membership *database.Membership, smsConfig *database.SMSConfig, emailConfig *database.EmailConfig,
	quotaLimit, quotaRemaining uint64) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Realm: %s - System Admin", realm.Name)
	m["realm"] = realm
	m["membership"] = membership
	m["systemSMSConfig"] = smsConfig
	m["systemEmailConfig"] = emailConfig
	m["supportsPerRealmSigning"] = c.db.SupportsPerRealmSigning()
	m["quotaLimit"] = quotaLimit
	m["quotaRemaining"] = quotaRemaining
	c.h.RenderHTML(w, "admin/realms/edit", m)
}

func (c *Controller) HandleRealmsAdd() http.Handler {
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

		realm, err := c.db.FindRealm(vars["realm_id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		user, err := c.db.FindUser(vars["user_id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Add the user to the realm.
		if err := user.AddToRealm(c.db, realm, rbac.LegacyRealmAdmin, currentUser); err != nil {
			flash.Error("Failed to add %s to %s: %s", user.Name, realm.Name, err)
			controller.Back(w, r, c.h)
			return
		}

		flash.Alert("Successfully added %q to %q", user.Name, realm.Name)
		controller.Back(w, r, c.h)
		return
	})
}

func (c *Controller) HandleRealmsRemove() http.Handler {
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

		realm, err := c.db.FindRealm(vars["realm_id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		user, err := c.db.FindUser(vars["user_id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Delete the user from the realm.
		if err := user.DeleteFromRealm(c.db, realm, currentUser); err != nil {
			flash.Error("Failed to add %s to %s: %s", user.Name, realm.Name, err)
			controller.Back(w, r, c.h)
			return
		}

		// Clear realm selection from session.
		controller.ClearSessionRealm(session)

		flash.Alert("Successfully removed %q from %q", user.Name, realm.Name)
		controller.Back(w, r, c.h)
		return
	})
}
