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

// Package config defines the environment baased configuration for this project.
// Each server has a unique config type.
package config

import (
	"context"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/pkg/secrets"

	"github.com/sethvargo/go-envconfig"
)

// Validatable indicates that a type can be validated.
type Validatable interface {
	Validate() error
}

// ProcessWith creates a new config with the given lookuper for parsing config.
func ProcessWith(ctx context.Context, spec interface{}, l envconfig.Lookuper) error {
	// Build a list of mutators. This list will grow as we initialize more of the
	// configuration, such as the secret manager.
	var mutatorFuncs []envconfig.MutatorFunc

	{
		// Load the secret manager configuration first - this needs to be loaded first
		// because other processors may need secrets.
		var smConfig secrets.Config
		if err := envconfig.ProcessWith(ctx, &smConfig, l); err != nil {
			return fmt.Errorf("unable to process secret configuration: %w", err)
		}

		sm, err := secrets.SecretManagerFor(ctx, &smConfig)
		if err != nil {
			return fmt.Errorf("unable to connect to secret manager: %w", err)
		}

		// Enable caching, if a TTL was provided.
		if ttl := smConfig.SecretCacheTTL; ttl > 0 {
			sm, err = secrets.WrapCacher(ctx, sm, ttl)
			if err != nil {
				return fmt.Errorf("unable to create secret manager cache: %w", err)
			}
		}

		// Update the mutators to process secrets.
		mutatorFuncs = append(mutatorFuncs, secrets.Resolver(sm, &smConfig))
	}

	// Parse the main configuration.
	if err := envconfig.ProcessWith(ctx, spec, l, mutatorFuncs...); err != nil {
		return err
	}

	if typ, ok := spec.(Validatable); ok {
		if err := typ.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func checkPositiveDuration(d time.Duration, name string) error {
	if d < 0 {
		return fmt.Errorf("%v must be a positive duration, got: %v", name, d)
	}
	return nil
}
