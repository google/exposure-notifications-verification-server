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

package database

// ManagedKey is an interface that allows for a realm to manage signing keys
// for different purposes.
type ManagedKey interface {
	Auditable

	// GetKID returns the public key version string
	GetKID() string
	// ManagedKeyID returns the reference to the key ID in the KMS.
	ManagedKeyID() string
	// IsActive() returns true if this key is active
	IsActive() bool

	SetManagedKeyID(keyID string)
	SetActive(active bool)

	// These are expected to be static across all instances of an implementing type.
	Table() string
	Purpose() string
}

// RealmManagedKey indicates that this key is owned by a realm.
type RealmManagedKey interface {
	ManagedKey
	SetRealmID(id uint)
}
