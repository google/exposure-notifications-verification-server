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

// Package cover implements the cover traffic API.
package cover

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"math/big"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"go.uber.org/zap"
)

const (
	minDelayMS  = 250
	varWindowMS = 500

	minRandomBytes = 400
	varBytesWindow = 200
)

// CoverAPI is an http.Handler for servering CoverRequest API calls.
type CoverAPI struct {
	logger *zap.SugaredLogger
}

// New creates a new CoverAPI.
func New(ctx context.Context) http.Handler {
	return &CoverAPI{logging.FromContext(ctx)}
}

func (c *CoverAPI) randomDelay() {
	delay, err := rand.Int(rand.Reader, big.NewInt(int64(varWindowMS)))
	if err != nil {
		c.logger.Errorf("error getting random int, using min sleep: %v", err)
		delay = big.NewInt(0)
	}

	length := time.Duration(minDelayMS+delay.Int64()) * time.Millisecond
	time.Sleep(length)
}

func (c *CoverAPI) randomData() (string, error) {
	varLength, err := rand.Int(rand.Reader, big.NewInt(varBytesWindow))
	if err != nil {
		return "", err
	}

	padding := make([]byte, minRandomBytes+varLength.Int64())
	if _, err := rand.Read(padding); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(padding), nil
}

func (c *CoverAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request api.CoverRequest
	if err := controller.BindJSON(w, r, &request); err != nil {
		c.logger.Errorf("failed to bind request: %v", err)
		controller.WriteJSON(w, http.StatusBadRequest, api.Error("invalid request: %v", err))
		return
	}

	// Don't immediately return.
	// TODO(mikehelmick) - this should be heursitically valid based on traffic.
	c.randomDelay()

	data, err := c.randomData()
	if err != nil {
		c.logger.Errorf("failed to generate random data: %v", err)
		controller.WriteJSON(w, http.StatusInternalServerError, api.Error("internal error"))
	}

	controller.WriteJSON(w, http.StatusOK, api.CoverResponse{Data: data})
}
