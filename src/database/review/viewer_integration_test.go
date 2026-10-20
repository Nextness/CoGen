// viewer_integration_test.go exercises the review family's viewer-facing
// anchor guard and summary-count reads against a database created by the
// production migration runner.
//go:build integration

package review_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/review"
)

// viewerAnchorHash is a structurally valid lowercase SHA-256 anchor content hash.
var viewerAnchorHash = strings.Repeat("a", 64)

// TestStoreAnchorRestoreReadsPreserveContextAndGeometry verifies the anchor
// work guard and the active-version geometry projection.
func TestStoreAnchorRestoreReadsPreserveContextAndGeometry(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	store := db.Review
	contextRecord, _, err := store.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := store.CreateAnchor(ctx, contextRecord.ID, a1Revision, "methods-1", viewerAnchorHash, 2, "Selected methods text", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}

	workID, err := store.AnchorWorkID(ctx, contextRecord.ID, anchor.ID)
	if err != nil || workID == nil {
		t.Fatalf("anchor work id = %v err=%v", workID, err)
	}
	if *workID != anchor.WorkID {
		t.Fatalf("anchor work id = %d, want %d", *workID, anchor.WorkID)
	}
	if workID, err := store.AnchorWorkID(ctx, contextRecord.ID, "missing-anchor"); err != nil || workID != nil {
		t.Fatalf("missing anchor work id = %v err=%v, want nil", workID, err)
	}

	restored, err := store.ActiveAnchorVersion(ctx, anchor.ID, anchor.Version.ID)
	if err != nil || restored == nil {
		t.Fatalf("active anchor version = %+v err=%v", restored, err)
	}
	if restored.PDFContentHash != viewerAnchorHash || restored.Page != 2 || restored.SelectedText != "Selected methods text" {
		t.Fatalf("active anchor version = %+v", restored)
	}
	if restored.RectanglesJSON == "" {
		t.Fatal("active anchor version lost its geometry")
	}
	if restored, err := store.ActiveAnchorVersion(ctx, anchor.ID, 999); err != nil || restored != nil {
		t.Fatalf("missing anchor version = %+v err=%v, want nil", restored, err)
	}
}

// TestStoreSummaryCountsCountActiveHeadsAndVersionAncestry verifies the
// current note, anchor, and decision-version counts for one work.
func TestStoreSummaryCountsCountActiveHeadsAndVersionAncestry(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	store := db.Review
	contextRecord, _, err := store.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := store.CreateAnchor(ctx, contextRecord.ID, a1Revision, "methods-1", viewerAnchorHash, 1, "text", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateNote(ctx, contextRecord.ID, a1Revision, "A review note."); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.AppendWorkReview(ctx, contextRecord.ID, a1Revision, nil, "approved", nil, nil); err != nil {
		t.Fatal(err)
	}

	counts, err := store.SummaryCounts(ctx, contextRecord.ID, anchor.WorkID)
	if err != nil || counts == nil {
		t.Fatalf("summary counts = %+v err=%v", counts, err)
	}
	if counts.NoteCount != 1 || counts.AnchorCount != 1 || counts.ReviewVersionCount != 1 {
		t.Fatalf("summary counts = %+v", counts)
	}

	empty, err := store.SummaryCounts(ctx, contextRecord.ID, anchor.WorkID+1)
	if err != nil || empty == nil || empty.NoteCount != 0 || empty.AnchorCount != 0 || empty.ReviewVersionCount != 0 {
		t.Fatalf("empty summary counts = %+v err=%v", empty, err)
	}
}
