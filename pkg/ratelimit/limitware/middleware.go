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

// Package limitware provides middleware for rate limiting HTTP handlers.
package limitware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/digest"
	"github.com/google/exposure-notifications-verification-server/pkg/realip"

	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/httplimit"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

// Middleware is a handler/mux that can wrap other middlware to implement HTTP
// rate limiting. It can rate limit based on an arbitrary KeyFunc, and supports
// anything that implements limiter.Store.
type Middleware struct {
	store   limiter.Store
	keyFunc httplimit.KeyFunc

	allowOnError bool
}

// Option is an option to the middleware.
type Option func(m *Middleware) *Middleware

// AllowOnError instructs the middleware to fail (internal server error) on
// connection errors. The default behavior is to fail on errors to Take.
func AllowOnError(v bool) Option {
	return func(m *Middleware) *Middleware {
		m.allowOnError = v
		return m
	}
}

// NewMiddleware creates a new middleware suitable for use as an HTTP handler.
// This function returns an error if either the Store or KeyFunc are nil.
func NewMiddleware(ctx context.Context, s limiter.Store, f httplimit.KeyFunc, opts ...Option) (*Middleware, error) {
	if s == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}

	if f == nil {
		return nil, fmt.Errorf("key function cannot be nil")
	}

	m := &Middleware{
		store:   s,
		keyFunc: f,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		m = opt(m)
	}

	return m, nil
}

// Handle returns the HTTP handler as a middleware. This handler calls Take() on
// the store and sets the common rate limiting headers. If the take is
// successful, the remaining middleware is called. If take is unsuccessful, the
// middleware chain is halted and the function renders a 429 to the caller with
// metadata about when it's safe to retry.
func (m *Middleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("ratelimit.Handle")

		result := enobs.ResultOK

		defer func(result *tag.Mutator) {
			ctx, err := tag.New(ctx, *result)
			if err != nil {
				logger.Warnw("failed to create context with additional tags", "error", err)
				// NOTE: do not return here. We should log it as success.
			}
			stats.Record(ctx, mRequest.M(1))
		}(&result)

		// Call the key function - if this fails, it's an internal server error.
		key, err := m.keyFunc(r)
		if err != nil {
			logger.Errorw("could not call key function", "error", err)
			result = enobs.ResultError("FAILED_TO_CALL_KEY_FUNCTION")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Take from the store.
		limit, remaining, reset, ok, err := m.store.Take(ctx, key)
		if err != nil {
			logger.Errorw("failed to take", "error", err)

			if !m.allowOnError {
				result = enobs.ResultError("FAILED_TO_TAKE")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		resetTime := time.Unix(0, int64(reset)).UTC().Format(time.RFC1123)

		// Set headers (we do this regardless of whether the request is permitted).
		w.Header().Set(httplimit.HeaderRateLimitLimit, strconv.FormatUint(limit, 10))
		w.Header().Set(httplimit.HeaderRateLimitRemaining, strconv.FormatUint(remaining, 10))
		w.Header().Set(httplimit.HeaderRateLimitReset, resetTime)

		// Fail if there were no tokens remaining.
		if !ok {
			logger.Infow("rate limited", "key", key)
			result = enobs.ResultError("RATE_LIMITED")
			w.Header().Set(httplimit.HeaderRetryAfter, resetTime)
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		// If we got this far, we're allowed to continue, so call the next middleware
		// in the stack to continue processing.
		next.ServeHTTP(w, r)
	})
}

// APIKeyFunc returns a default key function for ratelimiting on our API key
// header. Since APIKeys are assumed to be "public" at some point, they are rate
// limited by [realm,ip], and API keys have a 1-1 mapping to a realm.
func APIKeyFunc(ctx context.Context, db *database.Database, scope string, hmacKey []byte) httplimit.KeyFunc {
	ipAddrLimit := IPAddressKeyFunc(ctx, scope, hmacKey)

	return func(r *http.Request) (string, error) {
		// Procss the API key
		v := r.Header.Get("x-api-key")
		if v != "" {
			realmID := realmIDFromAPIKey(db, v)
			if realmID != 0 {
				dig, err := digest.HMAC(fmt.Sprintf("%d:%s", realmID, realip.FromGoogleCloud(r)), hmacKey)
				if err != nil {
					return "", fmt.Errorf("failed to digest api key: %w", err)
				}
				return fmt.Sprintf("%srealm:%s", scope, dig), nil
			}
		}

		return ipAddrLimit(r)
	}
}

// UserIDKeyFunc pulls the user out of the request context and uses that to
// ratelimit. It falls back to rate limiting by the client ip.
func UserIDKeyFunc(ctx context.Context, scope string, hmacKey []byte) httplimit.KeyFunc {
	ipAddrLimit := IPAddressKeyFunc(ctx, scope, hmacKey)

	return func(r *http.Request) (string, error) {
		// See if a user exists on the context
		currentUser := controller.UserFromContext(ctx)
		if currentUser != nil {
			dig, err := digest.HMACUint(currentUser.ID, hmacKey)
			if err != nil {
				return "", fmt.Errorf("failed to digest user id: %w", err)
			}
			return fmt.Sprintf("%suser:%s", scope, dig), nil
		}

		return ipAddrLimit(r)
	}
}

// IPAddressKeyFunc uses the client IP to rate limit.
func IPAddressKeyFunc(ctx context.Context, scope string, hmacKey []byte) httplimit.KeyFunc {
	return func(r *http.Request) (string, error) {
		// Get the remote addr
		ip := realip.FromGoogleCloud(r)

		dig, err := digest.HMAC(ip, hmacKey)
		if err != nil {
			return "", fmt.Errorf("failed to digest ip: %w", err)
		}
		return fmt.Sprintf("%sip:%s", scope, dig), nil
	}
}

// realmIDFromAPIKey extracts the realmID from the provided API key, handling v1
// and v2 API key formats.
func realmIDFromAPIKey(db *database.Database, apiKey string) uint64 {
	// v2 API keys encode in the realm to limit the db calls
	_, realmID, err := db.VerifyAPIKeySignature(apiKey)
	if err == nil {
		return realmID
	}

	// v1 API keys are more expensive
	app, err := db.FindAuthorizedAppByAPIKey(apiKey)
	if err == nil {
		return uint64(app.RealmID)
	}

	return 0
}
