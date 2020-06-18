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

	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/cache"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CertificateAPI struct {
	config      *config.Config
	db          *database.Database
	logger      *zap.SugaredLogger
	signer      signer.KeyManager
	pubKeyCache *cache.Cache
}

func New(ctx context.Context, config *config.Config, db *database.Database, signer signer.KeyManager, pubKeyCache *cache.Cache) controller.Controller {
	return &CertificateAPI{config, db, logging.FromContext(ctx), signer, pubKeyCache}
}

func (ca *CertificateAPI) getPublicKey(c *gin.Context, keyID string) (crypto.PublicKey, error) {
	// Get the public key for the Token Signing Key.
	keyCache, err := ca.pubKeyCache.WriteThruLookup(keyID,
		func() (interface{}, error) {
			signer, err := ca.signer.NewSigner(c.Request.Context(), ca.config.TokenSigningKey)
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

func (ca *CertificateAPI) validateToken(verToken string, publicKey crypto.PublicKey) (string, error) {
	// Parse and validate the verification token.
	token, err := jwt.ParseWithClaims(verToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil {
		ca.logger.Errorf("invalid verification token: %v", err)
		return "", fmt.Errorf("invalid verification token")
	}
	tokenClaims, ok := token.Claims.(*jwt.StandardClaims)
	if !ok {
		ca.logger.Errorf("invalid claims in verification token")
		return "", fmt.Errorf("invalid verification token")
	}
	if err := tokenClaims.Valid(); err != nil {
		ca.logger.Errorf("JWT is invalid: %v", err)
		return "", fmt.Errorf("verification token expired")
	}
	if !tokenClaims.VerifyIssuer(ca.config.TokenIssuer, true) || !tokenClaims.VerifyAudience(ca.config.TokenIssuer, true) {
		ca.logger.Errorf("jwt contains invalid iss/aud: iss %v aud: %v", tokenClaims.Issuer, tokenClaims.Audience)
		return "", fmt.Errorf("verification token not valid")
	}
	return tokenClaims.Id, nil
}

func (ca *CertificateAPI) Execute(c *gin.Context) {
	// APIKey should be verified by middleware.
	var request api.VerificationCertificateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ca.logger.Errorf("failed to bind request: %v", err)
		c.JSON(http.StatusBadRequest, api.ErrorReturn{Error: fmt.Sprintf("invalid request: %v", err)})
		return
	}

	publicKey, err := ca.getPublicKey(c, ca.config.TokenSigningKey)
	if err != nil {
		ca.logger.Errorf("pubPublicKey: %v", err)
		c.JSON(http.StatusInternalServerError, api.ErrorReturn{Error: "interal server error"})
		return
	}

	// Parse and validate the verification token.
	tokenID, err := ca.validateToken(request.VerificationToken, publicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorReturn{Error: err.Error()})
		return
	}

	// Get the signer based on Key configuration.
	signer, err := ca.signer.NewSigner(c.Request.Context(), ca.config.CertificateSigningKey)
	if err != nil {
		ca.logger.Errorf("unable to get signing key: %v", err)
		c.JSON(http.StatusInternalServerError, api.ErrorReturn{Error: "internal server error - unable to sign certificate"})
		return
	}

	// Create the Certificate
	now := time.Now().UTC()
	claims := v1alpha1.NewVerificationClaims()
	// This server curently does not provide any transmission risk overrides, although that is part
	// of the diagnosis verification protocol.
	// TODO(mikehelmick) - determine what, if anything we want in the reference server.
	claims.SignedMAC = request.ExposureKeyHMAC
	claims.StandardClaims.Audience = ca.config.CertificateAudience
	claims.StandardClaims.Issuer = ca.config.CertificateIssuer
	claims.StandardClaims.IssuedAt = now.Unix()
	claims.StandardClaims.ExpiresAt = now.Add(ca.config.CertificateDuration).Unix()
	claims.StandardClaims.NotBefore = now.Add(-1 * time.Second).Unix()

	certToken := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	certificate, err := jwthelper.SignJWT(certToken, signer)
	if err != nil {
		ca.logger.Errorf("error signing certificate: %v", err)
		c.JSON(http.StatusInternalServerError, api.ErrorReturn{Error: "internal server error - unable to sign certificate"})
		return
	}

	// To the transactional update to the database last so that if it fails, the client can retry.
	if err := ca.db.ClaimToken(tokenID); err != nil {
		ca.logger.Errorf("error claiming tokenId: %v err: %v", tokenID, err)
		c.JSON(http.StatusBadRequest, api.ErrorReturn{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, api.VerificationCertificateResponse{Certificate: certificate})
}
