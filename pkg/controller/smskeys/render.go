// Copyright 2021 the Exposure Notifications Verification Server authors
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

package smskeys

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
)

func (c *Controller) redirectShow(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/realm/sms-keys", http.StatusSeeOther)
}

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, r *http.Request, realm *database.Realm) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Authenticated SMS Signing keys")
	m["realm"] = realm

	keys, err := realm.ListSMSSigningKeys(c.db)
	if err != nil {
		controller.InternalError(w, r, c.h, err)
		return
	}

	m["realmKeys"] = keys

	maximumKeyVersions := c.db.MaxKeyVersions()
	m["maximumKeyVersions"] = maximumKeyVersions

	publicKeys := make(map[string]string)
	// Go through and load / parse all of the public keys for the realm.
	for _, k := range keys {
		if k.Active {
			m["activeRealmKey"] = k.GetKID()
			m["activePublicKey"] = ""
		}
		pk, err := c.publicKeyCache.GetPublicKey(ctx, k.KeyID, c.db.KeyManager())
		if err != nil {
			publicKeys[k.GetKID()] = fmt.Errorf("error loading public key: %w", err).Error()
		} else {
			pem, err := keyutils.EncodePublicKey(pk)
			if err != nil {
				publicKeys[k.GetKID()] = fmt.Errorf("error decoding public key: %w", err).Error()
			} else {
				publicKeys[k.GetKID()] = pem
				if k.Active {
					m["activePublicKey"] = pem
				}
			}
		}
	}
	m["publicKeys"] = publicKeys

	c.h.RenderHTML(w, "realmadmin/smskeys", m)
}
