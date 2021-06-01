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
	"github.com/google/exposure-notifications-verification-server/pkg/controller/userreport"
	"github.com/google/exposure-notifications-verification-server/pkg/cookiestore"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit/limitware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-limiter"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// ENXRedirect defines routes for the redirector service for ENX.
func ENXRedirect(
	ctx context.Context,
	cfg *config.RedirectConfig,
	db *database.Database,
	cacher cache.Cacher,
	smsSigner keys.KeyManager,
	limiterStore limiter.Store,
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

	// Share static assets with server.
	{
		staticFS := assets.ServerStaticFS()
		fs := http.FileServer(http.FS(staticFS))

		// Static assets.
		r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

		// Browers and devices seem to always hit this - serve it to keep our logs
		// cleaner.
		r.Path("/favicon.ico").Handler(fs)

		// Install robots.txt serving here to prevent indexing.
		r.Path("/robots.txt").Handler(fs)
	}

	// User report web-view configuration.
	{
		// Setup sessions
		sessionOpts := &sessions.Options{
			Domain:   cfg.Issue.ENExpressRedirectDomain,
			MaxAge:   int(cfg.SessionDuration.Seconds()),
			Secure:   !cfg.DevMode,
			SameSite: http.SameSiteStrictMode,
			HttpOnly: true,
		}
		sessions := cookiestore.New(func() ([][]byte, error) {
			return db.GetCookieHashAndEncryptionKeys()
		}, sessionOpts)

		// Limiter
		httplimiter, err := limitware.NewMiddleware(ctx, limiterStore,
			limitware.IPAddressKeyFunc(ctx, "redirect:ratelimit:", cfg.RateLimit.HMACKey),
			limitware.AllowOnError(false))
		if err != nil {
			return nil, fmt.Errorf("failed to create limiter middleware: %w", err)
		}

		// Load localization
		locales, err := i18n.Load(i18n.WithReloading(cfg.DevMode))
		if err != nil {
			return nil, fmt.Errorf("failed to setup i18n: %w", err)
		}

		// Process localization parameters.
		processLocale := middleware.ProcessLocale(locales)

		userReportController, err := userreport.New(locales, cacher, cfg, db, limiterStore, smsSigner, h)
		if err != nil {
			return nil, fmt.Errorf("failed to create code request controller: %w", err)
		}

		// Only allow this on the top level redirect domain
		allowedHostHeaders := []string{cfg.Issue.ENExpressRedirectDomain}
		hostHeaderCheck := middleware.RequireHostHeader(allowedHostHeaders, h, cfg.DevMode)

		// Using a different name, makes it so cookies don't interfer in local dev.
		requireSession := middleware.RequireNamedSession(sessions, "en-user-report", h)

		{ // handler for /report/issue, required values must be in the established session.
			sub := r.Path("/report/issue").Subrouter()
			sub.Use(hostHeaderCheck)
			sub.Use(requireSession)
			sub.Use(httplimiter.Handle)
			sub.Use(processLocale)
			sub.Use(middleware.AddOperatingSystemFromUserAgent())

			sub.Handle("", userReportController.HandleSend()).Methods(http.MethodPost)
		}

		{ // handler for the /report path - requires API KEY and NONCE headers.
			sub := r.Path("/report").Subrouter()
			loadTranslations, err := middleware.LoadDynamicTranslations(locales, db, cacher, cfg.TranslationRefreshPeriod)
			if err != nil {
				return nil, fmt.Errorf("failed to load initial set of translations: %w", err)
			}
			sub.Use(loadTranslations)
			sub.Use(hostHeaderCheck)
			sub.Use(requireSession)
			sub.Use(httplimiter.Handle)
			sub.Use(processLocale)

			// This allows developers to send GET requests in a browser with query params
			// to test the UI. Normally this is required to be initiated via POST and headers.
			if cfg.DevMode || cfg.AllowUserReportGet {
				sub.Use(middleware.QueryHeaderInjection(middleware.APIKeyHeader, "apikey"))
				sub.Use(middleware.QueryHeaderInjection(middleware.NonceHeader, "nonce"))
			}

			requireAPIKey := middleware.RequireAPIKey(cacher, db, h, []database.APIKeyType{
				database.APIKeyTypeDevice,
			})
			sub.Use(requireAPIKey)
			sub.Use(middleware.RequireNonce(h))

			indexMethods := []string{http.MethodPost}
			if cfg.DevMode || cfg.AllowUserReportGet {
				indexMethods = append(indexMethods, http.MethodGet)
			}
			sub.Handle("", userReportController.HandleIndex()).Methods(indexMethods...)
		}
	}

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
		return
	}))

	// Wrap the main router in the mutating middleware method. This cannot be
	// inserted as middleware because gorilla processes the method before
	// middleware.
	mux := http.NewServeMux()
	mux.Handle("/", middleware.MutateMethod()(r))
	return mux, nil
}
