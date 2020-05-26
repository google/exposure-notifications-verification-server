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

package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"log"
	"net/http"

	"github.com/mikehelmick/tek-verification-server/pkg/api"
	"github.com/mikehelmick/tek-verification-server/pkg/jsonclient"
)

func main() {
	pinFlag := flag.String("pin", "", "pin code to sign claims for")
	flag.Parse()

	randomBytes := make([]byte, 512)
	_, err := rand.Read(randomBytes)
	if err != nil {
		log.Fatalf("unable to get some randomness: %v", err)
	}
	fakeMAC := base64.StdEncoding.EncodeToString(randomBytes)

	// Make the request.
	url := "http://localhost:8080/api/verify"
	request := api.VerifyPINRequest{
		PIN:            *pinFlag,
		ExposureKeyMAC: fakeMAC,
	}
	log.Printf("Sending: %+v", request)

	var response api.VerifyPINResponse
	client := &http.Client{}

	err = jsonclient.MakeRequest(client, url, request, &response)
	if err != nil {
		log.Fatalf("error making API call: %v", err)
	}

	log.Printf("Received payload:\n----------\n%+v\n----------\n", response)
}
