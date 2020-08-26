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
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/dgrijalva/jwt-go"
	"go.uber.org/zap"
)

var (
	MetricPrefix = observability.MetricRoot + "/api/certificate"
)

type Controller struct {
	config      *config.APIServerConfig
	db          *database.Database
	h           *render.Renderer
	logger      *zap.SugaredLogger
	pubKeyCache *keyutils.PublicKeyCache // Cache of public keys for verification token verification.
	signerCache *cache.Cache             // Cache signers on a per-realm basis.
	kms         keys.KeyManager

	mTokenExpired      *stats.Int64Measure
	mTokenUsed         *stats.Int64Measure
	mTokenInvalid      *stats.Int64Measure
	mCertificateIssued *stats.Int64Measure
	mCertificateErrors *stats.Int64Measure
}

func New(ctx context.Context, config *config.APIServerConfig, db *database.Database, h *render.Renderer, kms keys.KeyManager) (*Controller, error) {
	logger := logging.FromContext(ctx)

	pubKeyCache, err := keyutils.NewPublicKeyCache(config.CertificateSigning.PublicKeyCacheDuration)
	if err != nil {
		return nil, fmt.Errorf("cannot create public key cache, likely invalid duration: %w", err)
	}

	signerCache, err := cache.New(config.CertificateSigning.SignerCacheDuration)
	if err != nil {
		return nil, fmt.Errorf("cannot create signer cache, likely invalid duration: %w", err)
	}

	mTokenExpired := stats.Int64(MetricPrefix+"/token_expired", "expired tokens on certificate issue", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/token_expired_count",
		Measure:     mTokenExpired,
		Description: "The count of expired tokens on certificate issue",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mTokenUsed := stats.Int64(MetricPrefix+"/token_used", "already used tokens on certificate issue", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/token_used_count",
		Measure:     mTokenUsed,
		Description: "The count of already used tokens on certificate issue",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mTokenInvalid := stats.Int64(MetricPrefix+"/invalid_token", "invalid tokens on certificate issue", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/invalid_token_count",
		Measure:     mTokenInvalid,
		Description: "The count of invalid tokens on certificate issue",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCertificateIssued := stats.Int64(MetricPrefix+"/issue", "certificates issued", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/issue_count",
		Measure:     mCertificateIssued,
		Description: "The count of certificates issued",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCertificateErrors := stats.Int64(MetricPrefix+"/errors", "certificate issue errors", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/error_count",
		Measure:     mCertificateErrors,
		Description: "The count of certificate issue errors",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	return &Controller{
		config:             config,
		db:                 db,
		h:                  h,
		logger:             logger,
		pubKeyCache:        pubKeyCache,
		signerCache:        signerCache,
		kms:                kms,
		mTokenExpired:      mTokenExpired,
		mTokenInvalid:      mTokenInvalid,
		mCertificateIssued: mCertificateIssued,
		mCertificateErrors: mCertificateErrors,
	}, nil
}

// Parses and validates the token against the configured keyID and public key.
// If the token si valid the token id (`tid') and subject (`sub`) claims are returned.
func (c *Controller) validateToken(ctx context.Context, verToken string, publicKey crypto.PublicKey) (string, *database.Subject, error) {
	// Parse and validate the verification token.
	token, err := jwt.ParseWithClaims(verToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		kidHeader := token.Header[verifyapi.KeyIDHeader]
		kid, ok := kidHeader.(string)
		if !ok {
			return nil, fmt.Errorf("missing 'kid' header in token")
		}
		if kid == c.config.TokenSigning.TokenSigningKeyID {
			return publicKey, nil
		}
		return nil, fmt.Errorf("no public key for specified 'kid' not found: %v", kid)
	})
	if err != nil {
		stats.Record(ctx, c.mTokenInvalid.M(1))
		c.logger.Errorf("invalid verification token: %v", err)
		return "", nil, fmt.Errorf("invalid verification token")
	}
	tokenClaims, ok := token.Claims.(*jwt.StandardClaims)
	if !ok {
		stats.Record(ctx, c.mTokenInvalid.M(1))
		c.logger.Errorf("invalid claims in verification token")
		return "", nil, fmt.Errorf("invalid verification token")
	}
	if err := tokenClaims.Valid(); err != nil {
		stats.Record(ctx, c.mTokenInvalid.M(1))
		c.logger.Errorf("JWT is invalid: %v", err)
		return "", nil, fmt.Errorf("verification token expired")
	}
	if !tokenClaims.VerifyIssuer(c.config.TokenSigning.TokenIssuer, true) || !tokenClaims.VerifyAudience(c.config.TokenSigning.TokenIssuer, true) {
		stats.Record(ctx, c.mTokenInvalid.M(1))
		c.logger.Errorf("jwt contains invalid iss/aud: iss %v aud: %v", tokenClaims.Issuer, tokenClaims.Audience)
		return "", nil, fmt.Errorf("verification token not valid")
	}
	subject, err := database.ParseSubject(tokenClaims.Subject)
	if err != nil {
		stats.Record(ctx, c.mTokenInvalid.M(1))
		return "", nil, fmt.Errorf("invalid subject: %w", err)
	}
	return tokenClaims.Id, subject, nil
}
