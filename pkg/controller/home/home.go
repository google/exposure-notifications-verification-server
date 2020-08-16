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
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"go.uber.org/zap"
)

type Controller struct {
	config *config.ServerConfig
	db     *database.Database
	h      *render.Renderer
	logger *zap.SugaredLogger

	pastDaysDuration   time.Duration
	displayAllowedDays string
}

// New creates a new controller for the home page.
func New(ctx context.Context, config *config.ServerConfig, db *database.Database, h *render.Renderer) *Controller {
	logger := logging.FromContext(ctx)

	pastDaysDuration := -1 * config.AllowedSymptomAge
	displayAllowedDays := fmt.Sprintf("%.0f", config.AllowedSymptomAge.Hours()/24.0)

	return &Controller{
		config: config,
		db:     db,
		h:      h,
		logger: logger,

		pastDaysDuration:   pastDaysDuration,
		displayAllowedDays: displayAllowedDays,
	}
}

func (c *Controller) HandleHome() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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
		// Set test date params
		now := time.Now().UTC()
		m["maxDate"] = now.Format("2006-01-02")
		m["minDate"] = now.Add(c.pastDaysDuration).Format("2006-01-02")
		m["maxSymptomDays"] = c.displayAllowedDays
		m["duration"] = realm.CodeDuration.Duration.String()
		m["hasSMSConfig"] = hasSMSConfig
		c.h.RenderHTML(w, "home", m)
	})
}
