// Copyright 2021 Google LLC
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

package userreport

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			// TODO(mikehelmick) message this better.
			controller.Unauthorized(w, r, c.h)
			return
		}

		realm := controller.RealmFromContext(ctx)
		if !(realm.AllowsUserReport() && realm.AllowAdminUserReport) {
			controller.NotFound(w, r, c.h)
			return
		}

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		// TODO(mikehelmick) - error if nonce isn't valid (encoding + length)
		nonce := controller.NonceFromContext(ctx)

		m := controller.TemplateMapFromContext(ctx)

		// stash the nonce value in the session
		controller.StoreSessionNonce(session, nonce)
		controller.StoreSessionRegion(session, realm.RegionCode)
		m.Title("Request a verification code")
		m["realm"] = realm
		c.h.RenderHTML(w, "report/index", m)
	})
}
