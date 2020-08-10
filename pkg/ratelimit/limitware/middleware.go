// Package limitware provides middleware for rate limiting HTTP handlers.
package limitware

import (
	"crypto/sha1"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/sethvargo/go-limiter"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/api/metric"
)

const (
	// HeaderRateLimitLimit, HeaderRateLimitRemaining, and HeaderRateLimitReset
	// are the recommended return header values from IETF on rate limiting. Reset
	// is in UTC time.
	HeaderRateLimitLimit     = "X-RateLimit-Limit"
	HeaderRateLimitRemaining = "X-RateLimit-Remaining"
	HeaderRateLimitReset     = "X-RateLimit-Reset"

	// HeaderRetryAfter is the header used to indicate when a client should retry
	// requests (when the rate limit expires), in UTC time.
	HeaderRetryAfter = "Retry-After"
)

// KeyFunc is a function that accepts an http request and returns a string key
// that uniquely identifies this request for the purpose of rate limiting.
//
// KeyFuncs are called on each request, so be mindful of performance and
// implement caching where possible. If a KeyFunc returns an error, the HTTP
// handler will return Internal Server Error and will NOT take from the limiter
// store.
type KeyFunc func(r *http.Request) (string, error)

// IPKeyFunc returns a function that keys data based on the incoming requests IP
// address. By default this uses the RemoteAddr, but you can also specify a list
// of headers which will be checked for an IP address first (e.g.
// "X-Forwarded-For"). Headers are retrieved using Header.Get(), which means
// they are case insensitive.
func IPKeyFunc(headers ...string) KeyFunc {
	return func(r *http.Request) (string, error) {
		for _, h := range headers {
			if v := r.Header.Get(h); v != "" {
				return v, nil
			}
		}

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return "", err
		}
		return ip, nil
	}
}

// Middleware is a handler/mux that can wrap other middlware to implement HTTP
// rate limiting. It can rate limit based on an arbitrary KeyFunc, and supports
// anything that implements limiter.Store.
type Middleware struct {
	store      limiter.Store
	keyFunc    KeyFunc
	meter      metric.Meter
	reqCounter metric.Int64Counter
}

// NewMiddleware creates a new middleware suitable for use as an HTTP handler.
// This function returns an error if either the Store or KeyFunc are nil.
func NewMiddleware(s limiter.Store, f KeyFunc) (*Middleware, error) {
	if s == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}

	if f == nil {
		return nil, fmt.Errorf("key function cannot be nil")
	}

	meter := global.Meter("github.com/sethvargo/go-limiter")
	rc, err := meter.NewInt64Counter("request_count", metric.WithDescription("counts number of requests processed by middleware and their status"))
	if err != nil {
		return nil, err
	}

	return &Middleware{
		store:      s,
		keyFunc:    f,
		meter:      meter,
		reqCounter: rc,
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
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Take from the store.
		limit, remaining, reset, ok := m.store.Take(key)
		resetTime := time.Unix(0, int64(reset)).UTC().Format(time.RFC1123)

		// Set headers (we do this regardless of whether the request is permitted).
		w.Header().Set(HeaderRateLimitLimit, strconv.FormatUint(limit, 10))
		w.Header().Set(HeaderRateLimitRemaining, strconv.FormatUint(remaining, 10))
		w.Header().Set(HeaderRateLimitReset, resetTime)

		// Record request status
		m.meter.RecordBatch(
			r.Context(),
			[]kv.KeyValue{kv.Bool("ok", ok)},
			m.reqCounter.Measurement(1),
		)

		// Fail if there were no tokens remaining.
		if !ok {
			w.Header().Set(HeaderRetryAfter, resetTime)
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		// If we got this far, we're allowed to continue, so call the next middleware
		// in the stack to continue processing.
		next.ServeHTTP(w, r)
	})
}

// APIKeyFunc returns a default key function for ratelimiting on our API key header.
func APIKeyFunc(db *database.Database) KeyFunc {
	ipKeyFunc := IPKeyFunc("X-Forwarded-For")

	return func(r *http.Request) (string, error) {
		v := r.Header.Get("X-API-Key")
		if v != "" {
			// v2 API keys encode the realm
			_, realmID, err := db.VerifyAPIKeySignature(v)
			if err == nil {
				return strconv.FormatUint(uint64(realmID), 10), nil
			}

			// v1 API keys do not, fallback to the database
			app, err := db.FindAuthorizedAppByAPIKey(v)
			if err == nil && app != nil {
				return strconv.FormatUint(uint64(app.RealmID), 10), nil
			}
		}

		// If no API key was provided, default to limiting by IP.
		return ipKeyFunc(r)
	}
}

// UserEmailKeyFunc pulls the user out of the request context and uses that to ratelimit.
func UserEmailKeyFunc() KeyFunc {
	ipKeyFunc := IPKeyFunc("X-Forwarded-For")

	return func(r *http.Request) (string, error) {
		user := controller.UserFromContext(r.Context())
		if user != nil && user.Email != "" {
			dig := sha1.Sum([]byte(user.Email))
			return fmt.Sprintf("%x", dig), nil
		}

		return ipKeyFunc(r)
	}
}
