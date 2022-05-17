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
	"fmt"
	"net/url"
	"strings"
)

// isAndroid determines if a User-Agent is a Android device.
func isAndroid(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), "android")
}

// isIOS determines if a User-Agent is an iOS EN device.
func isIOS(userAgent string) bool {
	lAgent := strings.ToLower(userAgent)
	return strings.Contains(lAgent, "iphone") || strings.Contains(lAgent, "Macintosh")
}

// decideRedirect selects where to redirect based on several signals.
// Returns are the URI to redirect to as a string and success/failure boolean.
func decideRedirect(region, userAgent string, u *url.URL, enxEnabled bool, appStoreData AppStoreData) (string, bool) {
	// Canonicalize path as lowercase.
	path := strings.ToLower(u.Path)

	// Check for browser type.
	onAndroid := isAndroid(userAgent)
	onIOS := isIOS(userAgent)

	// Extract the query params, if any. If there are no query params, the request
	// is treated as an onboarding request.
	query := u.Query()
	noQuery := len(query) == 0

	// On Android redirect to Play Store if App Link doesn't trigger and an a link
	// is set up.
	if onAndroid {
		// Is it headless ENX, special case w/ internal protocol.
		// If the server has been reached, we know ENX isn't active
		if appStoreData.AndroidHeadless {
			return fmt.Sprintf(androidHeadlessOnboarding, strings.ToLower(region)), true
		}

		if noQuery {
			// Is it an app that redirects to the play store.
			if v := appStoreData.AndroidURL; v != "" {
				return v, true
			}

			// Generic onboarding, w/ hint to region.
			return fmt.Sprintf(androidHeadlessOnboarding, strings.ToLower(region)), true
		}

		// There is an app for this host, build an intent into that app, passing the path and query params.
		if appStoreData.AndroidAppID != "" && appStoreData.AndroidURL != "" {
			intent := buildIntentURL(path, query, region, appStoreData.AndroidAppID, appStoreData.AndroidURL)
			return intent, true
		}

		// we know it's android so generic onboarding
		return fmt.Sprintf(androidHeadlessOnboarding, strings.ToLower(region)), true
	}

	// On iOS redirect to App Store if App Link doesn't trigger and an a link is set up.
	if onIOS {
		if noQuery {
			// Custom app registered, redirect to App Store.
			if v := appStoreData.IOSURL; v != "" {
				return v, true
			}
			// Redirect into the iOS EN Express onboarding.
			return iosOnboardingRedirect, true
		}

		// Send to app store, we don't build any kind of query path.
		// We assume if we got here, the app isn't properly registered to
		// handle the redirect service URL.
		if appStoreData.IOSURL != "" {
			return appStoreData.IOSURL, true
		}

		// Pass the path and query to the ens:// protocol.
		if enxEnabled {
			return buildEnsURL(path, query, region), true
		}

		return "", false
	}

	// If we got this far, it's an unknown device with no query params, redirect
	// to the generic marketing page.
	if noQuery {
		return genericOnboardingRedirect, true
	}

	// The request included no matching metadata, do nothing.
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
	u.RawQuery = q.Encode()

	return u.String()
}

// buildIntentURL returns the ens:// URL with fallback
// for the given path, query, and region.
func buildIntentURL(path string, query url.Values, region, appID, fallback string) string {
	u := &url.URL{
		Scheme: "intent",
		Path:   strings.TrimPrefix(path, "/"),
	}
	u.RawQuery = query.Encode()
	q := u.Query()
	q.Set("r", region)
	u.RawQuery = q.Encode()

	suffix := "#Intent"
	suffix += ";scheme=ens"
	suffix += ";package=" + appID
	suffix += ";action=android.intent.action.VIEW"
	suffix += ";category=android.intent.category.BROWSABLE"
	suffix += ";S.browser_fallback_url=" + url.QueryEscape(fallback)
	suffix += ";end"

	return u.String() + suffix
}
