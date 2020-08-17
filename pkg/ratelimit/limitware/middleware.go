// Package limitware provides middleware for rate limiting HTTP handlers.
package limitware

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"github.com/google/exposure-notifications-server/pkg/logging"

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
	store      limiter.Store
	keyFunc    httplimit.KeyFunc
	reqCounter *stats.Int64Measure
	keyErrors  *stats.Int64Measure
	logger     *zap.SugaredLogger
}

// NewMiddleware creates a new middleware suitable for use as an HTTP handler.
// This function returns an error if either the Store or KeyFunc are nil.
func NewMiddleware(ctx context.Context, s limiter.Store, f httplimit.KeyFunc) (*Middleware, error) {
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

	return &Middleware{
		store:      s,
		keyFunc:    f,
		reqCounter: rc,
		keyErrors:  ke,
		logger:     logger,
	}, nil
}

// Handle returns the HTTP handler as a middleware. This handler calls Take() on
// the store and sets the common rate limiting headers. If the take is
// successful, the remaining middleware is called. If take is unsuccessful, the
// middleware chain is halted and the function renders a 429 to the caller with
// metadata about when it's safe to retry.
func (m *Middleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Call the key function - if this fails, it's an internal server error.
		key, err := m.keyFunc(r)
		if err != nil {
			m.logger.Errorw("could not call key function", "error", err)
			stats.Record(r.Context(), m.keyErrors.M(1))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Take from the store.
		limit, remaining, reset, ok := m.store.Take(key)
		resetTime := time.Unix(0, int64(reset)).UTC().Format(time.RFC1123)

		// Set headers (we do this regardless of whether the request is permitted).
		w.Header().Set(httplimit.HeaderRateLimitLimit, strconv.FormatUint(limit, 10))
		w.Header().Set(httplimit.HeaderRateLimitRemaining, strconv.FormatUint(remaining, 10))
		w.Header().Set(httplimit.HeaderRateLimitReset, resetTime)

		// Record request status
		ctx, err := tag.New(r.Context(), tag.Insert(OkTag, fmt.Sprintf("%v", ok)))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		stats.Record(ctx, m.reqCounter.M(1))

		// Fail if there were no tokens remaining.
		if !ok {
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
// header. It falls back to rate limiting by the client ip.
func APIKeyFunc(ctx context.Context, scope string, db *database.Database) httplimit.KeyFunc {
	logger := logging.FromContext(ctx).Named("ratelimit")
	ipAddrLimit := IPAddressKeyFunc(ctx, scope)

	return func(r *http.Request) (string, error) {
		// Procss the API key
		v := r.Header.Get("X-API-Key")
		if v != "" {
			// v2 API keys encode the realm
			_, realmID, err := db.VerifyAPIKeySignature(v)
			if err == nil {
				logger.Debugw("limiting by api key v2 realm", "realm", realmID)
				dig := sha1.Sum([]byte(fmt.Sprintf("%d", realmID)))
				return fmt.Sprintf("apiserver:realm:%x", dig), nil
			}

			// v1 API keys do not, fallback to the database
			app, err := db.FindAuthorizedAppByAPIKey(v)
			if err == nil && app != nil {
				logger.Debugw("limiting by api key v1 realm", "realm", app.RealmID)
				dig := sha1.Sum([]byte(fmt.Sprintf("%d", app.RealmID)))
				return fmt.Sprintf("%s:realm:%x", scope, dig), nil
			}
		}

		return ipAddrLimit(r)
	}
}

// UserEmailKeyFunc pulls the user out of the request context and uses that to
// ratelimit. It falls back to rate limiting by the client ip.
func UserEmailKeyFunc(ctx context.Context, scope string) httplimit.KeyFunc {
	logger := logging.FromContext(ctx).Named("ratelimit")
	ipAddrLimit := IPAddressKeyFunc(ctx, scope)

	return func(r *http.Request) (string, error) {
		ctx := r.Context()

		// See if a user exists on the context
		user := controller.UserFromContext(ctx)
		if user != nil && user.Email != "" {
			logger.Debugw("limiting by user", "user", user.ID)
			dig := sha1.Sum([]byte(user.Email))
			return fmt.Sprintf("%s:user:%x", scope, dig), nil
		}

		return ipAddrLimit(r)
	}
}

// IPAddressKeyFunc uses the client IP to rate limit.
func IPAddressKeyFunc(ctx context.Context, scope string) httplimit.KeyFunc {
	logger := logging.FromContext(ctx).Named("ratelimit")

	return func(r *http.Request) (string, error) {
		// Get the remote addr
		ip := r.RemoteAddr

		// Check if x-forwarded-for exists, the load balancer sets this, and the
		// first entry is the real client IP
		xff := r.Header.Get("x-forwarded-for")
		if xff != "" {
			ip = strings.Split(xff, ",")[0]
		}

		logger.Debugw("limiting by ip", "ip", ip)
		dig := sha1.Sum([]byte(ip))
		return fmt.Sprintf("%s:ip:%x", scope, dig), nil
	}
}
