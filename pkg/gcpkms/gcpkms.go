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

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/google/exposure-notifications-verification-server/pkg/keys"
	"github.com/sethvargo/go-gcpkms/pkg/gcpkms"
)

// Compile time check that GCPKeyManager satisfies signer interface
var _ keys.Manager = (*GCPKeyManager)(nil)

// GCPKeyManager providers a crypto.Signer that uses GCP KSM to sign bytes.
type GCPKeyManager struct {
	client *kms.KeyManagementClient
}

func New(ctx context.Context) (keys.Manager, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GCPKeyManager{client}, nil
}

func (g *GCPKeyManager) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	signer, err := gcpkms.NewSigner(ctx, g.client, keyID)
	if err != nil {
		return nil, err
	}
	return signer, nil
}
