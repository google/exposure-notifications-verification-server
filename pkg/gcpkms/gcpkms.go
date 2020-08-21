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

// Package gcpkms implements cryptographic signing using Google Cloud Key Management Service.
package gcpkms

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/keys"

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/sethvargo/go-gcpkms/pkg/gcpkms"
	"google.golang.org/api/iterator"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

// Compile time check that GCPKeyManager satisfies signer interface
var _ keys.Manager = (*GCPKeyManager)(nil)

// GCPKeyManager providers a crypto.Signer that uses GCP KSM to sign bytes.
type GCPKeyManager struct {
	client *kms.KeyManagementClient
	config *Config
}

type kmsSigningKeyVersion struct {
	keyID       string
	createdAt   time.Time
	destroyedAt time.Time
	kayManager  *GCPKeyManager
}

func (k *kmsSigningKeyVersion) KeyID() string {
	return k.keyID
}

func (k *kmsSigningKeyVersion) CreatedAt() time.Time {
	return k.createdAt
}

func (k *kmsSigningKeyVersion) DetroyedAt() time.Time {
	return k.destroyedAt
}

func (k *kmsSigningKeyVersion) GetSigner(ctx context.Context) (crypto.Signer, error) {
	return k.kayManager.NewSigner(ctx, k.keyID)
}

// New creates a new Google Cloud KMS client confirming to keys.Manager.
// The parent resource indicates where new keys should be created.
func New(ctx context.Context, config *Config) (keys.Manager, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GCPKeyManager{client, config}, nil
}

func (g *GCPKeyManager) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	signer, err := gcpkms.NewSigner(ctx, g.client, keyID)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

func (g *GCPKeyManager) CreateSigningKeyVersion(ctx context.Context, keyRing string, name string) (string, error) {
	logger := logging.FromContext(ctx)
	getRequest := kmspb.GetCryptoKeyRequest{
		Name: fmt.Sprintf("%s/cryptoKeys/%s", keyRing, name),
	}
	logger.Infow("gcpkms.GetCryptoKey", "keyring", keyRing, "name", name)
	key, err := g.client.GetCryptoKey(ctx, &getRequest)
	if err != nil {
		return "", fmt.Errorf("cannot create version, key does not exist: %w", err)
	}

	createRequest := kmspb.CreateCryptoKeyVersionRequest{
		Parent: key.Name,
	}
	ver, err := g.client.CreateCryptoKeyVersion(ctx, &createRequest)
	if err != nil {
		return "", fmt.Errorf("gcpkms.CreateCryptoKeyVersion: %w", err)
	}
	return ver.Name, nil
}

func (g *GCPKeyManager) GetSigningKeyVersions(ctx context.Context, keyRing string, name string) ([]keys.SigningKeyVersion, error) {
	_, err := g.getOrCreateCryptoKey(ctx, keyRing, name)
	if err != nil {
		return nil, fmt.Errorf("unable to get crypto key: %w", err)
	}

	request := kmspb.ListCryptoKeyVersionsRequest{
		Parent:   fmt.Sprintf("%s/cryptoKeys/%s", keyRing, name),
		PageSize: 200,
	}

	results := make([]keys.SigningKeyVersion, 0)

	it := g.client.ListCryptoKeyVersions(ctx, &request)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error listing crypto keys: %w", err)
		}

		key := kmsSigningKeyVersion{
			keyID:      resp.Name,
			createdAt:  resp.GetCreateTime().AsTime(),
			kayManager: g,
		}
		if resp.DestroyEventTime != nil {
			key.destroyedAt = resp.GetDestroyEventTime().AsTime()
		}
		results = append(results, &key)
	}

	return results, nil
}

func (g GCPKeyManager) ProtectionLevel() kmspb.ProtectionLevel {
	if g.config.UseHSM {
		return kmspb.ProtectionLevel_HSM
	}
	return kmspb.ProtectionLevel_SOFTWARE
}

// projects/apollo-verification-us/locations/us/keyRings/signing/cryptoKeys/certificate-signing
func (g GCPKeyManager) getOrCreateCryptoKey(ctx context.Context, keyRing string, name string) (*kmspb.CryptoKey, error) {
	logger := logging.FromContext(ctx)
	getRequest := kmspb.GetCryptoKeyRequest{
		Name: fmt.Sprintf("%s/cryptoKeys/%s", keyRing, name),
	}
	logger.Infow("gcpkms.GetCryptoKey", "keyring", keyRing, "name", name)
	key, err := g.client.GetCryptoKey(ctx, &getRequest)
	if err == nil {
		return key, nil
	}

	// Attempt to create the crypto key in this key ring w/ our default settings.
	createRequest := kmspb.CreateCryptoKeyRequest{
		Parent:      fmt.Sprintf("%s", keyRing),
		CryptoKeyId: name,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ASYMMETRIC_SIGN,
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				ProtectionLevel: g.ProtectionLevel(),
				Algorithm:       kmspb.CryptoKeyVersion_EC_SIGN_P256_SHA256,
			},
		},
	}
	key, err = g.client.CreateCryptoKey(ctx, &createRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to create signing key: %w", err)
	}
	return key, nil
}
