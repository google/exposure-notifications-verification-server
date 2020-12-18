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

package issuelogic

import (
	"strings"
)

var (
	// scrubbers is a list of known Twilio error messages that contain the send to phone number.
	scrubbers = []struct {
		prefix string
		suffix string
	}{
		{
			prefix: "phone number: ",
			suffix: ", ",
		},
		{
			prefix: "'To' number ",
			suffix: " is not",
		},
	}
)

// scrubPhoneNumbers checks for phone numbers in known Twilio error strings that contains
// user phone numbers.
func scrubPhoneNumbers(s string) string {
	noScrubs := s
	for _, scrub := range scrubbers {
		pi := strings.Index(noScrubs, scrub.prefix)
		si := strings.Index(noScrubs, scrub.suffix)

		// if prefix is in the string and suffix is in the sting after the prefix
		if pi >= 0 && si > pi+len(scrub.prefix) {
			noScrubs = strings.Join([]string{
				noScrubs[0 : pi+len(scrub.prefix)],
				noScrubs[si:],
			}, "REDACTED")
		}
	}
	return noScrubs
}
