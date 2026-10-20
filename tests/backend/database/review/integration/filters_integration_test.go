// filters_integration_test.go verifies the generated review query guards keep
// their scope, cursor, state, work-revision, and search filters and that
// note-to-note links still resolve through the selected context.

package review_test

import "analysis/tests/support/go/testfixture"

import "analysis/tests/support/go/testenv"

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"analysis/backend/database/run"
	"analysis/backend/database/search"
	"analysis/backend/database/work"

	_ "modernc.org/sqlite"
)

// reviewFilterConfigPath points at the production database registry from the family test directory.
var reviewFilterConfigPath = testenv.Path("config", "database.something")

// reviewFilterFixture is one migrated database with two completed runs, an
// inherited review context, and two logical notes.
type reviewFilterFixture struct {
	db                         *testfixture.Database
	run1, run2                 int64
	revision1, revision2       int64
	context1, context2         int64
	note1, note2               int64
	note1Version, note2Version int64
	workID                     int64
}

// newReviewFilterFixture builds the two-run review fixture through production migrations.
func newReviewFilterFixture(t *testing.T) *reviewFilterFixture {
	t.Helper()
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "filter.db"), reviewFilterConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	searchID, err := db.Search.FindOrCreateSearch(ctx, "filter-search")
	if err != nil {
		t.Fatal(err)
	}
	searchRevisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "r1", ConfigArtifactHash: "config", ResolvedManifestHash: "manifest",
	})
	if err != nil {
		t.Fatal(err)
	}
	planID, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: searchRevisionID, ExecutionFingerprint: "fingerprint", ResolvedManifestHash: "manifest", InputManifestHash: "input",
	})
	if err != nil {
		t.Fatal(err)
	}
	run1, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, run1, "completed", ""); err != nil {
		t.Fatal(err)
	}
	run2, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, run2, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if err := db.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: run1, Username: "Researcher", Email: "researcher@example.test"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: run2}); err != nil {
		t.Fatal(err)
	}
	workID, err := db.Work.CreateWorkByDOI(ctx, "10.1000/filter")
	if err != nil {
		t.Fatal(err)
	}
	revision1, err := db.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: run1, ProducerStage: "normalize", Title: "A1 article",
	})
	if err != nil {
		t.Fatal(err)
	}
	revision2, err := db.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: run2, ProducerStage: "normalize", Title: "A2 article",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, runID := range []int64{run1, run2} {
		if err := db.Work.SetStageOutcome(ctx, work.StageOutcomeInput{
			RunID: runID, WorkID: workID, StageName: "validate", Outcome: "valid",
		}); err != nil {
			t.Fatal(err)
		}
	}
	context1, created, err := db.Review.CreateContext(ctx, run1, nil)
	if err != nil || !created {
		t.Fatalf("create first context: context=%+v created=%v err=%v", context1, created, err)
	}
	note2, err := db.Review.CreateNote(ctx, context1.ID, revision1, "Second note body")
	if err != nil {
		t.Fatal(err)
	}
	note1, err := db.Review.CreateNote(ctx, context1.ID, revision1, "Alpha note [[note:"+strconv.FormatInt(note2.ID, 10)+"]]")
	if err != nil {
		t.Fatal(err)
	}
	context2, created, err := db.Review.CreateContext(ctx, run2, &context1.ID)
	if err != nil || !created {
		t.Fatalf("create second context: context=%+v created=%v err=%v", context2, created, err)
	}
	return &reviewFilterFixture{
		db: db, run1: run1, run2: run2, revision1: revision1, revision2: revision2,
		context1: context1.ID, context2: context2.ID, note1: note1.ID, note2: note2.ID,
		note1Version: note1.Version.ID, note2Version: note2.Version.ID, workID: workID,
	}
}

// TestReviewGeneratedParentCandidateFilters verifies scope, cursor, and search filters.
func TestReviewGeneratedParentCandidateFilters(t *testing.T) {
	fixture := newReviewFilterFixture(t)
	ctx := context.Background()
	proposed, err := fixture.db.Review.ProposeParent(ctx, fixture.run2)
	if err != nil || proposed == nil || proposed.ContextID != fixture.context1 {
		t.Fatalf("propose parent=%+v err=%v, want context %d", proposed, err, fixture.context1)
	}
	sameSearch, err := fixture.db.Review.ListParentCandidates(ctx, fixture.run2, "same_search", "", 0, 10, "")
	if err != nil || len(sameSearch) != 1 || sameSearch[0].ContextID != fixture.context1 {
		t.Fatalf("same-search candidates=%+v err=%v, want context %d", sameSearch, err, fixture.context1)
	}
	all, err := fixture.db.Review.ListParentCandidates(ctx, fixture.run2, "all", "", 0, 10, "")
	if err != nil || len(all) != 1 || all[0].ContextID != fixture.context1 {
		t.Fatalf("all-scope candidates=%+v err=%v, want context %d", all, err, fixture.context1)
	}
	searched, err := fixture.db.Review.ListParentCandidates(ctx, fixture.run2, "all", "", 0, 10, "filter-search")
	if err != nil || len(searched) != 1 || searched[0].ContextID != fixture.context1 {
		t.Fatalf("searched candidates=%+v err=%v, want context %d", searched, err, fixture.context1)
	}
	missing, err := fixture.db.Review.ListParentCandidates(ctx, fixture.run2, "all", "", 0, 10, "absent-search")
	if err != nil || len(missing) != 0 {
		t.Fatalf("absent-search candidates=%+v err=%v, want none", missing, err)
	}
	cursorStartedAt := sameSearch[0].StartedAt
	cursorRunID := sameSearch[0].PipelineRunID
	afterCursor, err := fixture.db.Review.ListParentCandidates(ctx, fixture.run2, "same_search", cursorStartedAt, cursorRunID, 10, "")
	if err != nil || len(afterCursor) != 0 {
		t.Fatalf("cursor candidates=%+v err=%v, want none", afterCursor, err)
	}
}

