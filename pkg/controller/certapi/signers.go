// Copyright 2020 the Exposure Notifications Verification Server authors
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

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

type SignerInfo struct {
	Signer   crypto.Signer
	KeyID    string
	Issuer   string
	Audience string
	Duration time.Duration
}

func (c *Controller) getSignerForAuthApp(ctx context.Context, authApp *database.AuthorizedApp) (*SignerInfo, error) {
	return GetSignerForRealm(ctx, authApp.RealmID, c.config.CertificateSigning, c.signerCache, c.db, c.kms)
}

// GetSignerForRealm gets the certificate signer info for the given realm.
func GetSignerForRealm(ctx context.Context, realmID uint,
	cfg config.CertificateSigningConfig, cache *cache.Cache[*SignerInfo], db *database.Database, kms keys.KeyManager) (*SignerInfo, error) {
	signer, err := cache.WriteThruLookup(fmt.Sprintf("%d", realmID),
		func() (*SignerInfo, error) {
			realm, err := db.FindRealm(realmID)
			if err != nil {
				return nil, fmt.Errorf("unable to load realm settings: %w", err)
			}

			if !realm.UseRealmCertificateKey {
				// This realm is using the system key.
				signer, err := kms.NewSigner(ctx, cfg.CertificateSigningKey)
				if err != nil {
					return nil, fmt.Errorf("unable to get signing key from key manager: realmId: %d: %w", realmID, err)
				}
				return &SignerInfo{
					Signer:   signer,
					KeyID:    cfg.CertificateSigningKeyID,
					Issuer:   cfg.CertificateIssuer,
					Audience: cfg.CertificateAudience,
					Duration: cfg.CertificateDuration,
				}, nil
			}

			// Realm has custom signing keys.
			signingKey, err := realm.CurrentSigningKey(db)
			if err != nil || signingKey == nil {
				return nil, fmt.Errorf("unable to find current signing key for realm: %v: %w", realm.Model.ID, err)
			}

			// load the crypto.Signer for this keyID
			signer, err := kms.NewSigner(ctx, signingKey.KeyID)
			if err != nil {
				return nil, fmt.Errorf("unable to get signing key from key manager: realmId: %d: %w", realmID, err)
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
		return nil, fmt.Errorf("unable to get signer for realmID: %d: %w", realmID, err)
	}
	return signer, nil
}
