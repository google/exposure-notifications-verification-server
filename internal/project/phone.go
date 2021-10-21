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
	"fmt"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// CanonicalPhoneNumber returns the E.164 formatted phone number.
func CanonicalPhoneNumber(phone string, defaultRegion string) (string, error) {
	cc := strings.ToUpper(defaultRegion)
	pn, err := phonenumbers.Parse(phone, cc)
	if err != nil {
		return "", fmt.Errorf("phonenumbers.Parse: %w", err)
	}
	return phonenumbers.Format(pn, phonenumbers.E164), nil
}
