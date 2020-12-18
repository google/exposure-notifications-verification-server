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
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

// these UI elements are large, so half the default of 14
const recentCodesPageSize = 7

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.CodeRead) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		currentRealm := membership.Realm
		currentUser := membership.User

		pageParams, err := pagination.FromRequest(r)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}
		pageParams.Limit = recentCodesPageSize

		recentCodes, paginator, err := c.db.ListRecentCodes(currentRealm, currentUser, pageParams)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		var code database.VerificationCode
		c.renderStatus(ctx, w, &code, recentCodes, paginator)
	})
}

func (c *Controller) renderStatus(
	ctx context.Context,
	w http.ResponseWriter,
	code *database.VerificationCode,
	recentCodes []*database.VerificationCode,
	paginator *pagination.Paginator) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Verification code statuses")
	m["code"] = code
	m["recentCodes"] = recentCodes
	m["paginator"] = paginator
	c.h.RenderHTML(w, "codes/status", m)
	return
}
