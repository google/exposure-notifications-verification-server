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

package sms

import (
	"context"
	"os"
	"testing"
)

func TestTwilio_SendSMS(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skipf("🚧 Skipping twilio tests (short)!")
	}

	accountSid := os.Getenv("TWILIO_ACCOUNT_SID")
	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	if accountSid == "" || authToken == "" {
		t.Skipf("🚧 🚧 Skipping twilio tests (missing TWILIO_ACCOUNT_SID/TWILIO_AUTH_TOKEN)")
	}

	cases := []struct {
		name string
		from string
		to   string
		err  bool
	}{
		// The following numbers are "magic numbers" from Twilio to force their API
		// to return errors for testing:
		//
		// https://www.twilio.com/docs/iam/test-credentials#test-sms-messages-parameters-From
		{
			name: "invalid",
			from: "+15005550006",
			to:   "+15005550001",
			err:  true,
		},
		{
			name: "unroutable",
			from: "+15005550006",
			to:   "+15005550002",
			err:  true,
		},
		{
			name: "international",
			from: "+15005550006",
			to:   "+15005550003",
			err:  true,
		},
		{
			name: "blocked",
			from: "+15005550006",
			to:   "+15005550004",
			err:  true,
		},
		{
			name: "incapable",
			from: "+15005550006",
			to:   "+15005550009",
			err:  true,
		},
		{
			name: "sends",
			from: "+15005550006",
			to:   "+18144211811", // A real phone number
			err:  false,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			twilio, err := NewTwilio(ctx, accountSid, authToken, tc.from)
			if err != nil {
				t.Fatal(err)
			}

			err = twilio.SendSMS(ctx, tc.to, "testing 123")
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
		})
	}
}
