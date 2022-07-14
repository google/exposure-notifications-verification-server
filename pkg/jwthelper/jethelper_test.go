// Copyright 2022 the Exposure Notifications Verification Server authors
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

package jwthelper

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/go-cmp/cmp"
)

func TestSignJWT(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	iat := now.Unix()
	eat := now.Add(time.Hour).Unix()

	claims := &jwt.StandardClaims{
		Audience:  "testing",
		ExpiresAt: eat,
		Id:        "foo",
		IssuedAt:  iat,
		Issuer:    "test_sign_jwt",
		Subject:   "unit_test",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = "v1"

	signedJWT, err := SignJWT(token, key)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	parser := &jwt.Parser{}
	gotToken, err := parser.ParseWithClaims(signedJWT, &jwt.StandardClaims{}, func(tok *jwt.Token) (interface{}, error) {
		// For the test, just return the public key we just created.
		return key.Public(), nil
	})
	if err != nil {
		t.Fatalf("failed to parse signed JWT: %v", err)
	}

	if diff := cmp.Diff(token.Claims, gotToken.Claims); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
