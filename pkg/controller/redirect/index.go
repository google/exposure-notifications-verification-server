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

package redirect

import (
	"net"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("redirect.HandleIndex")

		// Strip of the port if that was passed along in the host header.
		baseHost := strings.ToLower(r.Host)
		if host, _, err := net.SplitHostPort(baseHost); err == nil {
			baseHost = host
		}

		var hostRegion string = ""
		for hostname, region := range c.hostnameToRegion {
			if hostname == baseHost {
				hostRegion = region
				break
			}
		}

		if hostRegion == "" {
			// If this is a mobile device, redirect into the OS specific picker as a
			// best effort.
			userAgent := r.UserAgent()
			if isAndroid(userAgent) {
				http.Redirect(w, r, androidOnboardingRedirect, http.StatusSeeOther)
				return
			}
			if isIOS(userAgent) {
				http.Redirect(w, r, iosOnboardingRedirect, http.StatusSeeOther)
				return
			}

			controller.NotFound(w, r, c.h)
			return
		}

		realm, err := c.db.FindRealmByRegion(hostRegion)
		if err != nil {
			if database.IsNotFound(err) {
				controller.NotFound(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Get App Store Data.
		var data AppStoreData
		cacheKey := &cache.Key{
			Namespace: "apps:appstoredata:by_region",
			Key:       hostRegion,
		}
		if err := c.cacher.Fetch(ctx, cacheKey, &data, c.config.AppCacheTTL, func() (interface{}, error) {
			logger.Debug("fetching new app store data")
			return c.getAppStoreData(realm.ID)
		}); err != nil {
			if database.IsNotFound(err) {
				controller.NotFound(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		if sendto, success := decideRedirect(hostRegion, r.UserAgent(), r.URL, realm.EnableENExpress, data); success {
			http.Redirect(w, r, sendto, http.StatusSeeOther)
			return
		}

		logger.Infow("no matching metadata for redirect",
			"host", r.Host,
			"userAgent", r.UserAgent())

		controller.NotFound(w, r, c.h)
		return
	})
}
