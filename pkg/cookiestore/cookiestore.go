// Copyright 2021 the Exposure Notifications Verification Server authors
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

// Package cookiestore provies an abstraction on top of the existing cookie
// store to handle hot-reloading of cookie HMAC and encryption keys.
package cookiestore

import (
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

// New returns a new session store
func New(fn EntropyFunc, opts *sessions.Options) sessions.Store {
	if opts == nil {
		opts = new(sessions.Options)
	}
	if opts.Path == "" {
		opts.Path = "/"
	}
	if opts.MaxAge <= 0 {
		opts.MaxAge = 30 * 86400 // 30d
	}

	codec := &HotCodec{
		entropyFunc: fn,
		maxAge:      opts.MaxAge,
	}

	cs := &sessions.CookieStore{
		Codecs:  []securecookie.Codec{codec},
		Options: opts,
	}
	return cs
}
