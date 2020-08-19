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

import "time"

type Stat struct {
	CodesIssued uint64
}

type Summary struct {
	CodesIssued1d  uint64
	CodesIssued7d  uint64
	CodesIssued30d uint64
}

// AccumulateDated conditionally accumulates a stat based on collection date.
func (s *Summary) AccumulateDated(stat Stat, statDate, collectionDate time.Time) {
	// All entires are 30d
	s.CodesIssued30d += stat.CodesIssued

	// Only one entry is 1d
	if statDate == collectionDate {
		s.CodesIssued1d += stat.CodesIssued
	}

	// Find 7d entries
	if statDate.After(collectionDate.AddDate(0, 0, -7)) {
		s.CodesIssued7d += stat.CodesIssued
	}
}
