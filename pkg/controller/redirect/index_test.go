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
			name:   "drop_bounce",
			path:   "v",
			query:  url.Values{"c": []string{"12345678"}, "bounce": []string{"123"}},
			region: "US-AA",
			exp:    "ens://v?c=12345678&r=US-AA",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, want := buildEnsURL(tc.path, tc.query, tc.region), tc.exp
			if got != want {
				t.Errorf("expected %q to be %q", got, want)
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

	appLinkBoth := AppStoreData{
		AndroidURL: "https://android.example.com/store/moosylvania",
		IOSURL:     "https://ios.example.com/store/moosylvania",
	}
	appLinkOnlyAndroid := AppStoreData{
		AndroidURL: "https://android.example.com/store/moosylvania",
		IOSURL:     "",
	}
	appLinkNeither := AppStoreData{
		AndroidURL: "",
		IOSURL:     "",
	}

	userAgentAndroid := "Android"
	userAgentIOS := "iPhone"
	userAgentNeither := "Neither"

	relativePinURL := url.URL{
		Path: "/v",
	}
	q := relativePinURL.Query()
	q.Set("c", "1234567890abcdef")
	relativePinURL.RawQuery = q.Encode()

	cases := []struct {
		name         string
		host         string
		url          string
		altURL       *url.URL
		userAgent    string
		appStoreData *AppStoreData
		expected     string
	}{
		{
			name:         "moosylvania_android_pre_bounce",
			url:          "https://moosylvania.gov/v?c=1234567890abcdef",
			userAgent:    userAgentAndroid,
			appStoreData: &appLinkBoth,
			expected:     "https://moosylvania.gov/v?bounce=1&c=1234567890abcdef",
		},
		{
			name:         "moosylvania_android_post_bounce",
			url:          "https://moosylvania.gov/v?bounce=1&c=1234567890abcdef",
			userAgent:    userAgentAndroid,
			appStoreData: &appLinkBoth,
			expected:     "https://android.example.com/store/moosylvania",
		},
		{
			name:         "moosylvania_android_pre_bounce_relative",
			altURL:       &relativePinURL,
			userAgent:    userAgentAndroid,
			appStoreData: &appLinkBoth,
			expected:     "/v?bounce=1&c=1234567890abcdef",
		},
		{
			name:         "moosylvania_android_no_applink_pre_bounce",
			url:          "https://moosylvania.gov/v?c=1234567890abcdef",
			userAgent:    userAgentAndroid,
			appStoreData: &appLinkNeither,
			expected:     "https://moosylvania.gov/v?bounce=1&c=1234567890abcdef",
		},
		{
			name:         "moosylvania_android_no_applink_post_bounce",
			url:          "https://moosylvania.gov/v?c=1234567890abcdef&bounce=1",
			userAgent:    userAgentAndroid,
			appStoreData: &appLinkNeither,
			expected:     "ens://v?c=1234567890abcdef&r=US-MOO",
		},
		{
			name:         "moosylvania_ios_no_applink",
			url:          "https://moosylvania.gov/v?c=1234567890abcdef",
			userAgent:    userAgentIOS,
			appStoreData: &appLinkOnlyAndroid,
			expected:     "ens://v?c=1234567890abcdef&r=US-MOO",
		},
		{
			name:         "moosylvania_ios_no_applink",
			url:          "https://moosylvania.gov/v?c=1234567890abcdef",
			userAgent:    userAgentIOS,
			appStoreData: &appLinkBoth,
			expected:     "https://ios.example.com/store/moosylvania",
		},
		{
			name:         "moosylvania_windows",
			url:          "https://moosylvania.gov/v?c=1234567890abcdef",
			userAgent:    userAgentNeither,
			appStoreData: &appLinkOnlyAndroid,
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
			result, success := decideRedirect("US-MOO", tc.userAgent, *url, *tc.appStoreData)
			if tc.expected != result {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
			if (tc.expected != "") != success {
				t.Errorf("expected doesn't match success")
			}
		})
	}
}
