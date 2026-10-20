// artifact_unit_test.go verifies the artifact family's transaction helper
// without a migrated database.
//go:build unit

package artifact

import (
	"context"
	"database/sql"
	"testing"

	generated "analysis/database/artifact/internal/sql"

	_ "modernc.org/sqlite"
)

// TestWithTxRollsBackAndRepanics verifies a panic inside the transaction body
// rolls back the transaction and propagates the panic.
func TestWithTxRollsBackAndRepanics(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	store := New(conn)

	defer func() {
		if recovered := recover(); recovered != "boom" {
			t.Fatalf("recovered = %v, want boom", recovered)
		}
	}()
	_ = store.withTx(context.Background(), func(*generated.Queries) error {
		panic("boom")
	})
}
