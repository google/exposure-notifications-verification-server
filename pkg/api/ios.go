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

package api

// IOSDataResponse is the iOS format is specified by:
//
//	https://developer.apple.com/documentation/safariservices/supporting_associated_domains
type IOSDataResponse struct {
	Applinks IOSAppLinks `json:"applinks"`

	// The following two fields are included for completeness' sake, but are not
	// currently populated/used by the system.
	Webcredentials *IOSAppstrings `json:"webcredentials,omitempty"`
	Appclips       *IOSAppstrings `json:"appclips,omitempty"`
}

type IOSAppLinks struct {
	Apps    []string    `json:"apps"`
	Details []IOSDetail `json:"details,omitempty"`
}

type IOSDetail struct {
	AppID string   `json:"appID,omitempty"`
	Paths []string `json:"paths,omitempty"`
}

type IOSAppstrings struct {
	Apps []string `json:"apps,omitempty"`
}
