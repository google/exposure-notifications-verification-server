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

package issueapi

import (
	"context"
	"sync"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"go.opencensus.io/tag"
)

// issueResult is the response returned from IssueLogic.IssueOne or IssueMany.
type issueResult struct {
	verCode     *database.VerificationCode
	errorReturn *api.ErrorReturn
	httpCode    int
	obsResult   tag.Mutator
}

func (result *issueResult) issueCodeResponse() *api.IssueCodeResponse {
	if result.errorReturn != nil {
		return &api.IssueCodeResponse{
			ErrorCode: result.errorReturn.ErrorCode,
			Error:     result.errorReturn.Error,
		}
	}

	v := result.verCode
	return &api.IssueCodeResponse{
		UUID:                   v.UUID,
		VerificationCode:       v.Code,
		ExpiresAt:              v.ExpiresAt.Format(time.RFC1123),
		ExpiresAtTimestamp:     v.ExpiresAt.UTC().Unix(),
		LongExpiresAt:          v.LongExpiresAt.Format(time.RFC1123),
		LongExpiresAtTimestamp: v.LongExpiresAt.UTC().Unix(),
	}
}

func (c *Controller) issueOne(ctx context.Context, request *api.IssueCodeRequest,
	authApp *database.AuthorizedApp, membership *database.Membership, realm *database.Realm) *issueResult {
	results := c.issueMany(ctx, []*api.IssueCodeRequest{request}, authApp, membership, realm)
	return results[0]
}

func (c *Controller) issueMany(ctx context.Context, requests []*api.IssueCodeRequest,
	authApp *database.AuthorizedApp, membership *database.Membership, realm *database.Realm) []*issueResult {
	// Generate codes
	results := make([]*issueResult, len(requests))
	for i, req := range requests {
		vCode, result := c.buildVerificationCode(ctx, req, authApp, membership, realm)
		if result != nil {
			results[i] = result
			continue
		}
		results[i] = c.issueCode(ctx, vCode, realm)
	}

	// Send SMS messages
	var wg sync.WaitGroup
	for i, result := range results {
		if result.errorReturn != nil {
			continue
		}

		wg.Add(1)
		go func(request *api.IssueCodeRequest, r *issueResult) {
			defer wg.Done()
			c.sendSMS(ctx, request, r, realm)
		}(requests[i], result)
	}

	wg.Wait() // wait the SMS work group to finish

	return results
}
