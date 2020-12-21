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
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"go.opencensus.io/tag"
)

// IssueResult is the response returned from IssueLogic.IssueOne or IssueMany.
type IssueResult struct {
	VerCode     *database.VerificationCode
	ErrorReturn *api.ErrorReturn
	HTTPCode    int
	obsResult   tag.Mutator
}

func (result *IssueResult) IssueCodeResponse() *api.IssueCodeResponse {
	if result.ErrorReturn != nil {
		return &api.IssueCodeResponse{
			ErrorCode: result.ErrorReturn.ErrorCode,
			Error:     result.ErrorReturn.Error,
		}
	}

	v := result.VerCode
	return &api.IssueCodeResponse{
		UUID:                   v.UUID,
		VerificationCode:       v.Code,
		ExpiresAt:              v.ExpiresAt.Format(time.RFC1123),
		ExpiresAtTimestamp:     v.ExpiresAt.UTC().Unix(),
		LongExpiresAt:          v.LongExpiresAt.Format(time.RFC1123),
		LongExpiresAtTimestamp: v.LongExpiresAt.UTC().Unix(),
	}
}

func (c *Controller) IssueOne(ctx context.Context, request *api.IssueCodeRequest) *IssueResult {
	results := c.IssueMany(ctx, []*api.IssueCodeRequest{request})
	return results[0]
}

func (c *Controller) IssueMany(ctx context.Context, requests []*api.IssueCodeRequest) []*IssueResult {
	realm := controller.RealmFromContext(ctx)
	// Generate codes
	results := make([]*IssueResult, len(requests))
	for i, req := range requests {
		vCode, result := c.BuildVerificationCode(ctx, req)
		if result != nil {
			results[i] = result
			continue
		}
		results[i] = c.IssueCode(ctx, vCode, realm)
	}

	// Send SMS messages
	var wg sync.WaitGroup
	for i, result := range results {
		if result.ErrorReturn != nil {
			continue
		}

		wg.Add(1)
		go func(request *api.IssueCodeRequest, r *IssueResult) {
			defer wg.Done()
			c.SendSMS(ctx, request, r, realm)
		}(requests[i], result)
	}

	wg.Wait() // wait the SMS work group to finish

	return results
}
