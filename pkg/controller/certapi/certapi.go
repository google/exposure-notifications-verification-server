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

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/google/exposure-notifications-verification-server/pkg/signer"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/dgrijalva/jwt-go"
	"go.uber.org/zap"
)

type Controller struct {
	config      *config.APIServerConfig
	db          *database.Database
	h           *render.Renderer
	logger      *zap.SugaredLogger
	pubKeyCache *cache.Cache
	signer      signer.KeyManager
}

func New(ctx context.Context, config *config.APIServerConfig, db *database.Database, h *render.Renderer, signer signer.KeyManager, pubKeyCache *cache.Cache) *Controller {
	logger := logging.FromContext(ctx)

	return &Controller{
		config:      config,
		db:          db,
		h:           h,
		logger:      logger,
		signer:      signer,
		pubKeyCache: pubKeyCache,
	}
}

func (c *Controller) getPublicKey(ctx context.Context, keyID string) (crypto.PublicKey, error) {
	// Get the public key for the Token Signing Key.
	keyCache, err := c.pubKeyCache.WriteThruLookup(keyID,
		func() (interface{}, error) {
			signer, err := c.signer.NewSigner(ctx, c.config.TokenSigningKey)
			if err != nil {
				return nil, err
			}
			return signer.Public(), nil
		})
	if err != nil {
		return nil, fmt.Errorf("unable to get public key for keyId %v: %w", c.config.TokenSigningKey, err)
	}
	publicKey, ok := keyCache.(crypto.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key in wrong format for %v: %w", c.config.TokenSigningKey, err)
	}
	return publicKey, nil
}

// Parses and validates the token against the configured keyID and public key.
// If the token si valid the token id (`tid') and subject (`sub`) claims are returned.
func (c *Controller) validateToken(verToken string, publicKey crypto.PublicKey) (string, *database.Subject, error) {
	// Parse and validate the verification token.
	token, err := jwt.ParseWithClaims(verToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		kidHeader := token.Header[verifyapi.KeyIDHeader]
		kid, ok := kidHeader.(string)
		if !ok {
			return nil, fmt.Errorf("missing 'kid' header in token")
		}
		if kid == c.config.TokenSigningKeyID {
			return publicKey, nil
		}
		return nil, fmt.Errorf("no public key for specified 'kid' not found: %v", kid)
	})
	if err != nil {
		c.logger.Errorf("invalid verification token: %v", err)
		return "", nil, fmt.Errorf("invalid verification token")
	}
	tokenClaims, ok := token.Claims.(*jwt.StandardClaims)
	if !ok {
		c.logger.Errorf("invalid claims in verification token")
		return "", nil, fmt.Errorf("invalid verification token")
	}
	if err := tokenClaims.Valid(); err != nil {
		c.logger.Errorf("JWT is invalid: %v", err)
		return "", nil, fmt.Errorf("verification token expired")
	}
	if !tokenClaims.VerifyIssuer(c.config.TokenIssuer, true) || !tokenClaims.VerifyAudience(c.config.TokenIssuer, true) {
		c.logger.Errorf("jwt contains invalid iss/aud: iss %v aud: %v", tokenClaims.Issuer, tokenClaims.Audience)
		return "", nil, fmt.Errorf("verification token not valid")
	}
	subject, err := database.ParseSubject(tokenClaims.Subject)
	if err != nil {
		return "", nil, fmt.Errorf("invalid subject: %w", err)
	}
	return tokenClaims.Id, subject, nil
}
