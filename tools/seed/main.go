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

// Package main provides a utility that bootstraps the initial database with
// users and realms.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	firebase "firebase.google.com/go"
	firebaseauth "firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	debug, _ := strconv.ParseBool(os.Getenv("LOG_DEBUG"))
	logger := logging.NewLogger(debug)
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx).Named("seed")

	// Database
	var dbConfig database.Config
	if err := config.ProcessWith(ctx, &dbConfig, envconfig.OsLookuper()); err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	db, err := dbConfig.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Firebase
	var fbConfig config.FirebaseConfig
	if err := config.ProcessWith(ctx, &fbConfig, envconfig.OsLookuper()); err != nil {
		return fmt.Errorf("failed to parse firebase config: %w", err)
	}

	fb, err := firebase.NewApp(ctx, &firebase.Config{
		DatabaseURL:   fbConfig.DatabaseURL,
		ProjectID:     fbConfig.ProjectID,
		StorageBucket: fbConfig.StorageBucket,
	})
	if err != nil {
		return fmt.Errorf("failed to setup firebase: %w", err)
	}
	firebaseAuth, err := fb.Auth(ctx)
	if err != nil {
		return fmt.Errorf("failed to configure firebase: %w", err)
	}

	// Create a realm
	realm1 := database.NewRealmWithDefaults("Narnia")
	realm1.RegionCode = "US-PA"
	realm1.AbusePreventionEnabled = true
	if err := db.SaveRealm(realm1, database.System); err != nil {
		return fmt.Errorf("failed to create realm: %w: %v", err, realm1.ErrorMessages())
	}
	logger.Infow("created realm", "realm", realm1)

	// Create another realm
	realm2 := database.NewRealmWithDefaults("Wonderland")
	realm2.AllowedTestTypes = database.TestTypeLikely | database.TestTypeConfirmed
	realm2.RegionCode = "US-WA"
	realm2.AbusePreventionEnabled = true
	if err := db.SaveRealm(realm2, database.System); err != nil {
		return fmt.Errorf("failed to create realm: %w: %v", err, realm2.ErrorMessages())
	}
	logger.Infow("created realm", "realm", realm2)

	// Create users
	user := &database.User{Email: "user@example.com", Name: "Demo User"}
	if _, err := db.FindUserByEmail(user.Email); database.IsNotFound(err) {
		user.AddRealm(realm1)
		user.AddRealm(realm2)
		if err := db.SaveUser(user, database.System); err != nil {
			return fmt.Errorf("failed to create user: %w: %v", err, user.ErrorMessages())
		}
		logger.Infow("created user", "user", user)
	}

	if err := createFirebaseUser(ctx, firebaseAuth, user); err != nil {
		return err
	}
	logger.Infow("enabled user", "user", user)

	unverified := &database.User{Email: "unverified@example.com", Name: "Unverified User"}
	if _, err := db.FindUserByEmail(unverified.Email); database.IsNotFound(err) {
		unverified.AddRealm(realm1)
		if err := db.SaveUser(unverified, database.System); err != nil {
			return fmt.Errorf("failed to create unverified: %w: %v", err, unverified.ErrorMessages())
		}
		logger.Infow("created user", "user", unverified)
	}

	admin := &database.User{Email: "admin@example.com", Name: "Admin User"}
	if _, err := db.FindUserByEmail(admin.Email); database.IsNotFound(err) {
		admin.AddRealm(realm1)
		admin.AddRealmAdmin(realm1)
		if err := db.SaveUser(admin, database.System); err != nil {
			return fmt.Errorf("failed to create admin: %w: %v", err, admin.ErrorMessages())
		}
		logger.Infow("created admin", "admin", admin)
	}

	if err := createFirebaseUser(ctx, firebaseAuth, admin); err != nil {
		return err
	}
	logger.Infow("enabled admin", "admin", admin)

	super := &database.User{Email: "super@example.com", Name: "Super User", Admin: true}
	if _, err := db.FindUserByEmail(super.Email); database.IsNotFound(err) {
		if err := db.SaveUser(super, database.System); err != nil {
			return fmt.Errorf("failed to create super: %w: %v", err, super.ErrorMessages())
		}
		logger.Infow("created super", "super", super)
	}

	if err := createFirebaseUser(ctx, firebaseAuth, super); err != nil {
		return err
	}
	logger.Infow("enabled super", "super", super)

	// Create a device API key
	deviceAPIKey, err := realm1.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       "Corona Capture",
		APIKeyType: database.APIKeyTypeDevice,
	}, admin)
	if err != nil {
		return fmt.Errorf("failed to create device api key: %w", err)
	}
	logger.Infow("created device api key", "key", deviceAPIKey)

	// Create some Apps
	apps := []*database.MobileApp{
		{
			Name:    "Example iOS app",
			RealmID: realm1.ID,
			URL:     "http://apple.com",
			OS:      database.OSTypeIOS,
			AppID:   "ios.example.app",
		},
		{
			Name:    "Example Android app",
			RealmID: realm1.ID,
			URL:     "http://google.com",
			OS:      database.OSTypeAndroid,
			AppID:   "android.example.app",
			SHA:     "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
		},
	}
	for i := range apps {
		app := apps[i]
		if err := db.SaveMobileApp(app, database.System); err != nil {
			return fmt.Errorf("failed to create app: %w", err)
		}
	}

	// Create an admin API key
	adminAPIKey, err := realm1.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       "Tracing Tracker",
		APIKeyType: database.APIKeyTypeAdmin,
	}, admin)
	if err != nil {
		return fmt.Errorf("failed to create admin api key: %w", err)
	}
	logger.Infow("created device api key", "key", adminAPIKey)

	return nil
}

func createFirebaseUser(ctx context.Context, firebaseAuth *firebaseauth.Client, user *database.User) error {
	existing, err := firebaseAuth.GetUserByEmail(ctx, user.Email)
	if err != nil && !firebaseauth.IsUserNotFound(err) {
		return fmt.Errorf("failed to get user by email %v: %w", user.Email, err)
	}

	// User exists, verify email
	if existing != nil {
		// Already verified
		if existing.EmailVerified {
			return nil
		}

		update := (&firebaseauth.UserToUpdate{}).
			EmailVerified(true)

		if _, err := firebaseAuth.UpdateUser(ctx, existing.UID, update); err != nil {
			return fmt.Errorf("failed to update user %v: %w", user.Email, err)
		}

		return nil
	}

	// User does not exist
	create := (&firebaseauth.UserToCreate{}).
		Email(user.Email).
		EmailVerified(true).
		DisplayName(user.Name).
		Password("password")

	if _, err := firebaseAuth.CreateUser(ctx, create); err != nil {
		return fmt.Errorf("failed to create user %v: %w", user.Email, err)
	}

	return nil
}
