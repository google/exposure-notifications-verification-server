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
package api

import (
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

type IssuePINRequest struct {
	ValidFor time.Duration               `json:"validFor"`
	Risks    []database.TransmissionRisk `json:"transmissionRisks"`
}

type IssuePINResponse struct {
	PIN   string `json:"pin"`
	Error string `json:"error"`
}

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
