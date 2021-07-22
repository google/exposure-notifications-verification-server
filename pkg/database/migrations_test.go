// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestMigrations_Incrementing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	lastID := int64(-1)
	for _, v := range db.Migrations(ctx) {
		parts := strings.SplitN(v.ID, "-", 2)
		if len(parts) < 2 {
			t.Fatalf("bad id: %v", v.ID)
		}

		idStr := parts[0]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			t.Fatal(err)
		}

		if id <= lastID {
			t.Errorf("%s: %d is not greater than %d", v.ID, id, lastID)
		}
		lastID = id
	}
}
