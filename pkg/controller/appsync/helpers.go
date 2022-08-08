// Copyright 2020 the Exposure Notifications Verification Server authors
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

package appsync

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/google/exposure-notifications-verification-server/internal/appsync"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/hashicorp/go-multierror"
)

// syncApps looks up the realm and associated list of MobileApps for each entry
// of AppsResponse. Then it checks to see if there exists an app with the
// AppResponse SHA hash, if not it creates a new MobileApp.
func (c *Controller) syncApps(ctx context.Context, apps *appsync.AppsResponse) *multierror.Error {
	logger := logging.FromContext(ctx).Named("appsync.syncApps")
	var merr *multierror.Error

	realms := map[string]*database.Realm{}
	appsByRealm := map[uint][]*database.MobileApp{}

	for _, app := range apps.Apps {
		realm, err := c.findRealmForApp(app, realms)
		if err != nil {
			if database.IsNotFound(err) {
				logger.Debugw("no app corresponds to region, skipping",
					"app", app.AndroidTarget.AppName,
					"region", app.Region)
			} else {
				merr = multierror.Append(merr, fmt.Errorf("unable to lookup realm for region %q: %w", app.Region, err))
			}
			continue
		}

		realm.AgencyBackgroundColor = strings.ToLower(app.AgencyColor)
		realm.AgencyImage = app.AgencyImage
		realm.DefaultLocale = app.DefaultLocale
		realm.UserReportLearnMoreURL = app.WebReportLearnMoreURL
		if err := c.db.SaveRealm(realm, database.System); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("unable to update agency information: %w", err))
			continue
		}

		if res, err := c.db.SyncRealmTranslations(realm.ID, app.Localizations); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("unable to sync localizations, realm: %d: %w", realm.ID, err))
			// don't skip the rest, still try and sync apps
		} else {
			logger.Infow("synced tranlations", "realm", realm.ID, "result", res)
		}

		realmApps, err := c.findAppsForRealm(realm.ID, appsByRealm)
		if err != nil {
			merr = multierror.Append(merr, fmt.Errorf("unable to list apps for realm %d: %w", realm.ID, err))
			continue
		}

		// Find out if this realm's applist already has an app with this fingerprint.
		hasSHA, hasGeneratedName := false, false
		for _, a := range realmApps {
			if a.SHA == app.SHA256CertFingerprints {
				hasSHA = true
			}
			if a.Name == generateAppName(app) {
				hasGeneratedName = true
			}
		}

		// Didn't find an app. make one.
		if !hasSHA {
			logger.Infow("app not found during sync, adding", "app", app)

			name := generateAppName(app)
			if hasGeneratedName { // add a random string to names on collision
				s, err := project.RandomBase64String(8)
				if err != nil {
					merr = multierror.Append(merr, fmt.Errorf("error generating app name: %w", err))
					continue
				}
				name += " " + s
			}

			playStoreURL := &url.URL{
				Scheme:   "https",
				Host:     playStoreHost,
				RawQuery: "id=" + app.PackageName,
			}

			newApp := &database.MobileApp{
				Name:            name,
				RealmID:         realm.ID,
				URL:             playStoreURL.String(),
				OS:              database.OSTypeAndroid,
				SHA:             app.SHA256CertFingerprints,
				AppID:           app.PackageName,
				Headless:        app.Headless,
				DisableRedirect: true, // For all ENX - use the on device picker, not play store.
			}
			if err := c.db.SaveMobileApp(newApp, database.System); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed saving mobile app: %w", err))
				continue
			}
		}
	}
	return merr
}

func (c *Controller) findRealmForApp(
	app appsync.App, realms map[string]*database.Realm,
) (*database.Realm, error) {
	var err error
	realm, has := realms[app.Region]
	if !has { // Find this apps region and cache it in our realms map
		realm, err = c.db.FindRealmByRegion(app.Region)
		if err != nil {
			return nil, err
		}
		realms[app.Region] = realm
	}
	return realm, nil
}

func (c *Controller) findAppsForRealm(
	realmID uint, appsByRealm map[uint][]*database.MobileApp,
) ([]*database.MobileApp, error) {
	var err error
	realmApps, has := appsByRealm[realmID]
	if !has { // Find all of the apps for this realm and cache that list in our appByRealmMap
		realmApps, err = c.db.ListActiveApps(realmID, database.WithAppOS(database.OSTypeAndroid))
		if err != nil {
			return nil, err
		}
		appsByRealm[realmID] = realmApps
	}
	return realmApps, nil
}

func generateAppName(app appsync.App) string {
	if app.AppName != "" {
		return app.AppName
	}
	return app.Region + " Android App"
}
