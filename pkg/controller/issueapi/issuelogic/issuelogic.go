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

// Package issuelogic contains the core logic for validating an issue request,
// validating the request, sending SMS messages, and converting to a response.
// It's expected that the caller handles rendering etc.
package issuelogic

import (
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-limiter"
)

type IssueLogic struct {
	config     config.IssueAPIConfig
	db         *database.Database
	limiter    limiter.Store
	authApp    *database.AuthorizedApp
	membership *database.Membership
	realm      *database.Realm
}

// New creates a new issue logic controller.
func New(config config.IssueAPIConfig, db *database.Database, limiter limiter.Store,
	authApp *database.AuthorizedApp, membership *database.Membership, realm *database.Realm) *IssueLogic {
	return &IssueLogic{
		config:     config,
		db:         db,
		limiter:    limiter,
		authApp:    authApp,
		membership: membership,
		realm:      realm,
	}
}
