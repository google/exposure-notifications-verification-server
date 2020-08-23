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

package certapi

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

type SignerInfo struct {
	Signer   crypto.Signer
	KeyID    string
	Issuer   string
	Audience string
	Duration time.Duration
}

func (c *Controller) getSignerForRealm(ctx context.Context, authApp *database.AuthorizedApp) (*SignerInfo, error) {
	sRealmID := fmt.Sprintf("%d", authApp.RealmID)
	signerCache, err := c.signerCache.WriteThruLookup(sRealmID,
		func() (interface{}, error) {
			realm, err := authApp.Realm(c.db)
			if err != nil {
				return nil, fmt.Errorf("unable to load realm settings: %w", err)
			}

			if !realm.UseRealmCertificateKey {
				// This realm is using the sytem key.
				signer, err := c.kms.NewSigner(ctx, c.config.VerificateSettings.CertificateSigningKey)
				if err != nil {
					return nil, fmt.Errorf("unable to get signing key from key manager: realmId: %v: %w", sRealmID, err)
				}
				return &SignerInfo{
					Signer:   signer,
					KeyID:    c.config.VerificateSettings.CertificateSigningKeyID,
					Issuer:   c.config.VerificateSettings.CertificateIssuer,
					Audience: c.config.VerificateSettings.CertificateAudience,
					Duration: c.config.VerificateSettings.CertificateDuration,
				}, nil
			}

			// Relam has custom signing keys.
			signingKey, err := realm.GetCurrentSigningKey(c.db)
			if err != nil || signingKey == nil {
				return nil, fmt.Errorf("unable to find current signing key for realm: %v: %w", realm.Model.ID, err)
			}

			// load the crypto.Signer for this keyID
			signer, err := c.kms.NewSigner(ctx, signingKey.KeyID)
			if err != nil {
				return nil, fmt.Errorf("unable to get signing key from key manager: realmId: %v: %w", sRealmID, err)
			}
			return &SignerInfo{
				Signer:   signer,
				KeyID:    signingKey.GetKID(),
				Issuer:   realm.CertificateIssuer,
				Audience: realm.CertificateAudience,
				Duration: realm.CertificateDuration.Duration,
			}, nil
		})
	if err != nil {
		return nil, fmt.Errorf("unable to get signer for realmID: %v: %w", sRealmID, err)
	}
	signer, ok := signerCache.(*SignerInfo)
	if !ok {
		return nil, fmt.Errorf("certificate signer in wrong format for realmID: %v: %w", sRealmID, err)
	}
	return signer, nil
}
