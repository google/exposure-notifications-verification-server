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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func Test_SMSSignature(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		keyID string
		time  time.Time
		phone string
		body  string
	}{
		{
			name:  "default",
			keyID: "v1",
			time:  time.Unix(0, 0),
			phone: "+11111111111",
			body:  "This is a message",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := SignSMS(key, tc.keyID, tc.time, SMSPurposeENReport, tc.phone, tc.body)
			if err != nil {
				t.Fatal(err)
			}

			parts := strings.Split(result, ":")

			for i, part := range parts {
				switch i {
				case 0:
					// First is "Authentication:"
					if !strings.HasSuffix(part, "\nAuthentication") {
						t.Errorf("missing authentication prefix: %q", part)
					}
				case 1:
					// Next is date
					date := tc.time.UTC().Format("0102")
					if got, want := part, date; got != want {
						t.Errorf("Expected %q to be %q", got, want)
					}
				case 2:
					// Next is key id
					if got, want := part, tc.keyID; got != want {
						t.Errorf("Expected %q to be %q", got, want)
					}
				case 3:
					// Next is signature
					b, err := base64.RawStdEncoding.DecodeString(part)
					if err != nil {
						t.Fatal(err)
					}

					signingString := smsSignatureString(tc.time, SMSPurposeENReport, tc.phone, tc.body+authPrefix)
					digest := sha256.Sum256([]byte(signingString))

					if !ecdsa.VerifyASN1(&key.PublicKey, digest[:], b) {
						t.Error("did not verify")
					}
				default:
					t.Fatalf("too many parts: %#v", parts)
				}
			}
		})
	}
}

func Test_smsSignatureString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{
			name: "default",
			in:   smsSignatureString(time.Unix(0, 0), SMSPurposeENReport, "+11111111111", "Hello world"),
			exp:  "EN Report.+11111111111.1970-01-01.Hello world",
		},
		{
			name: "default",
			in:   smsSignatureString(time.Unix(0, 0).In(time.FixedZone("Test/Test", -10000000)), SMSPurposeENReport, "+11111111111", "Hello world"),
			exp:  "EN Report.+11111111111.1970-01-01.Hello world",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got, want := tc.in, tc.exp; got != want {
				t.Errorf("expected %v to be %v", got, want)
			}
		})
	}
}
