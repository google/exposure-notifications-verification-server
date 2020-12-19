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

// Package apikey contains web controllers for listing and adding API Keys.
package apikey

import (
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type Controller struct {
	cacher cache.Cacher
	db     *database.Database
	h      render.Renderer
}

func New(cacher cache.Cacher, db *database.Database, h render.Renderer) *Controller {
	return &Controller{
		cacher: cacher,
		db:     db,
		h:      h,
	}
}
