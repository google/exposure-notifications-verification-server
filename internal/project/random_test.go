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

package project

import (
	"testing"
)

func TestRandomHexString(t *testing.T) {
	t.Parallel()

	length := 255
	s, err := RandomHexString(length)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(s), length; got != want {
		t.Errorf("random hex string incorrect length. got %d want %d", got, want)
	}
}

func TestRandomBase64String(t *testing.T) {
	t.Parallel()

	length := 255
	s, err := RandomBase64String(length)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(s), length; got != want {
		t.Errorf("random base64 string incorrect length. got %d want %d", got, want)
	}
}

func TestRandomBytes(t *testing.T) {
	t.Parallel()

	length := 255
	b, err := RandomBytes(length)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(b), length; got != want {
		t.Errorf("random bytes incorrect length. got %d want %d", got, want)
	}
}
