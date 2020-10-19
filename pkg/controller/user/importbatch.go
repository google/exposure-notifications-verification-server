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
	"errors"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/hashicorp/go-multierror"
)

func (c *Controller) HandleImportBatch() http.Handler {
	logger := c.logger.Named("user.HandleImportBatch")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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

			if created, err := user.CreateFirebaseUser(ctx, c.client); err != nil {
				logger.Errorw("Error creating firebase user", "error", err)
				batchErr = multierror.Append(batchErr, err)
				continue
			} else if created {
				newUsers = append(newUsers, &batchUser)

				if err := c.sendInvitation(ctx, user.Email); err != nil {
					batchErr = multierror.Append(batchErr, errors.New("send invitation failed"))
					continue
				}
			}

			if err := c.db.SaveUser(user, currentUser); err != nil {
				logger.Errorw("Error saving user", "error", err)
				batchErr = multierror.Append(batchErr, err)
				continue
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

func (c *Controller) sendInvitation(ctx context.Context, toEmail string) error {
	// Send email with emailer
	if c.emailer != nil {
		realmName := ""
		if realm := controller.RealmFromContext(ctx); realm != nil {
			realmName = realm.Name
		}

		from := c.emailer.From()
		message, err := controller.ComposeInviteEmail(ctx, c.h, c.client, toEmail, from, realmName)
		if err != nil {
			c.logger.Warnw("failed composing invitation", "error", err)
			return fmt.Errorf("failed composing invitation: %w", err)
		}
		if err := c.emailer.SendEmail(ctx, toEmail, message); err != nil {
			c.logger.Warnw("failed sending invitation", "error", err)
			return fmt.Errorf("failed sending invitation: %w", err)
		}

		return nil
	}

	// Fallback to Firebase

	if err := c.firebaseInternal.SendNewUserInvitation(ctx, toEmail); err != nil {
		c.logger.Warnw("failed sending invitation", "error", err)
		return fmt.Errorf("failed sending invitation: %w", err)
	}
	return nil
}
