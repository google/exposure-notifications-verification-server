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
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/dgrijalva/jwt-go"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
)

var (
	MetricPrefix = observability.MetricRoot + "/api/verify"
)

// Controller is a controller for the verification code verification API.
type Controller struct {
	config *config.APIServerConfig
	db     *database.Database
	h      *render.Renderer
	logger *zap.SugaredLogger
	kms    keys.KeyManager

	mCodeVerifyExpired     *stats.Int64Measure
	mCodeVerifyCodeUsed    *stats.Int64Measure
	mCodeVerifyInvalid     *stats.Int64Measure
	mCodeVerified          *stats.Int64Measure
	mCodeVerificationError *stats.Int64Measure
}

func New(ctx context.Context, config *config.APIServerConfig, db *database.Database, h *render.Renderer, kms keys.KeyManager) (*Controller, error) {
	logger := logging.FromContext(ctx)

	mCodeVerifyExpired := stats.Int64(MetricPrefix+"/code_expired", "The number of attempted claims on expired codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/code_expired_count",
		Measure:     mCodeVerifyExpired,
		Description: "The count of attempted claims on expired verification codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCodeVerifyCodeUsed := stats.Int64(MetricPrefix+"/code_used", "The number of attempted claims on already used codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/code_used_count",
		Measure:     mCodeVerifyCodeUsed,
		Description: "The count of attempted claims on an already used verification codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCodeVerifyInvalid := stats.Int64(MetricPrefix+"/code_invalid", "The number of attempted claims on invalid codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/code_invalid_count",
		Measure:     mCodeVerifyInvalid,
		Description: "The count of attempted claims on invalid verification codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCodeVerified := stats.Int64(MetricPrefix+"/code_verified", "The number of successfully claimed codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/code_verified_count",
		Measure:     mCodeVerified,
		Description: "The count of successfully verified codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCodeVerificationError := stats.Int64(MetricPrefix+"/error", "The number of other errors in code issue", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/error_count",
		Measure:     mCodeVerificationError,
		Description: "The count of errors issuing verification codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	return &Controller{
		config:                 config,
		db:                     db,
		h:                      h,
		logger:                 logger,
		kms:                    kms,
		mCodeVerifyExpired:     mCodeVerifyExpired,
		mCodeVerifyCodeUsed:    mCodeVerifyCodeUsed,
		mCodeVerifyInvalid:     mCodeVerifyInvalid,
		mCodeVerified:          mCodeVerified,
		mCodeVerificationError: mCodeVerificationError,
	}, nil
}

func (c *Controller) HandleVerify() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			controller.MissingAuthorizedApp(w, r, c.h)
			return
		}

		// This is a non terminal error, as we're only using the realm for stats.
		realm, err := authApp.Realm(c.db)
		if err != nil {
			c.logger.Errorf("unable to load realm", "error", err)
		} else {
			ctx, err = tag.New(ctx,
				tag.Upsert(observability.RealmTagKey, realm.Name))
			if err != nil {
				c.logger.Errorw("unable to record metrics for realm", "realmID", realm.ID, "error", err)
			}
		}

		var request api.VerifyCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.logger.Errorw("bad request", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
			stats.Record(ctx, c.mCodeVerificationError.M(1))
			return
		}

		// Get the signer based on Key configuration.
		signer, err := c.kms.NewSigner(ctx, c.config.TokenSigning.TokenSigningKey)
		if err != nil {
			c.logger.Errorw("failed to get signer", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
			stats.Record(ctx, c.mCodeVerificationError.M(1))
			return
		}

		// Process and validate the requested acceptable test types.
		acceptTypes, err := request.GetAcceptedTestTypes()
		if err != nil {
			c.logger.Errorf("invalid accept test types", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInvalidTestType))
			stats.Record(ctx, c.mCodeVerificationError.M(1))
			return
		}

		// Exchange the short term verification code for a long term verification token.
		// The token can be used to sign TEKs later.
		verificationToken, err := c.db.VerifyCodeAndIssueToken(authApp.RealmID, request.VerificationCode, acceptTypes, c.config.VerificationTokenDuration)
		if err != nil {
			c.logger.Errorw("failed to issue verification token", "error", err)
			switch {
			case errors.Is(err, database.ErrVerificationCodeExpired):
				stats.Record(ctx, c.mCodeVerifyExpired.M(1))
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code expired").WithCode(api.ErrVerifyCodeExpired))
			case errors.Is(err, database.ErrVerificationCodeUsed):
				stats.Record(ctx, c.mCodeVerifyCodeUsed.M(1))
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code invalid").WithCode(api.ErrVerifyCodeInvalid))
			case errors.Is(err, database.ErrVerificationCodeNotFound):
				stats.Record(ctx, c.mCodeVerifyInvalid.M(1))
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code invalid").WithCode(api.ErrVerifyCodeInvalid))
			case errors.Is(err, database.ErrUnsupportedTestType):
				stats.Record(ctx, c.mCodeVerifyInvalid.M(1))
				c.h.RenderJSON(w, http.StatusPreconditionFailed, api.Errorf("verification code has unsupported test type").WithCode(api.ErrUnsupportedTestType))
			default:
				stats.Record(ctx, c.mCodeVerificationError.M(1))
				c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
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
		token.Header[verifyapi.KeyIDHeader] = c.config.TokenSigning.TokenSigningKeyID
		signedJWT, err := jwthelper.SignJWT(token, signer)
		if err != nil {
			stats.Record(ctx, c.mCodeVerificationError.M(1))
			c.logger.Errorw("failed to sign token", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInternal))
			return
		}

		stats.Record(ctx, c.mCodeVerified.M(1))
		c.h.RenderJSON(w, http.StatusOK, api.VerifyCodeResponse{
			TestType:          verificationToken.TestType,
			SymptomDate:       verificationToken.FormatSymptomDate(),
			VerificationToken: signedJWT,
		})
	})
}
