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

// Small uiliity to generate random bytes and store them as secrets in Google
// Secret Manager.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"

	"github.com/google/exposure-notifications-server/pkg/logging"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/sethvargo/go-signalcontext"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

var (
	numBytesFlag  = flag.Uint("bytes", 0, "number of bytes to generate")
	idFlag        = flag.String("id", "", "ID fo the secret to generate")
	projectIDFlag = flag.String("project", "", "Google Cloud Project ID")
)

func main() {
	flag.Parse()

	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLoggerFromEnv().Named("gen-secret")
	ctx = logging.WithLogger(ctx, logger)
	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	if *numBytesFlag == 0 {
		return fmt.Errorf("--bytes must be a positive value, got 0")
	}
	if *idFlag == "" {
		return fmt.Errorf("--id must not be empty, this is the secret id")
	}
	if *projectIDFlag == "" {
		return fmt.Errorf("--project must not be empty, this is the GCP project id")
	}

	payload := make([]byte, *numBytesFlag)
	_, err := rand.Read(payload)
	if err != nil {
		return fmt.Errorf("error generating random secret: %w", err)
	}

	// Create the client.
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create secretmanager client: %w", err)
	}

	// Build the request.
	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/" + *projectIDFlag,
		SecretId: *idFlag,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}

	// Call the API to create the secret.
	secret, err := client.CreateSecret(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	if secret != nil {
		logger.Infow("created secret", "secret", secret)
	}

	// Encode to base64 so we can use our secret:// resolution in the server.
	data := base64.StdEncoding.EncodeToString(payload)
	// generate and create secret version.
	addReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: "projects/" + *projectIDFlag + "/secrets/" + *idFlag,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(data),
		},
	}

	// Call the API.
	version, err := client.AddSecretVersion(ctx, addReq)
	if err != nil {
		return fmt.Errorf("failed to add secret version: %w", err)
	}
	logger.Infow("added secret version", "version", version)

	return nil
}
