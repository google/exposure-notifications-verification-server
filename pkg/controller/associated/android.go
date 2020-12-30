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

package associated

import (
	"fmt"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

type AndroidData struct {
	Relation []string `json:"relation,omitempty"`
	Target   Target   `json:"target,omitempty"`
}

type Target struct {
	Namespace    string   `json:"namespace,omitempty"`
	PackageName  string   `json:"package_name,omitempty"`
	Fingerprints []string `json:"sha256_cert_fingerprints,omitempty"`
}

// AndroidData finds all the android data apps.
func (c *Controller) AndroidData(realmID uint) ([]AndroidData, error) {
	apps, err := c.db.ListActiveApps(realmID, database.WithAppOS(database.OSTypeAndroid))
	if err != nil {
		return nil, fmt.Errorf("failed to get android data: %w", err)
	}

	ret := make([]AndroidData, 0, len(apps))
	for i := range apps {
		ret = append(ret, AndroidData{
			Relation: []string{"delegate_permission/common.handle_all_urls"},
			Target: Target{
				Namespace:    "android_app",
				PackageName:  apps[i].AppID,
				Fingerprints: strings.Split(apps[i].SHA, "\n"),
			},
		})
	}

	return ret, nil
}
