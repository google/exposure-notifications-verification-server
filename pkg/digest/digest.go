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

// Package digest includes common digest helpers
package digest

import (
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"strconv"
)

// HMAC returns the sha1 HMAC of a given string as a hex-encoded string, and any
// errors that occur while hashing.
func HMAC(in string, key []byte) (string, error) {
	h := hmac.New(sha1.New, key)
	n, err := h.Write([]byte(in))
	if err != nil {
		return "", err
	}
	if got, want := n, len(in); got < want {
		return "", fmt.Errorf("only hashed %d of %d bytes", got, want)
	}
	dig := h.Sum(nil)
	return fmt.Sprintf("%x", dig), nil
}

func HMACInt(in int, key []byte) (string, error) {
	return HMACInt64(int64(in), key)
}

func HMACInt64(in int64, key []byte) (string, error) {
	return HMAC(strconv.FormatInt(in, 10), key)
}

func HMACUint(in uint, key []byte) (string, error) {
	return HMACUint64(uint64(in), key)
}

func HMACUint64(in uint64, key []byte) (string, error) {
	return HMAC(strconv.FormatUint(in, 10), key)
}
