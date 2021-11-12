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

package database

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/errcmp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func generateNonce(t testing.TB) []byte {
	t.Helper()
	buf := make([]byte, NonceLength)
	n, err := rand.Read(buf)
	if err != nil {
		t.Fatalf("unable to generate nonce")
	}
	if n != NonceLength {
		t.Fatalf("wrong number of bytes read: want: %v got: %v", NonceLength, n)
	}
	return buf
}

func TestFindUserReport(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		phoneNumber := "+12065551234"
		userReport, err := db.NewUserReport(phoneNumber, generateNonce(t), true)
		if err != nil {
			t.Fatalf("error creating user report: %v", err)
		}
		if err := db.db.Save(userReport).Error; err != nil {
			t.Fatalf("error writing user report: %v", err)
		}

		got, err := db.FindUserReport(phoneNumber)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(userReport, got, ApproxTime, cmpopts.IgnoreUnexported(Errorable{})); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
	})

	cases := []struct {
		name        string
		phone       string
		nonce       string
		want        string
		fieldErrors []string
	}{
		{
			name:  "poorly_encoded_nonce",
			phone: "+12065551235",
			nonce: ".foo",
			want:  "validation failed",
			fieldErrors: []string{
				"nonce is not using a valid base64 encoding",
			},
		},
		{
			name:  "poorly_encoded_nonce",
			phone: "+12065551235",
			nonce: base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3}),
			want:  "validation failed",
			fieldErrors: []string{
				"nonce is not the correct length, want: 256 got: 3",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			userReport, err := db.NewUserReport(tc.phone, []byte{0}, true)
			if err != nil {
				t.Fatalf("error creating user report: %v", err)
			}

			userReport.Nonce = tc.nonce // override the encoding from NewUserReport for testing errors.
			err = db.db.Save(userReport).Error
			errcmp.MustMatch(t, err, tc.want)
			if diff := cmp.Diff(tc.fieldErrors, userReport.ErrorMessages()); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestDeleteUserReport(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	realm.AddUserReportToAllowedTestTypes()
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	phoneNumber := "+8128675309"
	verCode := &VerificationCode{
		RealmID:       realm.ID,
		Code:          "12345678",
		LongCode:      "12345678",
		TestType:      "user-report",
		PhoneNumber:   phoneNumber,
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(time.Hour),
	}

	if err := realm.SaveVerificationCode(db, verCode); err != nil {
		t.Fatalf("unable to save code and user report: %v", err)
	}

	got, err := db.FindUserReport(phoneNumber)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("didn't find expected user_report record")
	}

	if err := db.DeleteUserReport(phoneNumber, SystemTest); err != nil {
		t.Fatal(err)
	}

	if _, err = db.FindUserReport(phoneNumber); !IsNotFound(err) {
		t.Errorf("expected not found, got %#v", err)
	}
}

func TestPurgeUserReports(t *testing.T) {
	t.Parallel()
	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	cases := []struct {
		name     string
		claimed  bool
		method   func() (int64, error)
		expected int
	}{
		{
			name:    "unclaimed",
			claimed: false,
			method: func() (int64, error) {
				return db.PurgeUnclaimedUserReports(500 * time.Millisecond)
			},
			expected: 1,
		},
		{
			name:    "claimed",
			claimed: true,
			method: func() (int64, error) {
				return db.PurgeClaimedUserReports(500 * time.Millisecond)
			},
			expected: 1,
		},
		{
			name:    "unclaimed_not_ready",
			claimed: false,
			method: func() (int64, error) {
				return db.PurgeUnclaimedUserReports(700 * time.Hour)
			},
			expected: 0,
		},
		{
			name:    "claimed_not_ready",
			claimed: true,
			method: func() (int64, error) {
				return db.PurgeClaimedUserReports(700 * time.Hour)
			},
			expected: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// tests not parallel because one purge test could interfer with the other.
			phoneNumber := tc.name
			userReport, err := db.NewUserReport(phoneNumber, generateNonce(t), true)
			if err != nil {
				t.Fatalf("error creating user report: %v", err)
			}
			userReport.CodeClaimed = tc.claimed
			if err := db.db.Save(userReport).Error; err != nil {
				t.Fatalf("error writing user report: %v", err)
			}

			got, err := db.FindUserReport(phoneNumber)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(userReport, got, ApproxTime, cmpopts.IgnoreUnexported(Errorable{})); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}

			time.Sleep(1 * time.Second)

			n, err := tc.method()
			if err != nil {
				t.Fatalf("error purging user reports: %v", err)
			}
			if n != int64(tc.expected) {
				t.Fatalf("expected %d record to be removed, got: %v", tc.expected, n)
			}
		})
	}
}
