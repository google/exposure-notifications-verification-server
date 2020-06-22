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
	"log"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

func main() {
	numBytesFlag := flag.Uint("bytes", 0, "number of bytes to generate")
	idFlag := flag.String("id", "", "ID fo the secret to generate")
	projectIDFlag := flag.String("project", "", "Google Cloud Project ID")
	flag.Parse()

	if *numBytesFlag == 0 {
		log.Fatalf("--bytes must be a positive value, got 0")
	}
	if *idFlag == "" {
		log.Fatalf("--id must not be empty, this is the secret id")
	}
	if *projectIDFlag == "" {
		log.Fatalf("--project must not be empty, this is the GCP project id")
	}

	payload := make([]byte, *numBytesFlag)
	_, err := rand.Read(payload)
	if err != nil {
		log.Fatalf("error generating random secret: %v", err)
	}

	// Create the client.
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create secretmanager client: %v", err)
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
	log.Printf("REQ: %+v", req)

	// Call the API to create the secret.
	result, err := client.CreateSecret(ctx, req)
	if err != nil {
		log.Printf("failed to create secret: %v", err)
	}
	if result != nil {
		log.Printf("Created secret: %v", result.Name)
	}
	log.Printf("Creating secret version.")

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
	addResult, err := client.AddSecretVersion(ctx, addReq)
	if err != nil {
		log.Fatalf("failed to add secret version: %v", err)
	}
	log.Printf("Added secret version: %s", addResult.Name)
}
