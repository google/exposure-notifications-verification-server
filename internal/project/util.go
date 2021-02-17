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
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

const (
	// PasswordSentinel is the password string inserted into forms.
	PasswordSentinel = "very-nice-try-maybe-next-time" //nolint:gosec
)

// SkipE2ESMS controls whether the e2e runners use SMS.
var SkipE2ESMS, _ = strconv.ParseBool(os.Getenv("E2E_SKIP_SMS"))

// TestPhoneNumber is a test phone number to use for tests. It's a "real" phone
// number (Google's support phone number), but none of the tests actually send
// SMS.
const TestPhoneNumber = "+18558361987"

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

// TestTimeout controls the individual test timeout, primarily used on a context
// for chromedp tests. By default, it's 2min, but it can be controlled with the
// TEST_TIMEOUT environment variable.
func TestTimeout() time.Duration {
	if v, _ := time.ParseDuration(os.Getenv("TEST_TIMEOUT")); v != 0 {
		return v
	}
	return 120 * time.Second
}
