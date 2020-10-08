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

// Package codestatus defines a web controller for the code status page of the verification
// server. This view allows users to view the status of previously-issued OTP codes.
package codestatus

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/gorilla/mux"
)

func (c *Controller) HandleExpireAPI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request api.ExpireCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		// Retrieve once to check permissions.
		_, errCode, apiErr := c.CheckCodeStatus(r, request.UUID)
		if apiErr != nil {
			c.h.RenderJSON(w, errCode, apiErr)
			return
		}

		code, err := c.db.ExpireCode(request.UUID)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK,
			&api.ExpireCodeResponse{
				ExpiresAtTimestamp:     code.ExpiresAt.UTC().Unix(),
				LongExpiresAtTimestamp: code.ExpiresAt.UTC().Unix(),
			})
	})
}

func (c *Controller) HandleExpirePage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

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

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		// Retrieve once to check permissions.
		code, _, apiErr := c.CheckCodeStatus(r, vars["uuid"])
		if apiErr != nil {
			flash.Error("Failed to expire code: %v.", apiErr.Error)
			if err := c.renderStatus(ctx, w, realm, currentUser, code); err != nil {
				controller.InternalError(w, r, c.h, err)
			}
			return
		}

		expiredCode, err := c.db.ExpireCode(vars["uuid"])
		if err != nil {
			flash.Error("Failed to process form: %v.", err)
			expiredCode = code
		} else {
			flash.Alert("Expired code.")
		}

		retCode := Code{}
		c.responseCode(ctx, r, expiredCode, &retCode)
		c.renderShow(ctx, w, retCode)
	})
}
