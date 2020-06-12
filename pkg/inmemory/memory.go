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

package inmemory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func init() {
	database.RegisterDatabase("inmemory", New)
}

type InMemoryDB struct {
	data map[string]*InMemoryPIN
	mu   sync.Mutex
}

func New(ctx context.Context) (database.Database, error) {
	return &InMemoryDB{
		data: make(map[string]*InMemoryPIN),
	}, nil
}

func (d *InMemoryDB) UpdateRevokeCheck(ctx context.Context, u database.User) error {
	return fmt.Errorf("UpdateRevokeCheck not implemented")
}

func (d *InMemoryDB) LookupUser(ctx context.Context, email string) (database.User, error) {
	return nil, fmt.Errorf("LookupUser not implemented")
}

func (d *InMemoryDB) InsertPIN(ctx context.Context, pin string, risks []database.TransmissionRisk, addClaims map[string]string, duration time.Duration) (database.IssuedPIN, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.data[pin]; ok {
		return nil, fmt.Errorf("pin alreay stored.")
	}

	record := &InMemoryPIN{
		pin:     pin,
		claims:  make(map[string]string, len(addClaims)),
		endTime: time.Now().UTC().Add(duration),
		risks:   make([]database.TransmissionRisk, len(risks)),
	}
	copy(record.risks, risks)
	for k, v := range addClaims {
		record.claims[k] = v
	}
	d.data[pin] = record
	return record, nil
}

func (d *InMemoryDB) RetrievePIN(ctx context.Context, pin string) (database.IssuedPIN, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	record, ok := d.data[pin]
	if !ok {
		return nil, fmt.Errorf("pin not found")
	}
	return record, nil
}

func (d *InMemoryDB) MarkPINClaimed(ctx context.Context, pin string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if record, ok := d.data[pin]; !ok {
		return fmt.Errorf("pin doesn't exist.")
	} else if record.IsClaimed() {
		return fmt.Errorf("pin already claimed")
	}
	d.data[pin].claimed = true
	return nil
}

type InMemoryPIN struct {
	pin     string
	risks   []database.TransmissionRisk
	claims  map[string]string
	endTime time.Time
	claimed bool
}

func (p *InMemoryPIN) Pin() string {
	return p.pin
}

func (p *InMemoryPIN) TransmissionRisks() []database.TransmissionRisk {
	return p.risks
}

func (p *InMemoryPIN) Claims() map[string]string {
	return p.claims
}

func (p *InMemoryPIN) EndTime() time.Time {
	return p.endTime
}

func (p *InMemoryPIN) IsClaimed() bool {
	return p.claimed
}
