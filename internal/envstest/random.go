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

package envstest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
)

// RandomBytes returns a byte slice of random values of the given length.
func RandomBytes(tb testing.TB, length int) []byte {
	buf := make([]byte, length)
	n, err := rand.Read(buf)
	if err != nil {
		tb.Fatal(err)
	}
	if n < length {
		tb.Fatal(fmt.Errorf("insufficient bytes read: %v, expected %v", n, length))
	}
	return buf
}

// RandomString returns a random hex-encoded string of the given length.
func RandomString(tb testing.TB, length int) string {
	return hex.EncodeToString(RandomBytes(tb, length/2))
}
