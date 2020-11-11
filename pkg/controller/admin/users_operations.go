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

	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
)

// HandleSystemAdminCreate creates a new system admin.
func (c *Controller) HandleSystemAdminCreate() http.Handler {
	type FormData struct {
		Email string `form:"email"`
		Name  string `form:"name"`
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

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var user database.User
			c.renderNewUser(ctx, w, &user)
			return
		}

		var form FormData
		err := controller.BindForm(w, r, &form)
		if err != nil {
			user := &database.User{
				Email: form.Email,
				Name:  form.Name,
			}

			flash.Error("Failed to process form: %v", err)
			c.renderNewUser(ctx, w, user)
			return
		}

		// See if the user already exists and use that record.
		user, err := c.db.FindUserByEmail(form.Email)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}

			// User does not exist, create a new one.
			user = &database.User{
				Name:  form.Email,
				Email: form.Email,
			}
		}

		user.SystemAdmin = true
		if err := c.db.SaveUser(user, currentUser); err != nil {
			flash.Error("Failed to create user: %v", err)
			c.renderNewUser(ctx, w, user)
			return
		}

		inviteComposer, err := c.inviteComposer(ctx, form.Email)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if _, err := c.authProvider.CreateUser(ctx, form.Email, form.Email, "", true, inviteComposer); err != nil {
			flash.Alert("Failed to create user: %v", err)
			c.renderNewUser(ctx, w, user)
		}

		flash.Alert("Successfully created system admin '%v'", user.Name)
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	})
}

func (c *Controller) renderNewUser(ctx context.Context, w http.ResponseWriter, user *database.User) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("New User - System Admin")
	m["user"] = user
	c.h.RenderHTML(w, "admin/users/new", m)
}

// HandleSystemAdminRevoke removes admin from a system admin.
func (c *Controller) HandleSystemAdminRevoke() http.Handler {
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

		user, err := c.db.FindUser(vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		if user.ID == currentUser.ID {
			flash.Error("Cannot remove yourself!")
			controller.Back(w, r, c.h)
			return
		}

		user.SystemAdmin = false
		if err := c.db.SaveUser(user, currentUser); err != nil {
			flash.Error("Failed to remove system admin: %v", err)
			controller.Back(w, r, c.h)
			return
		}

		flash.Alert("Successfully removed %v as a system admin", user.Email)
		controller.Back(w, r, c.h)
	})
}

// inviteComposer returns an email composer function that invites a user using
// the system email config.
func (c *Controller) inviteComposer(ctx context.Context, email string) (auth.InviteUserEmailFunc, error) {
	// Figure out email sending - since this is a system admin, only the system
	// credentials can be used.
	emailConfig, err := c.db.SystemEmailConfig()
	if err != nil {
		if database.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	emailer, err := emailConfig.Provider()
	if err != nil {
		return nil, err
	}

	// Return a function that does the actual sending.
	return func(ctx context.Context, inviteLink string) error {
		// Render the message invitation.
		message, err := c.h.RenderEmail("email/invite", map[string]interface{}{
			"ToEmail":    email,
			"FromEmail":  emailer.From(),
			"InviteLink": inviteLink,
			"RealmName":  "System Admin",
		})
		if err != nil {
			return fmt.Errorf("failed to render invite template: %w", err)
		}

		// Send the message.
		if err := emailer.SendEmail(ctx, email, message); err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
		return nil
	}, nil
}
