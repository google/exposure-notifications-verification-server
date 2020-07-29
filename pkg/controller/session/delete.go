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

// Package signout hold the controller for signing out a user / destroying their session.
package signout

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware/html"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type signoutController struct {
	config *config.ServerConfig
	db     *database.Database
	html   *render.HTML
}

// New creates a new signout controller. When run, clears the session cookie.
func New(config *config.ServerConfig, db *database.Database, html *render.HTML) http.Handler {
	return &signoutController{config, db, html}
}

func (soc *signoutController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set max age to negative to clear the cookie.
	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Value:  "",
		MaxAge: -1,
	})

	m := html.GetTemplateMap(r)
	m["firebase"] = soc.config.Firebase
	m["flash"] = flash.FromContext(w, r)
	soc.html.Render(w, "signout", m)
}
