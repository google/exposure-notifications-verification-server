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
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// pageSize is const for now, could provide options in UX
const pageSize = 25

type Pages struct {
	Previous int
	Current  int
	Next     int
	Offsets  []int
}

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		qvar := r.FormValue("offset")
		offset, _ := strconv.Atoi(qvar)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		count, err := realm.CountUsers(c.db)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		users, err := realm.ListUsers(c.db, offset, pageSize)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		var pages *Pages
		if count > pageSize {
			pages = &Pages{
				Current:  offset,
				Offsets:  []int{},
				Previous: -1,
				Next:     -1,
			}
			for i := 0; i < count; i += pageSize {
				pages.Offsets = append(pages.Offsets, i)
			}
			if offset != 0 {
				pages.Previous = offset - pageSize
				if pages.Previous < 0 {
					pages.Previous = 0
				}
			}
			if offset != count-pageSize {
				pages.Next = offset + pageSize
				if pages.Next > count-pageSize {
					pages.Next = count - pageSize
				}
			}
		}

		c.renderIndex(ctx, w, users, pages)
	})
}

func (c *Controller) renderIndex(ctx context.Context, w http.ResponseWriter, users []*database.User, pages *Pages) {
	m := controller.TemplateMapFromContext(ctx)
	m["users"] = users
	m["pages"] = pages
	c.h.RenderHTML(w, "users/index", m)
}
