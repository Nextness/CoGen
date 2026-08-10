//go:build integration

package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"analysis/database"
)

// TestFailedAttemptRetainsReviewer verifies reviewer capture precedes later attempt evidence writes.
func TestFailedAttemptRetainsReviewer(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.csv")
	if err := os.WriteFile(source, []byte("doi,title\n10.1000/reviewer,Reviewer fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(filepath.Join(directory, "metadata.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.DB.Exec(`CREATE TRIGGER fail_run_metric BEFORE INSERT ON pipeline_run_metrics
		BEGIN SELECT RAISE(ABORT, 'forced metric failure'); END`); err != nil {
		t.Fatal(err)
	}
	run := testWorkspaceRun(source)
	run.Reviewer = Reviewer{Username: "Fixture Reviewer", Email: "reviewer@example.test"}
	if _, err := StartWorkspaceAttempt(db, []byte("reviewer fixture"), run, false); err == nil {
		t.Fatal("expected forced attempt failure")
	}
	var runID int64
	var status string
	if err := db.DB.QueryRow("SELECT id, status FROM pipeline_runs ORDER BY id DESC LIMIT 1").Scan(&runID, &status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("run status=%q", status)
	}
	reviewer, err := db.PipelineRunReviewers.Get(runID)
	if err != nil || reviewer == nil || reviewer.Username != "Fixture Reviewer" || reviewer.Email != "reviewer@example.test" {
		t.Fatalf("reviewer=%+v err=%v", reviewer, err)
	}
}
