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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/jsonclient"
)

func main() {
	reportFlag := flag.String("type", "", "diagnosis report type: confirmed, likely, negative")
	onsetFlag := flag.String("onset", "", "Symptom onset date, YYYY-MM-DD format")
	apikeyFlag := flag.String("apikey", "", "API Key to use")
	addrFlag := flag.String("addr", "http://localhost:8080", "protocol, address and port on which to make the API call")
	timeoutFlag := flag.Duration("timeout", 5*time.Second, "request time out duration in the format: 0h0m0s")
	flag.Parse()

	// Make the request.
	url := *addrFlag + "/api/issue"
	request := api.IssueCodeRequest{
		TestType:    *reportFlag,
		SymptomDate: *onsetFlag,
	}
	client := &http.Client{
		Timeout: *timeoutFlag,
	}
	log.Printf("Sending: %+v", request)

	var response api.IssueCodeResponse

	headers := http.Header{}
	headers.Add("X-API-Key", *apikeyFlag)

	if err := jsonclient.MakeRequest(client, url, headers, request, &response); err != nil {
		log.Fatalf("error making API call: %v", err)
	}
	log.Printf("Result: \n%+v", response)
}
