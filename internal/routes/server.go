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

		codestatusController := codestatus.NewServer(ctx, cfg, db, h)
		codestatusRoutes(sub, codestatusController)
	}

	// mobileapp
	{
		sub := r.PathPrefix("/realm/mobile-apps").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentRealm)
		sub.Use(requireRealm)
		sub.Use(processFirewall)
		sub.Use(requireAdmin)
		sub.Use(requireVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		mobileappsController := mobileapps.New(ctx, cfg, cacher, db, h)
		mobileappsRoutes(sub, mobileappsController)
	}

	// apikeys
	{
		sub := r.PathPrefix("/realm/apikeys").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentRealm)
		sub.Use(requireRealm)
		sub.Use(processFirewall)
		sub.Use(requireAdmin)
		sub.Use(requireVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		apikeyController := apikey.New(ctx, cfg, cacher, db, h)
		apikeyRoutes(sub, apikeyController)
	}

	// users
	{
		sub := r.PathPrefix("/realm/users").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentRealm)
		sub.Use(requireRealm)
		sub.Use(processFirewall)
		sub.Use(requireAdmin)
		sub.Use(requireVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		userController := user.New(ctx, authProvider, cacher, cfg, db, h)
		userRoutes(sub, userController)
	}

	// realms
	{
		sub := r.PathPrefix("/realm").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentRealm)
		sub.Use(requireRealm)
		sub.Use(processFirewall)
		sub.Use(requireAdmin)
		sub.Use(requireVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		realmadminController := realmadmin.New(ctx, cacher, cfg, db, limiterStore, h)
		realmadminRoutes(sub, realmadminController)

		realmkeysController, err := realmkeys.New(ctx, cfg, db, certificateSigner, cacher, h)
		if err != nil {
			return nil, fmt.Errorf("failed to create realmkeys controller: %w", err)
		}
		realmkeysRoutes(sub, realmkeysController)
	}

	// JWKs
	{
		sub := r.PathPrefix("/jwks").Subrouter()
		sub.Use(rateLimit)

		jwksController, err := jwks.New(ctx, db, cacher, h)
		if err != nil {
			return nil, fmt.Errorf("failed to create jwks controller: %w", err)
		}
		jwksRoutes(sub, jwksController)
	}

	// System admin
	{
		sub := r.PathPrefix("/admin").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentRealm)
		sub.Use(requireSystemAdmin)
		sub.Use(rateLimit)

		adminController := admin.New(ctx, cfg, cacher, db, authProvider, limiterStore, h)
		systemAdminRoutes(sub, adminController)
	}

	// Wrap the main router in the mutating middleware method. This cannot be
	// inserted as middleware because gorilla processes the method before
	// middleware.
	mux := http.NewServeMux()
	mux.Handle("/", middleware.MutateMethod()(r))
	return mux, nil
}

// codestatusRoutes are the routes for checking code statuses.
func codestatusRoutes(r *mux.Router, c *codestatus.Controller) {
	r.Handle("/status", c.HandleIndex()).Methods("GET")
	r.Handle("/show/{uuid}", c.HandleShow()).Methods("GET")
	r.Handle("/{uuid}/expire", c.HandleExpirePage()).Methods("PATCH")
}

// mobileappsRoutes are the Mobile App routes.
func mobileappsRoutes(r *mux.Router, c *mobileapps.Controller) {
	r.Handle("", c.HandleIndex()).Methods("GET")
	r.Handle("", c.HandleCreate()).Methods("POST")
	r.Handle("/new", c.HandleCreate()).Methods("GET")
	r.Handle("/{id:[0-9]+}/edit", c.HandleUpdate()).Methods("GET")
	r.Handle("/{id:[0-9]+}", c.HandleShow()).Methods("GET")
	r.Handle("/{id:[0-9]+}", c.HandleUpdate()).Methods("PATCH")
	r.Handle("/{id:[0-9]+}/disable", c.HandleDisable()).Methods("PATCH")
	r.Handle("/{id:[0-9]+}/enable", c.HandleEnable()).Methods("PATCH")
}

// apikeyRoutes are the API key routes.
func apikeyRoutes(r *mux.Router, c *apikey.Controller) {
	r.Handle("", c.HandleIndex()).Methods("GET")
	r.Handle("", c.HandleCreate()).Methods("POST")
	r.Handle("/new", c.HandleCreate()).Methods("GET")
	r.Handle("/{id:[0-9]+}/edit", c.HandleUpdate()).Methods("GET")
	r.Handle("/{id:[0-9]+}", c.HandleShow()).Methods("GET")
	r.Handle("/{id:[0-9]+}", c.HandleUpdate()).Methods("PATCH")
	r.Handle("/{id:[0-9]+}/disable", c.HandleDisable()).Methods("PATCH")
	r.Handle("/{id:[0-9]+}/enable", c.HandleEnable()).Methods("PATCH")
}

