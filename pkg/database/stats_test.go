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

func TestStatAccumulation(t *testing.T) {
	now := time.Now()
	type dateAndCount struct {
		Stat
		date time.Time
	}
	tests := []struct {
		date     time.Time
		stats    []dateAndCount
		expected Summary
	}{
		{now, []dateAndCount{}, Summary{}},
		{now, []dateAndCount{{Stat{1}, now}}, Summary{1, 1, 1}},
		{now, []dateAndCount{
			{Stat{1}, now},
			{Stat{2}, now},
			{Stat{3}, now},
		}, Summary{6, 6, 6}},
		{now, []dateAndCount{{Stat{2}, now.AddDate(0, 0, -3)}}, Summary{0, 2, 2}},
		{now, []dateAndCount{{Stat{3}, now.AddDate(0, 0, -8)}}, Summary{0, 0, 3}},
		{now, []dateAndCount{
			{Stat{1}, now.AddDate(0, 0, -1)},
			{Stat{2}, now.AddDate(0, 0, -2)},
			{Stat{4}, now},
		}, Summary{4, 7, 7}},
	}
	for i, test := range tests {
		s := Summary{}
		for _, stat := range test.stats {
			s.AccumulateDated(stat.Stat, stat.date, test.date)
		}
		if s != test.expected {
			t.Fatalf("[%d] %+v != %+v\n", i, s, test.expected)
		}
	}
}
