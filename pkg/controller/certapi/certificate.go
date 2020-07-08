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

// Package certapi implements the token + TEK verification API.
package certapi

import (
	"context"
	"crypto"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/signer"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/cache"

	"github.com/dgrijalva/jwt-go"
	"go.uber.org/zap"
)

type CertificateAPI struct {
	config      *config.APIServerConfig
	db          *database.Database
	logger      *zap.SugaredLogger
	signer      signer.KeyManager
	pubKeyCache *cache.Cache
}

func New(ctx context.Context, config *config.APIServerConfig, db *database.Database, signer signer.KeyManager, pubKeyCache *cache.Cache) http.Handler {
	return &CertificateAPI{config, db, logging.FromContext(ctx), signer, pubKeyCache}
}

func (ca *CertificateAPI) getPublicKey(ctx context.Context, keyID string) (crypto.PublicKey, error) {
	// Get the public key for the Token Signing Key.
	keyCache, err := ca.pubKeyCache.WriteThruLookup(keyID,
		func() (interface{}, error) {
			signer, err := ca.signer.NewSigner(ctx, ca.config.TokenSigningKey)
			if err != nil {
				return nil, err
			}
			return signer.Public(), nil
		})
	if err != nil {
		return nil, fmt.Errorf("unable to get public key for keyId %v: %w", ca.config.TokenSigningKey, err)
	}
	publicKey, ok := keyCache.(crypto.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key in wrong format for %v: %w", ca.config.TokenSigningKey, err)
	}
	return publicKey, nil
}

// Parses and validates the token against the configured keyID and public key.
// If the token si valid the token id (`tid') and subject (`sub`) claims are returned.
func (ca *CertificateAPI) validateToken(verToken string, publicKey crypto.PublicKey) (string, *database.Subject, error) {
	// Parse and validate the verification token.
	token, err := jwt.ParseWithClaims(verToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		kidHeader := token.Header[verifyapi.KeyIDHeader]
		kid, ok := kidHeader.(string)
		if !ok {
			return nil, fmt.Errorf("missing 'kid' header in token")
		}
		if kid == ca.config.TokenSigningKeyID {
			return publicKey, nil
		}
		return nil, fmt.Errorf("no public key for pecified 'kid' not found: %v", kid)
	})
	if err != nil {
		ca.logger.Errorf("invalid verification token: %v", err)
		return "", nil, fmt.Errorf("invalid verification token")
	}
	tokenClaims, ok := token.Claims.(*jwt.StandardClaims)
	if !ok {
		ca.logger.Errorf("invalid claims in verification token")
		return "", nil, fmt.Errorf("invalid verification token")
	}
	if err := tokenClaims.Valid(); err != nil {
		ca.logger.Errorf("JWT is invalid: %v", err)
		return "", nil, fmt.Errorf("verification token expired")
	}
	if !tokenClaims.VerifyIssuer(ca.config.TokenIssuer, true) || !tokenClaims.VerifyAudience(ca.config.TokenIssuer, true) {
		ca.logger.Errorf("jwt contains invalid iss/aud: iss %v aud: %v", tokenClaims.Issuer, tokenClaims.Audience)
		return "", nil, fmt.Errorf("verification token not valid")
	}
	subject, err := database.ParseSubject(tokenClaims.Subject)
	if err != nil {
		return "", nil, fmt.Errorf("invalid subject: %w", err)
	}
	return tokenClaims.Id, subject, nil
}

func (ca *CertificateAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// APIKey should be verified by middleware.
	var request api.VerificationCertificateRequest
	if err := controller.BindJSON(w, r, &request); err != nil {
		ca.logger.Errorf("failed to bind request: %v", err)
		controller.WriteJSON(w, http.StatusBadRequest, api.Error("invalid request: %v", err))
		return
	}

	publicKey, err := ca.getPublicKey(ctx, ca.config.TokenSigningKey)
	if err != nil {
		ca.logger.Errorf("pubPublicKey: %v", err)
		controller.WriteJSON(w, http.StatusInternalServerError, api.Error("interal server error"))
		return
	}

	// Parse and validate the verification token.
	tokenID, subject, err := ca.validateToken(request.VerificationToken, publicKey)
	if err != nil {
		controller.WriteJSON(w, http.StatusBadRequest, api.Error(err.Error()))
		return
	}

	// Validate the HMAC length. SHA 256 HMAC must be 32 bytes in length.
	hmacBytes, err := base64util.DecodeString(request.ExposureKeyHMAC)
	if err != nil {
		controller.WriteJSON(w, http.StatusBadRequest, api.Error("ekeyhmac is not a valid base64 encoding: %v", err))
		return
	}
	if len(hmacBytes) != 32 {
		controller.WriteJSON(w, http.StatusBadRequest, api.Error("ekeyhmac is not the correct length want: 32 got: %v", len(hmacBytes)))
		return
	}

	// Get the signer based on Key configuration.
	signer, err := ca.signer.NewSigner(ctx, ca.config.CertificateSigningKey)
	if err != nil {
		ca.logger.Errorf("unable to get signing key: %v", err)
		controller.WriteJSON(w, http.StatusInternalServerError, api.Error("internal server error - unable to sign certificate"))
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
	claims.StandardClaims.Audience = ca.config.CertificateAudience
	claims.StandardClaims.Issuer = ca.config.CertificateIssuer
	claims.StandardClaims.IssuedAt = now.Unix()
	claims.StandardClaims.ExpiresAt = now.Add(ca.config.CertificateDuration).Unix()
	claims.StandardClaims.NotBefore = now.Add(-1 * time.Second).Unix()

	certToken := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	certToken.Header[verifyapi.KeyIDHeader] = ca.config.CertificateSigningKeyID
	certificate, err := jwthelper.SignJWT(certToken, signer)
	if err != nil {
		ca.logger.Errorf("error signing certificate: %v", err)
		controller.WriteJSON(w, http.StatusInternalServerError, api.Error("internal server error - unable to sign certificate"))
		return
	}

	// To the transactional update to the database last so that if it fails, the client can retry.
	if err := ca.db.ClaimToken(tokenID, subject); err != nil {
		ca.logger.Errorf("error claiming tokenId: %v err: %v", tokenID, err)
		controller.WriteJSON(w, http.StatusBadRequest, api.Error(err.Error()))
		return
	}

	controller.WriteJSON(w, http.StatusOK, api.VerificationCertificateResponse{Certificate: certificate})
}
