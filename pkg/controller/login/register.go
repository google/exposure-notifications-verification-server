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
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleRegisterPhone() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}
		realm := controller.RealmFromContext(ctx)

		m := controller.TemplateMapFromContext(ctx)

		mode := realm.EffectiveMFAMode(currentUser)
		m["mfaMode"] = &mode

		if controller.MFAPromptedFromSession(session) {
			m["isPrompt"] = true
		} else {
			// Mark prompted so we only prompt once.
			controller.StoreSessionMFAPrompted(session, true)
		}

		m["firebase"] = c.config.Firebase
		c.h.RenderHTML(w, "login/register-phone", m)
	})
}
