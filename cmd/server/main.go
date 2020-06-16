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

	"github.com/gin-gonic/gin"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/home"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/index"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/session"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/signout"

	// Automagically register concrete implementations.
	_ "github.com/google/exposure-notifications-verification-server/pkg/gcpkms"
)

func main() {
	ctx := context.Background()
	config, err := config.New(ctx)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	router := gin.Default()
	router.LoadHTMLGlob(config.KoDataPath + "/*")

	db, err := config.Database.Open()
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer db.Close()

	//signer, err := signer.NewDefault(ctx)
	//if err != nil {
	//	log.Fatalf("no key manager: %v", err)
	//}

	sessions := controller.NewSessionHelper(config, db)

	// Handlers for landing, signin, signout.
	indexController := index.New(config)
	router.GET("/", indexController.Execute)
	signoutController := signout.New(config, db, sessions)
	router.GET("/signout", signoutController.Execute)
	sessionController := session.New(ctx, config, db, sessions)
	router.POST("/session", sessionController.Execute)

	homeController := home.New(ctx, config, db, sessions)
	router.GET("/home", homeController.Execute)

	issueAPIController := issueapi.New(ctx, config, db, sessions)
	router.POST("/api/issue", issueAPIController.Execute)

	/* TODO(mikehelmick) - change to 2 step code <-> token exchange.
	verifyAPIController := verify.New(db, signer, signingKey)
	router.POST("/api/verify", verifyAPIController.Execute)
	*/

	// Admin pages
	apiKeyList := apikey.NewListController(ctx, config, db, sessions)
	router.GET("/apikeys", apiKeyList.Execute)
	apiKeySave := apikey.NewSaveController(ctx, config, db, sessions)
	router.POST("/apikeys/create", apiKeySave.Execute)

	router.Run()
}
