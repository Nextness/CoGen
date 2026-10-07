// fixture_integration_test.go provides the shared review lineage fixture used
// by the table-focused family tests and the legacy-adapter parity comparison.
//go:build integration

package review_test

import (
	"context"
	"path/filepath"
	"testing"

	"analysis/database"
	"analysis/database/run"

	_ "modernc.org/sqlite"
)

// newReviewLineageFixture opens one migrated database and builds completed A1
// and A2 runs with one overlapping stable work.
func newReviewLineageFixture(t *testing.T) (*database.Database, int64, int64, int64, int64) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "lineage.db"), reviewFilterConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a1Run, a2Run, _, a1Revision, a2Revision := createReviewLineageFixture(t, db)
	return db, a1Run, a2Run, a1Revision, a2Revision
}

// createReviewLineageFixture creates completed A1 and A2 runs with one overlapping stable work.
func createReviewLineageFixture(t *testing.T, db *database.Database) (int64, int64, int64, int64, int64) {
	t.Helper()
	exec := func(query string, args ...any) int64 {
		t.Helper()
		result, err := db.DB.Exec(query, args...)
		if err != nil {
			t.Fatalf("fixture query %q: %v", query, err)
		}
		id, _ := result.LastInsertId()
		return id
	}
	searchID := exec("INSERT INTO searches (search_id) VALUES ('review-search')")
	revisionID := exec("INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, 'r1', 'config', 'manifest')", searchID)
	planID := exec("INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash) VALUES (?, 'fingerprint', 'manifest', 'input')", revisionID)
	a1Run := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-01 00:00:00', '2026-01-01 00:01:00', 'completed', ?, 1)", planID)
	a2Run := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-02 00:00:00', '2026-01-02 00:01:00', 'completed', ?, 2)", planID)
	ctx := context.Background()
	if err := db.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: a1Run, Username: "Researcher", Email: "researcher@example.test"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: a2Run}); err != nil {
		t.Fatal(err)
	}
	workID := exec("INSERT INTO works (doi) VALUES ('10.1000/review')")
	makeRevision := func(runID int64, hash, title string) int64 {
		return exec(`INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, producer_stage)
			VALUES (?, ?, ?, ?, 'normalize')`, workID, runID, hash, title)
	}
	for _, runID := range []int64{a1Run, a2Run} {
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (?, ?, 'validate', 'valid')", runID, workID)
	}
	return a1Run, a2Run, workID, makeRevision(a1Run, "a1", "A1 article"), makeRevision(a2Run, "a2", "A2 article")
}
