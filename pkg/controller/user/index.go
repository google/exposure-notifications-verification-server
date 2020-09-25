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
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// pageSize is const for now, could provide options in UX
const pageSize = 25

type PageLabel struct {
	Offset int
	Label  string
}

type Pages struct {
	Previous int
	Current  int
	Next     int
	Offsets  []PageLabel
	Footer   string
}

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		offset := 0
		var err error
		if qvar := r.FormValue("offset"); qvar != "" {
			offset, err = strconv.Atoi(qvar)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
		}

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

		emailPrefix := r.FormValue("search")
		users, err := realm.ListUsers(c.db, offset, pageSize, emailPrefix)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		var pages *Pages
		if count > pageSize {
			pages = populatePageStrip(offset, count)
		}

		c.renderIndex(ctx, w, users, pages, emailPrefix)
	})
}

func populatePageStrip(offset, count int) *Pages {
	pages := &Pages{
		Current:  offset,
		Offsets:  []PageLabel{},
		Previous: -1,
		Next:     -1,
		Footer:   fmt.Sprintf("%d-%d of %d", offset, offset+pageSize, count),
	}

	// Calc start and end for paging as +/- 5 pages from current
	iStart := offset - pageSize*5
	if iStart <= 0 {
		iStart = 0
	}
	iEnd := iStart + pageSize*11
	if iEnd > count-pageSize {
		iEnd = count - pageSize
	}

	// Page number series
	for i := iStart; i <= iEnd; i += pageSize {
		pl := PageLabel{
			Offset: i,
			Label:  strconv.Itoa((i / pageSize) + 1),
		}
		pages.Offsets = append(pages.Offsets, pl)
	}

	// Fast forward halfway to the start
	if iStart > pageSize*5 {
		pages.Offsets[0] = PageLabel{
			Offset: iStart/2 - (iStart / 2 % pageSize),
			Label:  "<<",
		}
	}

	// Fast forward halfway to the end
	if iEnd < count-pageSize*5 {
		halfway := (count - iEnd) / 2
		pages.Offsets[len(pages.Offsets)-1] = PageLabel{
			Offset: halfway + iEnd - halfway%pageSize,
			Label:  ">>",
		}
	}

	// Previous button data
	if offset != 0 {
		pages.Previous = offset - pageSize
		if pages.Previous < 0 {
			pages.Previous = 0
		}
	}
	// Next button data
	if offset != count-pageSize {
		pages.Next = offset + pageSize
		if pages.Next > count-pageSize {
			pages.Next = count - pageSize
		}
	}
	return pages
}

func (c *Controller) renderIndex(
	ctx context.Context,
	w http.ResponseWriter,
	users []*database.User,
	pages *Pages,
	emailPrefix string) {
	m := controller.TemplateMapFromContext(ctx)
	m["search"] = emailPrefix
	m["users"] = users
	m["pages"] = pages
	c.h.RenderHTML(w, "users/index", m)
}
