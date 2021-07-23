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

package keyutils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// EncodePublicKey returns the base64 encoded PEM block.
func EncodePublicKey(publicKey crypto.PublicKey) (string, error) {
	switch typ := publicKey.(type) {
	case *ecdsa.PublicKey:
		ecdsaKey := publicKey.(*ecdsa.PublicKey)
		derBytes, err := x509.MarshalPKIXPublicKey(ecdsaKey)
		if err != nil {
			return "", fmt.Errorf("unable to parse public key: %w", err)
		}

		block := &pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: derBytes,
		}

		return string(pem.EncodeToMemory(block)), nil
	default:
		return "", fmt.Errorf("unsupported public key type: %T", typ)
	}
}
