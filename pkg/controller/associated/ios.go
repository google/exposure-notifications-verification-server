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

package associated

import (
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// iosAppIDs finds all the iOS app ids we know about.
func (c *Controller) iosAppIDs(realmID uint) ([]string, error) {
	apps, err := c.db.ListActiveApps(realmID, database.WithAppOS(database.OSTypeIOS))
	if err != nil {
		return nil, err
	}
	ret := make([]string, 0, len(apps))
	for i := range apps {
		ret = append(ret, apps[i].AppID)
	}
	return ret, nil
}

// IOSData gets the iOS app data.
func (c *Controller) IOSData(realmID uint) (*api.IOSDataResponse, error) {
	ids, err := c.iosAppIDs(realmID)
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	details := make([]api.IOSDetail, len(ids))
	for i, id := range ids {
		details[i] = api.IOSDetail{
			AppID: id,
			Paths: []string{"*"},
		}
	}

	return &api.IOSDataResponse{
		Applinks: api.IOSAppLinks{
			Apps:    []string{}, // expected always empty.
			Details: details,
		},
	}, nil
}
