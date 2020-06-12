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

const (
	userKind = "user"
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

func (d *Datastore) InsertPIN(ctx context.Context, pin string, risks []database.TransmissionRisk, addClaims map[string]string, duration time.Duration) (database.IssuedPIN, error) {
	return nil, nil
}

func (d *Datastore) RetrievePIN(ctx context.Context, pin string) (database.IssuedPIN, error) {
	return nil, nil
}

func (d *Datastore) MarkPINClaimed(ctx context.Context, pin string) error {
	return nil
}

func (d *Datastore) LookupUser(ctx context.Context, email string) (database.User, error) {
	// TODO(mikehelmick) - user lookup should be put through the write thru cache.
	k := gcpdatastore.NameKey(userKind, email, nil)
	var entity internalUser
	if err := d.client.Get(ctx, k, &entity); err != nil {
		return nil, fmt.Errorf("lookup user datastore.get: %w", err)
	}

	return entity.toUser(), nil
}

func (d *Datastore) UpdateRevokeCheck(ctx context.Context, u database.User) error {
	return nil
}
