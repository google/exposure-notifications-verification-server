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
	"time"

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

		// Parse and decode form.
		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			c.logger.Errorf("error parsing form: %v", err)
			flash.Error("Failed to process login: %v", err)
			c.h.RenderJSON(w, http.StatusBadRequest, nil)
			return
		}

		ttl := c.config.SessionCookieDuration
		cookie, err := c.client.SessionCookie(ctx, form.IDToken, ttl)
		if err != nil {
			c.logger.Errorf("unable to create client session: %v", err)
			flash.Error("Failed to create session: %v", err)
			c.h.RenderJSON(w, http.StatusUnauthorized, nil)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    cookie,
			Path:     "/",
			Expires:  time.Now().Add(ttl),
			MaxAge:   int(ttl.Seconds()),
			Secure:   !c.config.DevMode,
			SameSite: http.SameSiteStrictMode,
		})
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}
