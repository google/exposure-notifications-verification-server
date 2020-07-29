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

// Adds an SMS configuration to a realm.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/sethvargo/go-envconfig/pkg/envconfig"
	"github.com/sethvargo/go-signalcontext"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

var (
	realmID    = flag.Int64("realm", -1, "realm ID for which to add config")
	accountSid = flag.String("twilio-account-sid", "", "account sid")
	authToken  = flag.String("twilio-auth-token", "", "auth token, will be stored in secret manager")

	project    = flag.String("project", os.Getenv("GOOGLE_CLOUD_PROJECT"), "project in which to store the secret")
	secretName = flag.String("secret", "twilio-auth-token", "name of the secret to create")
	from       = flag.String("from", "", "from number")
)

func main() {
	ctx, done := signalcontext.OnInterrupt()
	err := realMain(ctx)
	done()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "successfully created sms config\n")
}

func realMain(ctx context.Context) error {
	flag.Parse()

	if *accountSid == "" {
		return fmt.Errorf("--twilio-account-sid is required")
	}

	if *authToken == "" {
		return fmt.Errorf("--twilio-auth-token is required")
	}

	if *project == "" {
		return fmt.Errorf("--project is required")
	}

	if *secretName == "" {
		return fmt.Errorf("--secret is required")
	}

	if *from == "" {
		return fmt.Errorf("--from is required")
	}

	var config database.Config
	if err := envconfig.Process(ctx, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	db, err := config.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	realm, err := db.GetRealm(*realmID)
	if err != nil {
		return fmt.Errorf("invalid --realm ID passed: %v, %v", *realmID, err)
	}
	log.Printf("Updating SMS config for realm: %v", realm.Name)

	// Create the secret
	pointer, err := createSecret(ctx, *project, *secretName, *authToken)
	if err != nil {
		return err
	}

	// Read the existing SMS config for the realm if there is on.
	if smsConfig, err := realm.GetSMSConfig(ctx, db); err != nil {
		return fmt.Errorf("unable to read existing SMS config for realm: %w", err)
	} else if smsConfig == nil {
		realm.SMSConfig = &database.SMSConfig{}
	}
	realm.SMSConfig.ProviderType = sms.ProviderType("TWILIO")
	realm.SMSConfig.TwilioAccountSid = *accountSid
	realm.SMSConfig.TwilioAuthToken = pointer
	realm.SMSConfig.TwilioFromNumber = *from
	if err := db.SaveRealm(realm); err != nil {
		return fmt.Errorf("failed to create sms entry: %w", err)
	}

	return nil
}

// createSecret creates a new secret or adds a new version to an existing
// secret.
func createSecret(ctx context.Context, project, id, payload string) (string, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create secret client: %w", err)
	}

	if _, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", project),
		SecretId: id,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}); err != nil {
		if terr, ok := grpcstatus.FromError(err); !ok || terr.Code() != grpccodes.AlreadyExists {
			return "", fmt.Errorf("failed to create secret: %w", err)
		}
	}

	version, err := client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: fmt.Sprintf("projects/%s/secrets/%s", project, id),
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(payload),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to add secret version: %v", err)
	}

	return version.Name, nil
}
