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
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func (c *Controller) checkCodeStatus(r *http.Request, uuid string) (*database.VerificationCode, int, *api.ErrorReturn) {
	ctx := r.Context()

	logger := logging.FromContext(ctx).Named("codes.CheckCodeStatus")

	authApp, membership, realm, err := c.getAuthorizationFromContext(ctx)
	if err != nil {
		return nil, http.StatusUnauthorized, api.Error(err)
	}

	code, err := realm.FindVerificationCodeByUUID(c.db, uuid)
	if err != nil {
		if database.IsNotFound(err) {
			logger.Debugw("code not found by UUID", "error", err)
			return nil, http.StatusNotFound,
				api.Errorf("code not found, it may have expired and been removed").WithCode(api.ErrVerifyCodeNotFound)
		}
		logger.Errorw("failed to check otp code status", "error", err)
		return nil, http.StatusInternalServerError,
			api.Errorf("failed to check otp code status, please try again").WithCode(api.ErrInternal)
	}

	logger.Debugw("Found code", "verificationCode", code)

	// The current user must have issued the code or be a realm admin.
	if membership != nil && !membership.Can(rbac.CodeRead) {
		return nil, http.StatusUnauthorized,
			api.Errorf("user does not have permission to check code statuses").WithCode(api.ErrVerifyCodeUserUnauth)
	}

	// The current app must have issued the code or be a realm admin.
	if authApp != nil && !(code.IssuingAppID == authApp.ID || authApp.IsAdminType()) {
		return nil, http.StatusUnauthorized,
			api.Errorf("API key does not match issuer").WithCode(api.ErrVerifyCodeUserUnauth)
	}
	return code, 0, nil
}

// getAuthorizationFromContext pulls the authorization from the context. If an
// API key is provided, it's used to lookup the realm. If a membership exists,
// it's used to provide the realm.
func (c *Controller) getAuthorizationFromContext(ctx context.Context) (*database.AuthorizedApp, *database.Membership, *database.Realm, error) {
	authorizedApp := controller.AuthorizedAppFromContext(ctx)
	if authorizedApp != nil {
		realm, err := authorizedApp.Realm(c.db)
		if err != nil {
			return nil, nil, nil, err
		}
		return authorizedApp, nil, realm, nil
	}

	membership := controller.MembershipFromContext(ctx)
	if membership != nil {
		realm := membership.Realm
		return nil, membership, realm, nil
	}

	return nil, nil, nil, fmt.Errorf("unable to identify authorized requestor")
}
