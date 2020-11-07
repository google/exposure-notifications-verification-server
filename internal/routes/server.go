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
	"path/filepath"

	"github.com/google/exposure-notifications-verification-server/internal/auth"
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
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit/limitware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/sethvargo/go-limiter"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// Server defines routes for the UI server.
func Server(
	ctx context.Context,
	cfg *config.ServerConfig,
	db *database.Database,
	authProvider auth.Provider,
	cacher cache.Cacher,
	certificateSigner keys.KeyManager,
	limiterStore limiter.Store,
) (http.Handler, error) {
	// Setup sessions
	sessions := sessions.NewCookieStore(cfg.CookieKeys.AsBytes()...)
	sessions.Options.Path = "/"
	sessions.Options.Domain = cfg.CookieDomain
	sessions.Options.MaxAge = int(cfg.SessionDuration.Seconds())
	sessions.Options.Secure = !cfg.DevMode
	sessions.Options.SameSite = http.SameSiteStrictMode

	// Create the router
	r := mux.NewRouter()

	// Inject template middleware - this needs to be first because other
	// middlewares may add data to the template map.
	populateTemplateVariables := middleware.PopulateTemplateVariables(cfg)
	r.Use(populateTemplateVariables)

	// Create the renderer
	h, err := render.New(ctx, cfg.AssetsPath, cfg.DevMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer: %w", err)
	}

	// Request ID injection
	populateRequestID := middleware.PopulateRequestID(h)
	r.Use(populateRequestID)

	// Logger injection.
	populateLogger := middleware.PopulateLogger(logging.FromContext(ctx))
	r.Use(populateLogger)

	httplimiter, err := limitware.NewMiddleware(ctx, limiterStore,
		limitware.UserIDKeyFunc(ctx, "server:ratelimit:", cfg.RateLimit.HMACKey),
		limitware.AllowOnError(false))
	if err != nil {
		return nil, fmt.Errorf("failed to create limiter middleware: %w", err)
	}

	// Install common security headers
	r.Use(middleware.SecureHeaders(cfg.DevMode, "html"))

	// Enable debug headers
	processDebug := middleware.ProcessDebug()
	r.Use(processDebug)

	// Install the CSRF protection middleware.
	configureCSRF := middleware.ConfigureCSRF(ctx, cfg, h)
	r.Use(configureCSRF)

	// Sessions
	requireSession := middleware.RequireSession(sessions, h)
	r.Use(requireSession)

	// Include the current URI
	currentPath := middleware.InjectCurrentPath()
	r.Use(currentPath)

	// Create common middleware
	requireAuth := middleware.RequireAuth(cacher, authProvider, db, h, cfg.SessionIdleTimeout, cfg.SessionDuration)
	requireVerified := middleware.RequireVerified(authProvider, db, h, cfg.SessionDuration)
	requireAdmin := middleware.RequireRealmAdmin(h)
	loadCurrentRealm := middleware.LoadCurrentRealm(cacher, db, h)
	requireRealm := middleware.RequireRealm(h)
	requireSystemAdmin := middleware.RequireSystemAdmin(h)
	requireMFA := middleware.RequireMFA(authProvider, h)
	processFirewall := middleware.ProcessFirewall(h, "server")
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
		loginController := login.New(ctx, authProvider, cfg, db, h)
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
			sub.Handle("/login/manage-account", loginController.HandleReceiveVerifyEmail()).
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
			sub.Handle("/login/manage-account", loginController.HandleShowVerifyEmail()).
				Queries("mode", "verifyEmail").Methods("GET")
			sub.Handle("/login/manage-account", loginController.HandleSubmitVerifyEmail()).
				Queries("mode", "verifyEmail").Methods("POST")

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
		issueapiController := issueapi.New(ctx, cfg, db, limiterStore, h)
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
		sub.Handle("/show/{uuid}", codeStatusController.HandleShow()).Methods("GET")
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

		userController := user.New(ctx, authProvider, cacher, cfg, db, h)
		userSub.Handle("", userController.HandleIndex()).Methods("GET")
		userSub.Handle("", userController.HandleCreate()).Methods("POST")
		userSub.Handle("/new", userController.HandleCreate()).Methods("GET")
		userSub.Handle("/import", userController.HandleImport()).Methods("GET")
		userSub.Handle("/import", userController.HandleImportBatch()).Methods("POST")
		userSub.Handle("/{id}/edit", userController.HandleUpdate()).Methods("GET")
		userSub.Handle("/{id}", userController.HandleShow()).Methods("GET")
		userSub.Handle("/{id}", userController.HandleUpdate()).Methods("PATCH")
		userSub.Handle("/{id}", userController.HandleDelete()).Methods("DELETE")
		userSub.Handle("/{id}/reset-password", userController.HandleResetPassword()).Methods("POST")
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
		realmSub.Handle("/stats", realmadminController.HandleShow(realmadmin.HTML)).Methods("GET")
		realmSub.Handle("/stats.json", realmadminController.HandleShow(realmadmin.JSON)).Methods("GET")
		realmSub.Handle("/stats.csv", realmadminController.HandleShow(realmadmin.CSV)).Methods("GET")
		realmSub.Handle("/stats/{date}", realmadminController.HandleStats()).Methods("GET")
		realmSub.Handle("/events", realmadminController.HandleEvents()).Methods("GET")

		realmKeysController, err := realmkeys.New(ctx, cfg, db, certificateSigner, cacher, h)
		if err != nil {
			return nil, fmt.Errorf("failed to create realmkeys controller: %w", err)
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
			return nil, fmt.Errorf("failed to create jwks controller: %w", err)
		}
		jwksSub.Handle("/{realm}", jwksController.HandleIndex()).Methods("GET")
	}

	// System admin.
	{
		adminSub := r.PathPrefix("/admin").Subrouter()
		adminSub.Use(requireAuth)
		adminSub.Use(loadCurrentRealm)
		adminSub.Use(requireSystemAdmin)
		adminSub.Use(rateLimit)

		adminController := admin.New(ctx, cfg, cacher, db, authProvider, limiterStore, h)
		adminSub.Handle("", http.RedirectHandler("/admin/realms", http.StatusSeeOther)).Methods("GET")
		adminSub.Handle("/realms", adminController.HandleRealmsIndex()).Methods("GET")
		adminSub.Handle("/realms", adminController.HandleRealmsCreate()).Methods("POST")
		adminSub.Handle("/realms/new", adminController.HandleRealmsCreate()).Methods("GET")
		adminSub.Handle("/realms/{id:[0-9]+}/edit", adminController.HandleRealmsUpdate()).Methods("GET")
		adminSub.Handle("/realms/{realm_id:[0-9]+}/add/{user_id:[0-9+]}", adminController.HandleRealmsAdd()).Methods("PATCH")
		adminSub.Handle("/realms/{realm_id:[0-9]+}/remove/{user_id:[0-9+]}", adminController.HandleRealmsRemove()).Methods("PATCH")
		adminSub.Handle("/realms/{id:[0-9]+}/realmadmin", adminController.HandleRealmsSelectAndAdmin()).Methods("GET")
		adminSub.Handle("/realms/{id:[0-9]+}", adminController.HandleRealmsUpdate()).Methods("PATCH")

		adminSub.Handle("/users", adminController.HandleUsersIndex()).Methods("GET")
		adminSub.Handle("/users/{id:[0-9]+}", adminController.HandleUserShow()).Methods("GET")
		adminSub.Handle("/users/{id:[0-9]+}", adminController.HandleUserDelete()).Methods("DELETE")
		adminSub.Handle("/users", adminController.HandleSystemAdminCreate()).Methods("POST")
		adminSub.Handle("/users/new", adminController.HandleSystemAdminCreate()).Methods("GET")
		adminSub.Handle("/users/{id:[0-9]+}/revoke", adminController.HandleSystemAdminRevoke()).Methods("DELETE")

		adminSub.Handle("/mobileapps", adminController.HandleMobileAppsShow()).Methods("GET")
		adminSub.Handle("/sms", adminController.HandleSMSUpdate()).Methods("GET", "POST")
		adminSub.Handle("/email", adminController.HandleEmailUpdate()).Methods("GET", "POST")

		adminSub.Handle("/caches", adminController.HandleCachesIndex()).Methods("GET")
		adminSub.Handle("/caches/clear/{id}", adminController.HandleCachesClear()).Methods("POST")

		adminSub.Handle("/info", adminController.HandleInfoShow()).Methods("GET")
	}

	// Wrap the main router in the mutating middleware method. This cannot be
	// inserted as middleware because gorilla processes the method before
	// middleware.
	mux := http.NewServeMux()
	mux.Handle("/", middleware.MutateMethod()(r))
	return mux, nil
}
