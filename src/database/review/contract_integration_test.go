// contract_integration_test.go pins the review family behavior contract:
// typed error identity, context eligibility and lineage, inherited head
// stability, reviewer privacy, bounded notes and links, anchor binding and
// geometry validation, and tombstone and restoration history.
//go:build integration

package review_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"analysis/database/review"
	"analysis/notes"
)

// reviewInsertRun inserts one review run into the fixture plan with the given lifecycle fields.
func reviewInsertRun(t *testing.T, fixture *reviewMutationFixture, status, startedAt string, attempt int) int64 {
	t.Helper()
	var planID int64
	if err := fixture.db.DB.QueryRow("SELECT execution_plan_id FROM pipeline_runs WHERE id=?", fixture.run1).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.db.DB.Exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', ?, ?, ?, ?, ?)", startedAt, startedAt, status, planID, attempt)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// reviewErrorKind returns the typed review error kind for one error.
func reviewErrorKind(t *testing.T, err error) string {
	t.Helper()
	var repositoryError *review.Error
	if !errors.As(err, &repositoryError) {
		t.Fatalf("error %v is not a typed review error", err)
	}
	return repositoryError.Kind
}

// TestReviewTypedErrorContract verifies conflict, lifecycle, not-found, and
// cancellation errors stay identifiable through wrapping.
func TestReviewTypedErrorContract(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	expected := int64(42)
	_, _, err := fixture.db.Review.AppendWorkReview(ctx, fixture.context1, fixture.revision1, &expected, "approved", nil, nil)
	var conflict *review.ConflictError
	if !review.IsConflict(err) || !errors.As(err, &conflict) {
		t.Fatalf("stale decision error=%v, want conflict", err)
	}
	if conflict.Expected == nil || *conflict.Expected != expected || conflict.Current != nil {
		t.Fatalf("conflict details=%+v", conflict)
	}
	if !review.IsConflict(fmt.Errorf("wrapped: %w", err)) {
		t.Fatal("wrapped conflict is no longer identifiable")
	}

	if err := fixture.db.Run.Trash(ctx, fixture.run1, "contract"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.db.Review.AppendWorkReview(ctx, fixture.context1, fixture.revision1, nil, "approved", nil, nil); err == nil {
		t.Fatal("read-only decision unexpectedly succeeded")
	} else if kind := reviewErrorKind(t, fmt.Errorf("wrapped: %w", err)); kind != "lifecycle" {
		t.Fatalf("read-only decision kind=%q, want lifecycle", kind)
	}
	if _, err := fixture.db.Review.GetWorkReview(ctx, fixture.context1, fixture.revision1+999); err == nil {
		t.Fatal("missing work revision unexpectedly resolved")
	} else if kind := reviewErrorKind(t, err); kind != "not_found" {
		t.Fatalf("missing work revision kind=%q, want not_found", kind)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fixture.db.Review.GetWorkReview(canceled, fixture.context1, fixture.revision1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read error=%v, want context.Canceled", err)
	}
}

// TestReviewContextEligibilityContract verifies contexts require completed,
// non-trashed runs and earlier eligible parents.
func TestReviewContextEligibilityContract(t *testing.T) {
	ctx := context.Background()
	t.Run("missing parent", func(t *testing.T) {
		fixture := newReviewMutationFixture(t)
		missing := int64(9999)
		if _, _, err := fixture.db.Review.CreateContext(ctx, fixture.run2, &missing); err == nil {
			t.Fatal("missing parent unexpectedly resolved")
		} else if kind := reviewErrorKind(t, err); kind != "not_found" {
			t.Fatalf("missing parent kind=%q, want not_found", kind)
		}
	})
	t.Run("running target", func(t *testing.T) {
		fixture := newReviewMutationFixture(t)
		running := reviewInsertRun(t, fixture, "running", "2026-01-03 00:00:00", 3)
		if _, _, err := fixture.db.Review.CreateContext(ctx, running, nil); err == nil {
			t.Fatal("running target unexpectedly initialized")
		} else if kind := reviewErrorKind(t, err); kind != "lifecycle" {
			t.Fatalf("running target kind=%q, want lifecycle", kind)
		}
	})
	t.Run("trashed target", func(t *testing.T) {
		fixture := newReviewMutationFixture(t)
		if err := fixture.db.Run.Trash(ctx, fixture.run2, "contract"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := fixture.db.Review.CreateContext(ctx, fixture.run2, nil); err == nil {
			t.Fatal("trashed target unexpectedly initialized")
		} else if kind := reviewErrorKind(t, err); kind != "lifecycle" {
			t.Fatalf("trashed target kind=%q, want lifecycle", kind)
		}
	})
	t.Run("trashed parent", func(t *testing.T) {
		fixture := newReviewMutationFixture(t)
		if err := fixture.db.Run.Trash(ctx, fixture.run1, "contract"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := fixture.db.Review.CreateContext(ctx, fixture.run2, &fixture.context1); err == nil {
			t.Fatal("trashed parent unexpectedly accepted")
		} else if kind := reviewErrorKind(t, err); kind != "lifecycle" {
			t.Fatalf("trashed parent kind=%q, want lifecycle", kind)
		}
	})
	t.Run("non-completed parent", func(t *testing.T) {
		fixture := newReviewMutationFixture(t)
		if _, err := fixture.db.DB.Exec("UPDATE pipeline_runs SET status='running' WHERE id=?", fixture.run1); err != nil {
			t.Fatal(err)
		}
		if _, _, err := fixture.db.Review.CreateContext(ctx, fixture.run2, &fixture.context1); err == nil {
			t.Fatal("running parent unexpectedly accepted")
		} else if kind := reviewErrorKind(t, err); kind != "lifecycle" {
			t.Fatalf("running parent kind=%q, want lifecycle", kind)
		}
	})
	t.Run("later parent", func(t *testing.T) {
		fixture := newReviewMutationFixture(t)
		context2, created, err := fixture.db.Review.CreateContext(ctx, fixture.run2, &fixture.context1)
		if err != nil || !created {
			t.Fatalf("create context2: context=%+v created=%v err=%v", context2, created, err)
		}
		earlier := reviewInsertRun(t, fixture, "completed", "2026-01-01 12:00:00", 3)
		if _, _, err := fixture.db.Review.CreateContext(ctx, earlier, &context2.ID); err == nil {
			t.Fatal("later parent unexpectedly accepted")
		} else if kind := reviewErrorKind(t, err); kind != "validation" {
			t.Fatalf("later parent kind=%q, want validation", kind)
		}
	})
	t.Run("parent conflict", func(t *testing.T) {
		fixture := newReviewMutationFixture(t)
		if _, created, err := fixture.db.Review.CreateContext(ctx, fixture.run2, &fixture.context1); err != nil || !created {
			t.Fatalf("create context2: created=%v err=%v", created, err)
		}
		if _, _, err := fixture.db.Review.CreateContext(ctx, fixture.run2, nil); err == nil {
			t.Fatal("conflicting parent unexpectedly accepted")
		} else {
			var parentConflict *review.ContextParentConflictError
			if !errors.As(err, &parentConflict) || parentConflict.Existing == nil || *parentConflict.Existing != fixture.context1 || parentConflict.Requested != nil {
				t.Fatalf("parent conflict=%+v err=%v", parentConflict, err)
			}
		}
	})
}

// TestReviewInheritedHeadStabilityContract verifies child contexts freeze
// parent heads and neither side rewrites the other's selected head.
func TestReviewInheritedHeadStabilityContract(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	note, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "Shared note")
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "methods-1", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	decision, _, err := fixture.db.Review.AppendWorkReview(ctx, fixture.context1, fixture.revision1, nil, "approved", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	context2, created, err := fixture.db.Review.CreateContext(ctx, fixture.run2, &fixture.context1)
	if err != nil || !created {
		t.Fatalf("create child context: context=%+v created=%v err=%v", context2, created, err)
	}

	inheritedDecision, err := fixture.db.Review.GetWorkReview(ctx, context2.ID, fixture.revision2)
	if err != nil || inheritedDecision.Version == nil || inheritedDecision.Version.ID != decision.Version.ID || inheritedDecision.InheritedFromContextID == nil || *inheritedDecision.InheritedFromContextID != fixture.context1 {
		t.Fatalf("inherited decision=%+v err=%v", inheritedDecision, err)
	}
	inheritedNote, err := fixture.db.Review.GetNote(ctx, context2.ID, note.ID)
	if err != nil || inheritedNote.Version.ID != note.Version.ID || inheritedNote.InheritedFromContextID == nil {
		t.Fatalf("inherited note=%+v err=%v", inheritedNote, err)
	}
	inheritedAnchor, err := fixture.db.Review.GetAnchor(ctx, context2.ID, anchor.ID)
	if err != nil || inheritedAnchor.Version.ID != anchor.Version.ID || inheritedAnchor.InheritedFromContextID == nil {
		t.Fatalf("inherited anchor=%+v err=%v", inheritedAnchor, err)
	}

	if _, _, err := fixture.db.Review.AppendWorkReview(ctx, context2.ID, fixture.revision2, &decision.Version.ID, "not_approved", []string{"unrelated"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.db.Review.AppendNoteVersion(ctx, context2.ID, note.ID, note.Version.ID, "active", "Edited in child"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.db.Review.AppendAnchorVersion(ctx, context2.ID, anchor.ID, anchor.Version.ID, "active", hash, 2, "Edited in child", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}}); err != nil {
		t.Fatal(err)
	}

	parentDecision, err := fixture.db.Review.GetWorkReview(ctx, fixture.context1, fixture.revision1)
	if err != nil || parentDecision.Version == nil || parentDecision.Version.ID != decision.Version.ID {
		t.Fatalf("parent decision moved: %+v err=%v", parentDecision, err)
	}
	parentNote, err := fixture.db.Review.GetNote(ctx, fixture.context1, note.ID)
	if err != nil || parentNote.Version.ID != note.Version.ID {
		t.Fatalf("parent note moved: %+v err=%v", parentNote, err)
	}
	parentAnchor, err := fixture.db.Review.GetAnchor(ctx, fixture.context1, anchor.ID)
	if err != nil || parentAnchor.Version.ID != anchor.Version.ID {
		t.Fatalf("parent anchor moved: %+v err=%v", parentAnchor, err)
	}
}

// TestReviewReviewerPrivacyContract verifies reviewer email never appears in
// displays or audit evidence and anonymous reviewers stay redacted.
func TestReviewReviewerPrivacyContract(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	decision, _, err := fixture.db.Review.AppendWorkReview(ctx, fixture.context1, fixture.revision1, nil, "approved", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	note, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "Private note")
	if err != nil {
		t.Fatal(err)
	}
	edited, _, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context1, note.ID, note.Version.ID, "active", "Private edit")
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "methods-1", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	moved, _, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, anchor.Version.ID, "active", hash, 2, "Updated", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}})
	if err != nil {
		t.Fatal(err)
	}
	displays := []string{decision.Version.ReviewerDisplay, edited.Version.ReviewerDisplay, moved.Version.ReviewerDisplay}
	workVersions, err := fixture.db.Review.ListWorkReviewVersions(ctx, fixture.context1, fixture.revision1, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range workVersions {
		displays = append(displays, version.ReviewerDisplay)
	}
	noteVersions, err := fixture.db.Review.ListNoteVersions(ctx, fixture.context1, note.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range noteVersions {
		displays = append(displays, version.ReviewerDisplay)
	}
	anchorVersions, err := fixture.db.Review.ListAnchorVersions(ctx, fixture.context1, anchor.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range anchorVersions {
		displays = append(displays, version.ReviewerDisplay)
	}
	for _, display := range displays {
		if display == "" || strings.Contains(display, "@") {
			t.Fatalf("reviewer display=%q leaks identity", display)
		}
	}
	leaked := reviewCount(t, fixture.db, `SELECT COUNT(*) FROM audit_events
		WHERE before_json LIKE '%researcher@example.test%'
		   OR after_json LIKE '%researcher@example.test%'
		   OR metadata_json LIKE '%researcher@example.test%'`)
	if leaked != 0 {
		t.Fatalf("audit evidence leaked the reviewer email in %d rows", leaked)
	}

	context2, created, err := fixture.db.Review.CreateContext(ctx, fixture.run2, &fixture.context1)
	if err != nil || !created {
		t.Fatalf("create child context: context=%+v created=%v err=%v", context2, created, err)
	}
	anonymous, _, err := fixture.db.Review.AppendWorkReview(ctx, context2.ID, fixture.revision2, &decision.Version.ID, "not_approved", []string{"unrelated"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if anonymous.Version.ReviewerDisplay != "Anonymous or redacted" {
		t.Fatalf("anonymous reviewer display=%q", anonymous.Version.ReviewerDisplay)
	}
}

// TestReviewNoteBoundsContract verifies note bodies and links keep their byte
// and UTF-16 position limits through the family API.
func TestReviewNoteBoundsContract(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	cases := []struct {
		name string
		body string
	}{
		{"oversized body", strings.Repeat("a", notes.MaxBodyBytes+1)},
		{"oversized target", "[[ext:https://example.test/" + strings.Repeat("a", notes.MaxTargetBytes) + "]]"},
		{"oversized display", "[[ext:https://example.test|" + strings.Repeat("d", notes.MaxDisplayBytes+1) + "]]"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, test.body); err == nil {
				t.Fatalf("%s unexpectedly accepted", test.name)
			} else {
				var syntax *review.NoteSyntaxError
				if !errors.As(err, &syntax) || len(syntax.Errors) == 0 {
					t.Fatalf("%s error=%v, want note syntax error", test.name, err)
				}
			}
		})
	}
	if notes := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_notes"); notes != 0 {
		t.Fatalf("rejected notes left %d rows", notes)
	}

	valid, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "Valid note")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context1, valid.ID, valid.Version.ID, "active", strings.Repeat("a", notes.MaxBodyBytes+1)); err == nil {
		t.Fatal("oversized edit unexpectedly accepted")
	}
	if versions := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_note_versions WHERE note_id=?", valid.ID); versions != 1 {
		t.Fatalf("oversized edit left %d versions, want 1", versions)
	}

	body := "😀 [[ext:https://example.test|x]]"
	document := notes.Parse(body)
	if len(document.Errors) != 0 || len(document.Links) != 1 || document.Links[0].Position != 3 {
		t.Fatalf("parser fixture=%+v", document)
	}
	positioned, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, body)
	if err != nil {
		t.Fatal(err)
	}
	version, err := fixture.db.Review.GetNoteVersion(ctx, fixture.context1, positioned.ID, positioned.Version.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(version.Links) != 1 || version.Links[0].UTF16Position != document.Links[0].Position || version.Links[0].UTF16Length != document.Links[0].Length {
		t.Fatalf("stored link=%+v, want parser link %+v", version.Links, document.Links)
	}
}

// TestReviewAnchorBindingContract verifies anchors keep their work revision and
// PDF content hash across inheritance and require both to detect an identical save.
func TestReviewAnchorBindingContract(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	hashA := strings.Repeat("a", 64)
	hashB := strings.Repeat("b", 64)
	rectangles := []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}}
	anchor, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "methods-1", hashA, 1, "Methods", rectangles)
	if err != nil {
		t.Fatal(err)
	}
	version, err := fixture.db.Review.GetAnchorVersion(ctx, fixture.context1, anchor.ID, anchor.Version.ID)
	if err != nil || version.WorkRevisionID != fixture.revision1 || version.PDFContentHash != hashA {
		t.Fatalf("created anchor version=%+v err=%v", version, err)
	}
	context2, created, err := fixture.db.Review.CreateContext(ctx, fixture.run2, &fixture.context1)
	if err != nil || !created {
		t.Fatalf("create child context: context=%+v created=%v err=%v", context2, created, err)
	}
	inheritedVersion, err := fixture.db.Review.GetAnchorVersion(ctx, context2.ID, anchor.ID, anchor.Version.ID)
	if err != nil || inheritedVersion.WorkRevisionID != fixture.revision1 || inheritedVersion.PDFContentHash != hashA {
		t.Fatalf("inherited anchor version=%+v err=%v", inheritedVersion, err)
	}
	moved, changed, err := fixture.db.Review.AppendAnchorVersion(ctx, context2.ID, anchor.ID, anchor.Version.ID, "active", hashB, 2, "Updated", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}})
	if err != nil || !changed || moved.Version.WorkRevisionID != fixture.revision2 || moved.Version.PDFContentHash != hashB {
		t.Fatalf("child anchor move=%+v changed=%v err=%v", moved, changed, err)
	}
	parentAnchor, err := fixture.db.Review.GetAnchor(ctx, fixture.context1, anchor.ID)
	if err != nil || parentAnchor.Version.ID != anchor.Version.ID {
		t.Fatalf("parent anchor moved: %+v err=%v", parentAnchor, err)
	}
	if _, changed, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, anchor.Version.ID, "active", hashA, 1, "Methods", rectangles); err != nil || changed {
		t.Fatalf("identical anchor save changed=%v err=%v", changed, err)
	}
	if _, changed, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, anchor.Version.ID, "active", hashB, 1, "Methods", rectangles); err != nil || !changed {
		t.Fatalf("hash change was treated as identical: changed=%v err=%v", changed, err)
	}
}

