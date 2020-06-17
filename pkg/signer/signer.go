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

// Package signer defines the interface for signing.
// TODO(mikehelmick) - this needs to be simplified. Do away w/ dynamic registry
// and more more towards what we have in exposure-notifications-server
// Make signing impl in exposure-notifications-server in pkg so we can depend on it.
package signer

import (
	"context"
	"crypto"
)

type KeyManager interface {
	NewSigner(ctx context.Context, keyID string) (crypto.Signer, error)
}
