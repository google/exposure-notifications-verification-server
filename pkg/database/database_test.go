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
	"io"
	"log"
	"reflect"
	"testing"

	"github.com/jinzhu/gorm"
)

var testDatabaseInstance *TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

type validateable interface {
	ErrorsFor(s string) []string
	BeforeSave(tx *gorm.DB) error
}

// exerciseValidation exercises zero value validation (not empty) for the given
// model and struct fields.
func exerciseValidation(t *testing.T, i validateable, structField, field string) {
	// Get interface underlying value.
	v := reflect.ValueOf(&i)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if !v.CanInterface() {
		t.Fatalf("%v cannot interface", v)
	}

	// Convert interface to struct.
	sv := reflect.ValueOf(v.Interface())
	for sv.Kind() == reflect.Ptr {
		sv = sv.Elem()
	}
	if sv.Kind() != reflect.Struct {
		t.Fatalf("%T is not a struct: %v", i, sv.Kind())
	}

	// Get struct field name.
	f := sv.FieldByName(structField)
	if !f.IsValid() {
		t.Fatalf("%s is not valid", structField)
	}
	if !f.CanSet() {
		t.Fatalf("%s is not settable", structField)
	}

	// Set to the zero value.
	valueV := reflect.Zero(f.Type())
	f.Set(valueV)

	// Create db
	var db gorm.DB
	db.SetLogger(gorm.Logger{LogWriter: log.New(io.Discard, "", 0)})

	// Run the validation.
	_ = i.BeforeSave(&gorm.DB{})
	if errs := i.ErrorsFor(field); len(errs) < 1 {
		t.Errorf("expected errors for %s", field)
	}
}
