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
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		pageParams, err := pagination.FromRequest(r)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		apps, paginator, err := realm.ListAuthorizedApps(c.db, pageParams)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderIndex(ctx, w, apps, paginator)
	})
}

// renderIndex renders the index page.
func (c *Controller) renderIndex(ctx context.Context, w http.ResponseWriter, apps []*database.AuthorizedApp, paginator *pagination.Paginator) {
	m := controller.TemplateMapFromContext(ctx)
	m["title"] = fmt.Sprintf("API keys - %s", m["title"])
	m["apps"] = apps
	m["paginator"] = paginator
	c.h.RenderHTML(w, "apikeys/index", m)
}
