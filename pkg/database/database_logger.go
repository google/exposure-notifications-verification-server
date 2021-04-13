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
	"database/sql/driver"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"go.uber.org/zap"
)

// GormZapLogger is a gorm logger than writes to a zap logger for structured
// logging.
type GormZapLogger struct {
	logger   *zap.SugaredLogger
	spacesRe *regexp.Regexp

	moduleRoot string
}

// NewGormZapLogger creates a new gorm logger.
func NewGormZapLogger(logger *zap.SugaredLogger) (*GormZapLogger, error) {
	spacesRe, err := regexp.Compile(`\s+`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regexp: %w", err)
	}

	// Disable the caller injection because gorm supplies us the caller.
	logger = logger.Desugar().WithOptions(zap.WithCaller(false)).Sugar()

	return &GormZapLogger{
		logger:     logger,
		spacesRe:   spacesRe,
		moduleRoot: project.Root() + "/",
	}, nil
}

// Print satisfies gorm's interface for a logger.
func (l *GormZapLogger) Print(v ...interface{}) {
	if len(v) == 0 {
		return
	}

	if len(v) == 1 {
		l.logger.DPanicf("only 1 element to Print: %#v", v)
		return
	}

	switch v[0] {
	case "info":
		msg, ok := v[1].(string)
		if !ok {
			l.logger.DPanicf("info result is not string (%T): %#v", v[1], v)
			return
		}
		msg = strings.TrimPrefix(msg, "[info]")
		msg = strings.TrimSpace(msg)

		l.logger.Debugw(msg)
	case "log":
		switch typ := v[2].(type) {
		case string:
			l.logger.Debugw(strings.TrimSpace(typ))
		case error:
			l.logger.Errorw(strings.TrimSpace(typ.Error()))
		default:
			l.logger.DPanicf("log result is not string (%T): %#v", v[2], v)
		}
	case "error":
		caller, err := l.formatCaller(v[1])
		if err != nil {
			l.logger.DPanic(err)
			return
		}

		l.logger.Errorw("gorm error",
			"caller", caller,
			"error", v[2])
	case "sql":
		caller, err := l.formatCaller(v[1])
		if err != nil {
			l.logger.DPanic(err)
			return
		}

		duration, ok := v[2].(time.Duration)
		if !ok {
			l.logger.DPanicf("duration is not time.Duration (%T): %#v", v[2], v)
		}
		duration = duration * time.Nanosecond

		sql, err := l.formatSQL(v[3])
		if err != nil {
			l.logger.DPanic(err)
			return
		}

		values, err := l.formatValues(v[4])
		if err != nil {
			l.logger.DPanic(err)
			return
		}

		rows, ok := v[5].(int64)
		if !ok {
			l.logger.DPanicf("rows is not int64 (%T): %#v", v[5], v)
			return
		}

		l.logger.Debugw(sql,
			"caller", caller,
			"duration", duration,
			"values", values,
			"rows", rows)
	default:
		l.logger.DPanicf("unknown log type %v: %#v", v[0], v)
	}
}

func (l *GormZapLogger) formatCaller(v interface{}) (string, error) {
	typ, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("formatCaller: %T is not string", v)
	}

	typ = strings.TrimPrefix(typ, l.moduleRoot)
	return typ, nil
}

// formatSQL makes SQL pretty
func (l *GormZapLogger) formatSQL(v interface{}) (string, error) {
	typ, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("formatSQL: %T is not string", v)
	}

	typ = l.spacesRe.ReplaceAllString(typ, " ")
	typ = strings.TrimSpace(typ)
	return typ, nil
}

// formatValues makes SQL input values pretty
func (l *GormZapLogger) formatValues(v interface{}) ([]string, error) {
	typ, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("formatValues: %T is not []interface{}", v)
	}

	result := make([]string, 0, len(typ))
	for _, v := range typ {
		ind := reflect.Indirect(reflect.ValueOf(v))

		if !ind.IsValid() {
			result = append(result, "NULL")
			continue
		}

		if t, ok := v.(driver.Valuer); ok {
			if val, err := t.Value(); err == nil && val != nil {
				result = append(result, fmt.Sprintf("'%v'", val))
			} else {
				result = append(result, "NULL")
			}
			continue
		}

		if t, ok := v.(time.Time); ok {
			if t.IsZero() {
				t = time.Unix(0, 0)
			}
			result = append(result, t.Format(time.RFC3339))
			continue
		}

		result = append(result, fmt.Sprintf("%v", v))
	}

	return result, nil
}
