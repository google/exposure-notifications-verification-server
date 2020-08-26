package sms

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
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
			twilio, err := NewTwilio(ctx, accountSid, authToken, tc.from)
			if err != nil {
				t.Fatal(err)
			}

			// Twilio is pretty flaky, retry if it failed unexpectedly
			b, err := retry.NewConstant(100 * time.Millisecond)
			if err != nil {
				t.Fatalf("failed to configure backoff: %v", err)
			}
			b = retry.WithMaxRetries(3, b)

			if err := retry.Do(ctx, b, func(_ context.Context) error {
				err = twilio.SendSMS(ctx, tc.to, "testing 123")
				if err != nil && err.Error() == "The 'To' number +15005550006 is not a valid phone number" {
					return retry.RetryableError(err)
				}
				if (err != nil) != tc.err {
					return err
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
