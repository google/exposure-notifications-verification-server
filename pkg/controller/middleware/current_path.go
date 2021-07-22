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

// Package middleware defines shared middleware for handlers.
package middleware

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/gorilla/mux"
)

type Path struct {
	uri *url.URL
}

func InjectCurrentPath() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			m := controller.TemplateMapFromContext(ctx)
			m["currentPath"] = &Path{uri: r.URL}

			ctx = controller.WithTemplateMap(ctx, m)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

func (p *Path) String() string {
	return p.uri.String()
}

func (p *Path) IsPath(s string) bool {
	return p.uri.RequestURI() == s
}

func (p *Path) IsDir(s string) bool {
	return strings.HasPrefix(p.uri.RequestURI(), s)
}

func (p *Path) IsFile(s string) bool {
	_, file := path.Split(p.uri.RequestURI())
	return file == s
}