// TestReviewAnchorGeometryContract verifies rectangles stay finite and
// normalized and anchor versions stay tied to a valid PDF page and hash.
func TestReviewAnchorGeometryContract(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	valid := review.AnchorRectangle{X: .1, Y: .2, Width: .3, Height: .1}
	sixtyFour := make([]review.AnchorRectangle, 64)
	for index := range sixtyFour {
		sixtyFour[index] = review.AnchorRectangle{X: 0, Y: 0, Width: .01, Height: .01}
	}
	invalid := []struct {
		name       string
		hash       string
		page       int
		text       string
		rectangles []review.AnchorRectangle
	}{
		{"uppercase hash", strings.Repeat("A", 64), 1, "Methods", []review.AnchorRectangle{valid}},
		{"non-hex hash", strings.Repeat("z", 64), 1, "Methods", []review.AnchorRectangle{valid}},
		{"short hash", strings.Repeat("a", 63), 1, "Methods", []review.AnchorRectangle{valid}},
		{"zero page", hash, 0, "Methods", []review.AnchorRectangle{valid}},
		{"blank text", hash, 1, "   ", []review.AnchorRectangle{valid}},
		{"oversized text", hash, 1, strings.Repeat("t", 16385), []review.AnchorRectangle{valid}},
		{"no rectangles", hash, 1, "Methods", nil},
		{"too many rectangles", hash, 1, "Methods", make([]review.AnchorRectangle, 65)},
		{"nan coordinate", hash, 1, "Methods", []review.AnchorRectangle{{X: math.NaN(), Y: .2, Width: .3, Height: .1}}},
		{"infinite coordinate", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: math.Inf(1), Width: .3, Height: .1}}},
		{"negative coordinate", hash, 1, "Methods", []review.AnchorRectangle{{X: -.1, Y: .2, Width: .3, Height: .1}}},
		{"out of range coordinate", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: 1.2, Width: .3, Height: .1}}},
		{"zero width", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: 0, Height: .1}}},
		{"negative height", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: -.1}}},
		{"width overflow", hash, 1, "Methods", []review.AnchorRectangle{{X: .6, Y: .2, Width: .5, Height: .1}}},
		{"height overflow", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .6, Width: .3, Height: .5}}},
	}
	for index, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			label := fmt.Sprintf("invalid-%d", index)
			if _, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, label, test.hash, test.page, test.text, test.rectangles); err == nil {
				t.Fatalf("%s unexpectedly accepted", test.name)
			} else if kind := reviewErrorKind(t, err); kind != "validation" {
				t.Fatalf("%s kind=%q, want validation", test.name, kind)
			}
		})
	}
	if anchors := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_anchors"); anchors != 0 {
		t.Fatalf("invalid anchors left %d rows", anchors)
	}
	boundary, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "boundary-1", hash, 1, strings.Repeat("t", 16384), sixtyFour)
	if err != nil || len(boundary.Version.Rectangles) != 64 {
		t.Fatalf("boundary anchor=%+v err=%v", boundary, err)
	}
	fullPage, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "boundary-2", hash, 1, "Methods", []review.AnchorRectangle{{X: 0, Y: 0, Width: 1, Height: 1}})
	if err != nil {
		t.Fatal(err)
	}
	deleted, changed, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, fullPage.ID, fullPage.Version.ID, "deleted", hash, 0, "", nil)
	if err != nil || !changed || deleted.Version.Page != nil || deleted.Version.SelectedText != nil || deleted.Version.Rectangles != nil {
		t.Fatalf("deleted anchor=%+v changed=%v err=%v", deleted, changed, err)
	}
	if _, _, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, fullPage.ID, deleted.Version.ID, "deleted", hash, 1, "Methods", []review.AnchorRectangle{valid}); err == nil {
		t.Fatal("deleted anchor with replacement geometry unexpectedly accepted")
	}
}

