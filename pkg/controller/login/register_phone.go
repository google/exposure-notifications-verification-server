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

// Package login defines the controller for the login page.
package login

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleRegisterPhone() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		// Mark that the user was prompted.
		controller.StoreSessionMFAPrompted(session, true)

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			// The user landed on this page but does not have a realm selected. They
			// could have navigated here from their profile page, or they could be a
			// system admin and were redirected here.
			c.renderRegisterPhone(ctx, w, nil)
			return
		}

		currentRealm := membership.Realm
		mode := currentRealm.EffectiveMFAMode(membership.CreatedAt)
		c.renderRegisterPhone(ctx, w, &mode)
	})
}

func (c *Controller) renderRegisterPhone(ctx context.Context, w http.ResponseWriter,
	mode *database.AuthRequirement) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Multi-factor authentication registration")
	m["mfaMode"] = mode
	m["firebase"] = c.config.Firebase
	c.h.RenderHTML(w, "login/register-phone", m)
}
