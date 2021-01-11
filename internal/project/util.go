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

// Package project defines global project helpers.
package project

import (
	"path/filepath"
	"runtime"
)

const (
	// PasswordSentinel is the password string inserted into forms.
	PasswordSentinel = "very-nice-try-maybe-next-time"
)

var _, self, _, _ = runtime.Caller(0)

// Root returns the filepath to the root of this project.
func Root() string {
	return filepath.Join(filepath.Dir(self), "..", "..")
}

// AllDigits returns true if all runes of a string are digits.
func AllDigits(val string) bool {
	if val == "" {
		return false
	}
	for _, c := range val {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
