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

// This server implements the database cleanup. The server itself is unauthenticated
// and should not be deployed as a public service.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/cleanup"

	"github.com/google/exposure-notifications-server/pkg/cache"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/ulule/limiter/v3"
	limithttp "github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func main() {
	ctx := context.Background()
	config, err := config.NewCleanupConfig(ctx)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	// Setup database
	db, err := config.Database.Open()
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer db.Close()

	r := mux.NewRouter()

	// Setup rate limiter
	limitInstance := limiter.New(memory.NewStore(), limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  int64(config.RateLimit),
	}, limiter.WithTrustForwardHeader(true))
	r.Use(limithttp.NewMiddleware(limitInstance).Handler)

	// Cleanup handler doesn't require authentication - does use locking to ensure
	// database isn't tipped over by cleanup.
	cleanupCache, err := cache.New(time.Minute)
	if err != nil {
		log.Fatalf("error creating cleanup cache: %v", err)
	}
	r.Handle("/", cleanup.New(ctx, config, cleanupCache, db)).Methods("GET")

	srv := &http.Server{
		Handler: handlers.CombinedLoggingHandler(os.Stdout, r),
		Addr:    "0.0.0.0:" + strconv.Itoa(config.Port),
	}
	log.Printf("Listening on: 127.0.0.1:%v", config.Port)
	log.Fatal(srv.ListenAndServe())
}
