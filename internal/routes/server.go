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
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codes"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/jwks"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/login"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/mobileapps"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmadmin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmkeys"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/smskeys"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/stats"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
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
	smsSigner keys.KeyManager,
	limiterStore limiter.Store,
) (http.Handler, error) {
	// Setup sessions
	sessions := sessions.NewCookieStore(cfg.CookieKeys.AsBytes()...)
	sessions.Options.Path = "/"
	sessions.Options.Domain = cfg.CookieDomain
	sessions.Options.MaxAge = int(cfg.SessionDuration.Seconds())
	sessions.Options.Secure = !cfg.DevMode
	sessions.Options.SameSite = http.SameSiteStrictMode
	sessions.Options.HttpOnly = true

	// Create the router
	r := mux.NewRouter()

	// Common observability context
	ctx, obs := middleware.WithObservability(ctx)
	r.Use(obs)

	// Inject template middleware - this needs to be first because other
	// middlewares may add data to the template map.
	populateTemplateVariables := middleware.PopulateTemplateVariables(cfg)
	r.Use(populateTemplateVariables)

	// Load localization
	locales, err := i18n.Load(cfg.LocalesPath, i18n.WithReloading(cfg.DevMode))
	if err != nil {
		return nil, fmt.Errorf("failed to setup i18n: %w", err)
	}

	// Process localization parameters.
	processLocale := middleware.ProcessLocale(locales)
	r.Use(processLocale)

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
	configureCSRF := middleware.ConfigureCSRF(cfg, h)
	r.Use(configureCSRF)

	// Sessions
	requireSession := middleware.RequireSession(sessions, h)
	r.Use(requireSession)

	// Include the current URI
	currentPath := middleware.InjectCurrentPath()
	r.Use(currentPath)

	// Create common middleware
	requireAuth := middleware.RequireAuth(cacher, authProvider, db, h, cfg.SessionIdleTimeout, cfg.SessionDuration)
	checkIdleNoAuth := middleware.CheckSessionIdleNoAuth(h, cfg.SessionIdleTimeout)
	requireEmailVerified := middleware.RequireEmailVerified(authProvider, h)
	loadCurrentMembership := middleware.LoadCurrentMembership(h)
	requireMembership := middleware.RequireMembership(h)
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
		sub.Handle("/health", controller.HandleHealthz(db, h)).Methods(http.MethodGet)
	}

	{
		loginController := login.New(authProvider, cacher, cfg, db, h)
		{
			sub := r.PathPrefix("").Subrouter()
			sub.Use(rateLimit)
			sub.Handle("/session", loginController.HandleCreateSession()).Methods(http.MethodPost)
			sub.Handle("/signout", loginController.HandleSignOut()).Methods(http.MethodGet)

			sub = r.PathPrefix("").Subrouter()
			sub.Use(rateLimit)
			sub.Use(checkIdleNoAuth)

			sub.Handle("/", loginController.HandleLogin()).Methods(http.MethodGet)
			sub.Handle("/login/reset-password", loginController.HandleShowResetPassword()).Methods(http.MethodGet)
			sub.Handle("/login/reset-password", loginController.HandleSubmitResetPassword()).Methods(http.MethodPost)
			sub.Handle("/login/manage-account", loginController.HandleShowSelectNewPassword()).
				Queries("oobCode", "", "mode", "resetPassword").Methods(http.MethodGet)
			sub.Handle("/login/manage-account", loginController.HandleSubmitNewPassword()).
				Queries("oobCode", "", "mode", "resetPassword").Methods(http.MethodPost)
			sub.Handle("/login/manage-account", loginController.HandleReceiveVerifyEmail()).
				Queries("oobCode", "{oobCode:.+}", "mode", "{mode:(?:verifyEmail|recoverEmail)}").Methods(http.MethodGet)

			// Realm selection & account settings
			sub = r.PathPrefix("").Subrouter()
			sub.Use(requireAuth)
			sub.Use(rateLimit)
			sub.Use(loadCurrentMembership)
			sub.Handle("/login", loginController.HandleReauth()).Methods(http.MethodGet)
			sub.Handle("/login", loginController.HandleReauth()).Queries("redir", "").Methods(http.MethodGet)
			sub.Handle("/login/post-authenticate", loginController.HandlePostAuthenticate()).Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch)
			sub.Handle("/login/select-realm", loginController.HandleSelectRealm()).Methods(http.MethodGet, http.MethodPost)
			sub.Handle("/login/change-password", loginController.HandleShowChangePassword()).Methods(http.MethodGet)
			sub.Handle("/login/change-password", loginController.HandleSubmitChangePassword()).Methods(http.MethodPost)
			sub.Handle("/account", loginController.HandleAccountSettings()).Methods(http.MethodGet)
			sub.Handle("/login/manage-account", loginController.HandleShowVerifyEmail()).
				Queries("mode", "verifyEmail").Methods(http.MethodGet)
			sub.Handle("/login/manage-account", loginController.HandleSubmitVerifyEmail()).
				Queries("mode", "verifyEmail").Methods(http.MethodPost)
			sub.Handle("/login/register-phone", loginController.HandleRegisterPhone()).Methods(http.MethodGet)
		}
	}

	// codes
	{
		sub := r.PathPrefix("/codes").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentMembership)
		sub.Use(requireMembership)
		sub.Use(processFirewall)
		sub.Use(requireEmailVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		sub.Handle("", http.RedirectHandler("/codes/issue", http.StatusSeeOther)).Methods(http.MethodGet)
		sub.Handle("/", http.RedirectHandler("/codes/issue", http.StatusSeeOther)).Methods(http.MethodGet)

		// API for creating new verification codes. Called via AJAX.
		issueapiController := issueapi.New(cfg, db, limiterStore, smsSigner, h)
		sub.Handle("/issue", issueapiController.HandleIssueUI()).Methods(http.MethodPost)
		sub.Handle("/batch-issue", issueapiController.HandleBatchIssueUI()).Methods(http.MethodPost)

		codesController := codes.NewServer(ctx, cfg, db, h)
		codesRoutes(sub, codesController)
	}

	// mobileapp
	{
		sub := r.PathPrefix("/realm/mobile-apps").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentMembership)
		sub.Use(requireMembership)
		sub.Use(processFirewall)
		sub.Use(requireEmailVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		mobileappsController := mobileapps.New(db, h)
		mobileappsRoutes(sub, mobileappsController)
	}

	// apikeys
	{
		sub := r.PathPrefix("/realm/apikeys").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentMembership)
		sub.Use(requireMembership)
		sub.Use(processFirewall)
		sub.Use(requireEmailVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		apikeyController := apikey.New(cacher, db, h)
		apikeyRoutes(sub, apikeyController)
	}

	// users
	{
		sub := r.PathPrefix("/realm/users").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentMembership)
		sub.Use(requireMembership)
		sub.Use(processFirewall)
		sub.Use(requireEmailVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		userController := user.New(authProvider, cacher, db, h)
		userRoutes(sub, userController)
	}

	// stats
	{
		sub := r.PathPrefix("/stats").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentMembership)
		sub.Use(requireMembership)
		sub.Use(processFirewall)
		sub.Use(requireEmailVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		statsController := stats.New(cacher, db, h)
		statsRoutes(sub, statsController)
	}

	// realms
	{
		sub := r.PathPrefix("/realm").Subrouter()
		sub.Use(requireAuth)
		sub.Use(loadCurrentMembership)
		sub.Use(requireMembership)
		sub.Use(processFirewall)
		sub.Use(requireEmailVerified)
		sub.Use(requireMFA)
		sub.Use(rateLimit)

		realmadminController := realmadmin.New(cfg, db, limiterStore, h)
		realmadminRoutes(sub, realmadminController)

		publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
		if err != nil {
			return nil, err
		}

		realmkeysController := realmkeys.New(cfg, db, certificateSigner, publicKeyCache, h)
		realmkeysRoutes(sub, realmkeysController)

		realmSMSKeysController := smskeys.New(cfg, db, publicKeyCache, h)
		if cfg.Features.EnableAuthenticatedSMS {
			realmSMSkeysRoutes(sub, realmSMSKeysController)
		}
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
		sub.Use(loadCurrentMembership)
		sub.Use(requireSystemAdmin)
		sub.Use(rateLimit)

		adminController := admin.New(cfg, cacher, db, authProvider, limiterStore, h)
		systemAdminRoutes(sub, adminController)
	}

	// Wrap the main router in the mutating middleware method. This cannot be
	// inserted as middleware because gorilla processes the method before
	// middleware.
	mux := http.NewServeMux()
	mux.Handle("/", middleware.MutateMethod()(r))
	return mux, nil
}