// TestReviewGeneratedNoteFilters verifies state, work-revision, search, cursor,
// link resolution, and backlink filters on inherited notes.
func TestReviewGeneratedNoteFilters(t *testing.T) {
	fixture := newReviewFilterFixture(t)
	ctx := context.Background()
	active, err := fixture.db.Review.ListNotesFiltered(ctx, fixture.context2, nil, 0, 10, "active", "")
	if err != nil || len(active) != 2 {
		t.Fatalf("active notes=%+v err=%v, want 2", active, err)
	}
	matched, err := fixture.db.Review.ListNotesFiltered(ctx, fixture.context2, nil, 0, 10, "active", "Alpha")
	if err != nil || len(matched) != 1 || matched[0].ID != fixture.note1 {
		t.Fatalf("searched notes=%+v err=%v, want note %d", matched, err, fixture.note1)
	}
	absent, err := fixture.db.Review.ListNotesFiltered(ctx, fixture.context2, nil, 0, 10, "active", "absent body")
	if err != nil || absent == nil || len(absent) != 0 {
		t.Fatalf("absent notes=%+v err=%v, want non-nil empty", absent, err)
	}
	onRevision, err := fixture.db.Review.ListNotesFiltered(ctx, fixture.context2, &fixture.revision2, 0, 10, "all", "")
	if err != nil || len(onRevision) != 2 {
		t.Fatalf("work-revision notes=%+v err=%v, want 2", onRevision, err)
	}
	otherRevision, err := fixture.db.Review.ListNotesFiltered(ctx, fixture.context2, &fixture.revision1, 0, 10, "all", "")
	if err != nil || len(otherRevision) != 0 {
		t.Fatalf("other-revision notes=%+v err=%v, want none", otherRevision, err)
	}
	cursor, err := fixture.db.Review.ListNotesFiltered(ctx, fixture.context2, nil, fixture.note1, 10, "all", "")
	if err != nil || len(cursor) != 1 || cursor[0].ID != fixture.note2 {
		t.Fatalf("cursor notes=%+v err=%v, want note %d", cursor, err, fixture.note2)
	}

	version, err := fixture.db.Review.GetNoteVersion(ctx, fixture.context2, fixture.note1, fixture.note1Version)
	if err != nil {
		t.Fatal(err)
	}
	if version == nil || len(version.Links) != 1 || !version.Links[0].Resolved || version.Links[0].NoteID == nil || *version.Links[0].NoteID != fixture.note2 {
		t.Fatalf("resolved note link=%+v", version)
	}
	backlinks, err := fixture.db.Review.ListBacklinks(ctx, fixture.context2, "note", strconv.FormatInt(fixture.note2, 10), 0, 0, 10)
	if err != nil || len(backlinks) != 1 || backlinks[0].ID != fixture.note1 {
		t.Fatalf("backlinks=%+v err=%v, want note %d", backlinks, err, fixture.note1)
	}
	scoped, err := fixture.db.Review.ListBacklinks(ctx, fixture.context2, "note", strconv.FormatInt(fixture.note2, 10), fixture.workID, 0, 10)
	if err != nil || len(scoped) != 1 {
		t.Fatalf("scoped backlinks=%+v err=%v, want one", scoped, err)
	}
	unscoped, err := fixture.db.Review.ListBacklinks(ctx, fixture.context2, "note", strconv.FormatInt(fixture.note2, 10), fixture.workID+999, 0, 10)
	if err != nil || unscoped == nil || len(unscoped) != 0 {
		t.Fatalf("unscoped backlinks=%+v err=%v, want non-nil empty slice", unscoped, err)
	}
	if empty, err := fixture.db.Review.ListBacklinks(ctx, 999, "pdf_page", "1", 0, 0, 10); err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("absent backlinks=%+v err=%v, want non-nil empty slice", empty, err)
	}

	if _, _, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context2, fixture.note2, fixture.note2Version+999, "deleted", ""); err == nil {
		t.Fatal("stale tombstone unexpectedly succeeded")
	}
	deleted, changed, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context2, fixture.note2, fixture.note2Version, "deleted", "")
	if err != nil || !changed || deleted.Version.State != "deleted" {
		t.Fatalf("delete note=%+v changed=%v err=%v", deleted, changed, err)
	}
	removed, err := fixture.db.Review.ListNotesFiltered(ctx, fixture.context2, nil, 0, 10, "removed", "")
	if err != nil || len(removed) != 1 || removed[0].ID != fixture.note2 {
		t.Fatalf("removed notes=%+v err=%v, want note %d", removed, err, fixture.note2)
	}
	remaining, err := fixture.db.Review.ListNotesFiltered(ctx, fixture.context2, nil, 0, 10, "active", "")
	if err != nil || len(remaining) != 1 || remaining[0].ID != fixture.note1 {
		t.Fatalf("remaining notes=%+v err=%v, want note %d", remaining, err, fixture.note1)
	}
	all, err := fixture.db.Review.ListNotesFiltered(ctx, fixture.context2, nil, 0, 10, "all", "")
	if err != nil || len(all) != 2 {
		t.Fatalf("all notes=%+v err=%v, want 2", all, err)
	}
}
