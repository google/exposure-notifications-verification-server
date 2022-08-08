// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

// HandleIndex renders the html to show a list of recent codes and form to query code status.
func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

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

		var code database.VerificationCode
		if err := c.renderStatus(ctx, w, currentRealm, currentUser, &code); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}
	})
}

func (c *Controller) renderStatus(
	ctx context.Context,
	w http.ResponseWriter,
	realm *database.Realm,
	user *database.User,
	code *database.VerificationCode,
) error {
	recentCodes, err := realm.ListRecentCodes(c.db, user)
	if err != nil {
		return err
	}

	m := controller.TemplateMapFromContext(ctx)
	m.Title("Verification code statuses")
	m["code"] = code
	m["recentCodes"] = recentCodes
	c.h.RenderHTML(w, "codes/status", m)
	return nil
}
