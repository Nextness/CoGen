package server

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync/atomic"

	"modernc.org/sqlite"
)

const queryBudgetDriverName = "analysis-viewer-budgeted-sqlite"

var errQueryBudgetExceeded = errors.New("API query budget exceeded")

// queryBudgetKey isolates the mutable per-request statement counter in request contexts.
type queryBudgetKey struct{}

// queryBudget records a hard SQL-statement ceiling and the work consumed by one request.
type queryBudget struct {
	limit    int64
	used     atomic.Int64
	exceeded atomic.Bool
}

// init registers the viewer-owned budgeted SQLite driver once per process.
func init() {
	sql.Register(queryBudgetDriverName, &queryBudgetDriver{inner: &sqlite.Driver{}})
}

// withQueryBudget attaches one hard statement budget to a request context.
func withQueryBudget(ctx context.Context, limit int) (context.Context, *queryBudget) {
	budget := &queryBudget{limit: int64(limit)}
	return context.WithValue(ctx, queryBudgetKey{}, budget), budget
}

// consumeQuery records one SQL statement and rejects work beyond the hard request ceiling.
func consumeQuery(ctx context.Context) error {
	budget, _ := ctx.Value(queryBudgetKey{}).(*queryBudget)
	if budget == nil || budget.limit == 0 {
		return nil
	}
	if budget.used.Add(1) <= budget.limit {
		return nil
	}
	budget.exceeded.Store(true)
	return errQueryBudgetExceeded
}

// queryBudgetDriver wraps viewer-owned SQLite connections with request-context statement accounting.
type queryBudgetDriver struct{ inner driver.Driver }

// Open delegates connection creation and wraps the result with request-budget accounting.
func (d *queryBudgetDriver) Open(name string) (driver.Conn, error) {
	connection, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return &queryBudgetConn{Conn: connection}, nil
}

// queryBudgetConn preserves the wrapped driver's optional interfaces while counting SQL work.
type queryBudgetConn struct{ driver.Conn }

// Prepare preserves non-context statement preparation and wraps the resulting statement.
func (c *queryBudgetConn) Prepare(query string) (driver.Stmt, error) {
	statement, err := c.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &queryBudgetStmt{Stmt: statement}, nil
}

// PrepareContext preserves context-aware statement preparation and wraps the resulting statement.
func (c *queryBudgetConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	preparer, ok := c.Conn.(driver.ConnPrepareContext)
	if !ok {
		return c.Prepare(query)
	}
	statement, err := preparer.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &queryBudgetStmt{Stmt: statement}, nil
}

// QueryContext charges one SQL statement before delegating a context-aware query.
func (c *queryBudgetConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := consumeQuery(ctx); err != nil {
		return nil, err
	}
	queryer, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return queryer.QueryContext(ctx, query, args)
}

// ExecContext charges one SQL statement before delegating a context-aware execution.
func (c *queryBudgetConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := consumeQuery(ctx); err != nil {
		return nil, err
	}
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return execer.ExecContext(ctx, query, args)
}

// BeginTx preserves context-aware transaction creation on the wrapped connection.
func (c *queryBudgetConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	if beginner, ok := c.Conn.(driver.ConnBeginTx); ok {
		return beginner.BeginTx(ctx, options)
	}
	return c.Conn.Begin()
}

// Ping preserves the wrapped driver's context-aware health check when available.
func (c *queryBudgetConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

// CheckNamedValue preserves the wrapped driver's argument conversion when available.
func (c *queryBudgetConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

// ResetSession preserves the wrapped driver's pooled-connection reset when available.
func (c *queryBudgetConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

// IsValid preserves the wrapped driver's pooled-connection validity check when available.
func (c *queryBudgetConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

// queryBudgetStmt counts prepared statement execution through request contexts.
type queryBudgetStmt struct{ driver.Stmt }

// ExecContext charges one SQL statement before executing a prepared statement.
func (s *queryBudgetStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := consumeQuery(ctx); err != nil {
		return nil, err
	}
	statement, ok := s.Stmt.(driver.StmtExecContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return statement.ExecContext(ctx, args)
}

// QueryContext charges one SQL statement before querying through a prepared statement.
func (s *queryBudgetStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := consumeQuery(ctx); err != nil {
		return nil, err
	}
	statement, ok := s.Stmt.(driver.StmtQueryContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return statement.QueryContext(ctx, args)
}
