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

package routes

import (
	"testing"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/sethvargo/go-limiter/memorystore"
)

func TestENXRedirect(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cfg := &config.RedirectConfig{}
	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	cacher, err := cache.NewNoop()
	if err != nil {
		t.Fatal(err)
	}

	limiterStore, err := memorystore.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	signer := keys.TestKeyManager(t)

	mux, closer, err := ENXRedirect(ctx, cfg, db, cacher, signer, limiterStore)
	t.Cleanup(closer)
	if err != nil {
		t.Fatal(err)
	}
	_ = mux
}
