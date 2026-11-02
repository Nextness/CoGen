// export_integration_test.go verifies bounded comment-export evidence selection
// against temporary production-migrated review fixtures.

package review_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"analysis/backend/database/review"
	"analysis/tests/support/go/testfixture"
)

// exportTestHash is one stable lowercase SHA-256 fixture identity.
const exportTestHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// exportOtherHash is a different stable PDF source identity.
const exportOtherHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// createExportAnchor creates one active anchor with normalized geometry.
func createExportAnchor(t *testing.T, db *testfixture.Database, contextID, revisionID int64, label, hash string) *review.Anchor {
	t.Helper()
	anchor, err := db.Review.CreateAnchor(context.Background(), contextID, revisionID, label, hash, 1, "Selected "+label, []review.AnchorRectangle{{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.1}})
	if err != nil {
		t.Fatal(err)
	}
	return anchor
}

// exportWorkID reads the stable work identity behind one normalized revision.
func exportWorkID(t *testing.T, db *testfixture.Database, revisionID int64) int64 {
	t.Helper()
	var workID int64
	if err := db.Conn.QueryRow(`SELECT work_id FROM work_revisions WHERE id = ?`, revisionID).Scan(&workID); err != nil {
		t.Fatal(err)
	}
	return workID
}

// newExportFixture opens one completed review context with a valid work revision.
func newExportFixture(t *testing.T) (*testfixture.Database, int64, int64, int64) {
	t.Helper()
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	contextRecord, _, err := db.Review.CreateContext(context.Background(), a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	return db, contextRecord.ID, exportWorkID(t, db, a1Revision), a1Revision
}

// TestReviewExportSelectsMatchingActiveHeads verifies matching anchors and full note bodies are selected.
func TestReviewExportSelectsMatchingActiveHeads(t *testing.T) {
	db, contextID, workID, revisionID := newExportFixture(t)
	ctx := context.Background()
	anchor := createExportAnchor(t, db, contextID, revisionID, "methods-1", exportTestHash)
	body := "A complete saved body with **formatting** and a [[anchor:" + anchor.ID + "|methods]] reference."
	note, err := db.Review.CreateNote(ctx, contextID, revisionID, body)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := db.Review.ExportEvidence(ctx, contextID, workID, exportTestHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.Anchors) != 1 || evidence.Anchors[0].VersionID != anchor.Version.ID {
		t.Fatalf("anchors=%+v", evidence.Anchors)
	}
	if evidence.Anchors[0].SelectedText != "Selected methods-1" || len(evidence.Anchors[0].Rectangles) != 1 {
		t.Fatalf("anchor snapshot=%+v", evidence.Anchors[0])
	}
	if len(evidence.Notes) != 1 || evidence.Notes[0].VersionID != note.Version.ID || evidence.Notes[0].Body != body {
		t.Fatalf("notes=%+v", evidence.Notes)
	}
	if len(evidence.Notes[0].AnchorIDs) != 1 || evidence.Notes[0].AnchorIDs[0] != anchor.ID {
		t.Fatalf("note anchor ids=%+v", evidence.Notes[0].AnchorIDs)
	}
}

// TestReviewExportIncludesInheritedHeads verifies inherited active heads remain selectable.
func TestReviewExportIncludesInheritedHeads(t *testing.T) {
	db, a1Run, a2Run, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := db.Review.CreateAnchor(ctx, a1.ID, a1Revision, "methods-1", exportTestHash, 1, "Selected methods", []review.AnchorRectangle{{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.1}})
	if err != nil {
		t.Fatal(err)
	}
	note, err := db.Review.CreateNote(ctx, a1.ID, a1Revision, "Inherited [[anchor:"+anchor.ID+"]].")
	if err != nil {
		t.Fatal(err)
	}
	a2, _, err := db.Review.CreateContext(ctx, a2Run, &a1.ID)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := db.Review.ExportEvidence(ctx, a2.ID, exportWorkID(t, db, a1Revision), exportTestHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.Anchors) != 1 || evidence.Anchors[0].VersionID != anchor.Version.ID {
		t.Fatalf("inherited anchors=%+v", evidence.Anchors)
	}
	if len(evidence.Notes) != 1 || evidence.Notes[0].VersionID != note.Version.ID {
		t.Fatalf("inherited notes=%+v", evidence.Notes)
	}
}

// TestReviewExportExcludesIneligibleEvidence verifies tombstones, page-only,
// general, and mixed-reference notes are excluded with reported reasons.
func TestReviewExportExcludesIneligibleEvidence(t *testing.T) {
	db, contextID, workID, revisionID := newExportFixture(t)
	ctx := context.Background()
	matching := createExportAnchor(t, db, contextID, revisionID, "matching", exportTestHash)
	other := createExportAnchor(t, db, contextID, revisionID, "other", exportOtherHash)
	if _, err := db.Review.CreateNote(ctx, contextID, revisionID, "Page only [[pdf:page=1]]."); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Review.CreateNote(ctx, contextID, revisionID, "General article note without a PDF reference."); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Review.CreateNote(ctx, contextID, revisionID, "Mixed [[anchor:"+matching.ID+"]] and [[anchor:"+other.ID+"]]."); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Review.CreateNote(ctx, contextID, revisionID, "Mismatched [[anchor:"+other.ID+"]]."); err != nil {
		t.Fatal(err)
	}
	deleted, err := db.Review.CreateNote(ctx, contextID, revisionID, "Deleted [[anchor:"+matching.ID+"]].")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Review.AppendNoteVersion(ctx, contextID, deleted.ID, deleted.Version.ID, "deleted", ""); err != nil {
		t.Fatal(err)
	}
	evidence, err := db.Review.ExportEvidence(ctx, contextID, workID, exportTestHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.Anchors) != 1 || evidence.Anchors[0].ID != matching.ID {
		t.Fatalf("matching anchors=%+v", evidence.Anchors)
	}
	if len(evidence.Notes) != 0 {
		t.Fatalf("ineligible notes selected: %+v", evidence.Notes)
	}
	if evidence.ExcludedAnchors != 1 || evidence.ExcludedNotes != 4 {
		t.Fatalf("excluded anchors=%d notes=%d", evidence.ExcludedAnchors, evidence.ExcludedNotes)
	}
	reasons := make(map[string]int)
	for _, exclusion := range evidence.Exclusions {
		reasons[exclusion.Reason] = exclusion.Count
	}
	if reasons["page_only_reference"] != 1 || reasons["unresolved_or_mismatched_anchor"] != 2 || reasons["no_pdf_reference"] != 1 || reasons["pdf_hash_or_geometry_mismatch"] != 1 {
		t.Fatalf("exclusion reasons=%+v", reasons)
	}
}

// TestReviewExportRejectsMissingContextAndOverflow verifies explicit empty and limit behavior.
func TestReviewExportRejectsMissingContextAndOverflow(t *testing.T) {
	db, contextID, workID, revisionID := newExportFixture(t)
	ctx := context.Background()
	if _, err := db.Review.ExportEvidence(ctx, 999999, workID, exportTestHash); !errors.Is(err, review.ErrExportContextNotFound) {
		t.Fatalf("missing context error=%v", err)
	}
	for index := 0; index <= review.MaxExportAnchors; index++ {
		createExportAnchor(t, db, contextID, revisionID, "overflow-"+strings.Repeat("x", index%5)+strings.Repeat("y", index/5), exportTestHash)
	}
	if _, err := db.Review.ExportEvidence(ctx, contextID, workID, exportTestHash); !errors.Is(err, review.ErrExportLimit) {
		t.Fatalf("overflow error=%v", err)
	} else {
		var limit *review.ExportLimitError
		if !errors.As(err, &limit) || limit.Kind != "annotation" {
			t.Fatalf("overflow detail=%+v", limit)
		}
	}
}

// TestReviewExportRejectsOversizedText verifies full prose is never silently truncated.
func TestReviewExportRejectsOversizedText(t *testing.T) {
	db, contextID, workID, revisionID := newExportFixture(t)
	ctx := context.Background()
	anchor := createExportAnchor(t, db, contextID, revisionID, "large", exportTestHash)
	body := strings.Repeat("x", 260000)
	for index := 0; index < 8; index++ {
		if _, err := db.Review.CreateNote(ctx, contextID, revisionID, body+" [[anchor:"+anchor.ID+"]]."); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Review.ExportEvidence(ctx, contextID, workID, exportTestHash); !errors.Is(err, review.ErrExportLimit) {
		t.Fatalf("oversized text error=%v", err)
	}
}

// TestReviewExportSnapshotKeepsOneHeadSet verifies concurrent note edits cannot
// mix evidence inside one export snapshot.
func TestReviewExportSnapshotKeepsOneHeadSet(t *testing.T) {
	db, contextID, workID, revisionID := newExportFixture(t)
	ctx := context.Background()
	anchor := createExportAnchor(t, db, contextID, revisionID, "snapshot", exportTestHash)
	note, err := db.Review.CreateNote(ctx, contextID, revisionID, "First body [[anchor:"+anchor.ID+"]].")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := db.Review.BeginExportSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	first, err := snapshot.Evidence(ctx, contextID, workID, exportTestHash)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Review.AppendNoteVersion(ctx, contextID, note.ID, note.Version.ID, "active", "Second body [[anchor:"+anchor.ID+"]]."); err != nil {
		t.Fatal(err)
	}
	second, err := snapshot.Evidence(ctx, contextID, workID, exportTestHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Notes) != 1 || len(second.Notes) != 1 || second.Notes[0].Body != first.Notes[0].Body {
		t.Fatalf("snapshot mixed heads: first=%+v second=%+v", first.Notes, second.Notes)
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	fresh, err := db.Review.ExportEvidence(ctx, contextID, workID, exportTestHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Notes) != 1 || !strings.Contains(fresh.Notes[0].Body, "Second body") {
		t.Fatalf("fresh evidence=%+v", fresh.Notes)
	}
}
