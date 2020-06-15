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

// Package risk contains the implementation of transmission risk override calculations.
// This is baed on test type and test date. From there, a set of transmission risk overrides
// is calculated.
//
// This server never has access to the actual temporary exposure keys, their rolling periods,
// or app assigned transmission risks.
package risk

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

// TestType represents risk calculation parameters.
type TestType struct {
	baseRisk  int
	daysValid int
	decays    bool
	decayDays int
	decayRisk int
}

var (
	TestTypeConfirmed *TestType
	TestTypeNegative  *TestType
	TestTypeLikely    *TestType
)

func init() {
	// TODO(mikehelmick) - THESE PARAMETERS NEED TO BE CONFIRMED
	TestTypeConfirmed = &TestType{
		baseRisk:  2,
		daysValid: 14,
		decays:    true,
		decayDays: 4,
		decayRisk: 1}
	TestTypeNegative = &TestType{
		baseRisk:  6,
		daysValid: 14,
		decays:    false,
	}
	TestTypeLikely = &TestType{
		baseRisk:  4,
		daysValid: 14,
		decays:    true,
		decayDays: 4,
		decayRisk: 0,
	}
}

type Calculator struct {
	TestType *TestType
	TestDate *time.Time
}

// New takes a test type and date and constructs a calculator.
func New(testType string, testDate *time.Time) (*Calculator, error) {
	t := strings.ToLower(testType)
	switch t {
	case "confirmed":
		return &Calculator{TestTypeConfirmed, testDate}, nil
	case "likely":
		return &Calculator{TestTypeLikely, testDate}, nil
	case "negative":
		return &Calculator{TestTypeNegative, testDate}, nil
	}
	return nil, fmt.Errorf("invalid test type: %v", testType)
}

// Overrides takes a calculator (union of testType and testDate) and calculates
// the transmission risk override vector.
func (c *Calculator) Overrides() v1alpha1.TransmissionRiskVector {
	or := make([]v1alpha1.TransmissionRiskOverride, 0)
	// TODO(mikehelmick) - implement this based on the test type.
	return or
}
