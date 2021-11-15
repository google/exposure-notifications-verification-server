// Copyright 2021 the Exposure Notifications Verification Server authors
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

package envstest

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/hashicorp/go-multierror"
)

const (
	RealmName  = "e2e-test-realm"
	RegionCode = "e2e-test"

	AdminKeyPrefix  = "e2e-admin-key."
	DeviceKeyPrefix = "e2e-device-key."
	StatsKeyPrefix  = "e2e-stats-key."

	AndroidAppPrefix = "e2e-android-"
	IOSAppPrefix     = "e2e-ios-"
)

// BootstrapResponse is the response from Bootstrap.
type BootstrapResponse struct {
	Realm        *database.Realm
	AdminAPIKey  string
	DeviceAPIKey string
	StatsAPIKey  string

	closers []func() error
}

// Bootstrap configures the database with an e2e realm (or re-uses one that
// already exists), and provisions new authorized apps for accessing the admin
// apis, device apis, and stats apis.
//
// It also provisions and enables the e2e realm. If the realm already exists, it
// updates the realms settings to enable settings and configuration that e2e
// expects.
//
// Callers should always call Cleanup() on the response to ensure temporary
// resources are purged.
func Bootstrap(ctx context.Context, db *database.Database) (*BootstrapResponse, error) {
	var resp BootstrapResponse

	// Find or create the test realm with the correct properties.
	realm, err := db.FindRealmByName(RealmName)
	if err != nil {
		if !database.IsNotFound(err) {
			return &resp, fmt.Errorf("failed to find realm: %w", err)
		}

		realm = database.NewRealmWithDefaults(RealmName)
		realm.RegionCode = RegionCode
		realm.IsE2E = true
	}

	logger := logging.FromContext(ctx)

	realm.AllowedTestTypes = database.TestTypeNegative | database.TestTypeConfirmed | database.TestTypeLikely
	realm.AddUserReportToAllowedTestTypes()
	realm.AllowBulkUpload = true
	realm.UseAuthenticatedSMS = true
	realm.AllowAdminUserReport = true
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		logger.Errorw("failed to save realm",
			"realm", realm,
			"err", err,
			"validation", realm.ErrorMessages())
		return &resp, fmt.Errorf("failed to save realm: %w", err)
	}
	resp.Realm = realm

	// Generate random entropy for resource names.
	entropy, err := project.RandomHexString(6)
	if err != nil {
		return &resp, fmt.Errorf("failed to create entropy: %w", err)
	}

	// Create admin key.
	adminAPIKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       AdminKeyPrefix + entropy,
		APIKeyType: database.APIKeyTypeAdmin,
	}, database.SystemTest)
	if err != nil {
		return &resp, fmt.Errorf("failed to create admin api key: %w", err)
	}
	resp.closers = append(resp.closers, func() error {
		return deleteAuthorizedApp(db, adminAPIKey)
	})

	// Create device key.
	deviceAPIKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       DeviceKeyPrefix + entropy,
		APIKeyType: database.APIKeyTypeDevice,
	}, database.SystemTest)
	if err != nil {
		return &resp, fmt.Errorf("failed to create device api key: %w", err)
	}
	resp.closers = append(resp.closers, func() error {
		return deleteAuthorizedApp(db, deviceAPIKey)
	})

	// Create stats key.
	statsAPIKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       StatsKeyPrefix + entropy,
		APIKeyType: database.APIKeyTypeStats,
	}, database.SystemTest)
	if err != nil {
		return &resp, fmt.Errorf("failed to create stats api key: %w", err)
	}
	resp.closers = append(resp.closers, func() error {
		return deleteAuthorizedApp(db, statsAPIKey)
	})

	// Create iOS mobile app.
	iosName := IOSAppPrefix + entropy
	iosApp := &database.MobileApp{
		Name:    iosName,
		RealmID: realm.ID,
		URL:     "https://ios.test.app",
		OS:      database.OSTypeIOS,
		AppID:   fmt.Sprintf("%s.com.test.app", iosName),
	}
	if err := db.SaveMobileApp(iosApp, database.SystemTest); err != nil {
		return &resp, fmt.Errorf("failed to create ios mobile app: %w", err)
	}
	resp.closers = append(resp.closers, func() error {
		return deleteMobileApp(db, iosApp)
	})

	// Create Android mobile app.
	androidName := AndroidAppPrefix + entropy
	androidApp := &database.MobileApp{
		Name:    androidName,
		RealmID: realm.ID,
		URL:     "https://android.test.app",
		OS:      database.OSTypeAndroid,
		AppID:   fmt.Sprintf("%s.com.test.app", androidName),
		SHA:     entropy + strings.Repeat("A", 89),
	}
	if err := db.SaveMobileApp(androidApp, database.SystemTest); err != nil {
		return &resp, fmt.Errorf("failed to create android mobile app: %w: %s", err, androidApp.ErrorMessages())
	}
	resp.closers = append(resp.closers, func() error {
		return deleteMobileApp(db, androidApp)
	})

	resp.AdminAPIKey = adminAPIKey
	resp.DeviceAPIKey = deviceAPIKey
	resp.StatsAPIKey = statsAPIKey

	return &resp, nil
}

// Cleanup deletes temporary resources created by the bootstrap.
func (r *BootstrapResponse) Cleanup() error {
	var merr *multierror.Error
	for _, closer := range r.closers {
		if err := closer(); err != nil {
			merr = multierror.Append(merr, err)
		}
	}
	return merr.ErrorOrNil()
}

// deleteAuthorizedApp is a helper for ensuring an authorized app is deleted.
func deleteAuthorizedApp(db *database.Database, key string) error {
	app, err := db.FindAuthorizedAppByAPIKey(key)
	if err != nil {
		if database.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to lookup api key: %w", err)
	}

	return db.RawDB().
		Unscoped().
		Where("id = ?", app.ID).
		Delete(&database.AuthorizedApp{}).
		Error
}

// deleteMobileApp is a helper for ensuring an mobile app is deleted.
func deleteMobileApp(db *database.Database, app *database.MobileApp) error {
	return db.RawDB().
		Unscoped().
		Where("id = ?", app.ID).
		Delete(&database.MobileApp{}).
		Error
}
