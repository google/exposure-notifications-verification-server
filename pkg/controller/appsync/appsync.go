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

// Package appsync syncs the published list of mobile apps to this server's db.
package appsync

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/hashicorp/go-multierror"
)

const playStoreLink = `https://play.google.com/store/apps/details?id=`

// Controller is a controller for the appsync service.
type Controller struct {
	config *config.AppSyncConfig
	db     *database.Database
	h      *render.Renderer
}

// New creates a new appsync controller.
func New(config *config.AppSyncConfig, db *database.Database, h *render.Renderer) (*Controller, error) {
	return &Controller{
		config: config,
		db:     db,
		h:      h,
	}, nil
}

// HandleSync performs the logic to sync mobile apps.
func (c *Controller) HandleSync() http.Handler {
	type AppSyncResult struct {
		OK     bool    `json:"ok"`
		Errors []error `json:"errors,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apps, err := clients.AppSync(c.config.AppSyncURL, c.config.Timeout, c.config.FileSizeLimitBytes)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// If there are any errors, return them
		if merr := c.syncApps(r.Context(), apps); merr != nil {
			if errs := merr.WrappedErrors(); len(errs) > 0 {
				c.h.RenderJSON(w, http.StatusInternalServerError, &AppSyncResult{
					OK:     false,
					Errors: errs,
				})
				return
			}
		}
		c.h.RenderJSON(w, http.StatusOK, &AppSyncResult{OK: true})
	})
}

func (c *Controller) syncApps(ctx context.Context, apps *clients.AppsResponse) *multierror.Error {
	logger := logging.FromContext(ctx).Named("appsync.syncApps")
	var err error
	var merr *multierror.Error

	realms := map[string]*database.Realm{}
	appsByRealm := map[uint][]*database.MobileApp{}

	for _, app := range apps.Apps {
		realm, has := realms[app.Region]
		if !has { // Find this apps region and cache it in our realms map
			realm, err = c.db.FindRealmByRegion(app.Region)
			if err != nil {
				merr = multierror.Append(merr, fmt.Errorf("unable to lookup realm %s: %w", app.Region, err))
				continue
			}
			realms[app.Region] = realm
		}

		realmApps, has := appsByRealm[realm.ID]
		if !has { // Find all of the apps for this realm and cache that list in our appByRealmMap
			realmApps, err := c.db.ListActiveApps(realm.ID, database.WithAppOS(database.OSTypeAndroid))
			if err != nil {
				merr = multierror.Append(merr, fmt.Errorf("unable to list apps for realm %d: %w", realm.ID, err))
				continue
			}
			appsByRealm[realm.ID] = realmApps
		}

		has = false // Find out if this realm's applist already has an app with this fingerprint.
		for _, a := range realmApps {
			if a.SHA == app.SHA256CertFingerprints {
				has = true
				break
			}
		}

		// Didn't find an app. make one.
		if !has {
			logger.Infof("App not found during sync. Adding app %#v", app)
			newApp := &database.MobileApp{
				Name:    app.Region + " Android App",
				RealmID: realm.ID,
				URL:     playStoreLink + app.PackageName,
				OS:      database.OSTypeAndroid,
				SHA:     app.SHA256CertFingerprints,
				AppID:   app.PackageName,
			}
			if err := c.db.SaveMobileApp(newApp, database.System); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed saving mobile app: %v", err))
				continue
			}
		}
	}
	return merr
}
