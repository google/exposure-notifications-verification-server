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

package database

import (
	"testing"
	"time"
)

func TestDatabase_ClaimModelerStatus(t *testing.T) {
	t.Parallel()

	// Create this now so we don't get clock skew
	later := time.Now().UTC().Add(modelerLockTime)

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	if err := db.ClaimModelerStatus(); err != nil {
		t.Fatal(err)
	}

	var status ModelerStatus
	if err := db.db.Model(&ModelerStatus{}).First(&status).Error; err != nil {
		t.Fatal(err)
	}

	if got, now := status.NotBefore, later; !got.After(now) {
		t.Errorf("expected %q to be after %q", got, now)
	}
}
