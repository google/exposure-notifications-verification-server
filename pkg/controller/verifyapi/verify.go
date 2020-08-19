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
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/google/exposure-notifications-verification-server/pkg/signer"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/dgrijalva/jwt-go"
	"go.uber.org/zap"
)

// Controller is a controller for the verification code verification API.
type Controller struct {
	config *config.APIServerConfig
	db     *database.Database
	h      *render.Renderer
	logger *zap.SugaredLogger
	signer signer.KeyManager
}

func New(ctx context.Context, config *config.APIServerConfig, db *database.Database, h *render.Renderer, signer signer.KeyManager) *Controller {
	logger := logging.FromContext(ctx)

	return &Controller{
		config: config,
		db:     db,
		h:      h,
		logger: logger,
		signer: signer,
	}
}

func (c *Controller) HandleVerify() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			controller.MissingAuthorizedApp(w, r, c.h)
			return
		}

		var request api.VerifyCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.logger.Errorw("bad request", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
			return
		}

		// Get the signer based on Key configuration.
		signer, err := c.signer.NewSigner(ctx, c.config.TokenSigningKey)
		if err != nil {
			c.logger.Errorw("failed to get signer", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
			return
		}

		// Process and validate the requested acceptable test types.
		acceptTypes, err := request.GetAcceptedTestTypes()
		if err != nil {
			c.logger.Errorf("invalid accept test types", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInvalidTestType))
			return
		}

		// Exchange the short term verification code for a long term verification token.
		// The token can be used to sign TEKs later.
		verificationToken, err := c.db.VerifyCodeAndIssueToken(authApp.RealmID, request.VerificationCode, acceptTypes, c.config.VerificationTokenDuration)
		if err != nil {
			c.logger.Errorw("failed to issue verification token", "error", err)
			switch {
			case errors.Is(err, database.ErrVerificationCodeExpired):
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code expired").WithCode(api.ErrTokenExpired))
			case errors.Is(err, database.ErrVerificationCodeUsed):
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code invalid").WithCode(api.ErrTokenInvalid))
			case errors.Is(err, database.ErrVerificationCodeNotFound):
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code invalid").WithCode(api.ErrTokenInvalid))
			case errors.Is(err, database.ErrUnsupportedTestType):
				c.h.RenderJSON(w, http.StatusPreconditionFailed, api.Errorf("verification code has unsupported test type").WithCode(api.ErrUnsupportedTestType))
			default:
				c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
			}
			return
		}

		subject := verificationToken.Subject()
		now := time.Now().UTC()
		claims := &jwt.StandardClaims{
			Audience:  c.config.TokenIssuer,
			ExpiresAt: now.Add(c.config.VerificationTokenDuration).Unix(),
			Id:        verificationToken.TokenID,
			IssuedAt:  now.Unix(),
			Issuer:    c.config.TokenIssuer,
			Subject:   subject.String(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		token.Header[verifyapi.KeyIDHeader] = c.config.TokenSigningKeyID
		signedJWT, err := jwthelper.SignJWT(token, signer)
		if err != nil {
			c.logger.Errorw("failed to sign token", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInternal))
			return
		}

		c.h.RenderJSON(w, http.StatusOK, api.VerifyCodeResponse{
			TestType:          verificationToken.TestType,
			SymptomDate:       verificationToken.FormatSymptomDate(),
			VerificationToken: signedJWT,
		})
	})
}
