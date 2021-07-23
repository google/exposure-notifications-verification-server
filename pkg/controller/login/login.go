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

// Package login defines the controller for the login page.
package login

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleLogin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// If there's no session, render the login page directly.
		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		// Check session idle timeout - we do this before checking if the overall
		// session is expired because they both have the save effect (session
		// revocation), but this check is less expensive.
		if t := controller.LastActivityFromSession(session); !t.IsZero() {
			// If it's been more than the TTL since we've seen this session,
			// expire it by creating a new empty session.
			if time.Since(t) > c.config.SessionIdleTimeout {
				flash.Error("Your session has expired due to inactivity.")
				controller.RedirectToLogout(w, r, c.h)
				return
			}
		}

		// Check upstream auth provider to see if the session is still valid.
		if err := c.authProvider.CheckRevoked(ctx, session); err != nil {
			if errors.Is(err, auth.ErrSessionMissing) {
				c.renderLogin(ctx, w)
				return
			}

			flash.Error("Your session has expired.")
			controller.RedirectToLogout(w, r, c.h)
			return
		}

		// If we got this far, the session is probably valid. Redirect to issue
		// page. However, the auth middleware will still run after the redirect.
		http.Redirect(w, r, "/login/post-authenticate", http.StatusSeeOther)
		return
	})
}

func (c *Controller) renderLogin(ctx context.Context, w http.ResponseWriter) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Login")
	m["firebase"] = c.config.Firebase
	c.h.RenderHTML(w, "login", m)
}
