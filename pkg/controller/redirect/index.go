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
	"net/http"
	"net/url"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Strip of the port if that was passed along in the host header.
		baseHost := strings.ToLower(r.Host)
		if i := strings.Index(baseHost, ":"); i > 0 {
			baseHost = baseHost[0:i]
		}

		var hostRegion string = ""
		for hostname, region := range c.hostnameToRegion {
			if hostname == baseHost {
				hostRegion = region
				break
			}
		}
		realm, err := c.db.FindRealmByRegion(hostRegion)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
		}

		// Get App Store Data.
		cacheKey := fmt.Sprintf("appstoredata:by_region:%s", hostRegion)
		var appStoreData AppStoreData
		if err := c.cacher.Fetch(ctx, cacheKey, &appStoreData, c.config.AppCacheTTL, func() (interface{}, error) {
			c.logger.Debug("fetching new app store data")
			return c.getAppStoreData(realm.ID)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if sendto, success := decideRedirect(hostRegion, r.UserAgent(), *r.URL, appStoreData); success {
			http.Redirect(w, r, sendto, http.StatusSeeOther)
			return
		}

		c.logger.Warnw("unknown host", "host", r.Host)
		m := controller.TemplateMapFromContext(ctx)
		m["requestURI"] = (&url.URL{
			Scheme: "https",
			Host:   r.Host,
			Path:   strings.TrimPrefix(r.URL.RequestURI(), "/"),
		}).String()
		c.h.RenderHTMLStatus(w, http.StatusNotFound, "404", m)
	})
}

// isAndroid determines if a User-Agent is a Android device.
func isAndroid(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), "android")
}

// isIOS determines if a User-Agent is an iOS EN device.
func isIOS(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), "iphone")
}

// decideRedirect selects where to redirect based on several signals.
func decideRedirect(region, userAgent string, url url.URL,
	appStoreData AppStoreData) (string, bool) {
	// Canonicalize path as lowercase.
	path := strings.ToLower(url.Path)

	// Check for browser type.
	onAndroid := isAndroid(userAgent)
	onIOS := isIOS(userAgent)

	// A subset of SMS clients (e.g. Facebook Messenger) open links
	// in inline WebViews without giving https Intents a opportunity
	// to trigger App Links.
	// Redirect to ourselves once to attempt to trigger the link.
	// Bounce to self if we haven't already (on Android only).
	// Keep track of state by including an extra bounce=1 url param.
	if onAndroid && url.Query().Get("bounce") == "" {
		q := url.Query()
		q.Set("bounce", "1")
		url.RawQuery = q.Encode()
		return url.String(), true
	}

	// On Android redirect to Play Store if App Link doesn't trigger
	// and an a link is set up.
	if onAndroid && appStoreData.AndroidURL != "" {
		return appStoreData.AndroidURL, true
	}

	// On iOS redirect to App Store if App Link doesn't trigger
	// and an a link is set up.
	if onIOS && appStoreData.IOSURL != "" {
		return appStoreData.IOSURL, true
	}

	if onIOS || onAndroid {
		return buildEnsURL(path, url.Query(), region), true
	}

	return "", false
}

// buildEnsURL returns the ens:// URL for the given path, query, and region.
func buildEnsURL(path string, query url.Values, region string) string {
	u := &url.URL{
		Scheme: "ens",
		Path:   strings.TrimPrefix(path, "/"),
	}
	u.RawQuery = query.Encode()
	q := u.Query()
	q.Set("r", region)
	q.Del("bounce")
	u.RawQuery = q.Encode()

	return u.String()
}
