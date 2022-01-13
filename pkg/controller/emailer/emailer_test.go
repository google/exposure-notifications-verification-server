// Copyright 2022 the Exposure Notifications Verification Server authors
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

package emailer

import (
	"testing"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"go.uber.org/zap/zaptest/observer"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func testExpectLog(tb testing.TB, lo *observer.ObservedLogs, msg string) {
	logs := lo.All()
	msgs := make([]string, 0, len(logs))
	for _, message := range logs {
		msgs = append(msgs, message.Message)
		if got, want := message.Message, msg; got == want {
			return
		}
	}

	tb.Errorf("expected one of %q to contain %q", msgs, msg)
}
