// Copyright 2021 Google LLC
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

// Package rotation implements periodic secret rotation.
package rotation

import (
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

const (
	tokenRotationLock        = "tokenRotationLock"
	verificationRotationLock = "verificationRotationLock"
)

type Controller struct {
	config     *config.RotationConfig
	db         *database.Database
	keyManager keys.SigningKeyManager
	h          *render.Renderer
}

func New(cfg *config.RotationConfig, db *database.Database, keyManager keys.SigningKeyManager, h *render.Renderer) *Controller {
	return &Controller{
		config:     cfg,
		db:         db,
		keyManager: keyManager,
		h:          h,
	}
}

// RotationActor is the actor in the database for rotation events.
var RotationActor database.Auditable = new(rotationActor)

type rotationActor struct{}

func (s *rotationActor) AuditID() string {
	return "rotation:1"
}

func (s *rotationActor) AuditDisplay() string {
	return "Rotation"
}
