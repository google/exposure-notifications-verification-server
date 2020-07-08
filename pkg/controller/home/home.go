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

// Package home defines a web controller for the home page of the verification
// server. This view allows users to issue OTP codes and tie them to a diagnosis
// and test date.
package home

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware/html"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	httpcontext "github.com/gorilla/context"
	"go.uber.org/zap"
)

type homeController struct {
	config             *config.ServerConfig
	db                 *database.Database
	html               *render.HTML
	logger             *zap.SugaredLogger
	pastDaysDuration   time.Duration
	displayAllowedDays string
}

// New creates a new controller for the home page.
func New(ctx context.Context, config *config.ServerConfig, db *database.Database, html *render.HTML) http.Handler {
	pastDaysDuration := -1 * config.AllowedSymptomAge

	return &homeController{
		config:             config,
		db:                 db,
		html:               html,
		logger:             logging.FromContext(ctx),
		pastDaysDuration:   pastDaysDuration,
		displayAllowedDays: fmt.Sprintf("%.0f", config.AllowedSymptomAge.Hours()/24.0),
	}
}

func (hc *homeController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawUser, ok := httpcontext.GetOk(r, "user")
	if !ok {
		flash.FromContext(w, r).Error("Unauthorized")
		http.Redirect(w, r, "/signout", http.StatusFound)
		return
	}
	user, ok := rawUser.(*database.User)
	if !ok {
		flash.FromContext(w, r).Error("internal error - you have been logged out.")
		http.Redirect(w, r, "/signout", http.StatusFound)
		return
	}

	m := html.GetTemplateMap(r)
	// Set test date params
	now := time.Now().UTC()
	m["maxDate"] = now.Format("2006-01-02")
	m["minDate"] = now.Add(hc.pastDaysDuration).Format("2006-01-02")
	m["maxSymptomDays"] = hc.displayAllowedDays
	m["duration"] = hc.config.CodeDuration.String()
	m["user"] = user
	hc.html.Render(w, "home", m)
}
