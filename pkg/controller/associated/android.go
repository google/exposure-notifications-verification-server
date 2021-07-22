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
	"fmt"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// AndroidData finds all the android data apps.
func (c *Controller) AndroidData(realmID uint) ([]api.AndroidDataResponse, error) {
	apps, err := c.db.ListActiveApps(realmID, database.WithAppOS(database.OSTypeAndroid))
	if err != nil {
		return nil, fmt.Errorf("failed to get android data: %w", err)
	}

	ret := make([]api.AndroidDataResponse, 0, len(apps))
	for i := range apps {
		ret = append(ret, api.AndroidDataResponse{
			Relation: []string{"delegate_permission/common.handle_all_urls"},
			Target: api.AndroidTarget{
				Namespace:    "android_app",
				PackageName:  apps[i].AppID,
				Fingerprints: strings.Split(apps[i].SHA, "\n"),
			},
		})
	}

	return ret, nil
}
