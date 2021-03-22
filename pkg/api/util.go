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

package api

import (
	"fmt"
	"strings"
)

// AcceptTypes is a map of string to struct.
type AcceptTypes map[string]struct{}

// GetAcceptedTestTypes validates the AcceptedTestTypes of a VerifyCodeRequest
// and upconverts the list into a set (map) of the effective accepted test types.
// If an invalid test type is in the API request, an error is returned.
func (v *VerifyCodeRequest) GetAcceptedTestTypes() (AcceptTypes, error) {
	accepted := AcceptTypes{}
	if len(v.AcceptTestTypes) == 0 {
		accepted.AddAcceptTypes(TestTypeConfirmed, TestTypeLikely, TestTypeNegative)
		return accepted, nil
	}

	for _, testType := range v.AcceptTestTypes {
		testType = strings.ToLower(testType)
		switch testType {
		case TestTypeConfirmed:
			accepted.AddAcceptTypes(TestTypeConfirmed)
		case TestTypeLikely:
			accepted.AddAcceptTypes(TestTypeConfirmed, TestTypeLikely)
		case TestTypeNegative:
			accepted.AddAcceptTypes(TestTypeConfirmed, TestTypeLikely, TestTypeNegative)
		case TestTypeUserReport:
			accepted.AddAcceptTypes(TestTypeConfirmed, TestTypeUserReport)
		default:
			return nil, fmt.Errorf("invalid accepted test type: %v", testType)
		}
	}

	return accepted, nil
}

// AddAcceptTypes takes a list of acceptable types and converts it to a map.
func (a AcceptTypes) AddAcceptTypes(types ...string) {
	for _, t := range types {
		a[t] = struct{}{}
	}
}
