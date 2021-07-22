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

package issueapi

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
)

type dateParseSettings struct {
	Name          string
	ParseError    string
	ValidateError string
}

var (
	// Cache the UTC time.Location, to speed runtime.
	utc *time.Location

	onsetSettings = dateParseSettings{
		Name:          "symptom onset",
		ParseError:    "FAILED_TO_PROCESS_SYMPTOM_ONSET_DATE",
		ValidateError: "SYMPTOM_ONSET_DATE_NOT_IN_VALID_RANGE",
	}
	testSettings = dateParseSettings{
		Name:          "test",
		ParseError:    "FAILED_TO_PROCESS_TEST_DATE",
		ValidateError: "TEST_DATE_NOT_IN_VALID_RANGE",
	}
)

func init() {
	var err error
	utc, err = time.LoadLocation("UTC")
	if err != nil {
		panic("should have found UTC")
	}
}

func (c *Controller) parseDate(d string, tzOffset int, parseSettings *dateParseSettings) (*time.Time, *IssueResult) {
	if d == "" {
		return nil, nil
	}

	parsed, err := time.Parse(project.RFC3339Date, d)
	if err != nil {
		return nil, &IssueResult{
			obsResult:   enobs.ResultError(parseSettings.ParseError),
			HTTPCode:    http.StatusBadRequest,
			ErrorReturn: api.Errorf("failed to process %s date: %v", parseSettings.Name, err).WithCode(api.ErrUnparsableRequest),
		}
	}
	// Max date is today (UTC time) and min date is AllowedTestAge ago, truncated.
	maxDate := timeutils.UTCMidnight(time.Now())
	minDate := timeutils.UTCMidnight(maxDate.Add(-1 * c.config.IssueConfig().AllowedSymptomAge))

	validatedDate, err := ValidateDate(parsed, minDate, maxDate, tzOffset)
	if err != nil {
		err := fmt.Errorf("%s date must be on/after %v and on/before %v. Received %v",
			parseSettings.Name,
			minDate.Format(project.RFC3339Date),
			maxDate.Format(project.RFC3339Date),
			parsed.Format(project.RFC3339Date),
		)
		return nil, &IssueResult{
			obsResult:   enobs.ResultError(parseSettings.ValidateError),
			HTTPCode:    http.StatusBadRequest,
			ErrorReturn: api.Error(err).WithCode(api.ErrInvalidDate),
		}
	}

	return validatedDate, nil
}

// ValidateDate validates the date given -- returning the time or an error.
func ValidateDate(date, minDate, maxDate time.Time, tzOffset int) (*time.Time, error) {
	// Check that all our dates are utc.
	if date.Location() != utc || minDate.Location() != utc || maxDate.Location() != utc {
		return nil, errors.New("dates weren't in UTC")
	}

	// If we're dealing with a timezone where the offset is earlier than this one,
	// we loosen up the lower bound. We might have the following circumstance:
	//
	//    Server time: UTC Aug 1, 12:01 AM
	//    Client time: UTC July 30, 11:01 PM (ie, tzOffset = -30)
	//
	// In this circumstance, we'll have the following:
	//
	//    minTime: UTC July 31, maxTime: Aug 1, clientTime: July 30.
	//
	// which would be an error. Loosening up the lower bound, by a day, keeps us
	// all ok.
	if tzOffset < 0 {
		if m := minDate.Add(-24 * time.Hour); m.After(date) {
			return nil, fmt.Errorf("date %v before min %v", date, m)
		}
	} else if minDate.After(date) {
		return nil, fmt.Errorf("date %v before min %v", date, minDate)
	}
	if date.After(maxDate) {
		return nil, fmt.Errorf("date %v after max %v", date, maxDate)
	}
	return &date, nil
}
