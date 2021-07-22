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

// Package mobileapps contains web controllers for listing and adding mobile
// apps.
package mobileapps

import (
	"context"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type Controller struct {
	db *database.Database
	h  *render.Renderer
}

func New(db *database.Database, h *render.Renderer) *Controller {
	return &Controller{
		db: db,
		h:  h,
	}
}

func templateMap(ctx context.Context) controller.TemplateMap {
	m := controller.TemplateMapFromContext(ctx)
	m["iOS"] = database.OSTypeIOS
	m["android"] = database.OSTypeAndroid
	return m
}
