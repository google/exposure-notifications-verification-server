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

// Package issueapi implements the API handler for taking a code request, assigning
// an OTP, saving it to the database and returning the result.
// This is invoked over AJAX from the Web frontend.
package issueapi

import (
	"context"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi/issuelogic"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi/issuemetric"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
)

// getAuthorizationFromContext pulls the authorization from the context. If an
// API key is provided, it's used to lookup the realm. If a membership exists,
// it's used to provide the realm.
func (c *Controller) getAuthorizationFromContext(ctx context.Context) (*database.AuthorizedApp, *database.Membership, *database.Realm, error) {
	authorizedApp := controller.AuthorizedAppFromContext(ctx)
	if authorizedApp != nil {
		realm, err := authorizedApp.Realm(c.db)
		if err != nil {
			return nil, nil, nil, err
		}
		return authorizedApp, nil, realm, nil
	}

	membership := controller.MembershipFromContext(ctx)
	if membership != nil {
		realm := membership.Realm
		return nil, membership, realm, nil
	}

	return nil, nil, nil, fmt.Errorf("unable to identify authorized requestor")
}

func recordObservability(ctx context.Context, result *issuelogic.IssueResult) {
	observability.RecordLatency(ctx, time.Now(), issuemetric.LatencyMs, &result.ObsBlame, &result.ObsResult)
}
