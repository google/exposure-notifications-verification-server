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
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		realmUsers, err := realm.ListUsers(c.db)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		creationCounts1d := make(map[uint]uint64)
		creationCounts7d := make(map[uint]uint64)
		creationCounts30d := make(map[uint]uint64)
		for _, user := range realmUsers {
			userStatsSummary, err := c.db.GetUserStatsSummary(user, realm)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
			creationCounts1d[user.ID] = userStatsSummary.CodesIssued1d
			creationCounts7d[user.ID] = userStatsSummary.CodesIssued7d
			creationCounts30d[user.ID] = userStatsSummary.CodesIssued30d
		}

		m := controller.TemplateMapFromContext(ctx)

		m["codesGenerated1d"] = creationCounts1d
		m["codesGenerated7d"] = creationCounts7d
		m["codesGenerated30d"] = creationCounts30d
		m["realmUsers"] = realmUsers

		c.h.RenderHTML(w, "users/index", m)
	})
}
