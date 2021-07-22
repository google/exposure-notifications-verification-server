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

package login

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleCreateSession() http.Handler {
	type FormData struct {
		IDToken string `form:"idToken,required"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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

		// Create the session cookie.
		if err := c.authProvider.StoreSession(ctx, session, &auth.SessionInfo{
			Data: map[string]interface{}{
				"id_token": form.IDToken,
			},
			TTL: c.config.SessionDuration,
		}); err != nil {
			flash.Error("Failed to create session: %v", err)
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			return
		}

		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}
