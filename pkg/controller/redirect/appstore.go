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
	AndroidURL   string `json:"androidURL"`
	AndroidAppID string `json:"androidAppID"`
	IOSURL       string `json:"iosURL"`
}

// getAppStoreData finds data tied to app store listings.
func (c *Controller) getAppStoreData(realmID uint) (*AppStoreData, error) {
	// Pick first Android app (in the realm) for Play Store redirect.
	androidURL := ""
	androidAppID := ""
	scopes := []database.Scope{database.WithAppOS(database.OSTypeAndroid)}
	androidApps, err := c.db.ListActiveApps(realmID, scopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to get Android Apps: %w", err)
	}
	if len(androidApps) > 0 {
		androidURL = androidApps[0].URL
		androidAppID = androidApps[0].AppID
	}

	// Pick first iOS app (in the realm) for Store redirect.
	iosURL := ""
	scopes = []database.Scope{database.WithAppOS(database.OSTypeIOS)}
	iosApps, err := c.db.ListActiveApps(realmID, scopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to get iOS Apps: %w", err)
	}
	if len(iosApps) > 0 {
		iosURL = iosApps[0].URL
	}

	return &AppStoreData{
		AndroidURL:   androidURL,
		AndroidAppID: androidAppID,
		IOSURL:       iosURL,
	}, nil
}
