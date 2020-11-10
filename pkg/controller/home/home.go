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
	"html/template"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type Controller struct {
	config *config.ServerConfig
	db     *database.Database
	h      *render.Renderer

	pastDaysDuration   time.Duration
	displayAllowedDays string
}

// New creates a new controller for the home page.
func New(ctx context.Context, config *config.ServerConfig, db *database.Database, h *render.Renderer) *Controller {
	pastDaysDuration := -1 * config.AllowedSymptomAge
	displayAllowedDays := fmt.Sprintf("%.0f", config.AllowedSymptomAge.Hours()/24.0)

	return &Controller{
		config: config,
		db:     db,
		h:      h,

		pastDaysDuration:   pastDaysDuration,
		displayAllowedDays: displayAllowedDays,
	}
}

func (c *Controller) HandleHome() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		hasSMSConfig, err := realm.HasSMSConfig(c.db)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		m.Title("Issue code")

		// Set test date params
		now := time.Now().UTC()
		m["maxDate"] = now.Format("2006-01-02")
		m["minDate"] = now.Add(c.pastDaysDuration).Format("2006-01-02")
		m["maxSymptomDays"] = c.displayAllowedDays
		m["duration"] = realm.CodeDuration.Duration.String()
		m["hasSMSConfig"] = hasSMSConfig

		// If the realm has a welcome message and it has not been displayed this
		// session, display it.
		if realm.WelcomeMessage != "" && !controller.WelcomeMessageDisplayedFromSession(session) {
			// Don't display it again.
			controller.StoreSessionWelcomeMessageDisplayed(session, true)

			// This is marked as HTML safe because it's run through bluemonday during
			// parsing. Also, realm admins are mostly trusted to not XSS themselves.
			m["welcomeMessage"] = template.HTML(realm.RenderWelcomeMessage())
		}

		// Render
		c.h.RenderHTML(w, "home", m)
	})
}
