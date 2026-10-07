// concurrency_integration_test.go verifies competing review saves produce
// exactly one success and one optimistic conflict for decisions, notes, and
// anchors.
//go:build integration

package review_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"analysis/database/review"
)

// TestReviewConcurrentNoteWriters verifies one optimistic head wins when two writers start from the same version.
func TestReviewConcurrentNoteWriters(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	contextRecord, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	note, err := db.Review.CreateNote(ctx, contextRecord.ID, a1Revision, "Initial concurrent body")
	if err != nil {
		t.Fatal(err)
	}
	results := runConcurrentReviewWriters(t, []func() error{
		func() error {
			_, _, err := db.Review.AppendNoteVersion(context.Background(), contextRecord.ID, note.ID, note.Version.ID, "active", "Writer one")
			return err
		},
		func() error {
			_, _, err := db.Review.AppendNoteVersion(context.Background(), contextRecord.ID, note.ID, note.Version.ID, "active", "Writer two")
			return err
		},
	})
	assertOneSuccessOneConflict(t, results)
	versions, err := db.Review.ListNoteVersions(ctx, contextRecord.ID, note.ID, 0, 10)
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions=%d err=%v, want 2", len(versions), err)
	}
}

// TestReviewConcurrentDecisionWriters verifies one optimistic head wins when two decision writers start from the same version.
func TestReviewConcurrentDecisionWriters(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	contextRecord, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := runConcurrentReviewWriters(t, []func() error{
		func() error {
			_, _, err := db.Review.AppendWorkReview(context.Background(), contextRecord.ID, a1Revision, nil, "approved", nil, nil)
			return err
		},
		func() error {
			_, _, err := db.Review.AppendWorkReview(context.Background(), contextRecord.ID, a1Revision, nil, "removed", []string{"duplicate"}, nil)
			return err
		},
	})
	assertOneSuccessOneConflict(t, results)
	if versions := reviewCount(t, db, "SELECT COUNT(*) FROM work_review_versions WHERE work_revision_id=?", a1Revision); versions != 1 {
		t.Fatalf("versions=%d, want 1", versions)
	}
}

// TestReviewConcurrentAnchorWriters verifies one optimistic head wins when two anchor writers start from the same version.
func TestReviewConcurrentAnchorWriters(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	contextRecord, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	anchor, err := db.Review.CreateAnchor(ctx, contextRecord.ID, a1Revision, "concurrent-anchor", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	results := runConcurrentReviewWriters(t, []func() error{
		func() error {
			_, _, err := db.Review.AppendAnchorVersion(context.Background(), contextRecord.ID, anchor.ID, anchor.Version.ID, "active", hash, 2, "Writer one", []review.AnchorRectangle{{X: .2, Y: .2, Width: .3, Height: .1}})
			return err
		},
		func() error {
			_, _, err := db.Review.AppendAnchorVersion(context.Background(), contextRecord.ID, anchor.ID, anchor.Version.ID, "active", hash, 3, "Writer two", []review.AnchorRectangle{{X: .3, Y: .2, Width: .3, Height: .1}})
			return err
		},
	})
	assertOneSuccessOneConflict(t, results)
	if versions := reviewCount(t, db, "SELECT COUNT(*) FROM review_anchor_versions WHERE anchor_id=?", anchor.ID); versions != 2 {
		t.Fatalf("versions=%d, want 2", versions)
	}
}

// runConcurrentReviewWriters starts every writer together and returns their errors.
func runConcurrentReviewWriters(t *testing.T, writers []func() error) []error {
	t.Helper()
	start := make(chan struct{})
	results := make(chan error, len(writers))
	var ready sync.WaitGroup
	ready.Add(len(writers))
	for _, writer := range writers {
		writer := writer
		go func() {
			ready.Done()
			<-start
			results <- writer()
		}()
	}
	ready.Wait()
	close(start)
	collected := make([]error, 0, len(writers))
	for range writers {
		collected = append(collected, <-results)
	}
	return collected
}

// assertOneSuccessOneConflict verifies competing writers produce one success and one optimistic conflict.
func assertOneSuccessOneConflict(t *testing.T, results []error) {
	t.Helper()
	var successes, conflicts int
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case review.IsConflict(err):
			conflicts++
		default:
			t.Fatalf("competing writer error=%v, want success or conflict", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("competing results successes=%d conflicts=%d, want one each", successes, conflicts)
	}
}
