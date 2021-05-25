// Copyright 2021 Google LLC
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

package clients

import (
	"context"
	"net/http"
)

// AppSyncClient is a client that talks to the appsync service.
type AppSyncClient struct {
	*client
}

// NewAppSyncClient creates a new app sync service http client.
func NewAppSyncClient(base string, opts ...Option) (*AppSyncClient, error) {
	client, err := newClient(base, "", opts...)
	if err != nil {
		return nil, err
	}

	return &AppSyncClient{
		client: client,
	}, nil
}

// AppSync triggers an application sync.
func (c *AppSyncClient) AppSync(ctx context.Context) (*AppsResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return nil, err
	}

	var out AppsResponse
	if err := c.doOK(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AppsResponse is the body for the published list of android apps.
type AppsResponse struct {
	Apps []App `json:"apps"`
}

type Translation struct {
	Language string `json:"lang"`
	Message  string `json:"message"`
}

type Localization struct {
	MessageID    string        `json:"msgid"`
	Translations []Translation `json:"translations"`
}

// App represents single app for the AppResponse body.
type App struct {
	Region        string `json:"region"`
	IsEnx         bool   `json:"is_enx,omitempty"`
	AndroidTarget `json:"android_target"`
	AgencyColor   string         `json:"agency_color"`
	AgencyImage   string         `json:"agency_image"`
	DefaultLocale string         `json:"default_locale"`
	Localizations []Localization `json:"localizations"`
}

// AndroidTarget holds the android metadata for an App of AppResponse.
type AndroidTarget struct {
	Namespace              string `json:"namespace"`
	AppName                string `json:"app_name,omitempty"`
	PackageName            string `json:"package_name"`
	SHA256CertFingerprints string `json:"sha256_cert_fingerprints"`
}
