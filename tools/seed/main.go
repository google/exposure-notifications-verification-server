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
//
//nolint:gosec // We don't need crypto/rand here
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"os/signal"
	"syscall"
	"time"

	firebase "firebase.google.com/go"
	firebaseauth "firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/rotation"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/jinzhu/gorm"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-server/pkg/timeutils"

	"github.com/sethvargo/go-envconfig"
)

var flagStats = flag.Bool("stats", false, "generate codes and statistics")

func main() {
	flag.Parse()

	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	logger := logging.NewLoggerFromEnv().Named("seed")
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

	// System token signing key
	var tokenConfig config.TokenSigningConfig
	if err := config.ProcessWith(ctx, &tokenConfig, envconfig.OsLookuper()); err != nil {
		return fmt.Errorf("failed to process token signing config: %w", err)
	}
	keyManager, err := keys.KeyManagerFor(ctx, &tokenConfig.Keys)
	if err != nil {
		return fmt.Errorf("failed to create token key manager: %w", err)
	}
	keyManagerTyp, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		return fmt.Errorf("token signing key manager is not SigningKeyManager (got %T)", keyManager)
	}

	// Get secret manager.
	var secretsConfig secrets.Config
	if err := config.ProcessWith(ctx, &secretsConfig, envconfig.OsLookuper()); err != nil {
		return fmt.Errorf("failed to process secrets config: %w", err)
	}
	secretManager, err := secrets.SecretManagerFor(ctx, &secretsConfig)
	if err != nil {
		return fmt.Errorf("failed to create secret manager: %w", err)
	}
	secretManagerTyp, ok := secretManager.(secrets.SecretVersionManager)
	if !ok {
		return fmt.Errorf("secret manager is not a secret version manager (is %T)", secretManager)
	}

	// Create a realm
	var realm1 *database.Realm
	realm1, err = db.FindRealmByName("Narnia")
	if err != nil {
		if database.IsNotFound(err) {
			realm1 = database.NewRealmWithDefaults("Narnia")
			realm1.RegionCode = "US-PA"
			realm1.AbusePreventionEnabled = true
			realm1.AddUserReportToAllowedTestTypes() // Enable user reporting on Narnia
			if err := db.SaveRealm(realm1, database.System); err != nil {
				return fmt.Errorf("failed to create realm: %w: %v", err, realm1.ErrorMessages())
			}
			logger.Infow("created realm", "realm", realm1)
		} else {
			return fmt.Errorf("failed to find realm: %w: %v", err, realm1.ErrorMessages())
		}
	}

	// Create another realm
	var realm2 *database.Realm
	realm2, err = db.FindRealmByName("Wonderland")
	if err != nil {
		if database.IsNotFound(err) {
			realm2 = database.NewRealmWithDefaults("Wonderland")
			realm2.AllowedTestTypes = database.TestTypeLikely | database.TestTypeConfirmed
			realm2.RegionCode = "US-WA"
			realm2.AbusePreventionEnabled = true
			if err := db.SaveRealm(realm2, database.System); err != nil {
				return fmt.Errorf("failed to create realm: %w: %v", err, realm2.ErrorMessages())
			}
			logger.Infow("created realm", "realm", realm2)
		} else {
			return fmt.Errorf("failed to find realm: %w: %v", err, realm1.ErrorMessages())
		}
	}

	// Create secrets - note we do this AFTER realm creation so it creates the
	// realm verification keys too.
	if err := createSecrets(ctx, db, keyManagerTyp, secretManagerTyp); err != nil {
		return fmt.Errorf("failed to create secrets: %w", err)
	}

	// Create some system sms from numbers
	if err := db.CreateOrUpdateSMSFromNumbers([]*database.SMSFromNumber{
		{
			Label: "USA",
			Value: "+15005550006",
		},
		{
			Label: "Mexico",
			Value: "55-1234-5678",
		},
	}); err != nil {
		return fmt.Errorf("failed to create sms from numbers: %w", err)
	}
	logger.Infow("created sms from numbers")

	// Create users
	user := &database.User{Email: "user@example.com", Name: "Demo User"}
	if _, err := db.FindUserByEmail(user.Email); database.IsNotFound(err) {
		if err := db.SaveUser(user, database.System); err != nil {
			return fmt.Errorf("failed to create user: %w: %v", err, user.ErrorMessages())
		}
		logger.Infow("created user", "user", user)
	}
	if err := user.AddToRealm(db, realm1, rbac.LegacyRealmUser, database.System); err != nil {
		return fmt.Errorf("failed to add user to realm 1: %w", err)
	}
	if err := user.AddToRealm(db, realm2, rbac.LegacyRealmUser, database.System); err != nil {
		return fmt.Errorf("failed to add user to realm 2: %w", err)
	}

	if err := createFirebaseUser(ctx, firebaseAuth, user); err != nil {
		return err
	}
	logger.Infow("enabled user", "user", user)

	unverified := &database.User{Email: "unverified@example.com", Name: "Unverified User"}
	if _, err := db.FindUserByEmail(unverified.Email); database.IsNotFound(err) {
		if err := db.SaveUser(unverified, database.System); err != nil {
			return fmt.Errorf("failed to create unverified: %w: %v", err, unverified.ErrorMessages())
		}
		logger.Infow("created user", "user", unverified)
	}
	if err := unverified.AddToRealm(db, realm1, rbac.LegacyRealmUser, database.System); err != nil {
		return fmt.Errorf("failed to add user to realm 1: %w", err)
	}

	admin := &database.User{Email: "admin@example.com", Name: "Admin User"}
	if _, err := db.FindUserByEmail(admin.Email); database.IsNotFound(err) {
		if err := db.SaveUser(admin, database.System); err != nil {
			return fmt.Errorf("failed to create admin: %w: %v", err, admin.ErrorMessages())
		}
		logger.Infow("created admin", "admin", admin)
	}
	if err := admin.AddToRealm(db, realm1, rbac.LegacyRealmAdmin, database.System); err != nil {
		return fmt.Errorf("failed to add user to realm 1: %w", err)
	}

	if err := createFirebaseUser(ctx, firebaseAuth, admin); err != nil {
		return err
	}
	logger.Infow("enabled admin", "admin", admin)

	super := &database.User{Email: "super@example.com", Name: "Super User", SystemAdmin: true}
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

	// Create device API keys
	for _, name := range []string{"Corona Capture", "Tracing Tracker"} {
		deviceAPIKey, err := realm1.CreateAuthorizedApp(db, &database.AuthorizedApp{
			Name:       name,
			APIKeyType: database.APIKeyTypeDevice,
		}, admin)
		if err != nil {
			return fmt.Errorf("failed to create device api key: %w", err)
		}
		logger.Infow("created device api key", "key", deviceAPIKey)
	}

	// Create some Apps
	apps := []*database.MobileApp{
		{
			Name:    "Example iOS app",
			RealmID: realm1.ID,
			URL:     "http://google.com/",
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
	for _, name := range []string{"Closet Cloud", "Internal Health System"} {
		adminAPIKey, err := realm1.CreateAuthorizedApp(db, &database.AuthorizedApp{
			Name:       name,
			APIKeyType: database.APIKeyTypeAdmin,
		}, admin)
		if err != nil {
			return fmt.Errorf("failed to create admin api key: %w", err)
		}
		logger.Infow("created admin api key", "key", adminAPIKey)
	}

	if *flagStats {
		maxPerDay, err := generateCodesAndStats(db, realm1)
		if err != nil {
			return fmt.Errorf("failed to generate stats: %w", err)
		}

		if err := generateKeyServerStats(db, realm1, maxPerDay); err != nil {
			return fmt.Errorf("failed to generate key-server stats: %w", err)
		}
	}

	return nil
}

// generateCodesAndStats exercises the system for the past 30 days with random
// values to simulate data that might appear in the real world. This is
// primarily used to test statistics and graphs.
func generateCodesAndStats(db *database.Database, realm *database.Realm) (map[string]int, error) {
	now := time.Now().UTC()

	users, _, err := db.ListUsers(pagination.UnlimitedResults)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	randomUser := func() *database.User {
		return users[rand.Intn(len(users))]
	}

	adminAuthorizedApps, _, err := realm.ListAuthorizedApps(db, pagination.UnlimitedResults,
		database.WithAuthorizedAppType(database.APIKeyTypeAdmin))
	if err != nil {
		return nil, fmt.Errorf("failed to list admin authorized apps: %w", err)
	}
	randomAdminAuthorizedApp := func() *database.AuthorizedApp {
		return adminAuthorizedApps[rand.Intn(len(adminAuthorizedApps))]
	}

	deviceAuthorizedApps, _, err := realm.ListAuthorizedApps(db, pagination.UnlimitedResults,
		database.WithAuthorizedAppType(database.APIKeyTypeDevice))
	if err != nil {
		return nil, fmt.Errorf("failed to list device authorized apps: %w", err)
	}
	randomDeviceAuthorizedApp := func() *database.AuthorizedApp {
		return deviceAuthorizedApps[rand.Intn(len(deviceAuthorizedApps))]
	}

	externalIDs := make([]string, 4)
	for i := range externalIDs {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("failed to read rand: %w", err)
		}
		externalIDs[i] = hex.EncodeToString(b)
	}
	randomExternalID := func() string {
		return externalIDs[rand.Intn(len(externalIDs))]
	}

	// Temporarily disable gorm logging, it's very chatty and increases the seed
	// time significantly.
	db.RawDB().LogMode(false)
	defer db.RawDB().LogMode(true)

	phoneNumber := uint64(10000000000)
	nonce := make([]byte, database.NonceLength)
	allowsUserReport := realm.AllowsUserReport()
	ctx := context.Background()

	tokensClaimedPerDay := make(map[string]int)

	for day := 1; day <= 30; day++ {
		max := rand.Intn(250)
		totalClaimed := 0
		date := now.Add(time.Duration(day) * -24 * time.Hour)
		for i := 0; i < max; i++ {
			// create local version for use for this sequence.
			date := date

			issuingUserID := uint(0)
			issuingAppID := uint(0)
			issuingExternalID := ""
			isUserReport := false

			// Random determine if this was issued by an app.
			if percentChance(50) {
				issuingAppID = randomAdminAuthorizedApp().ID

				// Random determine if the code had an external audit.
				if rand.Intn(2) == 0 {
					b := make([]byte, 8)
					if _, err := rand.Read(b); err != nil {
						return nil, fmt.Errorf("failed to read rand: %w", err)
					}
					issuingExternalID = randomExternalID()
				}
			} else if allowsUserReport && percentChance(30) {
				// Random chance that this is a user report.
				isUserReport = true
			} else {
				issuingUserID = randomUser().ID
			}

			code := fmt.Sprintf("%08d", rand.Intn(99999999))
			longCode := fmt.Sprintf("%015d", rand.Intn(999999999999999))
			testDate := now.Add(-48 * time.Hour)
			testType := "confirmed"

			verificationCode := &database.VerificationCode{
				Model: gorm.Model{
					CreatedAt: date,
				},
				RealmID:       realm.ID,
				Code:          code,
				ExpiresAt:     now.Add(15 * time.Minute),
				LongCode:      longCode,
				LongExpiresAt: now.Add(15 * 24 * time.Hour),
				TestType:      testType,
				SymptomDate:   &testDate,
				TestDate:      &testDate,

				IssuingUserID:     issuingUserID,
				IssuingAppID:      issuingAppID,
				IssuingExternalID: issuingExternalID,
			}
			if isUserReport {
				phoneNumber++
				verificationCode.PhoneNumber = fmt.Sprintf("+%d", phoneNumber)
				verificationCode.Nonce = nonce
				verificationCode.NonceRequired = true
				verificationCode.TestType = api.TestTypeUserReport
				testType = api.TestTypeUserReport
			}

			// If a verification code already exists, it will fail to save, and we retry.
			if err := db.SaveVerificationCode(verificationCode, realm); err != nil {
				return nil, fmt.Errorf("failed to create verification code: %w", err)
			}
			db.UpdateStats(ctx, verificationCode)

			// Determine if a code is claimed.
			if percentChance(90) {
				accept := map[string]struct{}{
					api.TestTypeConfirmed:  {},
					api.TestTypeLikely:     {},
					api.TestTypeNegative:   {},
					api.TestTypeUserReport: {},
				}

				// Some percentage of codes will fail to claim - force this by changing
				// the allowed test types to exclude "confirmed".
				if percentChance(30) {
					delete(accept, api.TestTypeConfirmed)
				}

				app := randomDeviceAuthorizedApp()

				// randomize issue to claim time
				if percentChance(25) {
					date = date.Add(time.Duration(rand.Intn(12))*time.Hour + time.Second)
				} else {
					date = date.Add(time.Duration(rand.Intn(60))*time.Minute + time.Second)
				}

				os := database.OSTypeUnknown
				if percentChance(99) {
					if percentChance(50) {
						os = database.OSTypeAndroid
					} else {
						os = database.OSTypeIOS
					}
				}

				request := &database.IssueTokenRequest{
					Time:        date,
					AuthApp:     app,
					VerCode:     longCode,
					AcceptTypes: accept,
					ExpireAfter: 24 * time.Hour,
					OS:          os,
				}
				if isUserReport {
					request.Nonce = nonce
				}
				token, err := db.VerifyCodeAndIssueToken(request)
				if err != nil {
					continue
				}

				// Determine if token is exchanged.
				if percentChance(75) {
					testType := testType

					// Determine if token claim should fail. Override the testType to
					// force the subject to mismatch.
					if percentChance(20) {
						testType = "likely"
					}

					if err := db.ClaimToken(date, app, token.TokenID, &database.Subject{
						TestType:    testType,
						SymptomDate: &testDate,
						TestDate:    &testDate,
					}); err != nil {
						continue
					}
					totalClaimed++
				}
			}
		}
		tokensClaimedPerDay[date.Format("2006-01-02")] = totalClaimed
	}

	return tokensClaimedPerDay, nil
}

