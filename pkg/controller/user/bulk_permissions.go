// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func (c *Controller) HandleBulkPermissions(action database.BulkPermissionAction) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentMembership := controller.MembershipFromContext(ctx)
		if currentMembership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !currentMembership.Can(rbac.UserWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}
		currentUser := currentMembership.User
		currentRealm := currentMembership.Realm

		bulkPermission := &database.BulkPermission{
			RealmID: currentRealm.ID,
		}
		if err := bindBulkPermissions(r, currentMembership, bulkPermission, action); err != nil {
			flash.Error("Failed to process bulk permissions: %s", err.Error())
			controller.Back(w, r, c.h)
			return
		}

		// Bulk add permissions.
		if err := bulkPermission.Apply(c.db, currentUser); err != nil {
			if database.IsValidationError(err) {
				flash.Error("Failed to apply bulk permissions: %s", err.Error())
				controller.Back(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		flash.Alert("Successfully updated permissions")
		controller.Back(w, r, c.h)
		return
	})
}

func bindBulkPermissions(r *http.Request, currentMembership *database.Membership, bulkPermission *database.BulkPermission, action database.BulkPermissionAction) error {
	type FormData struct {
		UserIDs     []uint            `form:"user_id"`
		Permissions []rbac.Permission `form:"permission"`
	}

	var form FormData
	formErr := controller.BindForm(nil, r, &form)
	bulkPermission.Action = action
	bulkPermission.UserIDs = form.UserIDs

	var rbacErr error
	switch action {
	case database.BulkPermissionActionAdd:
		// For adding permissions, authorize against the membership.
		bulkPermission.Permissions, rbacErr = rbac.CompileAndAuthorize(currentMembership.Permissions, form.Permissions)
	case database.BulkPermissionActionRemove:
		// It's valid to remove permissions that the current user does not have (so
		// long as they have permission to manage users).
		for _, v := range form.Permissions {
			bulkPermission.Permissions = bulkPermission.Permissions | v
		}
	}

	if formErr != nil {
		return formErr
	}
	return rbacErr
}
