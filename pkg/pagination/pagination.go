// Copyright 2020 the Exposure Notifications Verification Server authors
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

// Package pagination defines pagination helpers.
package pagination

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// QueryKeyPage is the URL querystring for the current page.
	QueryKeyPage = "page"

	// QueryKeyLimit is the URL querystring for the current per-page limit.
	QueryKeyLimit = "limit"

	// DefaultLimit is the default pagination limit, if one is not provided.
	DefaultLimit = uint64(14)

	// MaxLimit is the maximum size for a single pagination. If a limit higher
	// than this is provided, it's capped.
	MaxLimit = uint64(100)
)

// PageParams are the page parameters including the current page and per page
// limit.
type PageParams struct {
	// Page is the current page.
	Page uint64

	// Limit is the per-page limit.
	Limit uint64
}

// UnlimitedResults is a paginator that doesn't do pagination and returns all
// results (up to (1<<63)-1).
var UnlimitedResults = &PageParams{
	Page:  1,
	Limit: math.MaxInt64,
}

// FromRequest builds the PageParams from an http.Request. If there are no
// pagination parameters, the struct is returned with the default values.
func FromRequest(r *http.Request) (*PageParams, error) {
	page := uint64(1)
	if v := strings.TrimSpace(r.FormValue(QueryKeyPage)); v != "" {
		var err error
		page, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q query parameter: %w", QueryKeyPage, err)
		}
	}

	limit := DefaultLimit
	if v := strings.TrimSpace(r.FormValue(QueryKeyLimit)); v != "" {
		var err error
		limit, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q query parameter: %w", QueryKeyLimit, err)
		}
	}

	// Cap the maximum limit.
	if limit > MaxLimit {
		limit = MaxLimit
	}

	return &PageParams{
		Page:  page,
		Limit: limit,
	}, nil
}

// Paginator is a meta-struct that holds pagination information. It's largely a
// collection of pointers to Page structs. You may notice some repetition across
// fields - this is to make the struct easier to use inside Go template's where
// simple features like "get the last element of a slice" are actually very
// hard.
type Paginator struct {
	PrevPage  *Page
	PrevPages []*Page
	CurrPage  *Page
	NextPages []*Page
	NextPage  *Page
}

// Page represents a single page.
type Page struct {
	// Number is the numerical page number.
	Number uint64
}

// Link is the link to this page. It accepts the current URL, adds/modifies the
// page querystring parameter, and returns a new URL.
func (p *Page) Link(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}

	// If the page is 0 or 1, we want to drop the page param from the query.
	// Otherwise add the page param with the value.
	q := u.Query()
	if p.Number <= 1 {
		q.Del(QueryKeyPage)
	} else {
		q.Set(QueryKeyPage, strconv.FormatUint(p.Number, 10))
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}
