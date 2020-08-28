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
	"context"

	"github.com/google/exposure-notifications-verification-server/pkg/config"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
)

func SecureHeaders(ctx context.Context, devMode bool, serverType string) mux.MiddlewareFunc {
	options := secure.Options{
		BrowserXssFilter:     serverType == "html",
		ContentTypeNosniff:   true,
		FrameDeny:            serverType == "html",
		HostsProxyHeaders:    []string{"X-Forwarded-Host"},
		IsDevelopment:        devMode,
		SSLProxyHeaders:      map[string]string{"X-Forwarded-Proto": "https"},
		SSLRedirect:          !devMode,
		STSIncludeSubdomains: true,
		STSPreload:           true,
		STSSeconds:           315360000,
	}

	return secure.New(options).Handler
}
