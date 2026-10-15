//go:build integration

package server

import (
	"context"
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
	trashedRun, err := viewer.writeDB.Run.GetByID(context.Background(), runID)
	if err != nil || trashedRun == nil || trashedRun.TrashReason == nil || *trashedRun.TrashReason != "Superseded test attempt" {
		t.Fatalf("stored trash reason=%v err=%v", trashedRun, err)
	}
	trashEvents := countRunAuditEvents(t, viewer, runID, "run_trashed")
	if trashEvents != 1 {
		t.Fatalf("run_trashed events=%d", trashEvents)
	}

	status, unchanged := mutationJSON(t, handler, http.MethodPut, endpoint, `{"visibility_state":"trashed","reason":"Ignored no-op"}`, "")
	if status != http.StatusOK || unchanged["changed"] != false {
		t.Fatalf("unchanged trash: status=%d body=%v", status, unchanged)
	}
	status, restored := mutationJSON(t, handler, http.MethodPut, endpoint, `{"visibility_state":"active","reason":""}`, "")
	if status != http.StatusOK || restored["changed"] != true || restored["visibility_state"] != "active" {
		t.Fatalf("restore run: status=%d body=%v", status, restored)
	}
	restoredRun, err := viewer.writeDB.Run.GetByID(context.Background(), runID)
	if err != nil || restoredRun == nil || restoredRun.VisibilityState != "active" || restoredRun.TrashReason != nil {
		t.Fatalf("restored run=%+v err=%v", restoredRun, err)
	}
	restoreEvents := countRunAuditEvents(t, viewer, runID, "run_restored")
	if restoreEvents != 1 {
		t.Fatalf("run_restored events=%d", restoreEvents)
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
	runningRunID, err := viewer.writeDB.Run.StartRun(context.Background(), "run_lifecycle", "")
	if err != nil {
		t.Fatal(err)
	}
	runningEndpoint := fmt.Sprintf("/api/runs/%d/visibility", runningRunID)
	status, body := mutationJSON(t, handler, http.MethodPut, runningEndpoint, `{"visibility_state":"trashed","reason":"test"}`, "")
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
	trashedRun, err := viewer.writeDB.Run.GetByID(context.Background(), runID)
	if err != nil || trashedRun == nil || trashedRun.TrashedAt == nil {
		t.Fatalf("trashed run=%+v err=%v", trashedRun, err)
	}
	trashedAt := *trashedRun.TrashedAt
	events, err := viewer.writeDB.Audit.ListByRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	var occurredAt, actor, entityType, entityID, action, beforeJSON, afterJSON, metadataJSON string
	for _, event := range events {
		if event.Action == "run_trashed" {
			occurredAt, actor, entityType, entityID, action = event.OccurredAt, event.Actor, event.EntityType, event.EntityID, event.Action
			beforeJSON, afterJSON, metadataJSON = event.BeforeJSON, event.AfterJSON, event.MetadataJSON
		}
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

// countRunAuditEvents counts audit rows for one run action through the audit family store.
func countRunAuditEvents(t *testing.T, viewer *Server, runID int64, action string) int {
	t.Helper()
	events, err := viewer.writeDB.Audit.ListByRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range events {
		if event.Action == action {
			count++
		}
	}
	return count
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
