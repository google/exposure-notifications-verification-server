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
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleCreateSession() http.Handler {
	type FormData struct {
		IDToken     string `form:"idToken,required"`
		FactorCount int    `form:"factorCount"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("session.HandleCreateSession")
		cacheTTL := 5 * time.Minute

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		// Parse and decode form.
		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		// Get the session cookie from firebase.
		ttl := c.config.SessionDuration
		cookie, err := c.client.SessionCookie(ctx, form.IDToken, ttl)
		if err != nil {
			flash.Error("Failed to create session: %v", err)
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			return
		}

		// Verify the cookie and extract email.
		email, err := middleware.EmailFromFirebaseCookie(ctx, c.client, cookie)
		if err != nil {
			logger.Debugw("failed to verify cookie and extract email")
			flash.Error("Failed to verify session for user: %v", email)
			c.h.RenderJSON(w, http.StatusUnauthorized, nil)
		}

		// Require user to exist for login session.
		var user database.User
		cacheKey := fmt.Sprintf("users:by_email:%s", email)
		if err := c.cacher.Fetch(ctx, cacheKey, &user, cacheTTL, func() (interface{}, error) {
			return c.db.FindUserByEmail(email)
		}); err != nil {
			if database.IsNotFound(err) {
				logger.Debugw("user does not exist")
				flash.Error("User does not exist: %v", email)
				c.h.RenderJSON(w, http.StatusUnauthorized, nil)
				return
			}

			logger.Errorw("failed to lookup user", "error", err)
			flash.Error("Failed to look up user: %v", email)
			c.h.RenderJSON(w, http.StatusUnauthorized, nil)
			return
		}

		// Set the firebase cookie value in our session.
		controller.StoreSessionFirebaseCookie(session, cookie)

		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}
