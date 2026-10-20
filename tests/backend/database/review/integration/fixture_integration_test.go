// fixture_integration_test.go provides the shared review lineage fixture used
// by the table-focused family tests.

package review_test

import "analysis/tests/support/go/testfixture"

import (
	"context"
	"path/filepath"
	"testing"

	"analysis/backend/database/run"
	"analysis/backend/database/search"
	"analysis/backend/database/work"

	_ "modernc.org/sqlite"
)

// newReviewLineageFixture opens one migrated database and builds completed A1
// and A2 runs with one overlapping stable work.
func newReviewLineageFixture(t *testing.T) (*testfixture.Database, int64, int64, int64, int64) {
	t.Helper()
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "lineage.db"), reviewFilterConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a1Run, a2Run, _, a1Revision, a2Revision := createReviewLineageFixture(t, db)
	return db, a1Run, a2Run, a1Revision, a2Revision
}

// createReviewLineageFixture creates completed A1 and A2 runs with one overlapping stable work.
func createReviewLineageFixture(t *testing.T, db *testfixture.Database) (int64, int64, int64, int64, int64) {
	t.Helper()
	ctx := context.Background()
	searchID, err := db.Search.FindOrCreateSearch(ctx, "review-search")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "r1", ConfigArtifactHash: "config", ResolvedManifestHash: "manifest",
	})
	if err != nil {
		t.Fatal(err)
	}
	planID, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fingerprint", ResolvedManifestHash: "manifest", InputManifestHash: "input",
	})
	if err != nil {
		t.Fatal(err)
	}
	a1Run, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, a1Run, "completed", ""); err != nil {
		t.Fatal(err)
	}
	a2Run, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, a2Run, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if err := db.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: a1Run, Username: "Researcher", Email: "researcher@example.test"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: a2Run}); err != nil {
		t.Fatal(err)
	}
	workID, err := db.Work.CreateWorkByDOI(ctx, "10.1000/review")
	if err != nil {
		t.Fatal(err)
	}
	makeRevision := func(runID int64, title string) int64 {
		revisionID, err := db.Work.CreateRevision(ctx, &work.Revision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: "normalize", Title: title,
		})
		if err != nil {
			t.Fatal(err)
		}
		return revisionID
	}
	for _, runID := range []int64{a1Run, a2Run} {
		if err := db.Work.SetStageOutcome(ctx, work.StageOutcomeInput{
			RunID: runID, WorkID: workID, StageName: "validate", Outcome: "valid",
		}); err != nil {
			t.Fatal(err)
		}
	}
	return a1Run, a2Run, workID, makeRevision(a1Run, "A1 article"), makeRevision(a2Run, "A2 article")
}
