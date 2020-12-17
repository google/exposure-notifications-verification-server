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

// Package issueapi implements the API handler for taking a code request, assigning
// an OTP, saving it to the database and returning the result.
// This is invoked over AJAX from the Web frontend.
package issueapi

import (
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/sethvargo/go-limiter"
)

type Controller struct {
	config  config.IssueAPIConfig
	db      *database.Database
	h       render.Renderer
	limiter limiter.Store

	validTestType map[string]struct{}
}

// New creates a new IssueAPI controller.
func New(config config.IssueAPIConfig, db *database.Database, limiter limiter.Store, h render.Renderer) *Controller {
	return &Controller{
		config:  config,
		db:      db,
		h:       h,
		limiter: limiter,
		validTestType: map[string]struct{}{
			api.TestTypeConfirmed: {},
			api.TestTypeLikely:    {},
			api.TestTypeNegative:  {},
		},
	}
}
