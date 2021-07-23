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

package cookiestore

import (
	"fmt"

	"github.com/gorilla/securecookie"
)

// EntropyFunc is a function that returns the cookie hash and encryption keys.
// The first 32 bytes are the encryption key, the remaining bytes are the HMAC
// key.
type EntropyFunc func() ([][]byte, error)

var _ securecookie.Codec = (*HotCodec)(nil)

// HotCodec is a securecookie Codec that supports hot-loading its hash and
// encryption keys.
type HotCodec struct {
	maxAge      int
	entropyFunc EntropyFunc
}

// Encode implements securecookie Codec.
func (c *HotCodec) Encode(name string, value interface{}) (string, error) {
	cs, err := c.newSecureCookies()
	if err != nil {
		return "", fmt.Errorf("failed to make secure cookie: %w", err)
	}

	return securecookie.EncodeMulti(name, value, cs...)
}

// Decode implements securecookie Codec.
func (c *HotCodec) Decode(name, value string, dst interface{}) error {
	cs, err := c.newSecureCookies()
	if err != nil {
		return fmt.Errorf("failed to make secure cookie: %w", err)
	}
	return securecookie.DecodeMulti(name, value, dst, cs...)
}

// newSecureCookies creates a new collection of secure cookies from the data
// from the entropy function.
func (c *HotCodec) newSecureCookies() ([]securecookie.Codec, error) {
	bs, err := c.entropyFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to get cookie hash/encryption keys: %w", err)
	}

	codecs := make([]securecookie.Codec, len(bs))
	for i, b := range bs {
		if got, want := len(b), 32; got < want {
			return nil, fmt.Errorf("expected byte length %d to be at least %d", got, want)
		}

		// The first 32 bytes are the encryption key, remaining are the HMAC key.
		cookie := securecookie.New(b[32:], b[:32])
		cookie.MaxAge(c.maxAge)
		codecs[i] = cookie
	}
	return codecs, nil
}
