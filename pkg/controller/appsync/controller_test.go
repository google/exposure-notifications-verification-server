// Copyright 2021 Google LLC
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

package appsync

import (
	"testing"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cfg := &config.AppSyncConfig{
		AppSyncURL: "totally invalid" + string(rune(0x7f)),
	}

	if _, err := New(cfg, nil, nil); err == nil {
		t.Errorf("expected error")
	}
}
