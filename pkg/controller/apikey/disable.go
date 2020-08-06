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

package apikey

import (
	"net/http"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/gorilla/mux"
)

func (c *Controller) HandleDisable() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		id, err := strconv.ParseUint(vars["id"], 10, 64)
		if err != nil {
			flash.Error("Invalid authorized app")
			http.Redirect(w, r, "/apikeys", http.StatusSeeOther)
			return
		}

		if err := realm.DisableAuthorizedApp(uint(id)); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		flash.Alert("Successfully disabled API key")
		http.Redirect(w, r, "/apikeys", http.StatusSeeOther)
	})
}
