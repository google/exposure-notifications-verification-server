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
	"github.com/google/exposure-notifications-verification-server/pkg/controller/home"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/index"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/session"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/signout"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	// Automagically register concrete implementations.
	_ "github.com/google/exposure-notifications-verification-server/pkg/datastore"
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

	db, err := database.NewDefault(ctx)
	if err != nil {
		log.Fatalf("no database: %v", err)
	}
	//signer, err := signer.NewDefault(ctx)
	//if err != nil {
	//	log.Fatalf("no key manager: %v", err)
	//}

	sessions := controller.NewSessionHelper(config, db)

	// Handlers for landing, signin, signout.
	indexController := index.New(config)
	router.GET("/", indexController.Execute)
	signoutControlelr := signout.New(config, db, sessions)
	router.GET("/signout", signoutControlelr.Execute)
	sessionController := session.New(ctx, config, db, sessions)
	router.POST("/session", sessionController.Execute)

	homeController := home.New(ctx, config, db, sessions)
	router.GET("/home", homeController.Execute)

	/* Temporarily disable the OTP issuance and verification piece.
	   Focusing on hooking up authn/authz.
	issueController := issue.New("http://localhost:8080")
	router.POST("/issue", issueController.Execute)

	issueAPIController := issueapi.New(db)
	router.POST("/api/issue", issueAPIController.Execute)

	verifyAPIController := verify.New(db, signer, signingKey)
	router.POST("/api/verify", verifyAPIController.Execute)
	*/

	router.Run()
}
