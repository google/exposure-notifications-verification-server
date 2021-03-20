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
	"crypto"
	"fmt"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"go.opencensus.io/tag"
)

// IssueRequestInternal is used to join the base issue request with the
// optional nonce. If the nonce is provided and the the report type
// is self_report (user initiated code), then extra safeguards are applied.
type IssueRequestInternal struct {
	IssueRequest *api.IssueCodeRequest
	// These files are for user initiated report
	Nonce []byte
}

// IssueResult is the response returned from IssueLogic.IssueOne or IssueMany.
type IssueResult struct {
	VerCode     *database.VerificationCode
	ErrorReturn *api.ErrorReturn
	HTTPCode    int
	obsResult   tag.Mutator
}

// IssueCodeResponse converts an IssueResult to the external api response.
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

// IssueOne handles validating a single IssueCodeRequest, issuing a new code, and sending an SMS message.
func (c *Controller) IssueOne(ctx context.Context, request *IssueRequestInternal) *IssueResult {
	results := c.IssueMany(ctx, []*IssueRequestInternal{request})
	return results[0]
}

// IssueMany handles validating a list of IssueCodeRequest, issuing new codes, and sending SMS messages.
func (c *Controller) IssueMany(ctx context.Context, requests []*IssueRequestInternal) []*IssueResult {
	realm := controller.RealmFromContext(ctx)

	logger := logging.FromContext(ctx).Named("issueapi.IssueMany").
		With("realm", realm.ID)

	// Generate codes
	results := make([]*IssueResult, len(requests))
	for i, req := range requests {
		vCode, result := c.BuildVerificationCode(ctx, req.IssueRequest, realm)
		if result != nil {
			results[i] = result
			continue
		}
		vCode.Nonce = req.Nonce
		vCode.PhoneNumber = req.IssueRequest.Phone
		results[i] = c.IssueCode(ctx, vCode, realm)
	}

	defer c.recordStats(ctx, results)

	// Send SMS messages if there's an SMS provider.
	smsProvider, err := c.smsProviderFor(ctx, realm)
	if err != nil {
		logger.Errorw("failed to get sms provider", "error", err)
		errorAll(results, api.InternalError())
		return results
	}

	// Sign messages if the realm has it enabled.
	smsSigner, keyID, err := c.smsSignerFor(ctx, realm)
	if err != nil {
		logger.Errorw("failed to get sms signer", "error", err)
		errorAll(results, api.InternalError())
		return results
	}

	if smsProvider != nil {
		var wg sync.WaitGroup
		for i, result := range results {
			if result.ErrorReturn != nil {
				continue
			}

			wg.Add(1)
			go func(request *api.IssueCodeRequest, r *IssueResult) {
				defer wg.Done()
				c.SendSMS(ctx, realm, smsProvider, smsSigner, keyID, request, r)
			}(requests[i].IssueRequest, result)
		}
		wg.Wait()
	}

	return results
}

// recordStats increments stats for successfully issued codes
func (c *Controller) recordStats(ctx context.Context, results []*IssueResult) {
	codes := make([]*database.VerificationCode, 0, len(results))
	for _, result := range results {
		if result.ErrorReturn == nil {
			codes = append(codes, result.VerCode)
		}
	}
	c.db.UpdateStats(ctx, codes...)
}

// smsProviderFor returns the sms provider for the given realm. It pulls the
// value from a local in-memory cache.
func (c *Controller) smsProviderFor(ctx context.Context, realm *database.Realm) (sms.Provider, error) {
	key := fmt.Sprintf("realm:%d:sms_provider", realm.ID)
	result, err := c.localCache.WriteThruLookup(key, func() (interface{}, error) {
		return realm.SMSProvider(c.db)
	})
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}
	typ, ok := result.(sms.Provider)
	if !ok {
		return nil, fmt.Errorf("invalid type %T", result)
	}

	return typ, nil
}

// smsSignerFor returns the sms signer for the given realm. It pulls the value
// from a local in-memory cache.
func (c *Controller) smsSignerFor(ctx context.Context, realm *database.Realm) (crypto.Signer, string, error) {
	// Do not create a signer if the realm does not sign SMS.
	if !c.config.GetFeatureConfig().EnableAuthenticatedSMS || !realm.UseAuthenticatedSMS {
		return nil, "", nil
	}

	type cachedSMSSigner struct {
		signer crypto.Signer
		keyID  string
	}

	key := fmt.Sprintf("realm:%d:sms_signer", realm.ID)
	result, err := c.localCache.WriteThruLookup(key, func() (interface{}, error) {
		signingKey, err := realm.CurrentSMSSigningKey(c.db)
		if err != nil {
			return nil, fmt.Errorf("failed to get current sms signing key: %w", err)
		}

		smsSigner, err := c.smsSigner.NewSigner(ctx, signingKey.KeyID)
		if err != nil {
			return nil, fmt.Errorf("failed to create signer: %w", err)
		}

		return &cachedSMSSigner{
			signer: smsSigner,
			keyID:  signingKey.GetKID(),
		}, nil
	})
	if err != nil {
		return nil, "", err
	}

	if result == nil {
		return nil, "", nil
	}
	typ, ok := result.(*cachedSMSSigner)
	if !ok {
		return nil, "", fmt.Errorf("invalid type %T", result)
	}

	return typ.signer, typ.keyID, nil
}

// errorAll sets the ErrorReturn on all results to the provided value.
func errorAll(results []*IssueResult, err *api.ErrorReturn) {
	for _, result := range results {
		result.ErrorReturn = err
	}
}
