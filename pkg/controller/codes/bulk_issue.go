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

package codes

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

// HandleBulkIssue shows the page for bulk-issuing codes.
func (c *Controller) HandleBulkIssue() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.CodeBulkIssue) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		currentRealm := membership.Realm

		if !currentRealm.AllowBulkUpload {
			flash.Error("That feature is not enabled for your realm!")
			controller.Back(w, r, c.h)
			return
		}

		hasSMSConfig, err := currentRealm.HasSMSConfig(c.db)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}
		if !hasSMSConfig {
			t := controller.LocaleFromContext(ctx)
			if t == nil {
				controller.MissingLocale(w, r, c.h)
				return
			}
			flash.Error(t.Get("codes.bulk-issue.no-sms-provider"))
		}

		m := controller.TemplateMapFromContext(ctx)
		m["hasSMSConfig"] = hasSMSConfig
		m.Title("Bulk issue codes")
		c.h.RenderHTML(w, "codes/bulk-issue", m)
	})
}
