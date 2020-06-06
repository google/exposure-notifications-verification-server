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

package datastore

import (
	"context"
	"fmt"
	"time"

	gcpdatastore "cloud.google.com/go/datastore"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func init() {
	database.RegisterDatabase("datastore", New)
}

type Datastore struct {
	client *gcpdatastore.Client
}

func New(ctx context.Context) (database.Database, error) {
	client, err := gcpdatastore.NewClient(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error connecting to datastore: %w", err)
	}
	return &Datastore{client}, nil
}

func (d *Datastore) InsertPIN(pin string, risks []database.TransmissionRisk, addClaims map[string]string, duration time.Duration) (database.IssuedPIN, error) {
	return nil, nil
}

func (d *Datastore) RetrievePIN(pin string) (database.IssuedPIN, error) {
	return nil, nil
}

func (d *Datastore) MarkPINClaimed(pin string) error {
	return nil
}
