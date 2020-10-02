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
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) CheckCodeStatus(r *http.Request, uuid string) (*database.VerificationCode, int, *api.ErrorReturn) {
	ctx := r.Context()
	logger := c.logger.Named("codestatus.CheckCodeStatus")
	authApp, user, err := c.getAuthorizationFromContext(r)
	if err != nil {
		return nil, http.StatusUnauthorized, api.Error(err)
	}

	var realm *database.Realm
	if authApp != nil {
		realm, err = authApp.Realm(c.db)
		if err != nil {
			logger.Errorw("internal error", "error", err)
			return nil, http.StatusInternalServerError, api.InternalError()
		}
	} else {
		// if it's a user logged in, we can pull realm from the context.
		realm = controller.RealmFromContext(ctx)
	}
	if realm == nil {
		return nil, http.StatusBadRequest, api.Errorf("missing realm")
	}

	code, err := c.db.FindVerificationCodeByUUID(uuid)
	if err != nil {
		logger.Errorw("failed to check otp code status", "error", err)
		return nil, http.StatusInternalServerError,
			api.Errorf("failed to check otp code status, please try again").WithCode(api.ErrInternal)
	}

	logger.Debugw("Found code", "verificationCode", code)

	if code.UUID == "" { // if no row is found, code will not be populated
		logger.Errorw("failed to check otp code status", "error", "code not found")
		return nil, http.StatusNotFound,
			api.Errorf("code does not exist or is expired and removed").WithCode(api.ErrVerifyCodeNotFound)
	}

	// The current user must have issued the code or be a realm admin.
	if user != nil && !(code.IssuingUserID == user.ID || user.CanAdminRealm(realm.ID)) {
		logger.Errorw("failed to check otp code status", "error", "user email does not match issuing user")
		return nil, http.StatusUnauthorized,
			api.Errorf("failed to check otp code status: user does not match issuing user").WithCode(api.ErrVerifyCodeUserUnauth)
	}

	// The current app must have issued the code or be a realm admin.
	if authApp != nil && !(code.IssuingAppID == authApp.ID || authApp.IsAdminType()) {
		logger.Errorw("failed to check otp code status", "error", "auth app does not match issuing app")
		return nil, http.StatusUnauthorized,
			api.Errorf("failed to check otp code status: auth app does not match issuing app").WithCode(api.ErrVerifyCodeUserUnauth)
	}

	if code.RealmID != realm.ID {
		logger.Errorw("failed to check otp code status", "error", "realmID does not match")
		return nil, http.StatusNotFound,
			api.Errorf("code does not exist or is expired and removed").WithCode(api.ErrVerifyCodeNotFound)
	}
	return code, 0, nil
}

func (c *Controller) getAuthorizationFromContext(r *http.Request) (*database.AuthorizedApp, *database.User, error) {
	ctx := r.Context()

	authorizedApp := controller.AuthorizedAppFromContext(ctx)
	currentUser := controller.UserFromContext(ctx)

	if authorizedApp == nil && currentUser == nil {
		return nil, nil, fmt.Errorf("unable to identify authorized requestor")
	}

	return authorizedApp, currentUser, nil
}
