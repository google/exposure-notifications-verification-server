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

package user

import (
	"context"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
)

func (c *Controller) HandleShow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		// Pull the user from the id.
		user, err := realm.FindUser(c.db, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// TODO(sethvargo): support configurable time ranges
		now := time.Now().UTC()
		stats, err := user.Stats(c.db, realm.ID, now.Add(-7*24*time.Hour), now)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderShow(ctx, w, user, stats)
	})
}

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, user *database.User, stats []*database.UserStats) {
	m := controller.TemplateMapFromContext(ctx)
	m["user"] = user
	m["stats"] = stats
	c.h.RenderHTML(w, "users/show", m)
}
