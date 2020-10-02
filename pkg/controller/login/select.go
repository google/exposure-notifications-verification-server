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

package login

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleSelectRealm() http.Handler {
	type FormData struct {
		RealmID uint `form:"realm,required"`
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

		switch len(currentUser.Realms) {
		case 0:
			// If the user is a member of zero realms, it's possible they are an
			// admin. If so, redirect them to the admin page.
			if currentUser.Admin {
				http.Redirect(w, r, "/admin", http.StatusSeeOther)
				return
			}
		case 1:
			// If the user is only a member of one realm, set that and bypass selection.
			realm := currentUser.Realms[0]

			// The user is already logged in and the current realm matches the
			// expected realm - just redirect.
			if controller.RealmIDFromSession(session) == realm.ID {
				http.Redirect(w, r, "/home", http.StatusSeeOther)
				return
			}

			// Clear any flashes. It's possible that the user was redirected via a
			// "missing realm" because their session expired, but then we auto logged
			// them in and they are only a member of one realm. In that case, they'd
			// get an error that says "please select a realm" and a success message
			// that they successfully logged in.
			flash.Clear()

			controller.StoreSessionRealm(session, realm)
			http.Redirect(w, r, "/home", http.StatusSeeOther)
			return
		default:
			// Continue below
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			c.renderSelect(ctx, w, currentUser.Realms)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error(err.Error())
			c.renderSelect(ctx, w, currentUser.Realms)
			return
		}

		realm := currentUser.GetRealm(form.RealmID)
		if realm == nil {
			flash.Error("Please select a realm to continue.")
			c.renderSelect(ctx, w, currentUser.Realms)
			return
		}

		// Verify that the user has access to the realm.
		if !currentUser.CanViewRealm(realm.ID) {
			flash.Error("Invalid realm selection.")
			c.renderSelect(ctx, w, currentUser.Realms)
			return
		}

		controller.StoreSessionRealm(session, realm)
		http.Redirect(w, r, "/home", http.StatusSeeOther)
	})
}

// renderSelect renders the realm selection page.
func (c *Controller) renderSelect(ctx context.Context, w http.ResponseWriter, realms []*database.Realm) {
	m := controller.TemplateMapFromContext(ctx)
	m["realms"] = realms
	c.h.RenderHTML(w, "login/select-realm", m)
}
