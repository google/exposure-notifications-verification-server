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
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

func (c *Controller) HandleCertificate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			controller.MissingAuthorizedApp(w, r, c.h)
			return
		}

		publicKey, err := c.getPublicKey(ctx, c.config.TokenSigningKey)
		if err != nil {
			c.logger.Errorw("failed to get public key", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, nil)
			return
		}

		// Get the signer based on Key configuration.
		signer, err := c.signer.NewSigner(ctx, c.config.CertificateSigningKey)
		if err != nil {
			c.logger.Errorw("failed to get signer", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, nil)
			return
		}

		var request api.VerificationCertificateRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		// Parse and validate the verification token.
		tokenID, subject, err := c.validateToken(request.VerificationToken, publicKey)
		if err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		// Validate the HMAC length. SHA 256 HMAC must be 32 bytes in length.
		hmacBytes, err := base64util.DecodeString(request.ExposureKeyHMAC)
		if err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest,
				api.Errorf("exposure key HMAC is not a valid base64: %v", err))
			return
		}
		if len(hmacBytes) != 32 {
			c.h.RenderJSON(w, http.StatusBadRequest,
				api.Errorf("exposure key HMAC is not the correct length, want: 32 got: %v", len(hmacBytes)))
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

		// TODO(mikehelmick): Assign transmission risk overrides. Algorithm not set yet.
		claims.SignedMAC = request.ExposureKeyHMAC
		claims.StandardClaims.Audience = c.config.CertificateAudience
		claims.StandardClaims.Issuer = c.config.CertificateIssuer
		claims.StandardClaims.IssuedAt = now.Unix()
		claims.StandardClaims.ExpiresAt = now.Add(c.config.CertificateDuration).Unix()
		claims.StandardClaims.NotBefore = now.Add(-1 * time.Second).Unix()

		certToken := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		certToken.Header[verifyapi.KeyIDHeader] = c.config.CertificateSigningKeyID
		certificate, err := jwthelper.SignJWT(certToken, signer)
		if err != nil {
			c.logger.Errorw("failed to sign certificate", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		// Do the transactional update to the database last so that if it fails, the
		// client can retry.
		if err := c.db.ClaimToken(authApp.RealmID, tokenID, subject); err != nil {
			c.logger.Errorw("failed to claim token", "tokenID", tokenID, "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		c.h.RenderJSON(w, http.StatusOK, &api.VerificationCertificateResponse{
			Certificate: certificate,
		})
	})
}
