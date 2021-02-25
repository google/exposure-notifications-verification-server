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

package envstest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"log"

	"github.com/jinzhu/gorm"
)

var errNope = fmt.Errorf("test database instance says no")

// NewFailingDatabase database creates a new database connection that fails all
// requests.
func NewFailingDatabase() *gorm.DB {
	db, err := gorm.Open("postgres", sql.OpenDB(&SQLDriver{}))
	if err != nil {
		panic(err)
	}
	db.SetLogger(gorm.Logger{LogWriter: log.New(io.Discard, "", 0)})
	db.LogMode(false)
	return db
}

// SQLDriver is a test SQL driver implementation that returns an error for most
// queries and operations. It's used to test how the system behaves under
// connection errors or other database-level failures.
type SQLDriver struct{}

func (d *SQLDriver) Open(name string) (driver.Conn, error) {
	return &SQLConn{}, nil
}

func (d *SQLDriver) Connect(ctx context.Context) (driver.Conn, error) {
	return &SQLConn{}, nil
}

func (d *SQLDriver) Driver() driver.Driver {
	return &SQLDriver{}
}

type SQLConn struct{}

func (c *SQLConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errNope
}

func (c *SQLConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return nil, errNope
}

func (c *SQLConn) Begin() (driver.Tx, error) {
	return nil, errNope
}

func (c *SQLConn) BeginTx() (driver.Tx, error) {
	return nil, errNope
}

func (c *SQLConn) Commit() error {
	return errNope
}

func (c *SQLConn) Rollback() error {
	return errNope
}

func (c *SQLConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	return nil, errNope
}

func (c *SQLConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return nil, errNope
}

func (c *SQLConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return nil, errNope
}

func (c *SQLConn) Close() error {
	return nil
}