// userRoutes are the user routes.
func userRoutes(r *mux.Router, c *user.Controller) {
	r.Handle("", c.HandleIndex()).Methods("GET")
	r.Handle("", c.HandleCreate()).Methods("POST")
	r.Handle("/new", c.HandleCreate()).Methods("GET")
	r.Handle("/export.csv", c.HandleExport()).Methods("GET")
	r.Handle("/import", c.HandleImport()).Methods("GET")
	r.Handle("/import", c.HandleImportBatch()).Methods("POST")
	r.Handle("/{id:[0-9]+}/edit", c.HandleUpdate()).Methods("GET")
	r.Handle("/{id:[0-9]+}", c.HandleShow()).Methods("GET")
	r.Handle("/{id:[0-9]+}", c.HandleUpdate()).Methods("PATCH")
	r.Handle("/{id:[0-9]+}", c.HandleDelete()).Methods("DELETE")
	r.Handle("/{id:[0-9]+}/reset-password", c.HandleResetPassword()).Methods("POST")
}

// realmkeysRoutes are the realm key routes.
func realmkeysRoutes(r *mux.Router, c *realmkeys.Controller) {
	r.Handle("/keys", c.HandleIndex()).Methods("GET")
	r.Handle("/keys/{id:[0-9]+}", c.HandleDestroy()).Methods("DELETE")
	r.Handle("/keys/create", c.HandleCreateKey()).Methods("POST")
	r.Handle("/keys/upgrade", c.HandleUpgrade()).Methods("POST")
	r.Handle("/keys/save", c.HandleSave()).Methods("POST")
	r.Handle("/keys/activate", c.HandleActivate()).Methods("POST")
}

// realmadminRoutes are the realm admin routes.
func realmadminRoutes(r *mux.Router, c *realmadmin.Controller) {
	r.Handle("/settings", c.HandleSettings()).Methods("GET", "POST")
	r.Handle("/settings/enable-express", c.HandleEnableExpress()).Methods("POST")
	r.Handle("/settings/disable-express", c.HandleDisableExpress()).Methods("POST")
	r.Handle("/stats", c.HandleShow(realmadmin.HTML)).Methods("GET")
	r.Handle("/stats.json", c.HandleShow(realmadmin.JSON)).Methods("GET")
	r.Handle("/stats.csv", c.HandleShow(realmadmin.CSV)).Methods("GET")
	r.Handle("/stats/{date}", c.HandleStats()).Methods("GET")
	r.Handle("/events", c.HandleEvents()).Methods("GET")
}

// jwksRoutes are the JWK routes, rooted at /jwks.
func jwksRoutes(r *mux.Router, c *jwks.Controller) {
	r.Handle("/{realm_id:[0-9]+}", c.HandleIndex()).Methods("GET")
}

// systemAdminRoutes are the system routes, rooted at /admin.
func systemAdminRoutes(r *mux.Router, c *admin.Controller) {
	// Redirect / to /admin/realms
	r.Handle("", http.RedirectHandler("/admin/realms", http.StatusSeeOther)).Methods("GET")
	r.Handle("/", http.RedirectHandler("/admin/realms", http.StatusSeeOther)).Methods("GET")

	r.Handle("/realms", c.HandleRealmsIndex()).Methods("GET")
	r.Handle("/realms", c.HandleRealmsCreate()).Methods("POST")
	r.Handle("/realms/new", c.HandleRealmsCreate()).Methods("GET")
	r.Handle("/realms/{id:[0-9]+}/edit", c.HandleRealmsUpdate()).Methods("GET")
	r.Handle("/realms/{realm_id:[0-9]+}/add/{user_id:[0-9]+}", c.HandleRealmsAdd()).Methods("PATCH")
	r.Handle("/realms/{realm_id:[0-9]+}/remove/{user_id:[0-9]+}", c.HandleRealmsRemove()).Methods("PATCH")
	r.Handle("/realms/{id:[0-9]+}/realmadmin", c.HandleRealmsSelectAndAdmin()).Methods("GET")
	r.Handle("/realms/{id:[0-9]+}", c.HandleRealmsUpdate()).Methods("PATCH")

	r.Handle("/users", c.HandleUsersIndex()).Methods("GET")
	r.Handle("/users/{id:[0-9]+}", c.HandleUserShow()).Methods("GET")
	r.Handle("/users/{id:[0-9]+}", c.HandleUserDelete()).Methods("DELETE")
	r.Handle("/users", c.HandleSystemAdminCreate()).Methods("POST")
	r.Handle("/users/new", c.HandleSystemAdminCreate()).Methods("GET")
	r.Handle("/users/{id:[0-9]+}/revoke", c.HandleSystemAdminRevoke()).Methods("DELETE")

	r.Handle("/mobile-apps", c.HandleMobileAppsShow()).Methods("GET")
	r.Handle("/sms", c.HandleSMSUpdate()).Methods("GET", "POST")
	r.Handle("/email", c.HandleEmailUpdate()).Methods("GET", "POST")
	r.Handle("/events", c.HandleEventsShow()).Methods("GET")

	r.Handle("/caches", c.HandleCachesIndex()).Methods("GET")
	r.Handle("/caches/clear/{id}", c.HandleCachesClear()).Methods("POST")

	r.Handle("/info", c.HandleInfoShow()).Methods("GET")
}
