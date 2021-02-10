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
	"context"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/sethvargo/go-limiter"
	"go.opencensus.io/tag"
)

type Controller struct {
	config     config.IssueAPIConfig
	db         *database.Database
	localCache *cache.Cache
	limiter    limiter.Store
	smsSigner  keys.KeyManager
	h          *render.Renderer
}

// New creates a new IssueAPI controller.
func New(cfg config.IssueAPIConfig, db *database.Database, limiter limiter.Store, smsSigner keys.KeyManager, h *render.Renderer) *Controller {
	localCache, _ := cache.New(5 * time.Minute)

	return &Controller{
		config:     cfg,
		db:         db,
		localCache: localCache,
		limiter:    limiter,
		smsSigner:  smsSigner,
		h:          h,
	}
}

func recordObservability(ctx context.Context, startTime time.Time, result *IssueResult) {
	var blame tag.Mutator
	switch result.HTTPCode {
	case http.StatusOK:
		blame = enobs.BlameNone
	case http.StatusInternalServerError:
		blame = enobs.BlameServer
	default:
		blame = enobs.BlameClient
	}

	enobs.RecordLatency(ctx, startTime, mLatencyMs, &blame, &result.obsResult)
}
