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

// Package jwthelper implements some common methods on top of the JWT library.
// In particular, we handle signing in this code base and don't delegate to the library
// since we're using an external signer.
package jwthelper

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"fmt"
	"math/big"
	"strings"

	"github.com/golang-jwt/jwt"
)

// keyBytes is 256 bits / 32 bytes.
const keyBytes = 32

// SignJWT takes a JWT structure, extracts the signing string and signs it with the
// provided signer. The base64 serialized JWT is returned.
func SignJWT(token *jwt.Token, signer crypto.Signer) (string, error) {
	signingString, err := token.SigningString()
	if err != nil {
		return "", err
	}

	digest := sha256.Sum256([]byte(signingString))
	sig, err := signer.Sign(rand.Reader, digest[:], nil)
	if err != nil {
		return "", fmt.Errorf("error signing token: %w", err)
	}

	// Unpack the ASN1 signature. ECDSA signers are supposed to return this format
	// https://golang.org/pkg/crypto/#Signer
	// All supported signers in thise codebase are verified to return ASN1.
	var parsedSig struct{ R, S *big.Int }
	// ASN1 is not the expected format for an ES256 JWT signature.
	// The output format is specified here, https://tools.ietf.org/html/rfc7518#section-3.4
	// Reproduced here for reference.
	//    The ECDSA P-256 SHA-256 digital signature is generated as follows:
	//
	// 1 .  Generate a digital signature of the JWS Signing Input using ECDSA
	//      P-256 SHA-256 with the desired private key.  The output will be
	//      the pair (R, S), where R and S are 256-bit unsigned integers.
	_, err = asn1.Unmarshal(sig, &parsedSig)
	if err != nil {
		return "", fmt.Errorf("unable to unmarshal signature: %w", err)
	}

	// 2. Turn R and S into octet sequences in big-endian order, with each
	// 		array being be 32 octets long.  The octet sequence
	// 		representations MUST NOT be shortened to omit any leading zero
	// 		octets contained in the values.
	rBytes := parsedSig.R.Bytes()
	rBytesPadded := make([]byte, keyBytes)
	copy(rBytesPadded[keyBytes-len(rBytes):], rBytes)

	sBytes := parsedSig.S.Bytes()
	sBytesPadded := make([]byte, keyBytes)
	copy(sBytesPadded[keyBytes-len(sBytes):], sBytes)

	// 3. Concatenate the two octet sequences in the order R and then S.
	//	 	(Note that many ECDSA implementations will directly produce this
	//	 	concatenation as their output.)
	sig = make([]byte, 0, len(rBytesPadded)+len(sBytesPadded))
	sig = append(sig, rBytesPadded...)
	sig = append(sig, sBytesPadded...)

	return strings.Join([]string{signingString, jwt.EncodeSegment(sig)}, "."), nil
}
