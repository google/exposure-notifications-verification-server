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
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issue"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/verify"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/signer"

	// Automagically register concrete implementations.
	_ "github.com/google/exposure-notifications-verification-server/pkg/gcpkms"
	_ "github.com/google/exposure-notifications-verification-server/pkg/inmemory"
)

func templateDir() string {
	koPath := os.Getenv("KO_DATA_PATH")
	if koPath == "" {
		koPath = "./cmd/server/kodata"
	}
	return koPath
}

func main() {
	// Needs better configuration
	signingKey := os.Getenv("SIGNING_KEY")
	if signingKey == "" {
		log.Fatalf("Must set env variable `SIGNING_KEY` with reference to GCP KMS Key Version to us.")
	}

	router := gin.Default()
	router.LoadHTMLGlob(templateDir() + "/*")

	ctx := context.Background()
	db, err := database.NewDefault(ctx)
	if err != nil {
		log.Fatalf("no database: %v", err)
	}
	signer, err := signer.NewDefault(ctx)
	if err != nil {
		log.Fatalf("no key manager: %v", err)
	}

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index", gin.H{})
	})

	issueController := issue.New("http://localhost:8080")
	router.POST("/issue", issueController.Execute)

	issueAPIController := issueapi.New(db)
	router.POST("/api/issue", issueAPIController.Execute)

	verifyAPIController := verify.New(db, signer, signingKey)
	router.POST("/api/verify", verifyAPIController.Execute)

	router.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
