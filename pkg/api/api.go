package api

import (
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/mikehelmick/tek-verification-server/pkg/database"
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
