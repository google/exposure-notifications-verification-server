// Copyright 2021 the Exposure Notifications Verification Server authors
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

package alerts

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func (c *Controller) HandleCreate() http.Handler {
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

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var realmAdminPhone database.RealmAdminPhone
			c.renderNew(ctx, w, &realmAdminPhone)
			return
		}

		var realmAdminPhone database.RealmAdminPhone
		if err := bindCreateForm(r, &realmAdminPhone); err != nil {
			realmAdminPhone.AddError("", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderNew(ctx, w, &realmAdminPhone)
			return
		}

		err := currentRealm.CreateRealmAdminPhone(c.db, &realmAdminPhone, currentUser)
		if err != nil {
			if database.IsValidationError(err) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderNew(ctx, w, &realmAdminPhone)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		flash.Alert("Successfully created realm admin phone number for %q", realmAdminPhone.Name)
		http.Redirect(w, r, "/realm/alerts", http.StatusSeeOther)
	})
}

func bindCreateForm(r *http.Request, app *database.RealmAdminPhone) error {
	type FormData struct {
		Name        string `form:"name"`
		PhoneNumber string `form:"phoneNumber"`
	}

	var form FormData
	err := controller.BindForm(nil, r, &form)
	app.Name = form.Name
	app.PhoneNumber = form.PhoneNumber
	return err
}

// renderNew renders the edit page.
func (c *Controller) renderNew(ctx context.Context, w http.ResponseWriter, rap *database.RealmAdminPhone) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("New realm admin phone number")
	m["rap"] = rap
	c.h.RenderHTML(w, "alerts/new", m)
}
