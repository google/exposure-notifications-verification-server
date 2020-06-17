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

package controller

import (
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/gin-gonic/gin"
)

type SessionHelper struct {
	db     *database.Database
	config *config.Config
}

func NewSessionHelper(config *config.Config, db *database.Database) *SessionHelper {
	return &SessionHelper{db, config}
}

func (s *SessionHelper) SaveSession(c *gin.Context, cookie string) {
	c.SetCookie("session", cookie, int(s.config.SessionCookieDuration.Seconds()), "/", "", false, false)
}

func (s *SessionHelper) DestroySession(c *gin.Context) {
	// negative time deletes a cookie.
	c.SetCookie("session", "deleted", -1, "/", "", false, false)
}
