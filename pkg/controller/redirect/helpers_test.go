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
	"testing"
)

func TestBuildEnsURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		path   string
		query  url.Values
		region string
		exp    string
	}{
		{
			name:   "leading_slash_region",
			path:   "v",
			region: "/US-AA",
			exp:    "ens://v?r=%2FUS-AA",
		},
		{
			name:   "trailing_slash_region",
			path:   "v",
			region: "US-AA/",
			exp:    "ens://v?r=US-AA%2F",
		},
		{
			name:   "leading_slash_path",
			path:   "/v",
			region: "US-AA",
			exp:    "ens://v?r=US-AA",
		},
		{
			name:   "trailing_slash_path",
			path:   "v/",
			region: "US-AA",
			exp:    "ens://v/?r=US-AA",
		},
		{
			name:   "includes_code",
			path:   "v",
			query:  url.Values{"c": []string{"1234567890abcdef"}},
			region: "US-AA",
			exp:    "ens://v?c=1234567890abcdef&r=US-AA",
		},
		{
			name:   "includes_other",
			path:   "v",
			query:  url.Values{"foo": []string{"bar"}},
			region: "US-AA",
			exp:    "ens://v?foo=bar&r=US-AA",
		},
		{
			name:   "replace_region",
			path:   "v",
			query:  url.Values{"r": []string{"US-XX"}},
			region: "US-AA",
			exp:    "ens://v?r=US-AA",
		},
		{
			name:   "replace_just_region",
			path:   "v",
			query:  url.Values{"c": []string{"12345678"}, "r": []string{"DE"}},
			region: "US-BB",
			exp:    "ens://v?c=12345678&r=US-BB",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, want := buildEnsURL(tc.path, tc.query, tc.region), tc.exp
			if got != want {
				t.Errorf("Expected %q to be %q", got, want)
			}
		})
	}
}

func TestBuildIntentURL(t *testing.T) {
	t.Parallel()

	expectedSuffix := "#Intent" +
		";scheme=ens" +
		";package=gov.moosylvania.app" +
		";action=android.intent.action.VIEW" +
		";category=android.intent.category.BROWSABLE" +
		";S.browser_fallback_url=https%3A%2F%2Fplay.google.com%2Fstore%2Fapps%2Fdetails%3Fid%3Dgov.moosylvania.app" +
		";end"
	cases := []struct {
		name   string
		path   string
		query  url.Values
		region string
		exp    string
	}{
		{
			name:   "leading_slash_region",
			path:   "v",
			region: "/US-AA",
			exp:    "intent://v?r=%2FUS-AA" + expectedSuffix,
		},
		{
			name:   "trailing_slash_region",
			path:   "v",
			region: "US-AA/",
			exp:    "intent://v?r=US-AA%2F" + expectedSuffix,
		},
		{
			name:   "leading_slash_path",
			path:   "/v",
			region: "US-AA",
			exp:    "intent://v?r=US-AA" + expectedSuffix,
		},
		{
			name:   "trailing_slash_path",
			path:   "v/",
			region: "US-AA",
			exp:    "intent://v/?r=US-AA" + expectedSuffix,
		},
		{
			name:   "includes_code",
			path:   "v",
			query:  url.Values{"c": []string{"1234567890abcdef"}},
			region: "US-AA",
			exp:    "intent://v?c=1234567890abcdef&r=US-AA" + expectedSuffix,
		},
		{
			name:   "includes_other",
			path:   "v",
			query:  url.Values{"foo": []string{"bar"}},
			region: "US-AA",
			exp:    "intent://v?foo=bar&r=US-AA" + expectedSuffix,
		},
		{
			name:   "replace_region",
			path:   "v",
			query:  url.Values{"r": []string{"US-XX"}},
			region: "US-AA",
			exp:    "intent://v?r=US-AA" + expectedSuffix,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			appID := "gov.moosylvania.app"
			fallback := "https://play.google.com/store/apps/details?id=gov.moosylvania.app"
			got, want := buildIntentURL(tc.path, tc.query, tc.region, appID, fallback), tc.exp
			if got != want {
				t.Errorf("Expected %q to be %q", got, want)
			}
		})
	}
}

func TestAgentDetection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		userAgent string
		android   bool
		ios       bool
	}{
		{
			name:      "android_chrome",
			userAgent: "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 6P Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.83 Mobile Safari/537.36",
			android:   true,
			ios:       false,
		},
		{
			name:      "android_webview",
			userAgent: "Mozilla/5.0 (Linux; U; Android 2.2.1; en-us; Nexus One Build/FRG83) AppleWebKit/533.1 (KHTML, like Gecko) Version/4.0 Mobile Safari/533.1",
			android:   true,
			ios:       false,
		},
		{
			name:      "iphone_safari",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_3_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.5 Mobile/15E148 Safari/604.1",
			android:   false,
			ios:       true,
		},
		{
			name:      "iphone_safari",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_3_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.5 Mobile/15E148 Safari/604.1",
			android:   false,
			ios:       true,
		},
		{
			name:      "iphone_chrome",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 10_3 like Mac OS X) AppleWebKit/602.1.50 (KHTML, like Gecko) CriOS/56.0.2924.75 Mobile/14E5239e Safari/602.1",
			android:   false,
			ios:       true,
		},
		{
			name:      "windows_chrome",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36",
			android:   false,
			ios:       false,
		},
		{
			name:      "ipad_safari",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 11_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/11.0 Mobile/15E148 Safari/604.1",
			android:   false,
			// For ENX purposes exclude iPad as it's unsupported.
			ios: false,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			onAndroid := isAndroid(tc.userAgent)
			onIOS := isIOS(tc.userAgent)
			if onAndroid != tc.android || onIOS != tc.ios {
				t.Errorf("expected android=%t ios=%t, got android=%t ios=%t", tc.android, tc.ios, onAndroid, onIOS)
			}
		})
	}
}

