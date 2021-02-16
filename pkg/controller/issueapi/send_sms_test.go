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

package issueapi_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

type badSigner struct{}

func (b *badSigner) Public() crypto.PublicKey {
	return nil
}

func (b *badSigner) Sign(_ io.Reader, _ []byte, _ crypto.SignerOpts) ([]byte, error) {
	return nil, fmt.Errorf("test says nope")
}

func TestSMS_scrubPhoneNumber(t *testing.T) {
	t.Parallel()

	unreachable := "unreachable"
	invalid := "invalid"

	patterns := map[string]string{
		unreachable: "The 'To' phone number: %s, is not currently reachable using the 'From' phone number: 12345 via SMS.",
		invalid:     "The 'To' number %s is not a valid phone number.",
	}

	cases := []struct {
		input string
	}{
		{input: "+11235550098"},
		{input: "+44 123 555 123"},
		{input: "+12065551234"},
		{input: "whatever"},
	}

	for k, pattern := range patterns {
		for i, tc := range cases {
			k := k
			pattern := pattern
			tc := tc

			t.Run(fmt.Sprintf("case_%s_%2d", k, i), func(t *testing.T) {
				t.Parallel()

				errMsg := fmt.Sprintf(pattern, tc.input)
				got := issueapi.ScrubPhoneNumbers(errMsg)
				if strings.Contains(got, tc.input) {
					t.Errorf("phone number was not scrubbed, %v", got)
				}
			})
		}
	}
}

func TestSMS_sendSMS(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)
	db := harness.Database

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	realm.AllowBulkUpload = true
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatalf("failed to save realm: %v", err)
	}

	smsConfig := &database.SMSConfig{
		RealmID:      realm.ID,
		ProviderType: sms.ProviderTypeNoop,
	}
	if err := db.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	smsProvider, err := realm.SMSProvider(harness.Database)
	if err != nil {
		t.Fatal(err)
	}

	smsSigner, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	smsKeyID := "test"

	membership := &database.Membership{
		RealmID:     realm.ID,
		Realm:       realm,
		Permissions: rbac.CodeBulkIssue,
	}

	ctx = controller.WithMembership(ctx, membership)

	harness.Config.SMSSigning.FailClosed = false
	c := issueapi.New(harness.Config, db, harness.RateLimiter, harness.KeyManager, nil)

	request := &api.IssueCodeRequest{
		TestType:    "confirmed",
		SymptomDate: time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date),
		TZOffset:    0,
		Phone:       "+15005550006",
	}
	result := &issueapi.IssueResult{
		HTTPCode: http.StatusOK,
		VerCode: &database.VerificationCode{
			RealmID:       realm.ID,
			Code:          "00000001",
			LongCode:      "00000001ABC",
			Claimed:       true,
			TestType:      "confirmed",
			ExpiresAt:     time.Now().Add(time.Hour),
			LongExpiresAt: time.Now().Add(time.Hour),
		},
	}
	if err := db.SaveVerificationCode(result.VerCode, realm); err != nil {
		t.Fatal(err)
	}
	// un-hmac the codes so rollback can find them.
	result.VerCode.Code = "00000001"
	result.VerCode.LongCode = "00000001ABC"

	// Successful SMS send
	c.SendSMS(ctx, realm, smsProvider, nil, "", request, result)
	if result.ErrorReturn != nil {
		t.Fatalf("expected successful SMS, got %s", result.ErrorReturn)
	}
	if _, err := realm.FindVerificationCodeByUUID(db, result.VerCode.UUID); err != nil {
		t.Errorf("couldn't find code got %s: %v", result.VerCode.UUID, err)
	}

	// Successful SMS send with signature
	c.SendSMS(ctx, realm, smsProvider, smsSigner, smsKeyID, request, result)
	if result.ErrorReturn != nil {
		t.Fatalf("expected successful SMS, got %s", result.ErrorReturn)
	}
	if _, err := realm.FindVerificationCodeByUUID(db, result.VerCode.UUID); err != nil {
		t.Errorf("couldn't find code got %s: %v", result.VerCode.UUID, err)
	}

	// Failed SMS signature fails open
	{
		harness.Config.SMSSigning.FailClosed = false
		c := issueapi.New(harness.Config, db, harness.RateLimiter, harness.KeyManager, nil)
		c.SendSMS(ctx, realm, smsProvider, &badSigner{}, smsKeyID, request, result)
		if err := result.ErrorReturn; err != nil {
			t.Fatal(err)
		}
	}

	// Failed SMS signature fails closed
	{
		harness.Config.SMSSigning.FailClosed = true
		c := issueapi.New(harness.Config, db, harness.RateLimiter, harness.KeyManager, nil)
		c.SendSMS(ctx, realm, smsProvider, &badSigner{}, smsKeyID, request, result)
		err := result.ErrorReturn
		if err == nil {
			t.Fatal("expected error")
		}
		if got, want := err.Error, "failed to sign sms"; !strings.Contains(got, want) {
			t.Errorf("expected %q to be %q", got, want)
		}
		if got, want := err.ErrorCode, "sms_failure"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
	}

	// Failed SMS send
	failingSMSProvider, err := sms.NewNoopFail(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c.SendSMS(ctx, realm, failingSMSProvider, nil, "", request, result)
	if result.ErrorReturn == nil {
		t.Fatal("expected failed SMS, but got no error response")
	} else if result.ErrorReturn.ErrorCode != api.ErrSMSFailure {
		t.Fatal("expected SMS failure code")
	}
	if _, err := realm.FindVerificationCodeByUUID(db, result.VerCode.UUID); !database.IsNotFound(err) {
		t.Errorf("expected SMS failure to roll-back and delete code. got %v", err)
	}
}
