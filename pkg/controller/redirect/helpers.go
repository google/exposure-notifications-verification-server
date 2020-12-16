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
	"net/url"
	"strings"
)

// isAndroid determines if a User-Agent is a Android device.
func isAndroid(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), "android")
}

// isIOS determines if a User-Agent is an iOS EN device.
func isIOS(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), "iphone")
}

// decideRedirect selects where to redirect based on several signals.
func decideRedirect(region, userAgent string, url url.URL, appStoreData AppStoreData) (string, bool) {
	// Canonicalize path as lowercase.
	path := strings.ToLower(url.Path)

	// Check for browser type.
	onAndroid := isAndroid(userAgent)
	onIOS := isIOS(userAgent)

	// On Android redirect to Play Store if App Link doesn't trigger
	// and an a link is set up.
	if onAndroid && appStoreData.AndroidURL != "" && appStoreData.AndroidAppID != "" {
		return buildIntentURL(path, url.Query(), region, appStoreData.AndroidAppID, appStoreData.AndroidURL), true
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
