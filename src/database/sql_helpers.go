// sql_helpers.go provides shared nullable-value, timestamp, and transaction
// helpers used consistently across all database repositories.
package database

import (
	"context"
	"database/sql"
	"time"
)

// nullStr represents optional text consistently across workspace repositories.
func nullStr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// nullInt represents optional integer values consistently across workspace repositories.
func nullInt(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

// timestamp returns the current UTC time in the repository's persisted format.
func timestamp() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

// withTx runs fn inside a transaction, rolling back on error and committing on success.
func (d *Database) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		// Rollback on panic; if fn returned an error the tx is already rolled back.
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// scannable defines the behavior required of scannable implementations.
type scannable interface {
	Scan(dest ...any) error
}

// nullStrPtrVal returns a nullable SQL string's value or an empty string.
func nullStrPtrVal(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
