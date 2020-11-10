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

// Package verifyapi implements the exchange of the verification code
// (short term token) for a long term token that can be used to get a
// verification certification to send to the key server.
//
// This is steps 4/5 as specified here:
// https://developers.google.com/android/exposure-notifications/verification-system#flow-diagram
package verifyapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/dgrijalva/jwt-go"
)

func (c *Controller) HandleVerify() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := observability.WithBuildInfo(r.Context())

		logger := logging.FromContext(ctx).Named("verifyapi.HandleVerify")

		var blame = observability.BlameNone
		var result = observability.ResultOK()

		defer observability.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &blame)

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			controller.MissingAuthorizedApp(w, r, c.h)
			blame = observability.BlameClient
			result = observability.ResultError("MISSING_AUTHORIZED_APP")
			return
		}

		ctx = observability.WithRealmID(ctx, authApp.RealmID)

		var request api.VerifyCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			logger.Errorw("bad request", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
			blame = observability.BlameClient
			result = observability.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
			return
		}

		// Get the signer based on Key configuration.
		signer, err := c.kms.NewSigner(ctx, c.config.TokenSigning.ActiveKey())
		if err != nil {
			logger.Errorw("failed to get signer", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
			blame = observability.BlameServer
			result = observability.ResultError("FAILED_TO_GET_SIGNER")
			return
		}

		// Process and validate the requested acceptable test types.
		acceptTypes, err := request.GetAcceptedTestTypes()
		if err != nil {
			logger.Errorf("invalid accept test types", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInvalidTestType))
			blame = observability.BlameClient
			result = observability.ResultError("INVALID_ACCEPT_TEST_TYPES")
			return
		}

		// Exchange the short term verification code for a long term verification token.
		// The token can be used to sign TEKs later.
		verificationToken, err := c.db.VerifyCodeAndIssueToken(authApp.RealmID, request.VerificationCode, acceptTypes, c.config.VerificationTokenDuration)
		if err != nil {
			blame = observability.BlameClient
			switch {
			case errors.Is(err, database.ErrVerificationCodeExpired):
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code expired").WithCode(api.ErrVerifyCodeExpired))
				result = observability.ResultError("VERIFICATION_CODE_EXPIRED")
			case errors.Is(err, database.ErrVerificationCodeUsed):
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code invalid").WithCode(api.ErrVerifyCodeInvalid))
				result = observability.ResultError("VERIFICATION_CODE_INVALID")
			case errors.Is(err, database.ErrVerificationCodeNotFound):
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code invalid").WithCode(api.ErrVerifyCodeInvalid))
				result = observability.ResultError("VERIFICATION_CODE_NOT_FOUND")
			case errors.Is(err, database.ErrUnsupportedTestType):
				c.h.RenderJSON(w, http.StatusPreconditionFailed, api.Errorf("verification code has unsupported test type").WithCode(api.ErrUnsupportedTestType))
				result = observability.ResultError("VERIFICATION_CODE_UNSUPPORTED_TEST_TYPE")
			default:
				logger.Errorw("failed to issue verification token", "error", err)
				c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
				result = observability.ResultError("UNKNOWN_ERROR")
			}
			return
		}

		subject := verificationToken.Subject()
		now := time.Now().UTC()
		claims := &jwt.StandardClaims{
			Audience:  c.config.TokenSigning.TokenIssuer,
			ExpiresAt: now.Add(c.config.VerificationTokenDuration).Unix(),
			Id:        verificationToken.TokenID,
			IssuedAt:  now.Unix(),
			Issuer:    c.config.TokenSigning.TokenIssuer,
			Subject:   subject.String(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		token.Header[verifyapi.KeyIDHeader] = c.config.TokenSigning.ActiveKeyID()
		signedJWT, err := jwthelper.SignJWT(token, signer)
		if err != nil {
			logger.Errorw("failed to sign token", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInternal))
			blame = observability.BlameServer
			result = observability.ResultError("FAILED_TO_SIGN_TOKEN")
			return
		}

		c.h.RenderJSON(w, http.StatusOK, api.VerifyCodeResponse{
			TestType:          verificationToken.TestType,
			SymptomDate:       verificationToken.FormatSymptomDate(),
			TestDate:          verificationToken.FormatTestDate(),
			VerificationToken: signedJWT,
		})
	})
}