func TestDecideRedirect(t *testing.T) {
	t.Parallel()

	expectedSuffix := "#Intent" +
		";scheme=ens" +
		";package=gov.moosylvania.app" +
		";action=android.intent.action.VIEW" +
		";category=android.intent.category.BROWSABLE" +
		";S.browser_fallback_url=https%3A%2F%2Fandroid.example.com%2Fstore%2Fmoosylvania" +
		";end"

	appLinkBoth := AppStoreData{
		AndroidURL:   "https://android.example.com/store/moosylvania",
		AndroidAppID: "gov.moosylvania.app",
		IOSURL:       "https://ios.example.com/store/moosylvania",
	}
	appLinkNeither := AppStoreData{
		AndroidURL:   "",
		AndroidAppID: "",
		IOSURL:       "",
	}

	userAgentAndroid := "Android"
	userAgentIOS := "iPhone"
	userAgentNeither := "Neither"

	relativePinURL := url.URL{
		Path: "/v",
	}
	q := relativePinURL.Query()
	q.Set("c", "123456")
	relativePinURL.RawQuery = q.Encode()

	cases := []struct {
		name         string
		url          string
		altURL       *url.URL
		enxEnabled   bool
		userAgent    string
		appStoreData *AppStoreData
		expected     string
	}{
		// Android
		{
			name:         "android_both",
			url:          "https://moosylvania.gov/v?c=123456",
			userAgent:    userAgentAndroid,
			appStoreData: &appLinkBoth,
			expected:     "intent://v?c=123456&r=US-MOO" + expectedSuffix,
		},
		{
			name:         "android_both_relative",
			altURL:       &relativePinURL,
			userAgent:    userAgentAndroid,
			appStoreData: &appLinkBoth,
			expected:     "intent://v?c=123456&r=US-MOO" + expectedSuffix,
		},
		{
			name:         "android_no_applink_enx",
			url:          "https://moosylvania.gov/v?c=123456",
			enxEnabled:   true,
			userAgent:    userAgentAndroid,
			appStoreData: &appLinkNeither,
			expected:     "ens://v?c=123456&r=US-MOO",
		},
		{
			name:         "android_no_applink",
			url:          "https://moosylvania.gov/v?c=123456",
			enxEnabled:   false,
			userAgent:    userAgentAndroid,
			appStoreData: &appLinkNeither,
			expected:     "",
		},

		// iOS
		{
			name:         "ios_both",
			url:          "https://moosylvania.gov/v?c=123456",
			userAgent:    userAgentIOS,
			appStoreData: &appLinkBoth,
			expected:     "https://ios.example.com/store/moosylvania",
		},
		{
			name:         "ios_both_relative",
			url:          "https://moosylvania.gov/v?c=123456",
			altURL:       &relativePinURL,
			userAgent:    userAgentIOS,
			appStoreData: &appLinkBoth,
			expected:     "https://ios.example.com/store/moosylvania",
		},
		{
			name:         "ios_no_applink_enx",
			url:          "https://moosylvania.gov/v?c=123456",
			enxEnabled:   true,
			userAgent:    userAgentIOS,
			appStoreData: &appLinkNeither,
			expected:     "ens://v?c=123456&r=US-MOO",
		},
		{
			name:         "ios_no_applink",
			url:          "https://moosylvania.gov/v?c=123456",
			enxEnabled:   false,
			userAgent:    userAgentIOS,
			appStoreData: &appLinkNeither,
			expected:     "",
		},

		// Other
		{
			name:         "other",
			url:          "https://moosylvania.gov/v?c=123456",
			userAgent:    userAgentNeither,
			appStoreData: &appLinkBoth,
			expected:     "",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			url := tc.altURL
			if url == nil {
				otherURL, err := url.Parse(tc.url)
				if err != nil {
					t.Errorf("invalid url %s", tc.url)
				}
				url = otherURL
			}
			result, success := decideRedirect("US-MOO", tc.userAgent, url, tc.enxEnabled, *tc.appStoreData)
			if tc.expected != result {
				t.Errorf("expected %q to be %q", result, tc.expected)
			}
			if (tc.expected != "") != success {
				t.Errorf("expected doesn't match success")
			}
		})
	}
}
