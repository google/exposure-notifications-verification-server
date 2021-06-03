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

package redirect

import (
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

type AppStoreData struct {
	AndroidURL      string `json:"androidURL"`
	AndroidAppID    string `json:"androidAppID"`
	AndroidHeadless bool   `json:"androidHeadless"`
	IOSURL          string `json:"iosURL"`
}

// getAppStoreData finds data tied to app store listings.
func (c *Controller) getAppStoreData(realmID uint) (*AppStoreData, error) {
	// Pick first Android app (in the realm) for Play Store redirect.
	androidURL := ""
	androidAppID := ""
	headless := false
	androidApps, err := c.db.ListActiveApps(realmID, database.WithAppOS(database.OSTypeAndroid))
	if err != nil {
		return nil, fmt.Errorf("failed to get Android Apps: %w", err)
	}
	if len(androidApps) > 0 {
		// Find the first app that has redirects enabled.
		for _, app := range androidApps {
			if !app.DisableRedirect {
				androidURL = app.URL
				androidAppID = app.AppID
				headless = app.Headless
				break
			}
		}
	}

	// Pick first iOS app (in the realm) for Store redirect.
	iosURL := ""
	iosApps, err := c.db.ListActiveApps(realmID, database.WithAppOS(database.OSTypeIOS))
	if err != nil {
		return nil, fmt.Errorf("failed to get iOS Apps: %w", err)
	}
	if len(iosApps) > 0 {
		// Find the first app that has redirects enabled.
		for _, app := range iosApps {
			if !app.DisableRedirect {
				iosURL = app.URL
				break
			}
		}
	}

	return &AppStoreData{
		AndroidURL:      androidURL,
		AndroidAppID:    androidAppID,
		AndroidHeadless: headless,
		IOSURL:          iosURL,
	}, nil
}
