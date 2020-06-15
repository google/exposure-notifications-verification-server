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

// Package api defines the JSON-RPC API between the browser and the server as
// well as between mobile devices and the server.
package api

// IssueCodeRequest defines the parameters to request an new OTP (short term)
// code. This is called by the Web frontend.
// API is served at /api/issue
type IssueCodeRequest struct {
	TestType string `json:"testType"`
	TestDate string `json:"testDate"`
}

// IssueCodeResponse defines the response type for IssueCodeRequest.
type IssueCodeResponse struct {
	VerificationCode string `json:"code"`
	ExpiresAt        string `json:"expiresAt"`
	Error            string `json:"error"`
}

/*
type VerifyPINRequest struct {
	PIN            string `json:"pin"`
	ExposureKeyMAC string `json:"ekmac"`
}

type VerifyPINResponse struct {
	Error        string `json:"error"`
	Verification string `json:"verification"`
}

type VerificationClaims struct {
	PHAClaims         map[string]string           `json:"phaclaims"`
	TransmissionRisks []database.TransmissionRisk `json:"transmissionRisks"`
	SignedMAC         string                      `json:"signedmac"`
	jwt.StandardClaims
}

func NewVerificationClaims() *VerificationClaims {
	return &VerificationClaims{PHAClaims: make(map[string]string)}
}
*/
