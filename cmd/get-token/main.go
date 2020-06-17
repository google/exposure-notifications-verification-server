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

// Exchanges a verification code for a verification token.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/jsonclient"
)

func main() {
	codeFlag := flag.String("code", "", "verification code to exchange")
	apikeyFlag := flag.String("apikey", "", "API Key to use")
	flag.Parse()

	// Make the request.
	url := "http://localhost:8080/api/verify"
	request := api.VerifyCodeRequest{
		APIKey:           *apikeyFlag,
		VerificationCode: *codeFlag,
	}
	log.Printf("Sending: %+v", request)

	var response api.VerifyCodeResponse
	client := &http.Client{}

	if err := jsonclient.MakeRequest(client, url, request, &response); err != nil {
		log.Fatalf("error making API call: %v", err)
	}
	log.Printf("Result: \n%+v", response)
}
