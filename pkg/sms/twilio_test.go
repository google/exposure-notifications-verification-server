package sms

import (
	"context"
	"os"
	"strconv"
	"testing"
)

func TestTwilio_SendSMS(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skipf("🚧 Skipping twilio tests (short)!")
	}

	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_TWILIO_TESTS")); skip {
		t.Skipf("🚧 Skipping twilio tests (SKIP_TWILIO_TESTS is set)!")
	}

	accountSid := os.Getenv("TWILIO_ACCOUNT_SID")
	if accountSid == "" {
		t.Fatalf("missing TWILIO_ACCOUNT_SID")
	}

	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	if authToken == "" {
		t.Fatalf("missing TWILIO_AUTH_TOKEN")
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
			to:   "+15005550006",
			err:  false,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			twilio, err := NewTwilio(ctx, accountSid, authToken)
			if err != nil {
				t.Fatal(err)
			}

			err = twilio.SendSMS(ctx, tc.from, tc.to, "testing 123")
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
		})
	}
}
