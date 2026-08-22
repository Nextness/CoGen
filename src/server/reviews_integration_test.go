//go:build integration

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
	"analysis/pdfstore"
)

// TestReviewAPIInitializesAndMutatesMetadataOnly verifies the public review lifecycle, conflicts, parser errors, and PDF read-only ownership.
func TestReviewAPIInitializesAndMutatesMetadataOnly(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	var pdfRowsBefore int
	if err := fixture.server.pdfDB.QueryRow("SELECT COUNT(*) FROM pdf_documents").Scan(&pdfRowsBefore); err != nil {
		t.Fatal(err)
	}

	status, initial := requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID))
	if status != http.StatusOK || initial["context_initialized"] != false || initial["editable"] != false {
		t.Fatalf("initial review: status=%d body=%v", status, initial)
	}
	status, contextBody := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.runID), `{"parent_context_id":null}`, "")
	if status != http.StatusCreated || contextBody["context_initialized"] != true {
		t.Fatalf("create context: status=%d body=%v", status, contextBody)
	}
	status, parentConflict := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.runID), `{"parent_context_id":999999}`, "")
	if status != http.StatusConflict || parentConflict["error"].(map[string]any)["code"] != "context_parent_conflict" {
		t.Fatalf("context parent conflict: status=%d body=%v", status, parentConflict)
	}
	details := parentConflict["error"].(map[string]any)["details"].(map[string]any)
	if details["requested_parent_context_id"] != float64(999999) || details["existing_parent_context_id"] != nil {
		t.Fatalf("context parent conflict details=%v", details)
	}
	for _, path := range []string{
		fmt.Sprintf("/api/runs/%d/review-context", fixture.runID),
		fmt.Sprintf("/api/runs/%d/review-context-candidates?scope=same_search&limit=10", fixture.runID),
	} {
		if status, body := requestJSON(t, handler, path); status != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%v", path, status, body)
		}
	}
	status, pdfLessReview := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.notAvailableRevisionID), `{"expected_version_id":null,"status":"approved","sub_statuses":[],"reason":null}`, "")
	if status != http.StatusOK || pdfLessReview["changed"] != true {
		t.Fatalf("PDF-less decision: status=%d body=%v", status, pdfLessReview)
	}
	status, pdfLessNote := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/notes", fixture.runID, fixture.notAvailableRevisionID), `{"body":"PDF-independent note"}`, "")
	if status != http.StatusCreated || pdfLessNote["note"] == nil {
		t.Fatalf("PDF-less note: status=%d body=%v", status, pdfLessNote)
	}
	status, pdfLessAnchor := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/anchors", fixture.runID, fixture.notAvailableRevisionID), `{"label":"no-pdf","page":1,"selected_text":"Unavailable","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}]}`, "")
	if status != http.StatusConflict || pdfLessAnchor["error"].(map[string]any)["code"] != "pdf_unavailable" {
		t.Fatalf("PDF-less anchor: status=%d body=%v", status, pdfLessAnchor)
	}
	status, wrongRun := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID+1000, fixture.revisionID), `{"expected_version_id":null,"status":"approved","sub_statuses":[],"reason":null}`, "")
	if status != http.StatusNotFound {
		t.Fatalf("wrong run review: status=%d body=%v", status, wrongRun)
	}
	status, saved := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), `{"expected_version_id":null,"status":"approved","sub_statuses":[],"reason":"Relevant"}`, "")
	if status != http.StatusOK || saved["changed"] != true {
		t.Fatalf("save review: status=%d body=%v", status, saved)
	}
	reviewVersionID := int64(saved["review"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	status, articleDetail := requestJSON(t, handler, fmt.Sprintf("/api/articles/%d?run_id=%d", fixture.revisionID, fixture.runID))
	if status != http.StatusOK {
		t.Fatalf("article detail after review: status=%d body=%v", status, articleDetail)
	}
	foundReviewAudit := false
	for _, raw := range articleDetail["audit_events"].(map[string]any)["items"].([]any) {
		if raw.(map[string]any)["action"] == "work_review_version_created" {
			foundReviewAudit = true
			break
		}
	}
	if !foundReviewAudit {
		t.Fatalf("article audit events omitted saved review decision: %v", articleDetail["audit_events"])
	}
	status, unchangedReview := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), fmt.Sprintf(`{"expected_version_id":%d,"status":"approved","sub_statuses":[],"reason":"Relevant"}`, reviewVersionID), "")
	if status != http.StatusOK || unchangedReview["changed"] != false {
		t.Fatalf("unchanged review: status=%d body=%v", status, unchangedReview)
	}
	status, conflict := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), `{"expected_version_id":null,"status":"removed","sub_statuses":["duplicate"],"reason":null}`, "")
	if status != http.StatusConflict || conflict["error"].(map[string]any)["code"] != "version_conflict" {
		t.Fatalf("review conflict: status=%d body=%v", status, conflict)
	}

	status, syntax := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/notes", fixture.runID, fixture.revisionID), `{"body":"[[ext:javascript:alert(1)]]"}`, "")
	if status != http.StatusBadRequest || syntax["error"].(map[string]any)["details"].(map[string]any)["syntax_errors"] == nil {
		t.Fatalf("note syntax response: status=%d body=%v", status, syntax)
	}
	status, noteBody := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/notes", fixture.runID, fixture.revisionID), `{"body":"See [[pdf:page=1|the PDF]]."}`, "")
	if status != http.StatusCreated {
		t.Fatalf("create note: status=%d body=%v", status, noteBody)
	}
	noteID := int64(noteBody["note"].(map[string]any)["id"].(float64))
	noteVersionID := int64(noteBody["note"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	status, noteVersion := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/notes/%d/versions", fixture.runID, noteID), fmt.Sprintf(`{"expected_version_id":%d,"state":"active","body":"Edited [[pdf:page=1]]."}`, noteVersionID), "")
	if status != http.StatusCreated || noteVersion["changed"] != true {
		t.Fatalf("edit note: status=%d body=%v", status, noteVersion)
	}
	noteVersionID = int64(noteVersion["note"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	status, secondWorkNote := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/notes", fixture.runID, fixture.notAvailableRevisionID), `{"body":"Other article [[pdf:page=1]]."}`, "")
	if status != http.StatusCreated {
		t.Fatalf("create second-work page link: status=%d body=%v", status, secondWorkNote)
	}
	for _, path := range []string{
		fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/review/versions?limit=10", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/notes?limit=10", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/notes/%d", fixture.runID, noteID),
		fmt.Sprintf("/api/runs/%d/notes/%d/versions?limit=10", fixture.runID, noteID),
		fmt.Sprintf("/api/runs/%d/links/backlinks?target_type=pdf_page&target_id=1&work_revision_id=%d&limit=10", fixture.runID, fixture.revisionID),
	} {
		if status, body := requestJSON(t, handler, path); status != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%v", path, status, body)
		}
	}
	status, pageBacklinks := requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/links/backlinks?target_type=pdf_page&target_id=1&work_revision_id=%d&limit=10", fixture.runID, fixture.revisionID))
	if status != http.StatusOK || len(pageBacklinks["items"].([]any)) != 1 {
		t.Fatalf("work-scoped PDF page backlinks: status=%d body=%v", status, pageBacklinks)
	}
	status, deletedNote := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/notes/%d/versions", fixture.runID, noteID), fmt.Sprintf(`{"expected_version_id":%d,"state":"deleted","body":""}`, noteVersionID), "")
	if status != http.StatusCreated || deletedNote["changed"] != true {
		t.Fatalf("delete note: status=%d body=%v", status, deletedNote)
	}

	status, anchor := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/anchors", fixture.runID, fixture.revisionID), `{"label":"methods-1","page":1,"selected_text":"Methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}]}`, "")
	if status != http.StatusCreated || anchor["anchor"] == nil {
		t.Fatalf("create anchor: status=%d body=%v", status, anchor)
	}
	anchorID := anchor["anchor"].(map[string]any)["id"].(string)
	if anchorID == "methods-1" || anchor["anchor"].(map[string]any)["label"] != "methods-1" {
		t.Fatalf("generated anchor identity: %v", anchor["anchor"])
	}
	anchorVersionID := int64(anchor["anchor"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	status, duplicateAnchor := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/anchors", fixture.runID, fixture.revisionID), `{"label":"methods-1","page":1,"selected_text":"Duplicate","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}]}`, "")
	if status != http.StatusConflict || duplicateAnchor["error"].(map[string]any)["code"] != "anchor_label_conflict" {
		t.Fatalf("duplicate anchor label: status=%d body=%v", status, duplicateAnchor)
	}
	for _, path := range []string{
		fmt.Sprintf("/api/runs/%d/articles/%d/anchors?limit=10", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/anchors/%s/versions?limit=10", fixture.runID, anchorID),
	} {
		if status, body := requestJSON(t, handler, path); status != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%v", path, status, body)
		}
	}
	status, unchangedAnchor := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/anchors/%s/versions", fixture.runID, anchorID), fmt.Sprintf(`{"expected_version_id":%d,"state":"active","page":1,"selected_text":"Methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}]}`, anchorVersionID), "")
	if status != http.StatusOK || unchangedAnchor["changed"] != false {
		t.Fatalf("unchanged anchor: status=%d body=%v", status, unchangedAnchor)
	}
	status, deletedAnchor := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/anchors/%s/versions", fixture.runID, anchorID), fmt.Sprintf(`{"expected_version_id":%d,"state":"deleted","page":0,"selected_text":"","rectangles":[]}`, anchorVersionID), "")
	if status != http.StatusCreated || deletedAnchor["changed"] != true {
		t.Fatalf("delete anchor: status=%d body=%v", status, deletedAnchor)
	}
	deletedAnchorVersionID := int64(deletedAnchor["anchor"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	status, restoredAnchor := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/anchors/%s/versions", fixture.runID, anchorID), fmt.Sprintf(`{"expected_version_id":%d,"state":"active","restore_from_version_id":%d,"page":0,"selected_text":"","rectangles":[]}`, deletedAnchorVersionID, anchorVersionID), "")
	if status != http.StatusCreated || restoredAnchor["changed"] != true {
		t.Fatalf("restore anchor: status=%d body=%v", status, restoredAnchor)
	}
	restoredVersion := restoredAnchor["anchor"].(map[string]any)["version"].(map[string]any)
	if restoredVersion["selected_text"] != "Methods" || restoredVersion["page"] != float64(1) {
		t.Fatalf("restored anchor geometry=%v", restoredVersion)
	}
	status, evaluation := requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/evaluation", fixture.runID))
	if status != http.StatusOK {
		t.Fatalf("evaluation: status=%d body=%v", status, evaluation)
	}
	rows := evaluation["rows"].([]any)
	found := false
	for _, raw := range rows {
		row := raw.(map[string]any)
		if int64(row["work_revision_id"].(float64)) == fixture.revisionID {
			found = row["review_status"] == "approved" && row["review_inherited"] == false
		}
	}
	if !found {
		t.Fatalf("evaluation review overlay missing: %v", rows)
	}
	status, finalDetail := requestJSON(t, handler, fmt.Sprintf("/api/articles/%d?run_id=%d", fixture.revisionID, fixture.runID))
	if status != http.StatusOK {
		t.Fatalf("article detail after review mutations: status=%d body=%v", status, finalDetail)
	}
	actions := make(map[string]bool)
	for _, raw := range finalDetail["audit_events"].(map[string]any)["items"].([]any) {
		actions[raw.(map[string]any)["action"].(string)] = true
	}
	for _, action := range []string{"review_context_created", "work_review_version_created", "review_note_created", "review_note_version_created", "review_note_tombstoned", "review_anchor_created", "review_anchor_tombstoned", "review_anchor_version_created"} {
		if !actions[action] {
			t.Errorf("article audit omitted %s: %v", action, actions)
		}
	}
	encodedAudit, _ := json.Marshal(finalDetail["audit_events"])
	if strings.Contains(string(encodedAudit), "Edited [[pdf") || strings.Contains(string(encodedAudit), "Anchor history") {
		t.Fatalf("article audit leaked review content: %s", encodedAudit)
	}
	var pdfRowsAfter int
	if err := fixture.server.pdfDB.QueryRow("SELECT COUNT(*) FROM pdf_documents").Scan(&pdfRowsAfter); err != nil {
		t.Fatal(err)
	}
	if pdfRowsAfter != pdfRowsBefore {
		t.Fatalf("review write changed PDF database rows: before=%d after=%d", pdfRowsBefore, pdfRowsAfter)
	}
}

// TestReviewCollectionsTraverseBeyondOneHundred verifies every review cursor collection crosses the former hard boundary without gaps or duplicates.
func TestReviewCollectionsTraverseBeyondOneHundred(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	ctx := context.Background()
	contextRecord, _, err := fixture.server.writeDB.Reviews.CreateContext(ctx, fixture.runID, nil)
	if err != nil {
		t.Fatal(err)
	}

	var expectedReviewID *int64
	for index := 0; index < 101; index++ {
		reason := fmt.Sprintf("decision-%03d", index)
		state, changed, err := fixture.server.writeDB.Reviews.AppendWorkReview(ctx, contextRecord.ID, fixture.revisionID, expectedReviewID, "approved", nil, &reason)
		if err != nil || !changed || state.Version == nil {
			t.Fatalf("create review version %d: state=%+v changed=%v err=%v", index, state, changed, err)
		}
		value := state.Version.ID
		expectedReviewID = &value
	}

	notes := make([]*database.ReviewNote, 0, 101)
	for index := 0; index < 101; index++ {
		note, err := fixture.server.writeDB.Reviews.CreateNote(ctx, contextRecord.ID, fixture.revisionID,
			fmt.Sprintf("# Note %03d\n\nSee [[article:10.1000/viewer-available|article]].", index))
		if err != nil {
			t.Fatalf("create note %d: %v", index, err)
		}
		notes = append(notes, note)
	}
	historyNote := notes[0]
	for index := 1; index < 101; index++ {
		updated, changed, err := fixture.server.writeDB.Reviews.AppendNoteVersion(ctx, contextRecord.ID, historyNote.ID,
			historyNote.Version.ID, "active", fmt.Sprintf("# History %03d\n\nSee [[article:10.1000/viewer-available|article]].", index))
		if err != nil || !changed {
			t.Fatalf("create note version %d: changed=%v err=%v", index, changed, err)
		}
		historyNote = updated
	}

	var contentHash string
	if err := fixture.server.pdfDB.QueryRow("SELECT content_hash FROM pdf_documents WHERE doi='10.1000/viewer-available'").Scan(&contentHash); err != nil {
		t.Fatal(err)
	}
	anchors := make([]*database.ReviewAnchor, 0, 101)
	for index := 0; index < 101; index++ {
		anchor, err := fixture.server.writeDB.Reviews.CreateAnchor(ctx, contextRecord.ID, fixture.revisionID,
			fmt.Sprintf("anchor-%03d", index), contentHash, 1, fmt.Sprintf("Anchor %03d", index), []database.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
		if err != nil {
			t.Fatalf("create anchor %d: %v", index, err)
		}
		anchors = append(anchors, anchor)
	}
	historyAnchor := anchors[0]
	for index := 1; index < 101; index++ {
		updated, changed, err := fixture.server.writeDB.Reviews.AppendAnchorVersion(ctx, contextRecord.ID, historyAnchor.ID,
			historyAnchor.Version.ID, "active", contentHash, 1, fmt.Sprintf("Anchor history %03d", index), []database.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
		if err != nil || !changed {
			t.Fatalf("create anchor version %d: changed=%v err=%v", index, changed, err)
		}
		historyAnchor = updated
	}

	var planID int64
	if err := fixture.server.writeDB.DB.QueryRow("SELECT execution_plan_id FROM pipeline_runs WHERE id=?", fixture.runID).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 101; index++ {
		result, err := fixture.server.writeDB.DB.Exec(`INSERT INTO pipeline_runs
			(step, started_at, finished_at, status, execution_plan_id, attempt_number)
			VALUES ('review', '2020-01-01T00:00:00Z', '2020-01-01T00:01:00Z', 'completed', ?, ?)`, planID, 1000+index)
		if err != nil {
			t.Fatal(err)
		}
		runID, _ := result.LastInsertId()
		if _, err := fixture.server.writeDB.DB.Exec("INSERT INTO review_contexts (pipeline_run_id, created_at) VALUES (?, '2020-01-01T00:02:00Z')", runID); err != nil {
			t.Fatal(err)
		}
	}

	paths := []struct {
		name string
		path string
		key  string
	}{
		{"parent candidates", fmt.Sprintf("/api/runs/%d/review-context-candidates?scope=same_search", fixture.runID), "items"},
		{"decision versions", fmt.Sprintf("/api/runs/%d/articles/%d/review/versions", fixture.runID, fixture.revisionID), "items"},
		{"notes", fmt.Sprintf("/api/runs/%d/articles/%d/notes", fixture.runID, fixture.revisionID), "items"},
		{"note versions", fmt.Sprintf("/api/runs/%d/notes/%d/versions", fixture.runID, notes[0].ID), "items"},
		{"anchors", fmt.Sprintf("/api/runs/%d/articles/%d/anchors", fixture.runID, fixture.revisionID), "items"},
		{"anchor versions", fmt.Sprintf("/api/runs/%d/anchors/%s/versions", fixture.runID, historyAnchor.ID), "items"},
		{"backlinks", fmt.Sprintf("/api/runs/%d/links/backlinks?target_type=article&target_id=10.1000/viewer-available", fixture.runID), "items"},
	}
	for _, item := range paths {
		t.Run(item.name, func(t *testing.T) {
			rows := traverseReviewCollection(t, handler, item.path, item.key, 37)
			if len(rows) != 101 {
				t.Fatalf("traversed rows=%d want=101", len(rows))
			}
		})
	}

	status, firstPage := requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/notes?limit=1", fixture.runID, fixture.revisionID))
	if status != http.StatusOK {
		t.Fatalf("first cursor page: status=%d body=%v", status, firstPage)
	}
	wrongCursor := firstPage["next_cursor"].(string)
	status, _ = requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/anchors?limit=1&cursor=%s", fixture.runID, fixture.revisionID, url.QueryEscape(wrongCursor)))
	if status != http.StatusBadRequest {
		t.Fatalf("cross-collection cursor status=%d want=%d", status, http.StatusBadRequest)
	}
}

// traverseReviewCollection follows one opaque cursor envelope and rejects duplicate item identities.
func traverseReviewCollection(t *testing.T, handler http.Handler, path, key string, limit int) []any {
	t.Helper()
	seen := make(map[string]bool)
	items := make([]any, 0)
	cursor := ""
	for page := 0; ; page++ {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		requestPath := fmt.Sprintf("%s%slimit=%d", path, separator, limit)
		if cursor != "" {
			requestPath += "&cursor=" + url.QueryEscape(cursor)
		}
		status, body := requestJSON(t, handler, requestPath)
		if status != http.StatusOK {
			t.Fatalf("page %d status=%d body=%v", page, status, body)
		}
		pageItems := body[key].([]any)
		for _, raw := range pageItems {
			encoded, _ := json.Marshal(raw)
			identity := string(encoded)
			if seen[identity] {
				t.Fatalf("duplicate item on page %d: %s", page, identity)
			}
			seen[identity] = true
			items = append(items, raw)
		}
		hasMore := body["has_more"].(bool)
		if !hasMore {
			if body["next_cursor"] != nil {
				t.Fatalf("terminal next_cursor=%v", body["next_cursor"])
			}
			break
		}
		next, ok := body["next_cursor"].(string)
		if !ok || next == "" || next == cursor {
			t.Fatalf("page %d invalid continuation=%v", page, body["next_cursor"])
		}
		cursor = next
		if page > 10 {
			t.Fatal("cursor traversal did not terminate")
		}
	}
	return items
}

// TestAnchorRestoreRejectsChangedPDFContent verifies restoration cannot transfer historical geometry onto different bytes.
func TestAnchorRestoreRejectsChangedPDFContent(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	status, _ := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.runID), `{"parent_context_id":null}`, "")
	if status != http.StatusCreated {
		t.Fatalf("create context status=%d", status)
	}
	status, body := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/anchors", fixture.runID, fixture.revisionID), `{"label":"restore-check","page":1,"selected_text":"Methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}]}`, "")
	if status != http.StatusCreated {
		t.Fatalf("create anchor: status=%d body=%v", status, body)
	}
	anchor := body["anchor"].(map[string]any)
	anchorID := anchor["id"].(string)
	activeVersionID := int64(anchor["version"].(map[string]any)["id"].(float64))
	status, deleted := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/anchors/%s/versions", fixture.runID, anchorID), fmt.Sprintf(`{"expected_version_id":%d,"state":"deleted","page":0,"selected_text":"","rectangles":[]}`, activeVersionID), "")
	if status != http.StatusCreated {
		t.Fatalf("delete anchor: status=%d body=%v", status, deleted)
	}
	deletedVersionID := int64(deleted["anchor"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	registry := filepath.Join("..", "..", "config", "database.something")
	store, err := pdfstore.Open(filepath.Join(filepath.Dir(fixture.metadataPath), "corpus.pdf.db"), registry)
	if err != nil {
		t.Fatal(err)
	}
	replacementHash := strings.Repeat("f", 64)
	if _, err := store.DB.Exec(`INSERT INTO pdf_blobs (content_hash, byte_size, data, created_at) VALUES (?, 1, x'25', '2024-01-01T00:00:00Z');
		UPDATE pdf_documents SET content_hash=?, updated_at='2024-01-01T00:00:00Z' WHERE doi='10.1000/viewer-available'`, replacementHash, replacementHash); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	status, mismatch := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/anchors/%s/versions", fixture.runID, anchorID), fmt.Sprintf(`{"expected_version_id":%d,"state":"active","restore_from_version_id":%d,"page":0,"selected_text":"","rectangles":[]}`, deletedVersionID, activeVersionID), "")
	if status != http.StatusConflict || mismatch["error"].(map[string]any)["code"] != "anchor_pdf_changed" {
		t.Fatalf("changed PDF restore: status=%d body=%v", status, mismatch)
	}
}

// TestReviewHistoryRemainsReadableAfterTrash verifies lifecycle changes gate mutations without hiding existing evidence.
func TestReviewHistoryRemainsReadableAfterTrash(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	status, _ := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.runID), `{"parent_context_id":null}`, "")
	if status != http.StatusCreated {
		t.Fatalf("create context status=%d", status)
	}
	status, saved := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), `{"expected_version_id":null,"status":"approved","sub_statuses":[],"reason":null}`, "")
	if status != http.StatusOK {
		t.Fatalf("save review: status=%d body=%v", status, saved)
	}

	db, err := database.Open(fixture.metadataPath, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE pipeline_runs SET visibility_state='trashed', trashed_at=datetime('now') WHERE id=?", fixture.runID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		fmt.Sprintf("/api/runs/%d/review-context", fixture.runID),
		fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/review/versions?limit=10", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/notes?limit=10", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/anchors?limit=10", fixture.runID, fixture.revisionID),
	} {
		if status, body := requestJSON(t, handler, path); status != http.StatusOK {
			t.Fatalf("read %s: status=%d body=%v", path, status, body)
		}
	}
	status, review := requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID))
	if status != http.StatusOK || review["editable"] != false || review["editability"].(map[string]any)["decision"] != false {
		t.Fatalf("trashed review editability: status=%d body=%v", status, review)
	}
	status, rejected := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), `{"expected_version_id":null,"status":"removed","sub_statuses":[],"reason":null}`, "")
	if status != http.StatusConflict || rejected["error"].(map[string]any)["code"] != "run_not_reviewable" {
		t.Fatalf("trashed mutation: status=%d body=%v", status, rejected)
	}
}

// TestReviewDecisionAuditCapturesCompleteState verifies decision audit evidence records every changed review field.
func TestReviewDecisionAuditCapturesCompleteState(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	status, contextBody := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.runID), `{"parent_context_id":null}`, "")
	if status != http.StatusCreated || contextBody["context_initialized"] != true {
		t.Fatalf("create context: status=%d body=%v", status, contextBody)
	}
	status, first := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), `{"expected_version_id":null,"status":"approved","sub_statuses":[],"reason":"Initially met the inclusion criteria"}`, "")
	if status != http.StatusOK || first["changed"] != true {
		t.Fatalf("first review: status=%d body=%v", status, first)
	}
	firstVersionID := int64(first["review"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	status, second := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), fmt.Sprintf(`{"expected_version_id":%d,"status":"not_approved","sub_statuses":["out_of_scope","not_peer_reviewed"],"reason":"Excluded after full-text review"}`, firstVersionID), "")
	if status != http.StatusOK || second["changed"] != true {
		t.Fatalf("second review: status=%d body=%v", status, second)
	}

	status, articleDetail := requestJSON(t, handler, fmt.Sprintf("/api/articles/%d?run_id=%d", fixture.revisionID, fixture.runID))
	if status != http.StatusOK {
		t.Fatalf("article detail: status=%d body=%v", status, articleDetail)
	}
	var before, after struct {
		Status      string   `json:"status"`
		Reason      *string  `json:"reason"`
		Substatuses []string `json:"sub_statuses"`
	}
	found := false
	for _, raw := range articleDetail["audit_events"].(map[string]any)["items"].([]any) {
		event := raw.(map[string]any)
		if event["action"] != "work_review_version_created" {
			continue
		}
		if err := json.Unmarshal([]byte(event["after_json"].(string)), &after); err != nil || after.Status != "not_approved" {
			continue
		}
		if err := json.Unmarshal([]byte(event["before_json"].(string)), &before); err != nil {
			t.Fatalf("decode previous review audit state: %v", err)
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("updated review audit event not found: %v", articleDetail["audit_events"])
	}
	if before.Status != "approved" || before.Reason == nil || *before.Reason != "Initially met the inclusion criteria" || len(before.Substatuses) != 0 {
		t.Fatalf("previous review audit state = %+v", before)
	}
	if after.Reason == nil || *after.Reason != "Excluded after full-text review" || len(after.Substatuses) != 2 || after.Substatuses[0] != "not_peer_reviewed" || after.Substatuses[1] != "out_of_scope" {
		t.Fatalf("new review audit state = %+v", after)
	}
}

// TestReviewMutationTransportGuards verifies content type, body bounds, unknown fields, trailing JSON, and origin checks.
func TestReviewMutationTransportGuards(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	path := fmt.Sprintf("/api/runs/%d/review-context", fixture.runID)
	for _, test := range []struct {
		name, body, contentType, origin string
		want                            int
	}{
		{"media type", `{}`, "text/plain", "", http.StatusUnsupportedMediaType},
		{"unknown field", `{"unexpected":true}`, "application/json", "", http.StatusBadRequest},
		{"trailing value", `{"parent_context_id":null}{}`, "application/json", "", http.StatusBadRequest},
		{"origin", `{"parent_context_id":null}`, "application/json", "http://evil.test", http.StatusForbidden},
		{"oversized", `{"parent_context_id":null,"padding":"` + strings.Repeat("x", 524288) + `"}`, "application/json", "", http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, _ := mutationJSON(t, fixture.server.Handler(), http.MethodPost, path, test.body, test.origin, test.contentType)
			if status != test.want {
				t.Fatalf("status=%d want=%d", status, test.want)
			}
		})
	}
}

// TestReviewReadValidation verifies bounded pagination, target validation, and the local server configuration contract.
func TestReviewReadValidation(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	if !fixture.server.PDFStoreBound() {
		t.Fatal("fixture did not bind its companion PDF store")
	}
	configured := fixture.server.HTTPServer("127.0.0.1:8080")
	if configured.Addr != "127.0.0.1:8080" || configured.Handler == nil || configured.ReadHeaderTimeout == 0 {
		t.Fatalf("HTTP server configuration = %+v", configured)
	}
	for _, path := range []string{
		"/api/runs/invalid/review-context",
		fmt.Sprintf("/api/runs/%d/review-context-candidates?limit=0", fixture.runID),
		fmt.Sprintf("/api/runs/%d/review-context-candidates?cursor=invalid", fixture.runID),
		fmt.Sprintf("/api/runs/%d/articles/%d/review/versions?cursor=0", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/notes?limit=101", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/anchors?limit=invalid", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/links/backlinks?target_type=invalid&target_id=1", fixture.runID),
		fmt.Sprintf("/api/runs/%d/links/backlinks?target_type=pdf_page&target_id=1", fixture.runID),
	} {
		if status, body := requestJSON(t, handler, path); status != http.StatusBadRequest {
			t.Errorf("GET %s: status=%d body=%v", path, status, body)
		}
	}
}

// TestLoopbackAuthorityRejectsRebindingHost verifies the HTTP server boundary accepts only its exact bound authority.
func TestLoopbackAuthorityRejectsRebindingHost(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := enforceLoopbackAuthority("127.0.0.1:8080", next)
	for host, want := range map[string]int{"127.0.0.1:8080": http.StatusNoContent, "localhost:8080": http.StatusBadRequest, "127.0.0.1:9999": http.StatusBadRequest, "example.test:8080": http.StatusBadRequest} {
		request := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
		request.Host = host
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != want {
			t.Errorf("Host %q status=%d want=%d", host, recorder.Code, want)
		}
	}
}

// mutationJSON invokes one review mutation and decodes its object response for assertions.
func mutationJSON(t *testing.T, handler http.Handler, method, path, body, origin string, contentTypes ...string) (int, map[string]any) {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	contentType := "application/json"
	if len(contentTypes) != 0 {
		contentType = contentTypes[0]
	}
	request.Header.Set("Content-Type", contentType)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var result map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode mutation response: %v\n%s", err, recorder.Body.String())
	}
	return recorder.Code, result
}
