//go:build integration

package server

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"
)

// nanoTimestampPattern matches RFC3339Nano text with an optional fractional second.
var nanoTimestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z$`)

// TestRunVisibilityLifecycle verifies that Home can trash and restore terminal runs without deleting their immutable evidence.
func TestRunVisibilityLifecycle(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	endpoint := fmt.Sprintf("/api/runs/%d/visibility", runID)

	status, trashed := mutationJSON(t, handler, http.MethodPut, endpoint, `{"visibility_state":"trashed","reason":"Superseded test attempt"}`, "")
	if status != http.StatusOK || trashed["changed"] != true || trashed["visibility_state"] != "trashed" {
		t.Fatalf("trash run: status=%d body=%v", status, trashed)
	}
	if status, body := requestJSON(t, handler, "/api/runs"); status != http.StatusOK || containsRun(body, runID) {
		t.Fatalf("active runs after trash: status=%d body=%v", status, body)
	}
	if status, body := requestJSON(t, handler, "/api/runs?include_trashed=true"); status != http.StatusOK || !containsRun(body, runID) {
		t.Fatalf("all runs after trash: status=%d body=%v", status, body)
	}
	var trashReason string
	if err := viewer.writeDB.DB.QueryRow("SELECT trash_reason FROM pipeline_runs WHERE id=?", runID).Scan(&trashReason); err != nil || trashReason != "Superseded test attempt" {
		t.Fatalf("stored trash reason=%q err=%v", trashReason, err)
	}
	var trashEvents int
	if err := viewer.writeDB.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE pipeline_run_id=? AND action='run_trashed'", runID).Scan(&trashEvents); err != nil || trashEvents != 1 {
		t.Fatalf("run_trashed events=%d err=%v", trashEvents, err)
	}

	status, unchanged := mutationJSON(t, handler, http.MethodPut, endpoint, `{"visibility_state":"trashed","reason":"Ignored no-op"}`, "")
	if status != http.StatusOK || unchanged["changed"] != false {
		t.Fatalf("unchanged trash: status=%d body=%v", status, unchanged)
	}
	status, restored := mutationJSON(t, handler, http.MethodPut, endpoint, `{"visibility_state":"active","reason":""}`, "")
	if status != http.StatusOK || restored["changed"] != true || restored["visibility_state"] != "active" {
		t.Fatalf("restore run: status=%d body=%v", status, restored)
	}
	var restoredState string
	var clearedReason any
	if err := viewer.writeDB.DB.QueryRow("SELECT visibility_state, trash_reason FROM pipeline_runs WHERE id=?", runID).Scan(&restoredState, &clearedReason); err != nil || restoredState != "active" || clearedReason != nil {
		t.Fatalf("restored state=%q reason=%v err=%v", restoredState, clearedReason, err)
	}
	var restoreEvents int
	if err := viewer.writeDB.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE pipeline_run_id=? AND action='run_restored'", runID).Scan(&restoreEvents); err != nil || restoreEvents != 1 {
		t.Fatalf("run_restored events=%d err=%v", restoreEvents, err)
	}
}

// TestRunVisibilityValidation verifies lifecycle input, origin, and running-attempt protections.
func TestRunVisibilityValidation(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	endpoint := fmt.Sprintf("/api/runs/%d/visibility", runID)
	handler := viewer.Handler()

	for _, test := range []struct {
		name, body, origin string
		want               int
	}{
		{"invalid state", `{"visibility_state":"deleted","reason":""}`, "", http.StatusBadRequest},
		{"unknown field", `{"visibility_state":"trashed","unexpected":true}`, "", http.StatusBadRequest},
		{"foreign origin", `{"visibility_state":"trashed","reason":"test"}`, "http://evil.test", http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, _ := mutationJSON(t, handler, http.MethodPut, endpoint, test.body, test.origin)
			if status != test.want {
				t.Fatalf("status=%d want=%d", status, test.want)
			}
		})
	}
	if _, err := viewer.writeDB.DB.Exec("UPDATE pipeline_runs SET status='running' WHERE id=?", runID); err != nil {
		t.Fatal(err)
	}
	status, body := mutationJSON(t, handler, http.MethodPut, endpoint, `{"visibility_state":"trashed","reason":"test"}`, "")
	if status != http.StatusConflict || body["error"].(map[string]any)["code"] != "run_active" {
		t.Fatalf("running run lifecycle: status=%d body=%v", status, body)
	}
}

// TestRunVisibilityAuditEvidence verifies the endpoint persists exact audit evidence with nanosecond timestamp precision.
func TestRunVisibilityAuditEvidence(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	endpoint := fmt.Sprintf("/api/runs/%d/visibility", runID)

	status, _ := mutationJSON(t, handler, http.MethodPut, endpoint, `{"visibility_state":"trashed","reason":"Evidence check"}`, "")
	if status != http.StatusOK {
		t.Fatalf("trash run status=%d", status)
	}
	var trashedAt string
	if err := viewer.writeDB.DB.QueryRow("SELECT trashed_at FROM pipeline_runs WHERE id=?", runID).Scan(&trashedAt); err != nil {
		t.Fatal(err)
	}
	var occurredAt, actor, entityType, entityID, action, beforeJSON, afterJSON, metadataJSON string
	if err := viewer.writeDB.DB.QueryRow(`SELECT occurred_at, actor, entity_type, entity_id, action,
		before_json, after_json, metadata_json
		FROM audit_events WHERE pipeline_run_id=? AND action='run_trashed'`, runID).Scan(
		&occurredAt, &actor, &entityType, &entityID, &action, &beforeJSON, &afterJSON, &metadataJSON); err != nil {
		t.Fatal(err)
	}
	if occurredAt != trashedAt {
		t.Fatalf("audit occurred_at %q != trashed_at %q", occurredAt, trashedAt)
	}
	if !nanoTimestampPattern.MatchString(trashedAt) {
		t.Fatalf("trashed_at %q does not use RFC3339Nano precision", trashedAt)
	}
	if actor != "local_user" || entityType != "pipeline_run" || entityID != fmt.Sprint(runID) || action != "run_trashed" {
		t.Fatalf("audit identity = %q %q %q %q", actor, entityType, entityID, action)
	}
	if beforeJSON != `{"visibility_state":"active"}` || afterJSON != `{"visibility_state":"trashed"}` || metadataJSON != `{"source":"local_viewer"}` {
		t.Fatalf("audit state = %q %q %q", beforeJSON, afterJSON, metadataJSON)
	}
}

// containsRun reports whether a run-list API response includes the requested identifier.
func containsRun(body map[string]any, runID int64) bool {
	for _, raw := range body["runs"].([]any) {
		if int64(raw.(map[string]any)["id"].(float64)) == runID {
			return true
		}
	}
	return false
}
