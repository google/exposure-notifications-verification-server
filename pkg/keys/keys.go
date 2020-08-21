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

// Package keys defines the interface for signing.
package keys

import (
	"context"
	"crypto"
	"time"
)

// Manager represents the interface to the Key Management System.
type Manager interface {
	NewSigner(ctx context.Context, keyID string) (crypto.Signer, error)

	CreateSigningKeyVersion(ctx context.Context, keyRing string, name string) (string, error)
	GetSigningKeyVersions(ctx context.Context, keyRing string, name string) ([]SigningKeyVersion, error)

	// TODO(mikehelmick): for rotation, implement destroy
	// DestroySigningKeyVersion(ctx context.Context, keyID string) error
}

// SigningKeyVersion represents the necessary deatils that this application needs
// to manage signing keys in an external KMS.
type SigningKeyVersion interface {
	KeyID() string
	CreatedAt() time.Time
	DetroyedAt() time.Time
	GetSigner(ctx context.Context) (crypto.Signer, error)
}
