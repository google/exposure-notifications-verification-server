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

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

func (c *Controller) HandleDelete() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		flash := flash.FromContext(w, r)

		// Get the session
		session, err := c.sessions.Get(r, "session")
		if err != nil {
			// TODO(sethvargo): have a 500 page we can render
			c.logger.Errorw("failed to get session", "error", err)
			flash.Error("internal server error")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Set MaxAge to -1 to expire the session
		session.Options.MaxAge = -1

		// Save the session
		if err := session.Save(r, w); err != nil {
			// TODO(sethvargo): have a 500 page we can render
			c.logger.Errorw("failed to save session", "error", err)
			flash.Error("internal server error")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		m["firebase"] = c.config.Firebase
		m["flash"] = flash
		c.h.RenderHTML(w, "signout", m)
	})
}
