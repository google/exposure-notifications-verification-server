package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mikehelmick/tek-verification-server/pkg/controller/issue"
	"github.com/mikehelmick/tek-verification-server/pkg/controller/issueapi"
	"github.com/mikehelmick/tek-verification-server/pkg/controller/verify"
	"github.com/mikehelmick/tek-verification-server/pkg/database"
	"github.com/mikehelmick/tek-verification-server/pkg/signer"

	// Automagically register concrete implementations.
	_ "github.com/mikehelmick/tek-verification-server/pkg/gcpkms"
	_ "github.com/mikehelmick/tek-verification-server/pkg/inmemory"
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
