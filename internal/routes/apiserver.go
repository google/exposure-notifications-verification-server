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

package routes

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/verifyapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit/limitware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/mikehelmick/go-chaff"
	"github.com/sethvargo/go-limiter"

	"github.com/gorilla/mux"
)

// APIServer defines routes for the apiserver service.
func APIServer(
	ctx context.Context,
	cfg *config.APIServerConfig,
	db *database.Database,
	cacher cache.Cacher,
	limiterStore limiter.Store,
	tokenSigner keys.KeyManager,
	certificateSigner keys.KeyManager,
) (http.Handler, func(), error) {
	closer := func() {}

	// Create the router
	r := mux.NewRouter()

	// Common observability context
	ctx, obs := middleware.WithObservability(ctx)
	r.Use(obs)

	// Create the renderer
	h, err := render.New(ctx, nil, cfg.DevMode)
	if err != nil {
		return nil, closer, fmt.Errorf("failed to create renderer: %w", err)
	}

	// Request ID injection
	populateRequestID := middleware.PopulateRequestID(h)
	r.Use(populateRequestID)

	// Logger injection
	populateLogger := middleware.PopulateLogger(logging.FromContext(ctx))
	r.Use(populateLogger)

	// Recovery injection
	recovery := middleware.Recovery(h)
	r.Use(recovery)

	// Note that rate limiting is installed _after_ the chaff middleware because
	// we do not want chaff requests to count towards rate-limiting quota.
	httplimiter, err := limitware.NewMiddleware(ctx, limiterStore,
		limitware.APIKeyFunc(ctx, db, "apiserver:ratelimit:", cfg.RateLimit.HMACKey),
		limitware.AllowOnError(false))
	if err != nil {
		return nil, closer, fmt.Errorf("failed to create limiter middleware: %w", err)
	}
	rateLimit := httplimiter.Handle

	// Install common security headers
	r.Use(middleware.SecureHeaders(cfg.DevMode, "json"))

	// Enable debug headers
	processDebug := middleware.ProcessDebug()
	r.Use(processDebug)

	// Other common middlewares
	requireAPIKey := middleware.RequireAPIKey(cacher, db, h, []database.APIKeyType{
		database.APIKeyTypeDevice,
	})
	processFirewall := middleware.ProcessFirewall(h, "apiserver")

	// Health route
	r.Handle("/health", controller.HandleHealthz(db, h, cfg.IsMaintenanceMode())).Methods(http.MethodGet)

	// Make verify chaff tracker.
	verifyChaffTracker, err := chaff.NewTracker(chaff.NewJSONResponder(encodeVerifyResponse), chaff.DefaultCapacity)
	if err != nil {
		return nil, closer, fmt.Errorf("error creating verify chaffer: %w", err)
	}
	closer = func() {
		verifyChaffTracker.Close()
	}

	// Make cert chaff tracker.
	certChaffTracker, err := chaff.NewTracker(chaff.NewJSONResponder(encodeCertificateResponse), chaff.DefaultCapacity)
	if err != nil {
		return nil, closer, fmt.Errorf("error creating cert chaffer: %w", err)
	}
	closer = func() {
		verifyChaffTracker.Close()
		certChaffTracker.Close()
	}

	{
		sub := r.PathPrefix("/api/user-report").Subrouter()
		sub.Use(requireAPIKey)
		sub.Use(processFirewall)
		sub.Use(middleware.ProcessChaff(db, verifyChaffTracker, middleware.ChaffHeaderDetector()))
		sub.Use(rateLimit)

		// POST /api/user-report
		issueController := issueapi.New(cfg, db, limiterStore, certificateSigner, h)
		sub.Handle("", issueController.HandleUserReport()).Methods(http.MethodPost)
	}

	{
		sub := r.PathPrefix("/api/verify").Subrouter()
		sub.Use(requireAPIKey)
		sub.Use(processFirewall)
		sub.Use(middleware.ProcessChaff(db, verifyChaffTracker, middleware.ChaffHeaderDetector()))
		sub.Use(rateLimit)
		sub.Use(middleware.AddOperatingSystemFromUserAgent())

		// POST /api/verify
		verifyapiController := verifyapi.New(cfg, db, cacher, tokenSigner, h)
		sub.Handle("", verifyapiController.HandleVerify()).Methods(http.MethodPost)
	}

	{
		sub := r.PathPrefix("/api/certificate").Subrouter()
		sub.Use(requireAPIKey)
		sub.Use(processFirewall)
		sub.Use(middleware.ProcessChaff(db, certChaffTracker, middleware.ChaffHeaderDetector()))
		sub.Use(rateLimit)

		// POST /api/certificate
		certapiController, err := certapi.New(ctx, cfg, db, cacher, certificateSigner, h)
		if err != nil {
			return nil, closer, fmt.Errorf("failed to create certapi controller: %w", err)
		}
		sub.Handle("", certapiController.HandleCertificate()).Methods(http.MethodPost)
	}

	// Wrap the main router in the mutating middleware method. This cannot be
	// inserted as middleware because gorilla processes the method before
	// middleware.
	mux := http.NewServeMux()
	mux.Handle("/", middleware.MutateMethod()(r))
	return mux, closer, nil
}

// makePadFromChaff makes a Padding structure from chaff data.
// Note, the random chaff data will be longer than necessary, so we shorten it.
func makePadFromChaff(s string) api.Padding {
	return api.Padding(s)
}

func encodeVerifyResponse(s string) interface{} {
	return api.VerifyCodeResponse{Padding: makePadFromChaff(s)}
}

func encodeCertificateResponse(s string) interface{} {
	return api.VerificationCertificateResponse{Padding: makePadFromChaff(s)}
}
