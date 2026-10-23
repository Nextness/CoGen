// Package sqlprobe supplies a test-owned SQL driver for controlled storage boundaries.
package sqlprobe

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
)

// Call records the SQL and converted bind values sent by a public store operation.
type Call struct {
	SQL  string
	Args []driver.NamedValue
}

// Driver dispatches queries and writes to test-controlled callbacks without a real database.
type Driver struct {
	Query                      func(Call) (driver.Rows, error)
	Exec                       func(Call) (driver.Result, error)
	BeginError                 error
	Begins, Commits, Rollbacks int
}

// OpenDB opens a lazy handle over this test's isolated driver.
func (d *Driver) OpenDB() *sql.DB { return sql.OpenDB(connector{d}) }

// Open binds a connection to the controlled callbacks.
func (d *Driver) Open(string) (driver.Conn, error) { return &connection{d}, nil }

// connector avoids global driver registration and name collisions between tests.
type connector struct{ driver *Driver }

// Connect opens one controlled connection.
func (c connector) Connect(context.Context) (driver.Conn, error) { return c.driver.Open("") }

// Driver returns the connector's underlying driver.
func (c connector) Driver() driver.Driver { return c.driver }

// connection forwards SQL operations to the owning test.
type connection struct{ driver *Driver }

// Close releases the connection without external resources.
func (*connection) Close() error { return nil }

// Prepare rejects unexpected fallback operations so a missing callback cannot pass silently.
func (*connection) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected prepared statement")
}

// Begin tracks a public store's transaction boundary.
func (c *connection) Begin() (driver.Tx, error) {
	c.driver.Begins++
	if c.driver.BeginError != nil {
		return nil, c.driver.BeginError
	}
	return transaction{c.driver}, nil
}

// QueryContext invokes the query callback with the actual SQL and bindings.
func (c *connection) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.driver.Query == nil {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	return c.driver.Query(Call{query, append([]driver.NamedValue(nil), args...)})
}

// ExecContext invokes the write callback with the actual SQL and bindings.
func (c *connection) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.driver.Exec == nil {
		return nil, fmt.Errorf("unexpected write: %s", query)
	}
	return c.driver.Exec(Call{query, append([]driver.NamedValue(nil), args...)})
}

// transaction records whether a public operation commits or rolls back.
type transaction struct{ driver *Driver }

// Commit records successful completion.
func (tx transaction) Commit() error { tx.driver.Commits++; return nil }

// Rollback records cancellation or failure.
func (tx transaction) Rollback() error { tx.driver.Rollbacks++; return nil }

// Rows is a controlled result set with explicit column and row values.
type Rows struct {
	Names  []string
	Values [][]driver.Value
	index  int
}

// Columns returns the controlled result's column names.
func (r *Rows) Columns() []string { return r.Names }

// Close releases the in-memory result.
func (*Rows) Close() error { return nil }

// Next copies the next row and reports EOF when the result is exhausted.
func (r *Rows) Next(values []driver.Value) error {
	if r.index == len(r.Values) {
		return io.EOF
	}
	copy(values, r.Values[r.index])
	r.index++
	return nil
}

// Result supplies a stable inserted ID and affected-row count for controlled writes.
type Result struct{ ID, Affected int64 }

// LastInsertId returns the controlled inserted ID.
func (r Result) LastInsertId() (int64, error) { return r.ID, nil }

// RowsAffected returns the controlled affected count.
func (r Result) RowsAffected() (int64, error) { return r.Affected, nil }
