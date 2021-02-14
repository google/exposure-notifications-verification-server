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

package middleware_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"
)

func TestRequireMFA(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	authProvider, err := auth.NewLocal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireMFA := middleware.RequireMFA(authProvider, h)

	cases := []struct {
		name        string
		mfaVerified bool
		prompted    bool
		membership  *database.Membership
		code        int
	}{
		{
			name:       "missing_membership",
			membership: nil,
			code:       400,
		},
		{
			name:        "optional_verified",
			mfaVerified: true,
			membership: &database.Membership{
				Realm: &database.Realm{
					MFAMode: database.MFAOptional,
				},
			},
			code: 200,
		},
		{
			name:        "optional_not_verified",
			mfaVerified: false,
			membership: &database.Membership{
				Realm: &database.Realm{
					MFAMode: database.MFAOptional,
				},
			},
			code: 200,
		},
		{
			name:        "optional_prompt_verified",
			mfaVerified: true,
			membership: &database.Membership{
				Realm: &database.Realm{
					MFAMode: database.MFAOptionalPrompt,
				},
			},
			code: 200,
		},
		{
			name:        "optional_prompt_not_verified",
			mfaVerified: false,
			membership: &database.Membership{
				Realm: &database.Realm{
					MFAMode: database.MFAOptionalPrompt,
				},
			},
			code: 303,
		},
		{
			name:        "optional_prompt_not_verified_prompted",
			mfaVerified: false,
			prompted:    true,
			membership: &database.Membership{
				Realm: &database.Realm{
					MFAMode: database.MFAOptionalPrompt,
				},
			},
			code: 200,
		},
		{
			name:        "required_verified",
			mfaVerified: true,
			membership: &database.Membership{
				Realm: &database.Realm{
					MFAMode: database.MFAOptionalPrompt,
				},
			},
			code: 200,
		},
		{
			name:        "required_not_verified",
			mfaVerified: false,
			membership: &database.Membership{
				Realm: &database.Realm{
					MFAMode: database.MFAOptionalPrompt,
				},
			},
			code: 303,
		},
		{
			name:        "required_not_grace_allowed",
			mfaVerified: false,
			prompted:    true,
			membership: &database.Membership{
				Realm: &database.Realm{
					MFAMode:                database.MFARequired,
					MFARequiredGracePeriod: database.FromDuration(7 * 24 * time.Hour),
				},
				CreatedAt: time.Now().UTC(),
			},
			code: 200,
		},
		{
			name:        "required_not_grace_expired",
			mfaVerified: false,
			prompted:    true,
			membership: &database.Membership{
				Realm: &database.Realm{
					MFAMode:                database.MFARequired,
					MFARequiredGracePeriod: database.FromDuration(7 * 24 * time.Hour),
				},
				CreatedAt: time.Now().UTC().Add(-30 * 24 * time.Hour),
			},
			code: 303,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			session := &sessions.Session{}
			if err := authProvider.StoreSession(ctx, session, &auth.SessionInfo{
				Data: map[string]interface{}{
					"email":          "you@example.com",
					"email_verified": true,
					"mfa_enabled":    tc.mfaVerified,
					"revoked":        false,
				},
			}); err != nil {
				t.Fatal(err)
			}

			ctx := ctx
			ctx = controller.WithSession(ctx, session)
			if tc.membership != nil {
				ctx = controller.WithMembership(ctx, tc.membership)
			}
			if tc.prompted {
				controller.StoreSessionMFAPrompted(session, true)
			}

			r := httptest.NewRequest("GET", "/", nil)
			r = r.Clone(ctx)
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			requireMFA(emptyHandler()).ServeHTTP(w, r)
			w.Flush()

			if got, want := w.Code, tc.code; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
			if tc.code == 200 {
				stored := controller.MFAPromptedFromSession(session)
				if !stored {
					t.Errorf("expected mfa prompted to have been stored")
				}
			}
		})
	}
}
