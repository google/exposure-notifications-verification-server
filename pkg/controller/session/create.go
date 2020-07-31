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

package session

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

func (c *Controller) HandleCreate() http.Handler {
	type FormData struct {
		IDToken string `form:"idToken,required"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		flash := flash.FromContext(w, r)

		// Get the session
		session, err := c.sessions.Get(r, "session")
		if err != nil {
			c.logger.Errorw("failed to get session", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, nil)
			return
		}

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

		// Set the firebase cookie value in our session.
		session.Values["firebaseCookie"] = cookie

		// Save the session
		if err := session.Save(r, w); err != nil {
			c.logger.Errorw("failed to save session", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, nil)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}
