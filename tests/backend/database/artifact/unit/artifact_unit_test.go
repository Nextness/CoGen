// artifact_unit_test.go verifies transaction panic handling through the public atomic write.
package artifact_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync/atomic"
	"testing"

	"analysis/backend/database/artifact"
)

// panicDriverSequence makes repeated test invocations use distinct registered drivers.
var panicDriverSequence atomic.Uint64

// panicDriver supplies a controlled storage failure inside an artifact transaction.
type panicDriver struct{ rolledBack *bool }

// Open returns the controlled connection without touching a real database.
func (d panicDriver) Open(string) (driver.Conn, error) { return panicConnection{d.rolledBack}, nil }

// panicConnection panics when the first statement is prepared inside its transaction.
type panicConnection struct{ rolledBack *bool }

// Prepare injects the failure after the transaction has begun.
func (panicConnection) Prepare(string) (driver.Stmt, error) { panic("boom") }

// Close releases the controlled connection.
func (panicConnection) Close() error { return nil }

// Begin records a transaction whose rollback can be observed externally.
func (c panicConnection) Begin() (driver.Tx, error) { return panicTransaction{c.rolledBack}, nil }

// panicTransaction exposes the outcome of the public operation's transaction.
type panicTransaction struct{ rolledBack *bool }

// Commit completes a transaction without the injected failure.
func (panicTransaction) Commit() error { return nil }

// Rollback records that the failing operation undid its transaction.
func (tx panicTransaction) Rollback() error { *tx.rolledBack = true; return nil }

// TestWithTxRollsBackAndRepanics verifies a storage panic rolls back and propagates through CreateWithBlob.
func TestWithTxRollsBackAndRepanics(t *testing.T) {
	rolledBack := false
	name := fmt.Sprintf("artifact-panic-%d", panicDriverSequence.Add(1))
	sql.Register(name, panicDriver{&rolledBack})
	conn, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	defer func() {
		if recovered := recover(); recovered != "boom" {
			t.Fatalf("recovered = %v, want boom", recovered)
		}
		if !rolledBack {
			t.Fatal("artifact write propagated a panic without rolling back")
		}
	}()
	_, _ = artifact.New(conn).CreateWithBlob(context.Background(), artifact.CreateWithBlobInput{ContentHash: "panic", ContentType: "text/plain"})
}
