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

// Package clients provides functions for invoking the APIs of the verification server
package clients

import (
	"context"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/jsonclient"
)

// IssueCode uses the ADMIN API to issue a verification code.
// Currently does not accept the SMS param.
func IssueCode(ctx context.Context, hostname string, apiKey, testType, symptomDate string, timeout time.Duration) (*api.IssueCodeRequest, *api.IssueCodeResponse, error) {
	url := hostname + "/api/issue"
	request := api.IssueCodeRequest{
		TestType:    testType,
		SymptomDate: symptomDate,
	}
	client := &http.Client{
		Timeout: timeout,
	}

	var response api.IssueCodeResponse

	headers := http.Header{}
	headers.Add("X-API-Key", apiKey)

	if err := jsonclient.MakeRequest(ctx, client, url, headers, request, &response); err != nil {
		return &request, nil, err
	}
	return &request, &response, nil
}

// CheckCodeStatus uses the ADMIN API to retrieve the status of an OTP code.
func CheckCodeStatus(ctx context.Context, hostname string, apiKey, uuid string, timeout time.Duration) (*api.CheckCodeStatusRequest, *api.CheckCodeStatusResponse, error) {
	url := hostname + "/api/checkcodestatus"
	request := api.CheckCodeStatusRequest{
		UUID: uuid,
	}
	client := &http.Client{
		Timeout: timeout,
	}

	var response api.CheckCodeStatusResponse

	headers := http.Header{}
	headers.Add("X-API-Key", apiKey)

	if err := jsonclient.MakeRequest(ctx, client, url, headers, request, &response); err != nil {
		return &request, nil, err
	}
	return &request, &response, nil
}

// GetToken makes the API call to exchange a code for a token.
func GetToken(ctx context.Context, hostname, apikey, code string, timeout time.Duration) (*api.VerifyCodeRequest, *api.VerifyCodeResponse, error) {
	url := hostname + "/api/verify"
	request := api.VerifyCodeRequest{
		VerificationCode: code,
	}
	client := &http.Client{
		Timeout: timeout,
	}

	var response api.VerifyCodeResponse

	headers := http.Header{}
	headers.Add("X-API-Key", apikey)

	if err := jsonclient.MakeRequest(ctx, client, url, headers, request, &response); err != nil {
		return &request, nil, err
	}
	return &request, &response, nil
}

// GetCertificate exchanges a verification token + HMAC for a verification certificate.
func GetCertificate(ctx context.Context, hostname, apikey, token, hmac string, timeout time.Duration) (*api.VerificationCertificateRequest, *api.VerificationCertificateResponse, error) {
	url := hostname + "/api/certificate"
	request := api.VerificationCertificateRequest{
		VerificationToken: token,
		ExposureKeyHMAC:   hmac,
	}
	client := &http.Client{
		Timeout: timeout,
	}

	var response api.VerificationCertificateResponse

	headers := http.Header{}
	headers.Add("X-API-Key", apikey)

	if err := jsonclient.MakeRequest(ctx, client, url, headers, request, &response); err != nil {
		return &request, nil, err
	}
	return &request, &response, nil
}
