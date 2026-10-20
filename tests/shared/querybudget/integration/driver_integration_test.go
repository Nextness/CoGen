// driver_integration_test.go verifies statement budgets through database/sql and optional driver interfaces.
package querybudget_test

import (
	"analysis/internal/querybudget"
	"database/sql"
	"database/sql/driver"
	"errors"
	"modernc.org/sqlite"
	"testing"
)

// init registers an isolated test driver using the same production wrapper as the viewer.
func init() { sql.Register("querybudget-test-sqlite", querybudget.WrapDriver(&sqlite.Driver{})) }

// budgetedConnection describes only the standard interfaces promised by the production wrapper.
type budgetedConnection interface {
	driver.Conn
	driver.ConnPrepareContext
	driver.ExecerContext
	driver.ConnBeginTx
	driver.Pinger
	driver.Validator
	driver.SessionResetter
	driver.NamedValueChecker
}

// TestSQLStatementBudgetRejectsExcessWork verifies each attempted statement is charged and excess work is rejected.
func TestSQLStatementBudgetRejectsExcessWork(t *testing.T) {
	db, err := sql.Open("querybudget-test-sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, state := querybudget.With(t.Context(), 2)
	if _, err := db.ExecContext(ctx, "CREATE TABLE evidence (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO evidence DEFAULT VALUES"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM evidence").Scan(&count); !errors.Is(err, querybudget.ErrExceeded) {
		t.Fatalf("third query error=%v, want query budget error", err)
	}
	if used := state.Used(); used != 3 || !state.Exceeded() {
		t.Fatalf("query budget used=%d exceeded=%v", used, state.Exceeded())
	}

}

// TestQueryBudgetDriverInterfaces verifies statement, transaction, and optional driver paths.
func TestQueryBudgetDriverInterfaces(t *testing.T) {
	wrapped := querybudget.WrapDriver(&sqlite.Driver{})
	connection, err := wrapped.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	budgeted := connection.(budgetedConnection)
	ctx, state := querybudget.With(t.Context(), 8)
	if err := budgeted.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if !budgeted.IsValid() {
		t.Fatal("new SQLite connection is not valid")
	}
	if err := budgeted.ResetSession(ctx); err != nil {
		t.Fatal(err)
	}
	value := &driver.NamedValue{Ordinal: 1, Value: int64(1)}
	if err := budgeted.CheckNamedValue(value); err != nil && !errors.Is(err, driver.ErrSkip) {
		t.Fatal(err)
	}
	if _, err := budgeted.ExecContext(ctx, "CREATE TABLE evidence (id INTEGER PRIMARY KEY, value TEXT)", nil); err != nil {
		t.Fatal(err)
	}
	insert, err := budgeted.PrepareContext(ctx, "INSERT INTO evidence (value) VALUES (?)")
	if err != nil {
		t.Fatal(err)
	}
	defer insert.Close()
	if _, err := insert.(driver.StmtExecContext).ExecContext(ctx, []driver.NamedValue{{Ordinal: 1, Value: "prepared"}}); err != nil {
		t.Fatal(err)
	}
	query, err := budgeted.Prepare("SELECT value FROM evidence ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer query.Close()
	rows, err := query.(driver.StmtQueryContext).QueryContext(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	transaction, err := budgeted.BeginTx(ctx, driver.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatal(err)
	}
	if used := state.Used(); used != 3 {
		t.Fatalf("query budget used=%d, want 3", used)
	}
}

// TestBudgetDriverPreservesPrepareFailures verifies malformed SQL is rejected by the underlying driver through both standard preparation paths.
func TestBudgetDriverPreservesPrepareFailures(t *testing.T) {
	connection, err := querybudget.WrapDriver(&sqlite.Driver{}).Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	for name, prepare := range map[string]func() (driver.Stmt, error){
		"legacy": func() (driver.Stmt, error) { return connection.Prepare("SELECT FROM") },
		"context": func() (driver.Stmt, error) {
			return connection.(driver.ConnPrepareContext).PrepareContext(t.Context(), "SELECT FROM")
		},
	} {
		t.Run(name, func(t *testing.T) {
			statement, err := prepare()
			if err == nil {
				_ = statement.Close()
				t.Fatal("malformed SQL preparation succeeded")
			}
		})
	}
}
