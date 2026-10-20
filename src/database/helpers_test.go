// Shared helpers for database test files.
package database

import (
	"context"
	"path/filepath"
	"testing"

	"analysis/database/work"
)

// configPath is resolved relative to src/ (where tests run).
var testConfigPath = filepath.Join("..", "..", "config", "database.something")

// openTestDB supports the package test suite's open test db setup or assertions.
func openTestDB(t *testing.T) *Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath, testConfigPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db
}

// createReferenceMentionTestRevision supports the package test suite's create reference mention test revision setup or assertions.
func createReferenceMentionTestRevision(t *testing.T, db *Database, doi string) int64 {
	t.Helper()
	ctx := context.Background()
	workID, err := db.Work.CreateWorkByDOI(ctx, doi)
	if err != nil {
		t.Fatalf("create work: %v", err)
	}
	runID, err := db.Run.StartRun(ctx, "reference mention test", "q")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	revisionID, err := db.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageParse, Title: "Reference source",
	})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	return revisionID
}