// codesRoutes are the routes for checking codes.
func codesRoutes(r *mux.Router, c *codes.Controller) {
	r.Handle("/issue", c.HandleIssue()).Methods(http.MethodGet)
	r.Handle("/bulk-issue", c.HandleBulkIssue()).Methods(http.MethodGet)
	r.Handle("/status", c.HandleIndex()).Methods(http.MethodGet)
	r.Handle("/{uuid}", c.HandleShow()).Methods(http.MethodGet)
	r.Handle("/{uuid}/expire", c.HandleExpirePage()).Methods(http.MethodPatch)
}

// mobileappsRoutes are the Mobile App routes.
func mobileappsRoutes(r *mux.Router, c *mobileapps.Controller) {
	r.Handle("", c.HandleIndex()).Methods(http.MethodGet)
	r.Handle("", c.HandleCreate()).Methods(http.MethodPost)
	r.Handle("/new", c.HandleCreate()).Methods(http.MethodGet)
	r.Handle("/{id:[0-9]+}/edit", c.HandleUpdate()).Methods(http.MethodGet)
	r.Handle("/{id:[0-9]+}", c.HandleShow()).Methods(http.MethodGet)
	r.Handle("/{id:[0-9]+}", c.HandleUpdate()).Methods(http.MethodPatch)
	r.Handle("/{id:[0-9]+}/disable", c.HandleDisable()).Methods(http.MethodPatch)
	r.Handle("/{id:[0-9]+}/enable", c.HandleEnable()).Methods(http.MethodPatch)
}

