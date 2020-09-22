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

package mobileapp

import (
	"testing"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

var goodSHA = "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA"

func TestVerifySHA(t *testing.T) {
	tests := []struct {
		sha      string
		expected error
	}{
		{"", ErrBadSHALength},
		{"abc", ErrBadSHALength},
		{";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;", ErrBadSHALength},
		{"A:AAA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA", ErrBadSHAFormat},
		{goodSHA, nil},
	}

	for i, test := range tests {
		if res := verifySHA(test.sha); res != test.expected {
			t.Errorf("[%d] verifySHA(%q) = %v, expected %v", i, test.sha, res, test.expected)
		}
	}
}

func TestFormVerify(t *testing.T) {
	tests := []struct {
		form     FormData
		expected error
	}{
		{FormData{OS: database.OSTypeIOS}, ErrNoAppIDSpecified},
		{FormData{OS: database.OSTypeAndroid}, ErrNoAppIDSpecified},
		{FormData{OS: database.OSTypeIOS, AppID: "foo"}, nil},
		{FormData{OS: database.OSTypeAndroid, AppID: "foo"}, ErrBadSHALength},
		{FormData{OS: database.OSTypeAndroid, AppID: "foo", SHA: goodSHA}, nil},
	}

	for i, test := range tests {
		if res := test.form.verify(); res != test.expected {
			t.Errorf("[%d] form.vertify() = %v, expected %v", i, res, test.expected)
		}
	}
}
