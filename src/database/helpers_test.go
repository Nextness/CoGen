// Shared helpers for database test files.
package database

import (
	"path/filepath"
	"testing"
)

// configPath and migrationsDir are resolved relative to src/ (where tests run).
var (
	testConfigPath    = filepath.Join("..", "..", "config", "database.something")
	testMigrationsDir = filepath.Join("..", "..", "migrations")
)

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
	workID, err := db.Works.CreateByDOI(doi)
	if err != nil {
		t.Fatalf("create work: %v", err)
	}
	runID, err := db.PipelineRuns.StartRun("reference mention test", "q")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	revisionID, err := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse, Title: "Reference source",
	})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	return revisionID
}
