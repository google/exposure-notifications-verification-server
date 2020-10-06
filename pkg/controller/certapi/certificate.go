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
	"crypto"
	"errors"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"go.opencensus.io/stats"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
)

func (c *Controller) HandleCertificate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := observability.WithBuildInfo(r.Context())

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			stats.Record(ctx, mCertificateErrors.M(1))
			c.logger.Errorf("missing authorized app")
			controller.MissingAuthorizedApp(w, r, c.h)
			return
		}

		ctx = observability.WithRealmID(ctx, authApp.RealmID)

		stats.Record(ctx, mAttempts.M(1))

		// Get the public key for the token.
		allowedPublicKeys := make(map[string]crypto.PublicKey)
		for kid, keyRef := range c.config.AllowedTokenPublicKeys() {
			publicKey, err := c.pubKeyCache.GetPublicKey(ctx, keyRef, c.kms)
			if err != nil {
				c.logger.Errorw("failed to get public key", "error", err)
				c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
				return
			}
			allowedPublicKeys[kid] = publicKey
		}

		var request api.VerificationCertificateRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.logger.Errorw("failed to parse json request", "error", err)
			stats.Record(ctx, mCertificateErrors.M(1))
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrTokenInvalid))
			return
		}

		// Parse and validate the verification token.
		tokenID, subject, err := c.validateToken(ctx, request.VerificationToken, allowedPublicKeys)
		if err != nil {
			stats.Record(ctx, mCertificateErrors.M(1))
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrTokenInvalid))
			return
		}

		// Validate the HMAC length. SHA 256 HMAC must be 32 bytes in length.
		hmacBytes, err := base64util.DecodeString(request.ExposureKeyHMAC)
		if err != nil {
			stats.Record(ctx, mCertificateErrors.M(1))
			c.h.RenderJSON(w, http.StatusBadRequest,
				api.Errorf("exposure key HMAC is not a valid base64: %v", err).WithCode(api.ErrHMACInvalid))
			return
		}
		if len(hmacBytes) != 32 {
			stats.Record(ctx, mCertificateErrors.M(1))
			c.h.RenderJSON(w, http.StatusBadRequest,
				api.Errorf("exposure key HMAC is not the correct length, want: 32 got: %v", len(hmacBytes)).WithCode(api.ErrHMACInvalid))
			return
		}

		// determine the correct signing key to use.
		signerInfo, err := c.getSignerForRealm(ctx, authApp)
		if err != nil {
			stats.Record(ctx, mCertificateErrors.M(1))
			c.logger.Errorw("failed to get signer", "error", err)
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
		claims.StandardClaims.NotBefore = now.Add(-1 * time.Second).Unix()

		certToken := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		certToken.Header[verifyapi.KeyIDHeader] = signerInfo.KeyID
		certificate, err := jwthelper.SignJWT(certToken, signerInfo.Signer)
		if err != nil {
			stats.Record(ctx, mCertificateErrors.M(1))
			c.logger.Errorw("failed to sign certificate", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInternal))
			return
		}

		// Do the transactional update to the database last so that if it fails, the
		// client can retry.
		if err := c.db.ClaimToken(authApp.RealmID, tokenID, subject); err != nil {
			c.logger.Errorw("failed to claim token", "tokenID", tokenID, "error", err)
			switch {
			case errors.Is(err, database.ErrTokenExpired):
				stats.Record(ctx, mTokenExpired.M(1), mCertificateErrors.M(1))
				c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrTokenExpired))
			case errors.Is(err, database.ErrTokenUsed):
				stats.Record(ctx, mTokenUsed.M(1), mCertificateErrors.M(1))
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification token invalid").WithCode(api.ErrTokenExpired))
			case errors.Is(err, database.ErrTokenMetadataMismatch):
				stats.Record(ctx, mTokenInvalid.M(1), mCertificateErrors.M(1))
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification token invalid").WithCode(api.ErrTokenExpired))
			default:
				stats.Record(ctx, mTokenInvalid.M(1), mCertificateErrors.M(1))
				c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			}
			return
		}

		stats.Record(ctx, mCertificateIssued.M(1))
		c.h.RenderJSON(w, http.StatusOK, &api.VerificationCertificateResponse{
			Certificate: certificate,
		})
	})
}
