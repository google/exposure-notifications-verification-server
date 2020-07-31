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

package database

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/opencensus-integrations/redigo/redis"
)

type redisCommandInterceptorConn struct {
	redis.ConnWithContext

	mu   sync.RWMutex
	seqs []*redisSequence
}

type redisSequence struct {
	Kind    string
	Command string
	Args    []interface{}
}

func (rcic *redisCommandInterceptorConn) SendContext(ctx context.Context, commandName string, args ...interface{}) error {
	rcic.mu.Lock()
	rcic.seqs = append(rcic.seqs, &redisSequence{"SEND", commandName, args})
	rcic.mu.Unlock()

	return rcic.ConnWithContext.SendContext(ctx, commandName, args...)
}

func (rcic *redisCommandInterceptorConn) DoContext(ctx context.Context, commandName string, args ...interface{}) (interface{}, error) {
	rcic.mu.Lock()
	rcic.seqs = append(rcic.seqs, &redisSequence{"DO", commandName, args})
	rcic.mu.Unlock()

	return rcic.ConnWithContext.DoContext(ctx, commandName, args...)
}

func (rcic *redisCommandInterceptorConn) CloseContext(ctx context.Context) error {
	rcic.mu.Lock()
	rcic.seqs = append(rcic.seqs, &redisSequence{Kind: "CLOSE"})
	rcic.mu.Unlock()

	return rcic.ConnWithContext.CloseContext(ctx)
}

func TestRedisAsPassthroughCache(t *testing.T) {
	// Skip if Redis is not enabled.
	redisConn, err := redis.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		t.Skipf("Redis could not be reached: %v", err)
	}

	redisInterceptor := &redisCommandInterceptorConn{ConnWithContext: redisConn.(redis.ConnWithContext)}
	defer redisInterceptor.CloseContext(context.Background())

	// Okay Redis is available, now use it.
	db := NewTestDatabase(t)
	redisPool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redisInterceptor, nil
		},
	}
	db.SetRedisPool(redisPool)

	email := "dr@example.com"
	user := User{
		Email:       email,
		Name:        "Dr Example",
		Admin:       false,
		Disabled:    false,
		Realms:      []*Realm{},
		AdminRealms: []*Realm{},
	}

	if err := db.SaveUser(&user); err != nil {
		t.Fatalf("error creating user: %v", err)
	}
	userV1JSON, err := json.Marshal(user)
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.FindUser(email)
	if err != nil {
		t.Fatalf("error reading user from db: %v", err)
	}

	if diff := cmp.Diff(user, *got, approxTime); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	user.Admin = true
	if err := db.SaveUser(&user); err != nil {
		t.Fatalf("error updating user: %v", err)
	}
	userV2JSON, err := json.Marshal(user)
	if err != nil {
		t.Fatal(err)
	}

	got, err = db.FindUser(email)
	if err != nil {
		t.Fatalf("error reading user from db: %v", err)
	}

	if diff := cmp.Diff(user, *got, approxTime); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	got, err = db.GetUser(user.ID)
	if err != nil {
		t.Fatalf("error reading user from db by ID: %v", err)
	}

	if diff := cmp.Diff(user, *got, approxTime); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	user.Disabled = true
	if err := db.SaveUser(&user); err != nil {
		t.Fatalf("error failed to disable user: %v", err)
	}

	time.Sleep(time.Millisecond * 5)

	delCount, err := db.PurgeUsers(time.Millisecond)
	if err != nil {
		t.Fatalf("failed to purge users: %v", err)
	}
	if g, w := delCount, int64(1); g != w {
		t.Fatalf("failed to purge all users, count mismatch: got %d want %d", g, w)
	}
	userV3JSON, err := json.Marshal(user)
	if err != nil {
		t.Fatal(err)
	}
	// Now we shall compare with the results.
	want := []*redisSequence{
		// 1. The first User.save(...)
		{Kind: "SEND", Command: "MULTI"},
		{
			Kind:    "SEND",
			Command: "SET",
			Args: []interface{}{
				"USERS_email", "dr@example.com", userV1JSON,
			},
		},
		// Set that TTL so that the user data expires after a day unconditionally.
		{
			Kind:    "SEND",
			Command: "EXPIRE",
			Args: []interface{}{
				"USERS_email", "dr@example.com", float64(86400),
			},
		},
		{
			Kind:    "SEND",
			Command: "SET",
			Args: []interface{}{
				"USERS_id", user.ID, userV1JSON,
			},
		},
		// Set that TTL so that the user data expires after a day unconditionally.
		{
			Kind:    "SEND",
			Command: "EXPIRE",
			Args: []interface{}{
				"USERS_id", user.ID, float64(86400),
			},
		},
		{
			Kind:    "DO",
			Command: "EXEC",
		},

		// 2. The db.FindUser(email)
		{
			Kind:    "DO",
			Command: "GET",
			Args: []interface{}{
				"USERS_email", "dr@example.com",
			},
		},

		// 3. The db.Save(...) after User.Admin = true
		{
			Kind:    "SEND",
			Command: "MULTI",
		},
		{
			Kind:    "SEND",
			Command: "SET",
			Args: []interface{}{
				"USERS_email", "dr@example.com", userV2JSON,
			},
		},
		{
			Kind:    "SEND",
			Command: "EXPIRE",
			Args: []interface{}{
				"USERS_email", "dr@example.com", float64(86400),
			},
		},
		{
			Kind:    "SEND",
			Command: "SET",
			Args: []interface{}{
				"USERS_id", user.ID, userV2JSON,
			},
		},
		{
			Kind:    "SEND",
			Command: "EXPIRE",
			Args: []interface{}{
				"USERS_id", user.ID, float64(86400),
			},
		},
		{
			Kind:    "DO",
			Command: "EXEC",
		},

		// 4. The db.FindUser(email)
		{
			Kind:    "DO",
			Command: "GET",
			Args: []interface{}{
				"USERS_email", "dr@example.com",
			},
		},

		// 5. The db.FindUser(email)
		{
			Kind:    "DO",
			Command: "GET",
			Args: []interface{}{
				"USERS_id", user.ID,
			},
		},

		// 6. Disable the user.
		{
			Kind:    "SEND",
			Command: "MULTI",
		},
		{
			Kind:    "SEND",
			Command: "SET",
			Args: []interface{}{
				"USERS_email", "dr@example.com", userV3JSON,
			},
		},
		{
			Kind:    "SEND",
			Command: "EXPIRE",
			Args: []interface{}{
				"USERS_email", "dr@example.com", float64(86400),
			},
		},
		{
			Kind:    "SEND",
			Command: "SET",
			Args: []interface{}{
				"USERS_id", user.ID, userV3JSON,
			},
		},
		{
			Kind:    "SEND",
			Command: "EXPIRE",
			Args: []interface{}{
				"USERS_id", user.ID, float64(86400),
			},
		},
		{
			Kind:    "DO",
			Command: "EXEC",
		},

		// 7. The deletion.
		{
			Kind:    "SEND",
			Command: "MULTI",
		},
		{
			Kind:    "SEND",
			Command: "DEL",
			Args: []interface{}{
				"USERS_email", "dr@example.com",
			},
		},
		{
			Kind:    "SEND",
			Command: "DEL",
			Args: []interface{}{
				"USERS_id", user.ID,
			},
		},
		{
			Kind:    "DO",
			Command: "EXEC",
		},
	}
	if diff := cmp.Diff(redisInterceptor.seqs, want); diff != "" {
		t.Fatalf("Redis sequence mismatch: got - want +\n%s", diff)
	}
}
