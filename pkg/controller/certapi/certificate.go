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

package certapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/logging"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
)

const (
	HMACLength = 32
)

func (c *Controller) HandleCertificate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("certapi.HandleCertificate")

		blame := enobs.BlameNone
		result := enobs.ResultOK
		defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &blame, &result)

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			logger.Errorf("missing authorized app")
			blame = enobs.BlameClient
			result = enobs.ResultError("MISSING_AUTHORIZED_APP")

			controller.MissingAuthorizedApp(w, r, c.h)
			return
		}

		var request api.VerificationCertificateRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			logger.Errorw("failed to parse json request", "error", err)
			blame = enobs.BlameClient
			result = enobs.ResultError("FAILED_TO_PARSE_JSON_REQUEST")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrTokenInvalid))
			return
		}

		// Parse and validate the verification token.
		tokenID, subject, err := c.validateToken(ctx, request.VerificationToken)
		if err != nil {
			blame = enobs.BlameClient
			result = enobs.ResultError("FAILED_TO_VALIDATE_TOKEN")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrTokenInvalid))
			return
		}

		// Validate the HMAC length. SHA 256 HMAC must be 32 bytes in length.
		hmacBytes, err := base64util.DecodeString(request.ExposureKeyHMAC)
		if err != nil {
			logger.Debugw("provided invalid hmac, not base64", "error", err)
			blame = enobs.BlameClient
			result = enobs.ResultError("FAILED_TO_DECODE_HMAC")

			c.h.RenderJSON(w, http.StatusBadRequest,
				api.Errorf("exposure key HMAC is not a valid base64: %v", err).WithCode(api.ErrHMACInvalid))
			return
		}
		if l := len(hmacBytes); l != HMACLength {
			logger.Debugw("provided invalid hmac, wrong length", "length", l)
			blame = enobs.BlameClient
			result = enobs.ResultError("INVALID_HMAC_LENGTH")

			c.h.RenderJSON(w, http.StatusBadRequest,
				api.Errorf("exposure key HMAC is not the correct length, want: %v got: %v", HMACLength, l).WithCode(api.ErrHMACInvalid))
			return
		}

		// determine the correct signing key to use.
		signerInfo, err := c.getSignerForAuthApp(ctx, authApp)
		if err != nil {
			logger.Errorw("failed to get signer", "error", err)
			// FIXME: should we blame server here?
			blame = enobs.BlameServer
			result = enobs.ResultError("FAILED_TO_GET_SIGNER")

			c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
			return
		}

		// Create the Certificate
		now := time.Now().UTC()
		claims := verifyapi.NewVerificationClaims()
		// Assign the report type.
		claims.ReportType = subject.TestType
		if subject.SymptomDate != nil {
			claims.SymptomOnsetInterval = subject.SymptomInterval()
		}

		claims.SignedMAC = request.ExposureKeyHMAC
		claims.StandardClaims.Audience = signerInfo.Audience
		claims.StandardClaims.Issuer = signerInfo.Issuer
		claims.StandardClaims.IssuedAt = now.Unix()
		claims.StandardClaims.ExpiresAt = now.Add(signerInfo.Duration).Unix()
		claims.StandardClaims.NotBefore = now.Add(-1 * c.config.CertificateSigning.AllowedClockSkew).Unix()

		certToken := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		certToken.Header[verifyapi.KeyIDHeader] = signerInfo.KeyID
		certificate, err := jwthelper.SignJWT(certToken, signerInfo.Signer)
		if err != nil {
			logger.Errorw("failed to sign certificate", "error", err)
			blame = enobs.BlameServer
			result = enobs.ResultError("FAILED_TO_SIGN_JWT")

			c.h.RenderJSON(w, http.StatusInternalServerError, api.Error(err).WithCode(api.ErrInternal))
			return
		}

		// Do the transactional update to the database last so that if it fails, the
		// client can retry.
		if err := c.db.ClaimToken(now, authApp, tokenID, subject); err != nil {
			blame = enobs.BlameClient
			switch {
			case errors.Is(err, database.ErrTokenExpired):
				logger.Infow("failed to claim token, expired", "tokenID", tokenID, "error", err)
				result = enobs.ResultError("TOKEN_EXPIRED")
				c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrTokenExpired))
				return
			case errors.Is(err, database.ErrTokenUsed):
				logger.Infow("failed to claim token, already used", "tokenID", tokenID, "error", err)
				result = enobs.ResultError("TOKEN_USED")
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification token invalid").WithCode(api.ErrTokenExpired))
				return
			case errors.Is(err, database.ErrTokenMetadataMismatch):
				logger.Infow("failed to claim token, metadata mismatch", "tokenID", tokenID, "error", err)
				result = enobs.ResultError("TOKEN_METADATA_MISMATCH")
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification token invalid").WithCode(api.ErrTokenExpired))
				return
			default:
				blame = enobs.BlameServer
				logger.Errorw("failed to claim token, unknown", "tokenID", tokenID, "error", err)
				result = enobs.ResultError("UNKNOWN_TOKEN_CLAIM_ERROR")
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Error(err))
				return
			}
		}

		c.h.RenderJSON(w, http.StatusOK, &api.VerificationCertificateResponse{
			Certificate: certificate,
		})
	})
}
