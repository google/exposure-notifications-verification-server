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

package issueapi

import (
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleCheckCodeStatus() http.Handler {
	logger := c.logger.Named("issueapi.CheckCodeStatus")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var request api.CheckCodeStatusRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		authApp, user, err := c.getAuthorizationFromContext(r)
		if err != nil {
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			return
		}

		var realm *database.Realm
		if authApp != nil {
			realm, err = authApp.GetRealm(c.db)
			if err != nil {
				c.h.RenderJSON(w, http.StatusUnauthorized, nil)
				return
			}
		} else {
			// if it's a user logged in, we can pull realm from the context.
			realm = controller.RealmFromContext(ctx)
			if realm == nil {
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("missing realm"))
				return
			}
		}

		code, err := c.db.FindVerificationCodeByID(request.ID)
		if err != nil {
			logger.Errorw("failed to check otp code status", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to check otp code status, please try again"))
			return
		}

		if code.IssuingUser.Email != user.Email {
			logger.Errorw("failed to check otp code status", "error", "user email does not match issuing user")
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Errorf("failed to check otp code status: user does not match issuing user"))
			return
		}

		if code.IsExpired() {
			logger.Errorw("failed to check otp code status", "error", "code exists but is expired")
			c.h.RenderJSON(w, http.StatusNotFound, api.Errorf("failed to check otp code status"))
			return
		}

		if code.RealmID != realm.ID {
			logger.Errorw("failed to check otp code status", "error", "realmID does not match")
			c.h.RenderJSON(w, http.StatusNotFound, api.Errorf("failed to check otp code status"))
			return
		}

		if code.UUID != request.ID {
			logger.Errorw("failed to check otp code status", "error", "code not found")
			c.h.RenderJSON(w, http.StatusNotFound, api.Errorf("failed to check otp code status"))
			return
		}

		c.h.RenderJSON(w, http.StatusOK,
			&api.CheckCodeStatusResponse{
				Claimed:            code.Claimed,
				ExpiresAt:          code.ExpiresAt.Format(time.RFC1123),
				ExpiresAtTimestamp: code.ExpiresAt.Unix(),
			})
	})
}
