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

// Package email is logic for sending email invitations
package email

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/internal/firebase"
)

var _ Provider = (*firebase.Client)(nil)

// NewFirebase creates a new Smtp email sender with the given auth.
func NewFirebase(ctx context.Context) (Provider, error) {
	firebaseInternal, err := firebase.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to configure internal firebase client: %w", err)
	}
	return firebaseInternal, nil
}
