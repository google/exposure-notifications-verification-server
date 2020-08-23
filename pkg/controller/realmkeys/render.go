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

package realmkeys

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
)

/*
block, _ := pem.Decode([]byte(k.PublicKeyPEM))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("unable to decode PEM block containing PUBLIC KEY")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("x509.ParsePKIXPublicKey: %w", err)
	}

	switch typ := pub.(type) {
	case *ecdsa.PublicKey:
		return typ, nil
	default:
		return nil, fmt.Errorf("unsupported public key type: %T", typ)
	}
*/

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, r *http.Request, realm *database.Realm) {
	m := controller.TemplateMapFromContext(ctx)
	m["realm"] = realm

	m["supportsPerRealmSigning"] = c.db.SupportsPerRealmSigning()
	if c.db.SupportsPerRealmSigning() {
		c.logger.Infof("listing signing keys")
		keys, err := realm.ListSigningKeys(c.db)
		c.logger.Infow("list result", "error", err, "keys", keys)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
		}
		m["realmKeys"] = keys

		publicKeys := make(map[string]string)
		for _, k := range keys {
			if k.Active {
				m["activeRealmKey"] = k.GetKID()
				m["activePublicKey"] = ""
			}
			pk, err := c.publicKeyCache.GetPublicKey(ctx, k.KeyID, c.db.KeyManager())
			if err != nil {
				publicKeys[k.GetKID()] = fmt.Errorf("error loading public key: %v", err).Error()
			} else {
				pem, err := keyutils.EncodePublicKey(pk)
				if err != nil {
					publicKeys[k.GetKID()] = fmt.Errorf("error decoding public key: %v", err).Error()
				} else {
					publicKeys[k.GetKID()] = pem
					m["activePublicKey"] = pem
				}
			}
		}
		m["publicKeys"] = publicKeys
	}

	if !realm.UseRealmCertificateKey {
		// load the system information.
		m["certIssuer"] = c.config.VerificateSettings.CertificateIssuer
		m["certAudience"] = c.config.VerificateSettings.CertificateAudience
		m["certDuration"] = c.config.VerificateSettings.CertificateDuration
		m["certKeyID"] = c.config.VerificateSettings.CertificateSigningKeyID
		// Download and PEM encode the public key.
		publicKey, err := c.publicKeyCache.GetPublicKey(ctx, c.config.VerificateSettings.CertificateSigningKey, c.db.KeyManager())
		if err != nil {
			m["certPublicKeyError"] = fmt.Sprintf("Error loading public key: %v", err)
		} else {
			pem, err := keyutils.EncodePublicKey(publicKey)
			if err != nil {
				m["certPublicKeyError"] = err.Error()
			} else {
				m["certPublicKey"] = pem
			}
		}
	}

	c.h.RenderHTML(w, "realmkeys", m)
}