// generateKeyServerStats generates stats normally gathered from a key-server. This is
// primarily used to test statistics and graphs.
func generateKeyServerStats(db *database.Database, realm *database.Realm, maxPerDay map[string]int) error {
	if err := db.SaveKeyServerStats(&database.KeyServerStats{RealmID: realm.ID}); err != nil {
		return fmt.Errorf("failed create stats config: %w", err)
	}

	midnight := timeutils.UTCMidnight(time.Now())
	for day := 0; day < 30; day++ {
		date := midnight.Add(time.Duration(day) * -24 * time.Hour)

		max := 20 // lower default, otherwise generate realistic numbers.
		if v, ok := maxPerDay[date.Format("2006-01-02")]; ok {
			max = v
		}
		teksPublished := int64(max * 14)
		if teksPublished > 0 {
			teksPublished = rand.Int63n(teksPublished)
		}
		revisions := int64(max / 10)
		if revisions > 0 {
			revisions = rand.Int63n(revisions)
		}
		var missingOnset int64
		if limit := max / 4; limit > 0 {
			missingOnset = rand.Int63n(int64(limit))
		}

		day := &database.KeyServerStatsDay{
			RealmID:                   realm.ID,
			Day:                       date,
			PublishRequests:           randArr63n(int64(max), 3),
			TotalTEKsPublished:        teksPublished,
			RevisionRequests:          revisions,
			TEKAgeDistribution:        randArr63n(int64(max), 16),
			OnsetToUploadDistribution: randArr63n(15, 31),
			RequestsMissingOnsetDate:  missingOnset,
		}
		if err := db.SaveKeyServerStatsDay(day); err != nil {
			return fmt.Errorf("failed create stats day: %w", err)
		}
	}

	return nil
}

func randArr63n(n, length int64) []int64 {
	arr := make([]int64, length)
	for i := int64(0); i < length; i++ {
		if n > 0 {
			arr[i] = rand.Int63n(n)
		}
	}
	return arr
}

func percentChance(d int) bool {
	return rand.Intn(100) <= d
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

// createSecrets creates secrets. It re-uses the rotation worker logic and
// invokes the handler.
func createSecrets(ctx context.Context, db *database.Database, keyManager keys.SigningKeyManager, secretManager secrets.SecretVersionManager) error {
	cfg, err := config.NewRotationConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to create rotation config: %w", err)
	}

	h, err := render.New(ctx, nil, cfg.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	rotationController := rotation.New(cfg, db, keyManager, secretManager, h)

	if err := rotationController.RotateSecrets(ctx); err != nil {
		return fmt.Errorf("failed to create initial secrets: %w", err)
	}

	if err := rotationController.RotateTokenSigningKey(ctx); err != nil {
		return fmt.Errorf("failed to create initial token signing key: %w", err)
	}

	if err := rotationController.RotateVerificationKeys(ctx); err != nil {
		return fmt.Errorf("failed to create initial verification keys: %w", err)
	}

	return nil
}
