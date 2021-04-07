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

package routes

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/assets"
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/associated"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/redirect"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/gorilla/mux"
)

// ENXRedirect defines routes for the redirector service for ENX.
func ENXRedirect(
	ctx context.Context,
	cfg *config.RedirectConfig,
	db *database.Database,
	cacher cache.Cacher,
) (http.Handler, error) {
	// Create the router
	r := mux.NewRouter()

	// Common observability context
	ctx, obs := middleware.WithObservability(ctx)
	r.Use(obs)

	// Load localization
	locales, err := i18n.Load(i18n.WithReloading(cfg.DevMode))
	if err != nil {
		return nil, fmt.Errorf("failed to setup i18n: %w", err)
	}

	// Process localization parameters.
	processLocale := middleware.ProcessLocale(locales)
	r.Use(processLocale)

	// Create the renderer
	h, err := render.New(ctx, assets.ENXRedirectFS(), cfg.DevMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer: %w", err)
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

	// Install common security headers
	r.Use(middleware.SecureHeaders(cfg.DevMode, "html"))

	// Enable debug headers
	processDebug := middleware.ProcessDebug()
	r.Use(processDebug)

	// Handle health.
	r.Handle("/health", controller.HandleHealthz(db, h)).Methods(http.MethodGet)

	// iOS and Android include functionality to associate data between web-apps
	// and device apps. Things like handoff between websites and apps, or
	// shared credentials are the common usecases. The redirect server
	// publishes the metadata needed to share data between the two domains to
	// offer a more seemless experience between the website and apps. iOS and
	// Android publish specs as to what this format looks like, and both live
	// under the /.well-known directory on the server.
	//
	//   Android Specs:
	//     https://developer.android.com/training/app-links/verify-site-associations
	//   iOS Specs:
	//     https://developer.apple.com/documentation/safariservices/supporting_associated_domains
	//
	{
		wk := r.PathPrefix("/.well-known").Subrouter()

		// Enable the iOS and Android redirect handler.
		assocController, err := associated.New(cfg, db, cacher, h)
		if err != nil {
			return nil, fmt.Errorf("failed to create associated links controller: %w", err)
		}
		wk.PathPrefix("/apple-app-site-association").Handler(assocController.HandleIos()).Methods(http.MethodGet)
		wk.PathPrefix("/assetlinks.json").Handler(assocController.HandleAndroid()).Methods(http.MethodGet)
	}

	// Handle redirects.
	redirectController, err := redirect.New(db, cfg, cacher, h)
	if err != nil {
		return nil, fmt.Errorf("failed to create redirect controller: %w", err)
	}
	r.PathPrefix("/").Handler(redirectController.HandleIndex()).Methods(http.MethodGet)

	// Blanket handle any missing routes.
	r.NotFoundHandler = processLocale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller.NotFound(w, r, h)
	}))

	// Wrap the main router in the mutating middleware method. This cannot be
	// inserted as middleware because gorilla processes the method before
	// middleware.
	mux := http.NewServeMux()
	mux.Handle("/", middleware.MutateMethod()(r))
	return mux, nil
}
