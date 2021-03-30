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

package middleware

import (
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
)

// SecureHeaders sets a bunch of default secure headers that our servers should have.
func SecureHeaders(devMode bool, serverType string) mux.MiddlewareFunc {
	options := secure.Options{
		BrowserXssFilter:     false,
		ContentTypeNosniff:   true,
		FrameDeny:            serverType == "html",
		HostsProxyHeaders:    []string{"X-Forwarded-Host"},
		IsDevelopment:        devMode,
		ReferrerPolicy:       "strict-origin-when-cross-origin",
		SSLProxyHeaders:      map[string]string{"X-Forwarded-Proto": "https"},
		SSLRedirect:          !devMode,
		STSIncludeSubdomains: true,
		STSPreload:           true,
		STSSeconds:           315360000,
	}

	return secure.New(options).Handler
}
