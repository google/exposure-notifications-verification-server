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
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/buildinfo"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/associated"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/redirect"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/gorilla/mux"
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

	cfg, err := config.NewRedirectConfig(ctx)
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

	// Create the router
	r := mux.NewRouter()

	// Create the renderer
	h, err := render.New(ctx, cfg.AssetsPath, cfg.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	// Install common security headers
	r.Use(middleware.SecureHeaders(ctx, cfg.DevMode, "html"))

	// Enable debug headers
	processDebug := middleware.ProcessDebug(ctx)
	r.Use(processDebug)

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
	{ // .well-known directory
		wk := r.PathPrefix("/.well-known").Subrouter()

		// Enable the iOS and Android redirect handler.
		assocHandler, err := associated.New(ctx, cfg, db, cacher, h)
		if err != nil {
			return fmt.Errorf("failed to create associated links handler %w", err)
		}
		wk.PathPrefix("/apple-app-site-association").Handler(assocHandler.HandleIos()).Methods("GET")
		wk.PathPrefix("/assetlinks.json").Handler(assocHandler.HandleAndroid()).Methods("GET")
	}

	r.Handle("/health", controller.HandleHealthz(ctx, nil, h)).Methods("GET")

	redirectController, err := redirect.New(ctx, db, cfg, cacher, h)
	if err != nil {
		return err
	}
	r.PathPrefix("/").Handler(redirectController.HandleIndex()).Methods("GET")

	mux := http.NewServeMux()
	mux.Handle("/", r)

	srv, err := server.New(cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", cfg.Port)
	return srv.ServeHTTPHandler(ctx, mux)
}
