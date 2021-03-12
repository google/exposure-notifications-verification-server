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

var (
	allowedRedirects = map[string]struct{}{
		"login/register-phone": struct{}{},
	}
)

func redirectAllowed(r string) bool {
	_, ok := allowedRedirects[r]
	return ok
}

func (c *Controller) HandleReauth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// No session redirect for reauth

		if r := r.FormValue("redir"); redirectAllowed(r) {
			m := controller.TemplateMapFromContext(ctx)
			m["loginRedirect"] = r

			session := controller.SessionFromContext(ctx)
			flash := controller.Flash(session)
			flash.Alert("This operation is sensitive and requires recent authentication. Please sign-in again.")
			c.renderLogin(ctx, w)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
}