// TestReviewTombstoneRestorationContract verifies tombstones and restorations
// append new immutable versions instead of rewriting history.
func TestReviewTombstoneRestorationContract(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	note, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "Original body")
	if err != nil {
		t.Fatal(err)
	}
	deletedNote, changed, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context1, note.ID, note.Version.ID, "deleted", "")
	if err != nil || !changed || deletedNote.Version.Body != nil {
		t.Fatalf("delete note=%+v changed=%v err=%v", deletedNote, changed, err)
	}
	restoredNote, changed, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context1, note.ID, deletedNote.Version.ID, "active", "Restored body")
	if err != nil || !changed || restoredNote.Version.ParentVersionID == nil || *restoredNote.Version.ParentVersionID != deletedNote.Version.ID {
		t.Fatalf("restore note=%+v changed=%v err=%v", restoredNote, changed, err)
	}
	noteVersions, err := fixture.db.Review.ListNoteVersions(ctx, fixture.context1, note.ID, 0, 10)
	if err != nil || len(noteVersions) != 3 {
		t.Fatalf("note versions=%+v err=%v, want 3", noteVersions, err)
	}
	if noteVersions[0].ID != restoredNote.Version.ID || noteVersions[1].State != "deleted" || noteVersions[2].Body == nil || *noteVersions[2].Body != "Original body" {
		t.Fatalf("note history=%+v", noteVersions)
	}
	noteHead, err := fixture.db.Review.GetNote(ctx, fixture.context1, note.ID)
	if err != nil || noteHead.Version.ID != restoredNote.Version.ID {
		t.Fatalf("restored note head=%+v err=%v", noteHead, err)
	}

	anchor, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "methods-1", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	deletedAnchor, changed, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, anchor.Version.ID, "deleted", hash, 0, "", nil)
	if err != nil || !changed || deletedAnchor.Version.State != "deleted" {
		t.Fatalf("delete anchor=%+v changed=%v err=%v", deletedAnchor, changed, err)
	}
	restoredAnchor, changed, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, deletedAnchor.Version.ID, "active", hash, 3, "Restored", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}})
	if err != nil || !changed || restoredAnchor.Version.ParentVersionID == nil || *restoredAnchor.Version.ParentVersionID != deletedAnchor.Version.ID {
		t.Fatalf("restore anchor=%+v changed=%v err=%v", restoredAnchor, changed, err)
	}
	anchorVersions, err := fixture.db.Review.ListAnchorVersions(ctx, fixture.context1, anchor.ID, 0, 10)
	if err != nil || len(anchorVersions) != 3 {
		t.Fatalf("anchor versions=%+v err=%v, want 3", anchorVersions, err)
	}
	if anchorVersions[0].ID != restoredAnchor.Version.ID || anchorVersions[1].Page != nil || anchorVersions[1].SelectedText != nil || anchorVersions[1].Rectangles != nil {
		t.Fatalf("anchor history=%+v", anchorVersions)
	}
	if anchorVersions[2].Page == nil || *anchorVersions[2].Page != 1 || anchorVersions[2].SelectedText == nil || *anchorVersions[2].SelectedText != "Methods" {
		t.Fatalf("original anchor version was rewritten: %+v", anchorVersions[2])
	}
	anchorHead, err := fixture.db.Review.GetAnchor(ctx, fixture.context1, anchor.ID)
	if err != nil || anchorHead.Version.ID != restoredAnchor.Version.ID {
		t.Fatalf("restored anchor head=%+v err=%v", anchorHead, err)
	}
}
