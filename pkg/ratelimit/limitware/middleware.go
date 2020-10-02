// Package limitware provides middleware for rate limiting HTTP handlers.
package limitware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/digest"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/opencensus-integrations/redigo/redis"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/httplimit"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
)

var (
	MetricPrefix = observability.MetricRoot + "/ratelimit/limitware"
	OkTag, _     = tag.NewKey("ok")
)

// Middleware is a handler/mux that can wrap other middlware to implement HTTP
// rate limiting. It can rate limit based on an arbitrary KeyFunc, and supports
// anything that implements limiter.Store.
type Middleware struct {
	store       limiter.Store
	keyFunc     httplimit.KeyFunc
	reqCounter  *stats.Int64Measure
	keyErrors   *stats.Int64Measure
	takeErrors  *stats.Int64Measure
	rateLimited *stats.Int64Measure
	logger      *zap.SugaredLogger

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

	logger := logging.FromContext(ctx).Named("ratelimit")

	rc := stats.Int64(MetricPrefix+"/request", "requests seen by middleware", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/request_count",
		Measure:     rc,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	ke := stats.Int64(MetricPrefix+"/key_errors", "errors seen from key function", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/key_errors_count",
		Measure:     ke,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	te := stats.Int64(MetricPrefix+"/take_errors", "errors seen from take function", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/take_errors_count",
		Measure:     te,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	rl := stats.Int64(MetricPrefix+"/rate_limited", "rate limited requests", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/rate_limited_count",
		Measure:     rl,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	if err := view.Register(redis.ObservabilityMetricViews...); err != nil {
		return nil, fmt.Errorf("redis view registration failure: %w", err)
	}

	m := &Middleware{
		store:       s,
		keyFunc:     f,
		reqCounter:  rc,
		keyErrors:   ke,
		takeErrors:  te,
		rateLimited: rl,
		logger:      logger,
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
		ctx := observability.WithBuildInfo(r.Context())

		// Call the key function - if this fails, it's an internal server error.
		key, err := m.keyFunc(r)
		if err != nil {
			m.logger.Errorw("could not call key function", "error", err)
			stats.Record(ctx, m.keyErrors.M(1))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Take from the store.
		limit, remaining, reset, ok, err := m.store.Take(ctx, key)
		if err != nil {
			m.logger.Errorw("failed to take", "error", err)
			stats.Record(ctx, m.takeErrors.M(1))

			if !m.allowOnError {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		resetTime := time.Unix(0, int64(reset)).UTC().Format(time.RFC1123)

		// Set headers (we do this regardless of whether the request is permitted).
		w.Header().Set(httplimit.HeaderRateLimitLimit, strconv.FormatUint(limit, 10))
		w.Header().Set(httplimit.HeaderRateLimitRemaining, strconv.FormatUint(remaining, 10))
		w.Header().Set(httplimit.HeaderRateLimitReset, resetTime)

		// Record request status
		ctx, err = tag.New(ctx, tag.Insert(OkTag, fmt.Sprintf("%v", ok)))
		if err != nil {
			m.logger.Errorw("could not create tag", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		stats.Record(ctx, m.reqCounter.M(1))

		// Fail if there were no tokens remaining.
		if !ok {
			stats.Record(ctx, m.rateLimited.M(1))
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
	logger := logging.FromContext(ctx).Named("ratelimit.APIKeyFunc")
	ipAddrLimit := IPAddressKeyFunc(ctx, scope, hmacKey)

	return func(r *http.Request) (string, error) {
		// Procss the API key
		v := r.Header.Get("x-api-key")
		if v != "" {
			realmID := realmIDFromAPIKey(db, v)
			if realmID != 0 {
				logger.Debugw("limiting by realm from apikey")
				dig, err := digest.HMAC(fmt.Sprintf("%d:%s", realmID, remoteIP(r)), hmacKey)
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
	logger := logging.FromContext(ctx).Named("ratelimit.UserIDKeyFunc")
	ipAddrLimit := IPAddressKeyFunc(ctx, scope, hmacKey)

	return func(r *http.Request) (string, error) {
		ctx := r.Context()

		// See if a user exists on the context
		currentUser := controller.UserFromContext(ctx)
		if currentUser != nil {
			logger.Debugw("limiting by user", "user", currentUser.ID)
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
	logger := logging.FromContext(ctx).Named("ratelimit.IPAddressKeyFunc")

	return func(r *http.Request) (string, error) {
		// Get the remote addr
		ip := remoteIP(r)

		logger.Debugw("limiting by ip", "ip", ip)
		dig, err := digest.HMAC(ip, hmacKey)
		if err != nil {
			return "", fmt.Errorf("failed to digest ip: %w", err)
		}
		return fmt.Sprintf("%sip:%s", scope, dig), nil
	}
}

// remoteIP returns the "real" remote IP.
func remoteIP(r *http.Request) string {
	// Get the remote addr
	ip := r.RemoteAddr

	// Check if x-forwarded-for exists, the load balancer sets this, and the
	// first entry is the real client IP
	xff := r.Header.Get("x-forwarded-for")
	if xff != "" {
		ip = strings.Split(xff, ",")[0]
	}

	return ip
}

// realmIDFromAPIKey extracts the realmID from the provided API key, handling v1
// and v2 API key formats.
func realmIDFromAPIKey(db *database.Database, apiKey string) uint {
	// v2 API keys encode in the realm to limit the db calls
	_, realmID, err := db.VerifyAPIKeySignature(apiKey)
	if err == nil {
		return realmID
	}

	// v1 API keys are more expensive
	app, err := db.FindAuthorizedAppByAPIKey(apiKey)
	if err == nil {
		return app.RealmID
	}

	return 0
}
