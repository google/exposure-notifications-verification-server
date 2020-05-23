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
