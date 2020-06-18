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
	"log"
	"time"

	firebase "firebase.google.com/go"
	"github.com/gin-gonic/gin"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/cleanup"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/home"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/index"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/session"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/signout"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/verifyapi"
	"github.com/google/exposure-notifications-verification-server/pkg/gcpkms"

	"github.com/google/exposure-notifications-server/pkg/cache"
)

func main() {
	ctx := context.Background()
	config, err := config.New(ctx)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	// Setup database
	db, err := config.Database.Open()
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer db.Close()

	// Setup firebase
	app, err := firebase.NewApp(ctx, config.FirebaseConfig())
	if err != nil {
		log.Fatalf("failed to setup firebase: %v", err)
	}
	auth, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("failed to configure firebase auth: %v", err)
	}

	// Setup signer
	signer, err := gcpkms.New(ctx)
	if err != nil {
		log.Fatalf("error creating KeyManager: %v", err)
	}

	// Create the main router
	router := gin.Default()
	router.LoadHTMLGlob(config.AssetsPath + "/*")
	router.Use(middleware.FlashHandler(ctx))

	// Handlers for landing, signin, signout.
	indexController := index.New(config)
	router.GET("/", indexController.Execute)
	signoutController := signout.New(config, db)
	router.GET("/signout", signoutController.Execute)
	sessionController := session.New(ctx, config, db)
	router.POST("/session", sessionController.Execute)

	// Cleanup handler doesn't require authentication - does use locking to ensure
	// database isn't tipped over by cleanup.
	cleanupCache, _ := cache.New(time.Minute)
	cleanupController := cleanup.New(ctx, config, cleanupCache, db)
	router.GET("/cleanup", cleanupController.Execute)

	// User pages, requires auth
	{
		router := router.Group("/")
		router.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration))

		homeController := home.New(ctx, config, db)
		router.GET("/home", homeController.Execute)

		// API for creating new verification codes. Called via AJAX.
		issueAPIController := issueapi.New(ctx, config, db)
		router.POST("/api/issue", issueAPIController.Execute)
	}

	// Admin pages, requires admin auth
	{
		router := router.Group("/")
		router.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration))
		router.Use(middleware.RequireAdmin(ctx))

		apiKeyList := apikey.NewListController(ctx, config, db)
		router.GET("/apikeys", apiKeyList.Execute)

		apiKeySave := apikey.NewSaveController(ctx, config, db)
		router.POST("/apikeys/create", apiKeySave.Execute)
	}

	// Device APIs for exchanging short for long term tokens and signing TEKs with
	// long term tokens. The group requires API Key based auth.
	{
		apiKeyCache, err := cache.New(config.APIKeyCacheDuration)
		if err != nil {
			log.Fatalf("error establishing API Key cache: %v", err)
		}
		publicKeyCache, err := cache.New(config.PublicKeyCacheDuration)
		if err != nil {
			log.Fatalf("error establishing Public Key Cache: %v", err)
		}
		apiKeyGroup := router.Group("/", middleware.APIKeyAuth(ctx, db, apiKeyCache))
		verifyAPI := verifyapi.New(ctx, config, db, signer)
		apiKeyGroup.POST("/api/verify", verifyAPI.Execute) // /api/verify
		certAPI := certapi.New(ctx, config, db, signer, publicKeyCache)
		apiKeyGroup.POST("/api/certificate", certAPI.Execute)
	}

	router.Run()
}
