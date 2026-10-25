// anchor_consistency_integration_test.go verifies anchor writes carry the
// reader's PDF hash and tombstones remain possible after PDF removal.
package server_test

import (
	serverhelpers "analysis/tests/support/go/backend/server"

	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestAnchorMutationsRequireDisplayedPDFHash verifies create, active edit, and
// restoration require the exact displayed hash before metadata commit.
func TestAnchorMutationsRequireDisplayedPDFHash(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status, _ := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.RunID), `{"parent_context_id":null}`, "")
	if status != http.StatusCreated {
		t.Fatalf("create context status=%d", status)
	}
	hash := pdfStatusForWork(t, handler, fixture.AvailableID)["content_hash"].(string)
	base := fmt.Sprintf(`{"label":"hash-bound","page":1,"selected_text":"Methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}]`)
	createPath := fmt.Sprintf("/api/runs/%d/articles/%d/anchors", fixture.RunID, fixture.RevisionID)
	if code, body := mutationJSON(t, handler, http.MethodPost, createPath, base+`}`, ""); code != http.StatusBadRequest {
		t.Fatalf("create without hash = %d %v", code, body)
	}
	if code, body := mutationJSON(t, handler, http.MethodPost, createPath, base+fmt.Sprintf(`,"pdf_content_hash":"%s"}`, strings.Repeat("0", 64)), ""); code != http.StatusConflict || body["error"].(map[string]any)["code"] != "anchor_pdf_changed" {
		t.Fatalf("create with wrong hash = %d %v", code, body)
	}
	code, created := mutationJSON(t, handler, http.MethodPost, createPath, base+fmt.Sprintf(`,"pdf_content_hash":"%s"}`, hash), "")
	if code != http.StatusCreated {
		t.Fatalf("create with matching hash = %d %v", code, created)
	}
	anchor := created["anchor"].(map[string]any)
	anchorID := anchor["id"].(string)
	versionID := int64(anchor["version"].(map[string]any)["id"].(float64))
	versionPath := fmt.Sprintf("/api/runs/%d/anchors/%s/versions", fixture.RunID, anchorID)
	active := fmt.Sprintf(`{"expected_version_id":%d,"state":"active","page":1,"selected_text":"Methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}]`, versionID)
	if code, body := mutationJSON(t, handler, http.MethodPost, versionPath, active+`}`, ""); code != http.StatusBadRequest {
		t.Fatalf("active edit without hash = %d %v", code, body)
	}
	if code, body := mutationJSON(t, handler, http.MethodPost, versionPath, active+fmt.Sprintf(`,"pdf_content_hash":"%s"}`, strings.Repeat("0", 64)), ""); code != http.StatusConflict || body["error"].(map[string]any)["code"] != "anchor_pdf_changed" {
		t.Fatalf("active edit with wrong hash = %d %v", code, body)
	}
	if code, body := mutationJSON(t, handler, http.MethodPost, versionPath, active+fmt.Sprintf(`,"pdf_content_hash":"%s"}`, hash), ""); code != http.StatusOK || body["changed"] != false {
		t.Fatalf("active edit with matching hash = %d %v", code, body)
	}
	code, deleted := mutationJSON(t, handler, http.MethodPost, versionPath, fmt.Sprintf(`{"expected_version_id":%d,"state":"deleted","page":0,"selected_text":"","rectangles":[]}`, versionID), "")
	if code != http.StatusCreated || deleted["changed"] != true {
		t.Fatalf("tombstone without hash = %d %v", code, deleted)
	}
	deletedVersionID := int64(deleted["anchor"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	restore := fmt.Sprintf(`{"expected_version_id":%d,"state":"active","restore_from_version_id":%d,"page":0,"selected_text":"","rectangles":[]`, deletedVersionID, versionID)
	if code, body := mutationJSON(t, handler, http.MethodPost, versionPath, restore+fmt.Sprintf(`,"pdf_content_hash":"%s"}`, strings.Repeat("0", 64)), ""); code != http.StatusConflict || body["error"].(map[string]any)["code"] != "anchor_pdf_changed" {
		t.Fatalf("restore with wrong hash = %d %v", code, body)
	}
	if code, body := mutationJSON(t, handler, http.MethodPost, versionPath, restore+fmt.Sprintf(`,"pdf_content_hash":"%s"}`, hash), ""); code != http.StatusCreated || body["changed"] != true {
		t.Fatalf("restore with matching hash = %d %v", code, body)
	}
}

// TestAnchorTombstoneSurvivesPDFRemoval verifies active edits and restorations
// remain blocked while metadata tombstoning works without current PDF bytes.
func TestAnchorTombstoneSurvivesPDFRemoval(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status, _ := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.RunID), `{"parent_context_id":null}`, "")
	if status != http.StatusCreated {
		t.Fatalf("create context status=%d", status)
	}
	pdf := pdfStatusForWork(t, handler, fixture.AvailableID)
	hash := pdf["content_hash"].(string)
	etag := pdf["mutation_etag"].(string)
	createPath := fmt.Sprintf("/api/runs/%d/articles/%d/anchors", fixture.RunID, fixture.RevisionID)
	code, created := mutationJSON(t, handler, http.MethodPost, createPath, fmt.Sprintf(`{"label":"remove-after-pdf","page":1,"selected_text":"Methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}],"pdf_content_hash":"%s"}`, hash), "")
	if code != http.StatusCreated {
		t.Fatalf("create anchor = %d %v", code, created)
	}
	anchor := created["anchor"].(map[string]any)
	anchorID := anchor["id"].(string)
	versionID := int64(anchor["version"].(map[string]any)["id"].(float64))
	code, removed := pdfMutationRequest(t, handler, http.MethodDelete, fixture.RunID, fixture.RevisionID, nil, 0, "", etag)
	if code != http.StatusOK {
		t.Fatalf("remove PDF = %d %v", code, removed)
	}
	versionPath := fmt.Sprintf("/api/runs/%d/anchors/%s/versions", fixture.RunID, anchorID)
	status, review := serverhelpers.RequestJSON(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.RunID, fixture.RevisionID))
	if status != http.StatusOK {
		t.Fatalf("article review after removal = %d %v", status, review)
	}
	editability := review["editability"].(map[string]any)
	if editability["anchors"] != false || editability["anchor_removal"] != true {
		t.Fatalf("anchor editability after removal = %v", editability)
	}
	active := fmt.Sprintf(`{"expected_version_id":%d,"state":"active","page":1,"selected_text":"Methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}],"pdf_content_hash":"%s"}`, versionID, hash)
	if code, body := mutationJSON(t, handler, http.MethodPost, versionPath, active, ""); code != http.StatusConflict || body["error"].(map[string]any)["code"] != "pdf_unavailable" {
		t.Fatalf("active edit after removal = %d %v", code, body)
	}
	code, deleted := mutationJSON(t, handler, http.MethodPost, versionPath, fmt.Sprintf(`{"expected_version_id":%d,"state":"deleted","page":0,"selected_text":"","rectangles":[]}`, versionID), "")
	if code != http.StatusCreated || deleted["changed"] != true {
		t.Fatalf("tombstone after removal = %d %v", code, deleted)
	}
	var raw map[string]any
	encoded, _ := json.Marshal(deleted["anchor"])
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["version"].(map[string]any)["pdf_content_hash"] != hash {
		t.Fatalf("tombstone hash = %v", raw["version"])
	}
}
