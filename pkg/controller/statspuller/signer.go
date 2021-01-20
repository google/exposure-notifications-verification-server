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

package statspuller

import (
	"context"
	"crypto"
	"fmt"
)

type SignerInfo struct {
	Signer crypto.Signer
	KeyID  string
	Issuer string
}

func (c *Controller) getSignerForRealm(ctx context.Context, realmID uint) (*SignerInfo, error) {
	signerCache, err := c.signerCache.WriteThruLookup(fmt.Sprintf("%d", realmID),
		func() (interface{}, error) {
			realm, err := c.db.FindRealm(realmID)
			if err != nil {
				return nil, fmt.Errorf("unable to load realm settings: %w", err)
			}

			if !realm.UseRealmCertificateKey {
				// This realm is using the system key.
				signer, err := c.kms.NewSigner(ctx, c.config.CertificateSigning.CertificateSigningKey)
				if err != nil {
					return nil, fmt.Errorf("unable to get signing key from key manager: realmId: %d: %w", realmID, err)
				}
				return &SignerInfo{
					Signer: signer,
					KeyID:  c.config.CertificateSigning.CertificateSigningKeyID,
					Issuer: c.config.CertificateSigning.CertificateIssuer,
				}, nil
			}

			// Realm has custom signing keys.
			signingKey, err := realm.GetCurrentSigningKey(c.db)
			if err != nil || signingKey == nil {
				return nil, fmt.Errorf("unable to find current signing key for realm: %v: %w", realm.Model.ID, err)
			}

			// load the crypto.Signer for this keyID
			signer, err := c.kms.NewSigner(ctx, signingKey.KeyID)
			if err != nil {
				return nil, fmt.Errorf("unable to get signing key from key manager: realmId: %d: %w", realmID, err)
			}
			return &SignerInfo{
				Signer: signer,
				KeyID:  signingKey.GetKID(),
				Issuer: realm.CertificateIssuer,
			}, nil
		})
	if err != nil {
		return nil, fmt.Errorf("unable to get signer for realmID: %d: %w", realmID, err)
	}
	signer, ok := signerCache.(*SignerInfo)
	if !ok {
		return nil, fmt.Errorf("certificate signer in wrong format for realmID: %d: %w", realmID, err)
	}
	return signer, nil
}
