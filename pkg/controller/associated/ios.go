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

// The iOS format is specified by:
//   https://developer.apple.com/documentation/safariservices/supporting_associated_domains

import (
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

type IOSData struct {
	Applinks Applinks `json:"applinks"`

	// The following two fields are included for completeness' sake, but are not
	// currently populated/used by the system.
	Webcredentials *Appstrings `json:"webcredentials,omitempty"`
	Appclips       *Appstrings `json:"appclips,omitempty"`
}

type Applinks struct {
	Details []Detail `json:"details,omitempty"`
}

type Detail struct {
	AppIds     []string    `json:"appIDs,omitempty"`
	Components []Component `json:"components,omitempty"`
}

type Component struct {
	Path    string `json:"/,omitempty"`
	Param   string `json:"?,omitempty"`
	Exclude *bool  `json:"exclude,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type Appstrings struct {
	Apps []string `json:"apps,omitempty"`
}

// getAppIds finds all the iOS app ids we know about.
func (c *Controller) getAppIds(realmID uint) ([]string, error) {
	apps, err := c.db.ListActiveAppsByOS(realmID, database.OSTypeIOS)
	if err != nil {
		return nil, err
	}
	ret := make([]string, 0, len(apps))
	for i := range apps {
		ret = append(ret, apps[i].AppID)
	}
	return ret, nil
}

// getIosData gets the iOS app data.
func (c *Controller) getIosData(region string) (*IOSData, error) {
	realm, err := c.db.FindRealmByRegion(region)
	if err != nil {
		return nil, fmt.Errorf("unable to lookup realm")
	}

	ids, err := c.getAppIds(realm.ID)
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	return &IOSData{
		Applinks: Applinks{
			Details: []Detail{{
				AppIds: ids,
				Components: []Component{
					{Path: "/*", Comment: "handle all urls"},
				}},
			},
		},
	}, nil
}
