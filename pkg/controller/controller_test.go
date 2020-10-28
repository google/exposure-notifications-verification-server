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

package controller

import "testing"

func TestPrefixInList(t *testing.T) {
	tests := []struct {
		target   string
		strs     []string
		expected bool
	}{
		{"foo", []string{}, false},
		{"foo", []string{"foo", "bar"}, true},
		{"foo", []string{"bar", "foofoofoo"}, true},
		{"foo", []string{"Foo", "bar"}, false},
		{"foo", []string{" foo", "bar"}, false},
	}
	for i, test := range tests {
		if res := prefixInList(test.strs, test.target); res != test.expected {
			t.Errorf("[%d] prefixInList(%v, %v) = %v, expected %v", i, test.strs, test.target, res, test.expected)
		}
	}
}