// apikeyRoutes are the API key routes.
func apikeyRoutes(r *mux.Router, c *apikey.Controller) {
	r.Handle("", c.HandleIndex()).Methods(http.MethodGet)
	r.Handle("", c.HandleCreate()).Methods(http.MethodPost)
	r.Handle("/new", c.HandleCreate()).Methods(http.MethodGet)
	r.Handle("/{id:[0-9]+}/edit", c.HandleUpdate()).Methods(http.MethodGet)
	r.Handle("/{id:[0-9]+}", c.HandleShow()).Methods(http.MethodGet)
	r.Handle("/{id:[0-9]+}", c.HandleUpdate()).Methods(http.MethodPatch)
	r.Handle("/{id:[0-9]+}/disable", c.HandleDisable()).Methods(http.MethodPatch)
	r.Handle("/{id:[0-9]+}/enable", c.HandleEnable()).Methods(http.MethodPatch)
}

// userRoutes are the user routes.
func userRoutes(r *mux.Router, c *user.Controller) {
	r.Handle("", c.HandleIndex()).Methods(http.MethodGet)
	r.Handle("", c.HandleCreate()).Methods(http.MethodPost)
	r.Handle("/new", c.HandleCreate()).Methods(http.MethodGet)
	r.Handle("/export.csv", c.HandleExport()).Methods(http.MethodGet)
	r.Handle("/import", c.HandleImport()).Methods(http.MethodGet)
	r.Handle("/import", c.HandleImportBatch()).Methods(http.MethodPost)
	r.Handle("/bulk-permissions/add", c.HandleBulkPermissions(database.BulkPermissionActionAdd)).Methods(http.MethodPost)
	r.Handle("/bulk-permissions/remove", c.HandleBulkPermissions(database.BulkPermissionActionRemove)).Methods(http.MethodPost)
	r.Handle("/{id:[0-9]+}/edit", c.HandleUpdate()).Methods(http.MethodGet)
	r.Handle("/{id:[0-9]+}", c.HandleShow()).Methods(http.MethodGet)
	r.Handle("/{id:[0-9]+}", c.HandleUpdate()).Methods(http.MethodPatch)
	r.Handle("/{id:[0-9]+}", c.HandleDelete()).Methods(http.MethodDelete)
	r.Handle("/{id:[0-9]+}/reset-password", c.HandleResetPassword()).Methods(http.MethodPost)
}

