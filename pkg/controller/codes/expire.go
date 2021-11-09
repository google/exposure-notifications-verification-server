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
	"errors"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
)

// HandleExpireAPI handles the verification code expiry API via JSON
func (c *Controller) HandleExpireAPI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get realm from context.
		authorizedApp := controller.AuthorizedAppFromContext(ctx)
		if authorizedApp == nil {
			controller.Unauthorized(w, r, c.h)
			return
		}
		realm, err := authorizedApp.Realm(c.db)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		var request api.ExpireCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		// Retrieve once to check permissions.
		_, errCode, apiErr := c.checkCodeStatus(r, request.UUID)
		if apiErr != nil {
			c.h.RenderJSON(w, errCode, apiErr)
			return
		}

		code, err := realm.ExpireCode(c.db, request.UUID, authorizedApp)
		if err != nil {
			if database.IsNotFound(err) {
				controller.NotFound(w, r, c.h)
				return
			}

			if errors.Is(err, database.ErrCodeAlreadyClaimed) {
				controller.BadRequest(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK,
			&api.ExpireCodeResponse{
				ExpiresAtTimestamp:     code.ExpiresAt.UTC().Unix(),
				LongExpiresAtTimestamp: code.LongExpiresAt.UTC().Unix(),
			})
	})
}

// HandleExpirePage handles the verification code expire PATCH request made from
// the show page.
func (c *Controller) HandleExpirePage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

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
		if !membership.Can(rbac.CodeExpire) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		currentRealm := membership.Realm
		currentUser := membership.User

		// Retrieve once to check permissions.
		code, _, apiErr := c.checkCodeStatus(r, vars["uuid"])
		if apiErr != nil {
			flash.Error("Failed to expire code: %v.", apiErr.Error)
			if err := c.renderStatus(ctx, w, currentRealm, currentUser, code); err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
			return
		}

		expiredCode, err := currentRealm.ExpireCode(c.db, vars["uuid"], currentUser)
		if err != nil {
			flash.Error("Failed to process form: %v.", err)
			expiredCode = code
		} else {
			flash.Alert("Expired code.")
		}

		retCode, err := c.responseCode(ctx, expiredCode)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderShow(ctx, w, retCode)
	})
}
