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

package user

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/hashicorp/go-multierror"
)

func (c *Controller) HandleImportBatch() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("user.HandleImportBatch")

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.UserWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		var request api.UserBatchRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			logger.Errorw("error decoding request", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		realmMemberships, err := membership.Realm.MembershipPermissionMap(c.db)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		newUsers, batchErr := c.importUsers(ctx, membership.Realm, realmMemberships, membership.User, request.Users, request.SendInvites)

		response := &api.UserBatchResponse{
			NewUsers: newUsers,
		}

		if err := batchErr.ErrorOrNil(); err != nil {
			response.Error = err.Error()
			response.ErrorCode = fmt.Sprintf("%d", http.StatusInternalServerError)

			if len(newUsers) == 0 { // We return partial success if any succeeded.
				c.h.RenderJSON(w, http.StatusInternalServerError, response)
				return
			}
		}

		c.h.RenderJSON(w, http.StatusOK, response)
	})
}

func (c *Controller) importUsers(ctx context.Context,
	realm *database.Realm, realmMemberships map[uint]rbac.Permission, actor database.Auditable,
	users []api.BatchUser, sendInvites bool) ([]*api.BatchUser, *multierror.Error) {
	logger := logging.FromContext(ctx).Named("user.importUsers")

	addedUsers := make([]*api.BatchUser, 0, len(users))
	var batchErr *multierror.Error

	for i, batchUser := range users {
		// See if the user already exists by email - they may be a member of another
		// realm.
		user, err := c.db.FindUserByEmail(batchUser.Email)
		if err != nil {
			if !database.IsNotFound(err) {
				logger.Errorw("error finding user", "error", err)
				batchErr = multierror.Append(batchErr, err)
				continue
			}

			user = new(database.User)
			user.Email = batchUser.Email
			user.Name = batchUser.Name

			if err := c.db.SaveUser(user, actor); err != nil {
				logger.Errorw("error saving user", "error", err)
				batchErr = multierror.Append(batchErr, err)
				continue
			}
		}

		// Create the user's membership in the realm.
		var permission rbac.Permission
		if existing, ok := realmMemberships[user.ID]; ok {
			permission = existing
		}
		permission = permission | rbac.CodeIssue | rbac.CodeBulkIssue | rbac.CodeRead | rbac.CodeExpire
		if err := user.AddToRealm(c.db, realm, permission, actor); err != nil {
			logger.Errorw("failed to add user to realm",
				"user_id", user.ID, "realm_id", realm.ID, "error", err)
			batchErr = multierror.Append(batchErr, err)
			continue
		}

		// Create the invitation email composer.
		inviteComposer, err := controller.SendInviteEmailFunc(ctx, c.db, c.h, user.Email, realm)
		if err != nil {
			batchErr = multierror.Append(batchErr, err)
			continue
		}

		// Create the user in the auth provider. This could be a noop depending on
		// the auth provider.
		if _, err = c.authProvider.CreateUser(ctx, user.Name, user.Email, "", sendInvites, inviteComposer); err != nil {
			logger.Errorw("failed to import user", "user", user.Email, "error", err)
			batchErr = multierror.Append(batchErr, err)
			continue
		}

		addedUsers = append(addedUsers, &users[i])
	}
	return addedUsers, batchErr
}
