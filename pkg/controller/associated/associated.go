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

// Package associated handles the iOS and Android associated app handler
// protocols. For more discussion of these protocols, please see:
//
// Android:
//   https://developer.android.com/training/app-links/verify-site-associations
//
// iOS:
//   https://developer.apple.com/documentation/safariservices/supporting_associated_domains
package associated

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type Controller struct {
	config           *config.RedirectConfig
	hostnameToRegion map[string]string
	cacher           cache.Cacher
	db               *database.Database
	h                *render.Renderer
}

func New(config *config.RedirectConfig, db *database.Database, cacher cache.Cacher, h *render.Renderer) (*Controller, error) {
	cfgMap, err := config.HostnameToRegion()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Controller{
		config:           config,
		db:               db,
		cacher:           cacher,
		h:                h,
		hostnameToRegion: cfgMap,
	}, nil
}

func (c *Controller) getRegion(r *http.Request) string {
	// Get the hostname first
	baseHost := strings.ToLower(r.Host)
	if host, _, err := net.SplitHostPort(baseHost); err == nil {
		baseHost = host
	}

	// return the mapped region code (or default, "", if not found)
	return c.hostnameToRegion[baseHost]
}
