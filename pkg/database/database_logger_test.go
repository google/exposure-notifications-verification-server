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

package database

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type customZapWriter struct {
	bytes.Buffer
}

func (w *customZapWriter) Sync() error { return nil }

func TestGormZapLogger(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []interface{}
		exp  string
	}{
		{
			name: "sql_rows",
			in: []interface{}{
				"sql",
				"pkg/database/authorized_app.go:279",
				time.Duration(2029786),
				"INSERT INTO \"authorized_apps\" (\"created_at\",\"updated_at\",\"deleted_at\",\"realm_id\",\"name\",\"api_key_preview\",\"api_key\",\"api_key_type\") VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING \"authorized_apps\".\"id\"",
				[]interface{}{"2021-02-19T10:08:26.564289-05:00", "2021-02-19T10:08:26.564289-05:00", nil, 2, "Closet Cloud", "CJTCAu", "gj_3Mg_397WDTNI5decfgWostzJWuEvmLxj5UWuC_cfRzF-yflr0fP4D_gnjHpC8SuWZroeWOIIXIQDgQKeeLg", 1},
				int64(1),
			},
			exp: "DEBUG\tINSERT INTO \"authorized_apps\" (\"created_at\",\"updated_at\",\"deleted_at\",\"realm_id\",\"name\",\"api_key_preview\",\"api_key\",\"api_key_type\") VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING \"authorized_apps\".\"id\"\t{\"caller\": \"pkg/database/authorized_app.go:279\", \"duration\": \"2.029786ms\", \"values\": [\"2021-02-19T10:08:26.564289-05:00\", \"2021-02-19T10:08:26.564289-05:00\", \"NULL\", \"2\", \"Closet Cloud\", \"CJTCAu\", \"gj_3Mg_397WDTNI5decfgWostzJWuEvmLxj5UWuC_cfRzF-yflr0fP4D_gnjHpC8SuWZroeWOIIXIQDgQKeeLg\", \"1\"], \"rows\": 1}",
		},
		{
			name: "sql_no_rows",
			in: []interface{}{
				"sql",
				"pkg/database/mobile_app.go:235",
				time.Duration(3268414),
				"SELECT * FROM \"mobile_apps\"    WHERE (id = $1) ORDER BY \"mobile_apps\".\"id\" ASC LIMIT 1",
				[]interface{}{0},
				int64(0),
			},
			exp: "DEBUG\tSELECT * FROM \"mobile_apps\" WHERE (id = $1) ORDER BY \"mobile_apps\".\"id\" ASC LIMIT 1\t{\"caller\": \"pkg/database/mobile_app.go:235\", \"duration\": \"3.268414ms\", \"values\": [\"0\"], \"rows\": 0}",
		},
		{
			name: "info",
			in: []interface{}{
				"info",
				"[info] the british are coming",
			},
			exp: "DEBUG\tthe british are coming",
		},
		{
			name: "log",
			in: []interface{}{
				"log",
				"pkg/database/mobile_app.go:235",
				fmt.Errorf("something is broken"),
			},
			exp: "ERROR\tgorm error\t{\"caller\": \"pkg/database/mobile_app.go:235\", \"error\": \"something is broken\"}",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var b customZapWriter
			out := zapcore.Lock(&b)

			lvl := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return true
			})

			enc := zap.NewDevelopmentEncoderConfig()
			enc.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
				// do nothing to make the test output predicatable
			}

			core := zapcore.NewCore(
				zapcore.NewConsoleEncoder(enc), out, lvl)

			logger := zap.New(core).Sugar()

			gormLogger, err := NewGormZapLogger(logger)
			if err != nil {
				t.Fatal(err)
			}

			gormLogger.Print(tc.in...)

			if err := logger.Sync(); err != nil {
				t.Fatal(err)
			}

			if got, want := strings.TrimSpace(b.String()), tc.exp; got != want {
				t.Errorf("invalid result:\n+%q\n-%q\n", got, want)
			}
		})
	}
}