// realmkeysRoutes are the realm key routes.
func realmkeysRoutes(r *mux.Router, c *realmkeys.Controller) {
	r.Handle("/keys", c.HandleIndex()).Methods(http.MethodGet)
	r.Handle("/keys/{id:[0-9]+}", c.HandleDestroy()).Methods(http.MethodDelete)
	r.Handle("/keys/create", c.HandleCreateKey()).Methods(http.MethodPost)
	r.Handle("/keys/upgrade", c.HandleUpgrade()).Methods(http.MethodPost)
	r.Handle("/keys/automatic", c.HandleAutomaticRotate()).Methods(http.MethodPost)
	r.Handle("/keys/manual", c.HandleManualRotate()).Methods(http.MethodPost)
	r.Handle("/keys/save", c.HandleSave()).Methods(http.MethodPost)
	r.Handle("/keys/activate", c.HandleActivate()).Methods(http.MethodPost)
}

// realmSMSkeysRoutes are the realm key routes.
func realmSMSkeysRoutes(r *mux.Router, c *smskeys.Controller) {
	r.Handle("/sms-keys", c.HandleIndex()).Methods(http.MethodGet)
	r.Handle("/sms-keys", c.HandleCreateKey()).Methods(http.MethodPost)
	r.Handle("/sms-keys/enable", c.HandleEnable()).Methods(http.MethodPut)
	r.Handle("/sms-keys/disable", c.HandleDisable()).Methods(http.MethodPut)
	r.Handle("/sms-keys/{id:[0-9]+}", c.HandleDestroy()).Methods(http.MethodDelete)
	r.Handle("/sms-keys/activate", c.HandleActivate()).Methods(http.MethodPost)
}

// statsRoutes are the statistics routes, rooted at /stats.
func statsRoutes(r *mux.Router, c *stats.Controller) {
	r.Handle("/realm.csv", c.HandleRealmStats(stats.TypeCSV)).Methods(http.MethodGet)
	r.Handle("/realm.json", c.HandleRealmStats(stats.TypeJSON)).Methods(http.MethodGet)

	r.Handle("/realm/users.csv", c.HandleRealmUsersStats(stats.TypeCSV)).Methods(http.MethodGet)
	r.Handle("/realm/users.json", c.HandleRealmUsersStats(stats.TypeJSON)).Methods(http.MethodGet)

	r.Handle("/realm/users/{id}.csv", c.HandleRealmUserStats(stats.TypeCSV)).Methods(http.MethodGet)
	r.Handle("/realm/users/{id}.json", c.HandleRealmUserStats(stats.TypeJSON)).Methods(http.MethodGet)

	r.Handle("/realm/api-keys/{id}.csv", c.HandleRealmAuthorizedAppStats(stats.TypeCSV)).Methods(http.MethodGet)
	r.Handle("/realm/api-keys/{id}.json", c.HandleRealmAuthorizedAppStats(stats.TypeJSON)).Methods(http.MethodGet)

	r.Handle("/realm/external-issuers.csv", c.HandleRealmExternalIssuersStats(stats.TypeCSV)).Methods(http.MethodGet)
	r.Handle("/realm/external-issuers.json", c.HandleRealmExternalIssuersStats(stats.TypeJSON)).Methods(http.MethodGet)

	r.Handle("/realm/key-server.csv", c.HandleKeyServerStats(stats.TypeCSV)).Methods(http.MethodGet)
	r.Handle("/realm/key-server.json", c.HandleKeyServerStats(stats.TypeJSON)).Methods(http.MethodGet)
}

