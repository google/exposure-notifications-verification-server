package verify

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/mikehelmick/tek-verification-server/pkg/api"
	"github.com/mikehelmick/tek-verification-server/pkg/controller"
	"github.com/mikehelmick/tek-verification-server/pkg/database"
	"github.com/mikehelmick/tek-verification-server/pkg/signer"
)

type VerifyAPI struct {
	database database.Database
	signer   signer.KeyManager
	key      string
}

func New(db database.Database, km signer.KeyManager, key string) controller.Controller {
	return &VerifyAPI{db, km, key}
}

func (vapi *VerifyAPI) Execute(c *gin.Context) {
	ctx := c.Request.Context()
	var request api.VerifyPINRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := api.VerifyPINResponse{}

	issuedPIN, err := vapi.database.RetrievePIN(request.PIN)
	if err != nil {
		response.Error = err.Error()
		c.JSON(http.StatusBadRequest, response)
		return
	}

	if issuedPIN.IsClaimed() {
		response.Error = "that pin code has been used already"
		c.JSON(http.StatusBadRequest, response)
		return
	}

	if issuedPIN.EndTime().Before(time.Now().UTC()) {
		response.Error = "that pin code has expired"
		c.JSON(http.StatusBadRequest, response)
		return
	}

	log.Printf("Ussing ISSUED PIN: %+v", issuedPIN)

	// Build the JWT
	claims := api.NewVerificationClaims()
	// Add the transmission risks associated w/ this pin.
	claims.TransmissionRisks = make([]database.TransmissionRisk, len(issuedPIN.TransmissionRisks()))
	copy(claims.TransmissionRisks, issuedPIN.TransmissionRisks())
	// Add extended PHA claims
	for k, v := range issuedPIN.Claims() {
		claims.PHAClaims[k] = v
	}
	claims.SignedMAC = request.ExposureKeyMAC
	claims.StandardClaims.Audience = "gov.exposure-notifications-server"
	claims.StandardClaims.Issuer = "Public Health Gov"
	claims.StandardClaims.ExpiresAt = time.Now().UTC().Add(-5 * time.Minute).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)

	signer, err := vapi.signer.NewSigner(ctx, "projects/tek-demo-server/locations/us/keyRings/signing/cryptoKeys/teksigning/cryptoKeyVersions/1")
	if err != nil {
		response.Error = fmt.Sprintf("unable to connect to key management service: %v", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	signingString, err := token.SigningString()
	if err != nil {
		response.Error = fmt.Sprintf("unable to get signing string: %v", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	digest := sha256.Sum256([]byte(signingString))
	sig, err := signer.Sign(rand.Reader, digest[:], nil)
	if err != nil {
		response.Error = fmt.Sprintf("error signing JWT: %v", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// unpack asn1 signature. This is specific to GCP KMS - needs interface.
	var parsedSig struct{ R, S *big.Int }
	_, err = asn1.Unmarshal(sig, &parsedSig)
	if err != nil {
		response.Error = fmt.Sprintf("unable to parse JWT signature: %v", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	keyBytes := 256 / 8
	if 256%8 > 0 {
		keyBytes++
	}

	rBytes := parsedSig.R.Bytes()
	rBytesPadded := make([]byte, keyBytes)
	copy(rBytesPadded[keyBytes-len(rBytes):], rBytes)

	sBytes := parsedSig.S.Bytes()
	sBytesPadded := make([]byte, keyBytes)
	copy(sBytesPadded[keyBytes-len(sBytes):], sBytes)

	sig = append(rBytesPadded, sBytesPadded...)

	vapi.database.MarkPINClaimed(request.PIN)

	response.Verification = strings.Join([]string{signingString, jwt.EncodeSegment(sig)}, ".")
	c.JSON(http.StatusOK, response)
}
