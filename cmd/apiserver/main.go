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

// This server implements the device facing APIs for exchaning verification codes
// for tokens and tokens for certificates.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/verifyapi"
	"github.com/google/exposure-notifications-verification-server/pkg/gcpkms"

	"github.com/google/exposure-notifications-server/pkg/cache"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/ulule/limiter/v3"
	limithttp "github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func main() {
	ctx := context.Background()
	config, err := config.NewAPIServerConfig(ctx)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	// Setup database
	db, err := config.Database.Open()
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer db.Close()

	// Setup signer
	signer, err := gcpkms.New(ctx)
	if err != nil {
		log.Fatalf("error creating KeyManager: %v", err)
	}

	r := mux.NewRouter()
	// Setup rate limiter
	limitInstance := limiter.New(memory.NewStore(), limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  int64(config.RateLimit),
	}, limiter.WithTrustForwardHeader(true))
	r.Use(limithttp.NewMiddleware(limitInstance).Handler)

	apiKeyCache, err := cache.New(config.APIKeyCacheDuration)
	if err != nil {
		log.Fatalf("error establishing API Key cache: %v", err)
	}
	// Install the APIKey Auth Middleware
	r.Use(middleware.APIKeyAuth(ctx, db, apiKeyCache).Handle)

	publicKeyCache, err := cache.New(config.PublicKeyCacheDuration)
	if err != nil {
		log.Fatalf("error establishing Public Key Cache: %v", err)
	}

	r.Handle("/api/verify", verifyapi.New(ctx, config, db, signer)).Methods("POST")
	r.Handle("/api/certificate", certapi.New(ctx, config, db, signer, publicKeyCache)).Methods("POST")

	srv := &http.Server{
		Handler: handlers.CombinedLoggingHandler(os.Stdout, r),
		Addr:    "0.0.0.0:" + strconv.Itoa(config.Port),
	}
	log.Printf("Listening on: 127.0.0.1:%v", config.Port)
	log.Fatal(srv.ListenAndServe())
}
