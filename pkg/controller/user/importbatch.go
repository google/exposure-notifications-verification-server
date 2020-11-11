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
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/hashicorp/go-multierror"
)

func (c *Controller) HandleImportBatch() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("user.HandleImportBatch")

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		var request api.UserBatchRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			logger.Errorw("Error decoding request", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		newUsers := make([]*api.BatchUser, 0, len(request.Users))

		var batchErr *multierror.Error
		for _, batchUser := range request.Users {
			// See if the user already exists by email - they may be a member of another
			// realm.
			user, err := c.db.FindUserByEmail(batchUser.Email)
			if err != nil {
				if !database.IsNotFound(err) {
					logger.Errorw("Error finding user", "error", err)
					batchErr = multierror.Append(batchErr, err)
					continue
				}

				user = new(database.User)
				user.Email = batchUser.Email
				user.Name = batchUser.Name
			}
			user.Realms = append(user.Realms, realm)

			// Save the user in the database.
			if err := c.db.SaveUser(user, currentUser); err != nil {
				logger.Errorw("Error saving user", "error", err)
				batchErr = multierror.Append(batchErr, err)
				continue
			}

			// Create the invitation email composer.
			inviteComposer, err := controller.SendInviteEmailFunc(ctx, c.db, c.h, user.Email)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			// Create the user in the auth provider. This could be a noop depending on
			// the auth provider.
			created, err := c.authProvider.CreateUser(ctx, user.Name, user.Email, "", request.SendInvites, inviteComposer)
			if err != nil {
				logger.Errorw("failed to import user", "user", user.Email, "error", err)
				batchErr = multierror.Append(batchErr, err)
				continue
			}

			if created {
				newUsers = append(newUsers, &batchUser)
			}
		}

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
