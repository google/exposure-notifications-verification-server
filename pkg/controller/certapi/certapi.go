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

// Package certapi implements the token + TEK verification API.
package certapi

import (
	"context"
	"fmt"

	vcache "github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/golang-jwt/jwt"
)

type Controller struct {
	config      *config.APIServerConfig
	db          *database.Database
	cacher      vcache.Cacher
	h           *render.Renderer
	pubKeyCache *keyutils.PublicKeyCache // Cache of public keys for verification token verification.
	signerCache *cache.Cache             // Cache signers on a per-realm basis.
	kms         keys.KeyManager
}

func New(ctx context.Context, config *config.APIServerConfig, db *database.Database, cacher vcache.Cacher, kms keys.KeyManager, h *render.Renderer) (*Controller, error) {
	pubKeyCache, err := keyutils.NewPublicKeyCache(ctx, cacher, config.CertificateSigning.PublicKeyCacheDuration)
	if err != nil {
		return nil, fmt.Errorf("cannot create public key cache, likely invalid duration: %w", err)
	}

	// This has to be in-memory because the signer has state and connection pools.
	signerCache, err := cache.New(config.CertificateSigning.SignerCacheDuration)
	if err != nil {
		return nil, fmt.Errorf("cannot create signer cache, likely invalid duration: %w", err)
	}

	return &Controller{
		config:      config,
		db:          db,
		cacher:      cacher,
		h:           h,
		pubKeyCache: pubKeyCache,
		signerCache: signerCache,
		kms:         kms,
	}, nil
}

// Parses and validates the token against the configured keyID and public key.
// A map of valid 'kid' values is supported.
// If the token is valid the token id (`tid') and subject (`sub`) claims are returned.
func (c *Controller) validateToken(ctx context.Context, verToken string) (string, *database.Subject, error) {
	logger := logging.FromContext(ctx).Named("certapi.validateToken")

	parser := &jwt.Parser{
		SkipClaimsValidation: true, // we manually check claims.Valid() below
	}
	// Parse and validate the verification token.
	token, err := parser.ParseWithClaims(verToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		kidHeader := token.Header[verifyapi.KeyIDHeader]
		kid, ok := kidHeader.(string)
		if !ok {
			return nil, fmt.Errorf("missing 'kid' header in token")
		}

		tokenSigningKey, err := c.db.FindTokenSigningKeyByUUIDCached(ctx, c.cacher, kid)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup token signing key: %w", err)
		}

		publicKey, err := c.pubKeyCache.GetPublicKey(ctx, tokenSigningKey.KeyVersionID, c.kms)
		if err != nil {
			return nil, fmt.Errorf("failed to find public key for kid %q: %w", kid, err)
		}
		return publicKey, nil
	})
	if err != nil {
		logger.Debugw("invalid verification token", "error", err)
		return "", nil, fmt.Errorf("invalid verification token")
	}

	tokenClaims, ok := token.Claims.(*jwt.StandardClaims)
	if !ok {
		logger.Infow("invalid verification token", "error", "claims are not StandardClaims")
		return "", nil, fmt.Errorf("invalid verification token")
	}
	if err := tokenClaims.Valid(); err != nil {
		logger.Infow("invalid verification token", "error", "jwt is invalid", "jwtError", err)
		return "", nil, fmt.Errorf("verification token expired")
	}
	if !tokenClaims.VerifyIssuer(c.config.TokenSigning.TokenIssuer, true) || !tokenClaims.VerifyAudience(c.config.TokenSigning.TokenIssuer, true) {
		logger.Infow("invalid verification token",
			"error", "jwt contains invalid iss/aud",
			"issuer", tokenClaims.Issuer,
			"audience", tokenClaims.Audience)
		return "", nil, fmt.Errorf("verification token not valid")
	}
	subject, err := database.ParseSubject(tokenClaims.Subject)
	if err != nil {
		return "", nil, fmt.Errorf("invalid subject: %w", err)
	}
	return tokenClaims.Id, subject, nil
}