// realmadminRoutes are the realm admin routes.
func realmadminRoutes(r *mux.Router, c *realmadmin.Controller) {
	r.Handle("/settings", c.HandleSettings()).Methods(http.MethodGet, http.MethodPost)
	r.Handle("/settings/enable-express", c.HandleEnableExpress()).Methods(http.MethodPost)
	r.Handle("/settings/disable-express", c.HandleDisableExpress()).Methods(http.MethodPost)
	r.Handle("/stats", c.HandleStats()).Methods(http.MethodGet)
	r.Handle("/events", c.HandleEvents()).Methods(http.MethodGet)
}

// jwksRoutes are the JWK routes, rooted at /jwks.
func jwksRoutes(r *mux.Router, c *jwks.Controller) {
	r.Handle("/{realm_id:[0-9]+}", c.HandleIndex()).Methods(http.MethodGet)
}

// systemAdminRoutes are the system routes, rooted at /admin.
func systemAdminRoutes(r *mux.Router, c *admin.Controller) {
	// Redirect / to /admin/realms
	r.Handle("", http.RedirectHandler("/admin/realms", http.StatusSeeOther)).Methods(http.MethodGet)
	r.Handle("/", http.RedirectHandler("/admin/realms", http.StatusSeeOther)).Methods(http.MethodGet)

	r.Handle("/realms", c.HandleRealmsIndex()).Methods(http.MethodGet)
	r.Handle("/realms", c.HandleRealmsCreate()).Methods(http.MethodPost)
	r.Handle("/realms/new", c.HandleRealmsCreate()).Methods(http.MethodGet)
	r.Handle("/realms/{id:[0-9]+}/edit", c.HandleRealmsUpdate()).Methods(http.MethodGet)
	r.Handle("/realms/{realm_id:[0-9]+}/add/{user_id:[0-9]+}", c.HandleRealmsAdd()).Methods(http.MethodPatch)
	r.Handle("/realms/{realm_id:[0-9]+}/remove/{user_id:[0-9]+}", c.HandleRealmsRemove()).Methods(http.MethodPatch)
	r.Handle("/realms/{id:[0-9]+}", c.HandleRealmsUpdate()).Methods(http.MethodPatch)

	r.Handle("/users", c.HandleUsersIndex()).Methods(http.MethodGet)
	r.Handle("/users/{id:[0-9]+}", c.HandleUserShow()).Methods(http.MethodGet)
	r.Handle("/users/{id:[0-9]+}", c.HandleUserDelete()).Methods(http.MethodDelete)
	r.Handle("/users", c.HandleSystemAdminCreate()).Methods(http.MethodPost)
	r.Handle("/users/new", c.HandleSystemAdminCreate()).Methods(http.MethodGet)
	r.Handle("/users/{id:[0-9]+}/revoke", c.HandleSystemAdminRevoke()).Methods(http.MethodDelete)

	r.Handle("/mobile-apps", c.HandleMobileAppsShow()).Methods(http.MethodGet)
	r.Handle("/sms", c.HandleSMSUpdate()).Methods(http.MethodGet, http.MethodPost)
	r.Handle("/email", c.HandleEmailUpdate()).Methods(http.MethodGet, http.MethodPost)
	r.Handle("/events", c.HandleEventsShow()).Methods(http.MethodGet)

	r.Handle("/caches", c.HandleCachesIndex()).Methods(http.MethodGet)
	r.Handle("/caches/clear/{id}", c.HandleCachesClear()).Methods(http.MethodPost)

	r.Handle("/info", c.HandleInfoShow()).Methods(http.MethodGet)
}
