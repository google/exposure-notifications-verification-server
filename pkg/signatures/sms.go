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

package signatures

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
)

const (
	// dot represents a period and colon represents a colon, which are used as a
	// separator in some signatures.
	dot   = "."
	colon = ":"

	// authPrefix is the prefix to the authentication bits.
	authPrefix = "\nAuthentication:"
)

// SMSPurpose is an SMS purpose, used in signature calculation.
type SMSPurpose string

const (
	// SMSPurposeENReport is an SMS purpose for EN reporting.
	SMSPurposeENReport   SMSPurpose = "EN Report"
	SMSPurposeUserReport SMSPurpose = "User Report"
)

// SignSMS returns a new SMS message with the embedded signature using the
// provided signer.
func SignSMS(signer crypto.Signer, keyID string, t time.Time, purpose SMSPurpose, phone, body string) (string, error) {
	t = t.UTC()

	newBody := body + authPrefix
	signingString := smsSignatureString(t, purpose, phone, newBody)

	digest := sha256.Sum256([]byte(signingString))
	sig, err := signer.Sign(rand.Reader, digest[:], nil)
	if err != nil {
		return "", fmt.Errorf("failed to sign sms: %w", err)
	}

	date := t.Format("0102")
	return newBody + date + colon + keyID + colon + b64(sig), nil
}

// smsSignatureString builds the string that is to be signed. The provided date
// will be converted to UTC and then appended in ISO 8601 format. The phone
// number must be in E.164 format. The body must be the complete message body
// including any codes and links.
func smsSignatureString(t time.Time, purpose SMSPurpose, phone, body string) string {
	t = t.UTC()
	return string(purpose) + dot + phone + dot + t.Format(project.RFC3339Date) + dot + body
}

// b64 is a helper that does base64 encoding for signatures.
func b64(in []byte) string {
	return base64.RawStdEncoding.EncodeToString(in)
}
