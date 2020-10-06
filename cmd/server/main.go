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

package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	iFB "github.com/google/exposure-notifications-verification-server/internal/firebase"
	"github.com/google/exposure-notifications-verification-server/pkg/buildinfo"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codestatus"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/home"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/jwks"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/login"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/mobileapps"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmadmin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmkeys"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit/limitware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"

	firebase "firebase.google.com/go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	debug, _ := strconv.ParseBool(os.Getenv("LOG_DEBUG"))
	logger := logging.NewLogger(debug)
	logger = logger.With("build_id", buildinfo.BuildID)
	logger = logger.With("build_tag", buildinfo.BuildTag)

	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("successful shutdown")
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	cfg, err := config.NewServerConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	// Setup monitoring
	logger.Info("configuring observability exporter")
	oeConfig := cfg.ObservabilityExporterConfig()
	oe, err := observability.NewFromEnv(ctx, oeConfig)
	if err != nil {
		return fmt.Errorf("unable to create ObservabilityExporter provider: %w", err)
	}
	if err := oe.StartExporter(); err != nil {
		return fmt.Errorf("error initializing observability exporter: %w", err)
	}
	defer oe.Close()
	logger.Infow("observability exporter", "config", oeConfig)

	// Setup sessions
	sessions := sessions.NewCookieStore(cfg.CookieKeys.AsBytes()...)
	sessions.Options.Path = "/"
	sessions.Options.Domain = cfg.CookieDomain
	sessions.Options.MaxAge = int(cfg.SessionDuration.Seconds())
	sessions.Options.Secure = !cfg.DevMode
	sessions.Options.SameSite = http.SameSiteStrictMode

	// Setup cacher
	cacher, err := cache.CacherFor(ctx, &cfg.Cache, cache.MultiKeyFunc(
		cache.HMACKeyFunc(sha1.New, cfg.Cache.HMACKey),
		cache.PrefixKeyFunc("cache:"),
	))
	if err != nil {
		return fmt.Errorf("failed to create cacher: %w", err)
	}
	defer cacher.Close()

	// Setup database
	db, err := cfg.Database.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.OpenWithCacher(ctx, cacher); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Setup signers
	certificateSigner, err := keys.KeyManagerFor(ctx, &cfg.CertificateSigning.Keys)
	if err != nil {
		return fmt.Errorf("failed to create certificate key manager: %w", err)
	}

	// Setup firebase
	app, err := firebase.NewApp(ctx, cfg.FirebaseConfig())
	if err != nil {
		return fmt.Errorf("failed to setup firebase: %w", err)
	}
	auth, err := app.Auth(ctx)
	if err != nil {
		return fmt.Errorf("failed to configure firebase: %w", err)
	}
	firebaseInternal, err := iFB.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to configure internal firebase client: %w", err)
	}

	// Create the router
	r := mux.NewRouter()

	// Inject template middleware - this needs to be first because other
	// middlewares may add data to the template map.
	populateTemplateVariables := middleware.PopulateTemplateVariables(ctx, cfg)
	r.Use(populateTemplateVariables)

	// Create the renderer
	h, err := render.New(ctx, cfg.AssetsPath, cfg.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	// Rate limiting
	limiterStore, err := ratelimit.RateLimiterFor(ctx, &cfg.RateLimit)
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer limiterStore.Close(ctx)

	httplimiter, err := limitware.NewMiddleware(ctx, limiterStore,
		limitware.UserIDKeyFunc(ctx, "server:ratelimit:", cfg.RateLimit.HMACKey),
		limitware.AllowOnError(false))
	if err != nil {
		return fmt.Errorf("failed to create limiter middleware: %w", err)
	}

	// Install common security headers
	r.Use(middleware.SecureHeaders(ctx, cfg.DevMode, "html"))

	// Enable debug headers
	processDebug := middleware.ProcessDebug(ctx)
	r.Use(processDebug)

	// Install the CSRF protection middleware.
	configureCSRF := middleware.ConfigureCSRF(ctx, cfg, h)
	r.Use(configureCSRF)

	// Sessions
	requireSession := middleware.RequireSession(ctx, sessions, h)
	r.Use(requireSession)

	// Include the current URI
	currentPath := middleware.InjectCurrentPath()
	r.Use(currentPath)

	// Create common middleware
	requireAuth := middleware.RequireAuth(ctx, cacher, auth, db, h, cfg.SessionIdleTimeout, cfg.SessionDuration)
	requireVerified := middleware.RequireVerified(ctx, auth, db, h, cfg.SessionDuration)
	requireAdmin := middleware.RequireRealmAdmin(ctx, h)
	loadCurrentRealm := middleware.LoadCurrentRealm(ctx, cacher, db, h)
	requireRealm := middleware.RequireRealm(ctx, h)
	requireSystemAdmin := middleware.RequireAdmin(ctx, h)
	requireMFA := middleware.RequireMFA(ctx, h)
	processFirewall := middleware.ProcessFirewall(ctx, h, "server")
	rateLimit := httplimiter.Handle

	{
		static := filepath.Join(cfg.AssetsPath, "static")
		fs := http.FileServer(http.Dir(static))
		r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

		// Browers and devices seem to always hit this - serve it to keep our logs
		// cleaner.
		r.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(static, "favicon.ico"))
		}))
	}

	{
		sub := r.PathPrefix("").Subrouter()
		sub.Handle("/health", controller.HandleHealthz(ctx, &cfg.Database, h)).Methods("GET")
	}

	{
		loginController := login.New(ctx, firebaseInternal, auth, cfg, db, h)
		{
			sub := r.PathPrefix("").Subrouter()
			sub.Use(rateLimit)

			sub.Handle("/", loginController.HandleLogin()).Methods("GET")
			sub.Handle("/login/reset-password", loginController.HandleShowResetPassword()).Methods("GET")
			sub.Handle("/login/reset-password", loginController.HandleSubmitResetPassword()).Methods("POST")
			sub.Handle("/login/manage-account", loginController.HandleShowSelectNewPassword()).
				Queries("oobCode", "", "mode", "resetPassword").Methods("GET")
			sub.Handle("/login/manage-account", loginController.HandleSubmitNewPassword()).
				Queries("oobCode", "", "mode", "resetPassword").Methods("POST")
			sub.Handle("/login/manage-account", loginController.HandleSubmitVerifyEmail()).
				Queries("oobCode", "{oobCode:.+}", "mode", "{mode:(?:verifyEmail|recoverEmail)}").Methods("GET")
			sub.Handle("/session", loginController.HandleCreateSession()).Methods("POST")
			sub.Handle("/signout", loginController.HandleSignOut()).Methods("GET")

			// Realm selection & account settings
			sub = r.PathPrefix("").Subrouter()
			sub.Use(requireAuth)
			sub.Use(rateLimit)
			sub.Use(loadCurrentRealm)
			sub.Handle("/login", loginController.HandleReauth()).Methods("GET")
			sub.Handle("/login", loginController.HandleReauth()).Queries("redir", "").Methods("GET")
			sub.Handle("/login/select-realm", loginController.HandleSelectRealm()).Methods("GET", "POST")
			sub.Handle("/login/change-password", loginController.HandleShowChangePassword()).Methods("GET")
			sub.Handle("/login/change-password", loginController.HandleSubmitChangePassword()).Methods("POST")
			sub.Handle("/account", loginController.HandleAccountSettings()).Methods("GET")

			// Verifying email requires the user is logged in
			sub = r.PathPrefix("").Subrouter()
			sub.Use(requireAuth)
			sub.Use(rateLimit)
			sub.Use(loadCurrentRealm)
			sub.Use(requireRealm)
			sub.Use(processFirewall)
			sub.Handle("/login/manage-account", loginController.HandleShowVerifyEmail()).
				Queries("mode", "verifyEmail").Methods("GET")

			// SMS auth registration is realm-specific, so it needs to load the current realm.
			sub = r.PathPrefix("").Subrouter()
			sub.Use(requireAuth)
			sub.Use(rateLimit)
			sub.Use(loadCurrentRealm)
			sub.Use(requireRealm)
			sub.Use(processFirewall)
			sub.Use(requireVerified)
			sub.Handle("/login/register-phone", loginController.HandleRegisterPhone()).Methods("GET")
		}
	}

	{
		sub := r.PathPrefix("/home").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentRealm)
		sub.Use(requireRealm)
		sub.Use(processFirewall)
		sub.Use(requireVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		homeController := home.New(ctx, cfg, db, h)
		sub.Handle("", homeController.HandleHome()).Methods("GET")

		// API for creating new verification codes. Called via AJAX.
		issueapiController, err := issueapi.New(ctx, cfg, db, limiterStore, h)
		if err != nil {
			return fmt.Errorf("issueapi.New: %w", err)
		}
		sub.Handle("/issue", issueapiController.HandleIssue()).Methods("POST")
	}

	{
		sub := r.PathPrefix("/code").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentRealm)
		sub.Use(requireRealm)
		sub.Use(processFirewall)
		sub.Use(requireVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		codeStatusController := codestatus.NewServer(ctx, cfg, db, h)
		sub.Handle("/status", codeStatusController.HandleIndex()).Methods("GET")
		sub.Handle("/show", codeStatusController.HandleShow()).Methods("POST")
		sub.Handle("/{uuid}/expire", codeStatusController.HandleExpirePage()).Methods("PATCH")
	}

	// mobileapp
	{
		sub := r.PathPrefix("/mobile-apps").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentRealm)
		sub.Use(requireRealm)
		sub.Use(processFirewall)
		sub.Use(requireAdmin)
		sub.Use(requireVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		mobileappsController := mobileapps.New(ctx, cfg, cacher, db, h)
		sub.Handle("", mobileappsController.HandleIndex()).Methods("GET")
		sub.Handle("", mobileappsController.HandleCreate()).Methods("POST")
		sub.Handle("/new", mobileappsController.HandleCreate()).Methods("GET")
		sub.Handle("/{id:[0-9]+}/edit", mobileappsController.HandleUpdate()).Methods("GET")
		sub.Handle("/{id:[0-9]+}", mobileappsController.HandleShow()).Methods("GET")
		sub.Handle("/{id:[0-9]+}", mobileappsController.HandleUpdate()).Methods("PATCH")
		sub.Handle("/{id:[0-9]+}/disable", mobileappsController.HandleDisable()).Methods("PATCH")
		sub.Handle("/{id:[0-9]+}/enable", mobileappsController.HandleEnable()).Methods("PATCH")
	}

	// apikeys
	{
		sub := r.PathPrefix("/apikeys").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentRealm)
		sub.Use(requireRealm)
		sub.Use(processFirewall)
		sub.Use(requireAdmin)
		sub.Use(requireVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		apikeyController := apikey.New(ctx, cfg, cacher, db, h)
		sub.Handle("", apikeyController.HandleIndex()).Methods("GET")
		sub.Handle("", apikeyController.HandleCreate()).Methods("POST")
		sub.Handle("/new", apikeyController.HandleCreate()).Methods("GET")
		sub.Handle("/{id}/edit", apikeyController.HandleUpdate()).Methods("GET")
		sub.Handle("/{id}", apikeyController.HandleShow()).Methods("GET")
		sub.Handle("/{id}", apikeyController.HandleUpdate()).Methods("PATCH")
		sub.Handle("/{id}/disable", apikeyController.HandleDisable()).Methods("PATCH")
		sub.Handle("/{id}/enable", apikeyController.HandleEnable()).Methods("PATCH")
	}

	// users
	{
		userSub := r.PathPrefix("/users").Subrouter()
		userSub.Use(requireAuth)
		userSub.Use(loadCurrentRealm)
		userSub.Use(requireRealm)
		userSub.Use(processFirewall)
		userSub.Use(requireAdmin)
		userSub.Use(requireVerified)
		userSub.Use(requireMFA)
		userSub.Use(rateLimit)

		userController := user.New(ctx, firebaseInternal, auth, cacher, cfg, db, h)
		userSub.Handle("", userController.HandleIndex()).Methods("GET")
		userSub.Handle("", userController.HandleIndex()).
			Queries("offset", "{[0-9]*}", "email", "").Methods("GET")
		userSub.Handle("", userController.HandleCreate()).Methods("POST")
		userSub.Handle("/new", userController.HandleCreate()).Methods("GET")
		userSub.Handle("/import", userController.HandleImport()).Methods("GET")
		userSub.Handle("/import", userController.HandleImportBatch()).Methods("POST")
		userSub.Handle("/{id}/edit", userController.HandleUpdate()).Methods("GET")
		userSub.Handle("/{id}", userController.HandleShow()).Methods("GET")
		userSub.Handle("/{id}", userController.HandleUpdate()).Methods("PATCH")
		userSub.Handle("/{id}", userController.HandleDelete()).Methods("DELETE")
	}

	// realms
	{
		realmSub := r.PathPrefix("/realm").Subrouter()
		realmSub.Use(requireAuth)
		realmSub.Use(loadCurrentRealm)
		realmSub.Use(requireRealm)
		realmSub.Use(processFirewall)
		realmSub.Use(requireAdmin)
		realmSub.Use(requireVerified)
		realmSub.Use(requireMFA)
		realmSub.Use(rateLimit)

		realmadminController := realmadmin.New(ctx, cacher, cfg, db, limiterStore, h)
		realmSub.Handle("/settings", realmadminController.HandleSettings()).Methods("GET", "POST")
		realmSub.Handle("/settings/enable-express", realmadminController.HandleEnableExpress()).Methods("POST")
		realmSub.Handle("/settings/disable-express", realmadminController.HandleDisableExpress()).Methods("POST")
		realmSub.Handle("/stats", realmadminController.HandleShow()).Methods("GET")
		realmSub.Handle("/events", realmadminController.HandleEvents()).Methods("GET")

		realmKeysController, err := realmkeys.New(ctx, cfg, db, certificateSigner, cacher, h)
		if err != nil {
			return fmt.Errorf("failed to create realmkeys controller: %w", err)
		}
		realmSub.Handle("/keys", realmKeysController.HandleIndex()).Methods("GET")
		realmSub.Handle("/keys/{id}", realmKeysController.HandleDestroy()).Methods("DELETE")
		realmSub.Handle("/keys/create", realmKeysController.HandleCreateKey()).Methods("POST")
		realmSub.Handle("/keys/upgrade", realmKeysController.HandleUpgrade()).Methods("POST")
		realmSub.Handle("/keys/save", realmKeysController.HandleSave()).Methods("POST")
		realmSub.Handle("/keys/activate", realmKeysController.HandleActivate()).Methods("POST")
	}

	// jwks
	{
		jwksSub := r.PathPrefix("/jwks").Subrouter()
		jwksSub.Use(rateLimit)

		jwksController, err := jwks.New(ctx, db, cacher, h)
		if err != nil {
			return fmt.Errorf("failed to create jwks controller: %w", err)
		}
		jwksSub.Handle("/{realm}", jwksController.HandleIndex()).Methods("GET")
	}

	// System admin.
	{
		adminSub := r.PathPrefix("/admin").Subrouter()
		adminSub.Use(requireAuth)
		adminSub.Use(loadCurrentRealm)
		adminSub.Use(requireVerified)
		adminSub.Use(requireSystemAdmin)
		adminSub.Use(rateLimit)

		adminController := admin.New(ctx, cfg, db, auth, h)
		adminSub.Handle("", http.RedirectHandler("/admin/realms", http.StatusSeeOther)).Methods("GET")
		adminSub.Handle("/realms", adminController.HandleRealmsIndex()).Methods("GET")
		adminSub.Handle("/realms", adminController.HandleRealmsCreate()).Methods("POST")
		adminSub.Handle("/realms/new", adminController.HandleRealmsCreate()).Methods("GET")
		adminSub.Handle("/realms/{id:[0-9]+}/edit", adminController.HandleRealmsUpdate()).Methods("GET")
		adminSub.Handle("/realms/{id:[0-9]+}/join", adminController.HandleRealmsJoin()).Methods("PATCH")
		adminSub.Handle("/realms/{id:[0-9]+}/leave", adminController.HandleRealmsLeave()).Methods("PATCH")
		adminSub.Handle("/realms/{id:[0-9]+}", adminController.HandleRealmsUpdate()).Methods("PATCH")

		adminSub.Handle("/sms", adminController.HandleSMSUpdate()).Methods("GET", "POST")

		adminSub.Handle("/users", adminController.HandleUsersIndex()).Methods("GET")
		adminSub.Handle("/users", adminController.HandleUsersCreate()).Methods("POST")
		adminSub.Handle("/users/new", adminController.HandleUsersCreate()).Methods("GET")
		adminSub.Handle("/users/{id:[0-9]+}", adminController.HandleUsersDelete()).Methods("DELETE")

		adminSub.Handle("/info", adminController.HandleInfoShow()).Methods("GET")
	}

	// Wrap the main router in the mutating middleware method. This cannot be
	// inserted as middleware because gorilla processes the method before
	// middleware.
	mux := http.NewServeMux()
	mux.Handle("/", middleware.MutateMethod(ctx)(r))

	srv, err := server.New(cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", cfg.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, mux))
}
