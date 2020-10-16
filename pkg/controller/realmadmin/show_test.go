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

package realmadmin

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func TestFormatStats(t *testing.T) {
	now := timeutils.Midnight(time.Now())
	yesterday := timeutils.Midnight(now.Add(-24 * time.Hour))
	tests := []struct {
		data    []*database.RealmUserStats
		names   []string
		numDays int
	}{
		{[]*database.RealmUserStats{}, []string{}, 0},
		{
			[]*database.RealmUserStats{
				{UserID: 1, Name: "Rocky", CodesIssued: 10, Date: now},
				{UserID: 1, Name: "Bullwinkle", CodesIssued: 1, Date: now},
			},
			[]string{"Rocky", "Bullwinkle"},
			1,
		},
		{
			[]*database.RealmUserStats{
				{UserID: 1, Name: "Rocky", CodesIssued: 10, Date: yesterday},
				{UserID: 1, Name: "Rocky", CodesIssued: 10, Date: now},
			},
			[]string{"Rocky"},
			2,
		},
	}

	for i, test := range tests {
		names, format := formatData(test.data)
		sort.Strings(test.names)
		sort.Strings(names)
		if !reflect.DeepEqual(test.names, names) {
			t.Errorf("[%d] %v != %v", i, names, test.names)
		}
		if len(format) != test.numDays {
			t.Errorf("[%d] len(format) = %d, expected %d", i, len(format), test.numDays)
		}
		for _, f := range format {
			if len(f) != len(test.names)+1 {
				t.Errorf("[%d] len(codesIssued) = %d, expected %d", i, len(f), len(test.names)+1)
			}
		}
	}
}
