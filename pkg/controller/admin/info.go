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

package admin

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/jinzhu/gorm"
)

// HandleInfoShow renders the list of system admins.
func (c *Controller) HandleInfoShow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		m := controller.TemplateMapFromContext(ctx)

		// Keys
		tokenSigningKeys, err := c.db.ListTokenSigningKeys()
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}
		m["tokenSigningKeys"] = tokenSigningKeys

		// Secrets
		secrets, err := c.db.ListSecrets(func(db *gorm.DB) *gorm.DB {
			return db.Order("secrets.type")
		}, database.InConsumableSecretOrder())
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}
		secretsMap := make(map[database.SecretType][]*database.Secret)
		for _, secret := range secrets {
			secretsMap[secret.Type] = append(secretsMap[secret.Type], secret)
		}
		m["secrets"] = secretsMap

		m.Title("Info - System Admin")
		c.h.RenderHTML(w, "admin/info", m)
	})
}
