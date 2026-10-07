// parity_integration_test.go verifies the legacy flat review adapter and the
// review family store produce identical head, version, link, anchor, and audit
// graphs on cloned copies of one migrated fixture.
//go:build integration

package review_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"analysis/database"
	"analysis/database/review"
)

// TestReviewFamilyParityWithLegacyAdapter verifies the adapters and family
// store agree on reads, writes, and resulting review evidence graphs.
func TestReviewFamilyParityWithLegacyAdapter(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join(t.TempDir(), "fixture.db")
	fixture, err := database.Open(fixturePath, reviewFilterConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	a1Run, a2Run, _, a1Revision, a2Revision := createReviewLineageFixture(t, fixture)
	if _, err := fixture.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(t.TempDir(), "legacy.db")
	familyPath := filepath.Join(t.TempDir(), "family.db")
	copyReviewDatabaseFile(t, fixturePath, legacyPath)
	copyReviewDatabaseFile(t, fixturePath, familyPath)
	legacy, err := database.OpenExisting(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	family, err := database.OpenExisting(familyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer family.Close()

	legacyContext, _, err := legacy.Reviews.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	familyContext, _, err := family.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	if legacyContext.ID != familyContext.ID {
		t.Fatalf("CreateContext id parity: legacy=%d family=%d", legacyContext.ID, familyContext.ID)
	}

	reason := "parity reason"
	legacyDecision, _, err := legacy.Reviews.AppendWorkReview(ctx, legacyContext.ID, a1Revision, nil, "not_approved", []string{"duplicate"}, &reason)
	if err != nil {
		t.Fatal(err)
	}
	familyDecision, _, err := family.Review.AppendWorkReview(ctx, familyContext.ID, a1Revision, nil, "not_approved", []string{"duplicate"}, &reason)
	if err != nil {
		t.Fatal(err)
	}
	if legacyDecision.Version.ID != familyDecision.Version.ID {
		t.Fatalf("AppendWorkReview id parity: legacy=%d family=%d", legacyDecision.Version.ID, familyDecision.Version.ID)
	}

	legacyNote, err := legacy.Reviews.CreateNote(ctx, legacyContext.ID, a1Revision, "Parity note [[ext:https://example.test|x]]")
	if err != nil {
		t.Fatal(err)
	}
	familyNote, err := family.Review.CreateNote(ctx, familyContext.ID, a1Revision, "Parity note [[ext:https://example.test|x]]")
	if err != nil {
		t.Fatal(err)
	}
	if legacyNote.ID != familyNote.ID || legacyNote.Version.ID != familyNote.Version.ID {
		t.Fatalf("CreateNote parity: legacy=%d/%d family=%d/%d", legacyNote.ID, legacyNote.Version.ID, familyNote.ID, familyNote.Version.ID)
	}
	legacyNote, _, err = legacy.Reviews.AppendNoteVersion(ctx, legacyContext.ID, legacyNote.ID, legacyNote.Version.ID, "active", "Parity edit")
	if err != nil {
		t.Fatal(err)
	}
	familyNote, _, err = family.Review.AppendNoteVersion(ctx, familyContext.ID, familyNote.ID, familyNote.Version.ID, "active", "Parity edit")
	if err != nil {
		t.Fatal(err)
	}
	if legacyNote.Version.ID != familyNote.Version.ID {
		t.Fatalf("AppendNoteVersion id parity: legacy=%d family=%d", legacyNote.Version.ID, familyNote.Version.ID)
	}

	hash := strings.Repeat("a", 64)
	legacyAnchor, err := legacy.Reviews.CreateAnchor(ctx, legacyContext.ID, a1Revision, "parity-anchor", hash, 1, "Parity text", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	familyAnchor, err := family.Review.CreateAnchor(ctx, familyContext.ID, a1Revision, "parity-anchor", hash, 1, "Parity text", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	if legacyAnchor.Label != familyAnchor.Label || legacyAnchor.Version.ID != familyAnchor.Version.ID {
		t.Fatalf("CreateAnchor parity: legacy=%s/%d family=%s/%d", legacyAnchor.Label, legacyAnchor.Version.ID, familyAnchor.Label, familyAnchor.Version.ID)
	}
	legacyAnchor, _, err = legacy.Reviews.AppendAnchorVersion(ctx, legacyContext.ID, legacyAnchor.ID, legacyAnchor.Version.ID, "active", hash, 2, "Parity moved", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}})
	if err != nil {
		t.Fatal(err)
	}
	familyAnchor, _, err = family.Review.AppendAnchorVersion(ctx, familyContext.ID, familyAnchor.ID, familyAnchor.Version.ID, "active", hash, 2, "Parity moved", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}})
	if err != nil {
		t.Fatal(err)
	}
	if legacyAnchor.Version.ID != familyAnchor.Version.ID {
		t.Fatalf("AppendAnchorVersion id parity: legacy=%d family=%d", legacyAnchor.Version.ID, familyAnchor.Version.ID)
	}

	legacyChild, _, err := legacy.Reviews.CreateContext(ctx, a2Run, &legacyContext.ID)
	if err != nil {
		t.Fatal(err)
	}
	familyChild, _, err := family.Review.CreateContext(ctx, a2Run, &familyContext.ID)
	if err != nil {
		t.Fatal(err)
	}
	legacyChildDecision, _, err := legacy.Reviews.AppendWorkReview(ctx, legacyChild.ID, a2Revision, &legacyDecision.Version.ID, "approved", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	familyChildDecision, _, err := family.Review.AppendWorkReview(ctx, familyChild.ID, a2Revision, &familyDecision.Version.ID, "approved", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	legacyChildNote, _, err := legacy.Reviews.AppendNoteVersion(ctx, legacyChild.ID, legacyNote.ID, legacyNote.Version.ID, "active", "Child edit")
	if err != nil {
		t.Fatal(err)
	}
	familyChildNote, _, err := family.Review.AppendNoteVersion(ctx, familyChild.ID, familyNote.ID, familyNote.Version.ID, "active", "Child edit")
	if err != nil {
		t.Fatal(err)
	}
	legacyChildAnchor, _, err := legacy.Reviews.AppendAnchorVersion(ctx, legacyChild.ID, legacyAnchor.ID, legacyAnchor.Version.ID, "deleted", hash, 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	familyChildAnchor, _, err := family.Review.AppendAnchorVersion(ctx, familyChild.ID, familyAnchor.ID, familyAnchor.Version.ID, "deleted", hash, 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if legacyChild.ID != familyChild.ID || legacyChildDecision.Version.ID != familyChildDecision.Version.ID || legacyChildNote.Version.ID != familyChildNote.Version.ID || legacyChildAnchor.Version.ID != familyChildAnchor.Version.ID {
		t.Fatalf("child mutation parity: legacy=%d/%d/%d/%d family=%d/%d/%d/%d",
			legacyChild.ID, legacyChildDecision.Version.ID, legacyChildNote.Version.ID, legacyChildAnchor.Version.ID,
			familyChild.ID, familyChildDecision.Version.ID, familyChildNote.Version.ID, familyChildAnchor.Version.ID)
	}

	for _, query := range reviewParityQueries() {
		assertReviewQueryParity(t, legacy.DB, family.DB, query)
	}
	legacyRead, err := legacy.Reviews.GetWorkReview(ctx, legacyChild.ID, a2Revision)
	if err != nil {
		t.Fatal(err)
	}
	familyRead, err := family.Review.GetWorkReview(ctx, familyChild.ID, a2Revision)
	if err != nil {
		t.Fatal(err)
	}
	if legacyRead.Version.ID != familyRead.Version.ID || legacyRead.Version.CreatedInContextID != familyRead.Version.CreatedInContextID ||
		legacyRead.Version.ParentVersionID == nil || familyRead.Version.ParentVersionID == nil || *legacyRead.Version.ParentVersionID != *familyRead.Version.ParentVersionID {
		t.Fatalf("read parity: legacy=%+v family=%+v", legacyRead, familyRead)
	}
}

// reviewParityQueries returns the review graph queries compared between the legacy adapter and family store.
func reviewParityQueries() []string {
	return []string{
		"SELECT id, pipeline_run_id, parent_context_id FROM review_contexts ORDER BY id",
		"SELECT review_context_id, work_id, work_revision_id, review_version_id FROM review_context_work_heads ORDER BY review_context_id, work_id",
		"SELECT id, work_id, work_revision_id, created_in_context_id, parent_version_id, status, reason FROM work_review_versions ORDER BY id",
		"SELECT review_version_id, sub_status FROM work_review_version_substatuses ORDER BY review_version_id, sub_status",
		"SELECT review_context_id, note_id, note_version_id FROM review_context_note_heads ORDER BY review_context_id, note_id",
		"SELECT id, work_id FROM review_notes ORDER BY id",
		"SELECT id, note_id, parent_version_id, created_in_context_id, state, body FROM review_note_versions ORDER BY id",
		"SELECT note_version_id, ordinal, target_type, raw_target, display_text, utf16_position, utf16_length FROM review_note_links ORDER BY note_version_id, ordinal",
		"SELECT 'anchor' AS id, work_id, label FROM review_anchors ORDER BY id",
		"SELECT id, 'anchor' AS anchor_id, parent_version_id, created_in_context_id, work_revision_id, pdf_content_hash, state, page, selected_text, rectangles_json FROM review_anchor_versions ORDER BY id",
		"SELECT review_context_id, 'anchor' AS anchor_id, anchor_version_id FROM review_context_anchor_heads ORDER BY review_context_id, anchor_id",
		"SELECT entity_type, entity_id, action, before_json, after_json, metadata_json FROM audit_events WHERE entity_type IN ('review_context','work_review_version','review_note_version','review_anchor_version') ORDER BY id",
	}
}

// copyReviewDatabaseFile copies one checkpointed SQLite database file.
func copyReviewDatabaseFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertReviewQueryParity compares every row of one review graph query.
func assertReviewQueryParity(t *testing.T, legacy, family *sql.DB, query string) {
	t.Helper()
	legacyRows := reviewParityRows(t, legacy, query)
	familyRows := reviewParityRows(t, family, query)
	if !reflect.DeepEqual(legacyRows, familyRows) {
		t.Fatalf("review parity mismatch for %q:\nlegacy=%v\nfamily=%v", query, legacyRows, familyRows)
	}
}

// reviewParityRows renders every row of one query as ordered comparable text.
func reviewParityRows(t *testing.T, db *sql.DB, query string) [][]string {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]string
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		rendered := make([]string, len(names))
		for index, value := range values {
			rendered[index] = formatReviewDatabaseValue(value)
		}
		result = append(result, rendered)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// formatReviewDatabaseValue renders one scanned SQLite value as comparable text.
func formatReviewDatabaseValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "<null>"
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}
