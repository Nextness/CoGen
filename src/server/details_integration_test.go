// details_integration_test.go tests the article, author, and reference
// detail endpoints.
//
//go:build integration

package server

import (
	"net/http"
	"testing"
)

// TestAPIDetailsArticleAuthorReference verifies api details article author reference.
func TestAPIDetailsArticleAuthorReference(t *testing.T) {
	path, runID, revisionID, mentionID := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	for _, path := range []string{
		"/api/articles/" + stringID(revisionID),
		"/api/authors/1?run_id=" + stringID(runID),
		"/api/references/" + stringID(mentionID),
	} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{
		"/api/articles/not-a-number",
	} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s: status=%d", path, response.Code)
		}
	}
}

// TestAPIAuthorDetailIncludesRunScopedIdentityCandidates verifies candidate evidence moved from the corpus table into author detail.
func TestAPIAuthorDetailIncludesRunScopedIdentityCandidates(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	status, body := requestJSON(t, viewer.Handler(), "/api/authors/2?run_id="+stringID(runID))
	if status != http.StatusOK {
		t.Fatalf("author detail: status=%d body=%v", status, body)
	}
	evidence := body["identity_evidence"].([]any)
	if len(evidence) != 1 {
		t.Fatalf("identity evidence=%#v, want one resolution", evidence)
	}
	resolution := evidence[0].(map[string]any)
	if resolution["status"] != "orcid_is_unclear" {
		t.Errorf("status=%#v, want orcid_is_unclear", resolution["status"])
	}
	candidates := resolution["candidates"].([]any)
	if len(candidates) != 2 || candidates[0].(map[string]any)["candidate_orcid"] != "0000-0001-2345-6789" {
		t.Errorf("candidates=%#v, want ranked provider evidence", candidates)
	}

	if status, invalid := requestJSON(t, viewer.Handler(), "/api/authors/2?run_id=invalid"); status != http.StatusBadRequest {
		t.Fatalf("invalid run filter: status=%d body=%v", status, invalid)
	}
}

// TestAPIReferenceResolutionUsesFinalRevision verifies api reference resolution uses final revision.
func TestAPIReferenceResolutionUsesFinalRevision(t *testing.T) {
	fixture := viewerReferenceResolutionFixture(t)
	viewer, err := Open(fixture.path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()

	code, article := requestJSON(t, handler, "/api/articles/"+stringID(fixture.citingRevisionID))
	if code != http.StatusOK {
		t.Fatalf("article response: code=%d body=%v", code, article)
	}
	references := article["references"].([]any)
	if len(references) != 2 {
		t.Fatalf("article references = %#v, want exactly the two stored mentions", references)
	}
	for _, raw := range references {
		reference := raw.(map[string]any)
		switch int64(reference["id"].(float64)) {
		case fixture.resolvedMentionID:
			if reference["resolved_revision_id"] != float64(fixture.normalizedTargetID) {
				t.Errorf("resolved revision = %#v, want %d", reference["resolved_revision_id"], fixture.normalizedTargetID)
			}
			if reference["resolved_title"] != fixture.normalizedTargetTitle {
				t.Errorf("resolved title = %#v, want %q", reference["resolved_title"], fixture.normalizedTargetTitle)
			}
		case fixture.externalMentionID:
			if reference["resolved_revision_id"] != nil || reference["resolved_title"] != nil {
				t.Errorf("external reference target = %#v, want no resolved target", reference)
			}
		default:
			t.Errorf("unexpected reference = %#v", reference)
		}
	}

	code, detail := requestJSON(t, handler, "/api/references/"+stringID(fixture.resolvedMentionID))
	if code != http.StatusOK {
		t.Fatalf("reference response: code=%d body=%v", code, detail)
	}
	reference := detail["reference"].(map[string]any)
	if reference["resolved_revision_id"] != float64(fixture.normalizedTargetID) {
		t.Errorf("reference detail resolved revision = %#v, want %d", reference["resolved_revision_id"], fixture.normalizedTargetID)
	}
	if reference["resolved_title"] != fixture.normalizedTargetTitle {
		t.Errorf("reference detail resolved title = %#v, want %q", reference["resolved_title"], fixture.normalizedTargetTitle)
	}
}

// TestAPIArticleDetailIncludesRunScopedActivityHistory verifies api article detail includes run scoped activity history.
func TestAPIArticleDetailIncludesRunScopedActivityHistory(t *testing.T) {
	fixture := viewerArticleActivityFixture(t)
	viewer, err := Open(fixture.path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()

	code, article := requestJSON(t, handler, "/api/articles/"+stringID(fixture.normalizedRevisionID))
	if code != http.StatusOK {
		t.Fatalf("article response: code=%d body=%v", code, article)
	}
	stages := article["stage_outcomes"].([]any)
	wantStages := []struct {
		name    string
		outcome string
	}{
		{"parse", "parsed"},
		{"deduplicate", "deduplicated"},
		{"enrich_metadata", "enriched"},
		{"enrich_identity", "enriched"},
		{"validate", "valid"},
		{"normalize", "normalized"},
	}
	if len(stages) != len(wantStages) {
		t.Fatalf("stage outcomes = %#v, want %d rows", stages, len(wantStages))
	}
	for index, want := range wantStages {
		stage := stages[index].(map[string]any)
		if stage["stage_name"] != want.name || stage["outcome"] != want.outcome {
			t.Errorf("stage[%d] = %#v, want %s/%s", index, stage, want.name, want.outcome)
		}
	}

	correlations := make(map[string]bool)
	for _, raw := range article["audit_events"].([]any) {
		correlations[raw.(map[string]any)["correlation_id"].(string)] = true
	}
	for _, correlationID := range []string{
		"current-metadata-enrichment", "current-identity-enrichment", "current-validation", "manual-pdf",
	} {
		if !correlations[correlationID] {
			t.Errorf("audit events missing %q: %#v", correlationID, correlations)
		}
	}
	if correlations["previous-validation"] {
		t.Errorf("audit events included prior-run validation: %#v", correlations)
	}

	enriched := make(map[string]bool)
	for _, raw := range article["enriched_fields"].([]any) {
		enriched[raw.(map[string]any)["metadata_json"].(string)] = true
	}
	for _, metadata := range []string{
		`{"field":"title","provider":"crossref"}`,
		`{"field":"orcid","provider":"orcid"}`,
	} {
		if !enriched[metadata] {
			t.Errorf("enriched fields missing %s: %#v", metadata, enriched)
		}
	}

	code, discarded := requestJSON(t, handler, "/api/articles/"+stringID(fixture.discardedRevisionID))
	if code != http.StatusOK {
		t.Fatalf("discarded response: code=%d body=%v", code, discarded)
	}
	for _, raw := range discarded["stage_outcomes"].([]any) {
		stage := raw.(map[string]any)
		if stage["stage_name"] == "validate" {
			if stage["outcome"] != "discarded" || stage["reason"] != fixture.discardedReason {
				t.Errorf("discarded validation stage = %#v, want discarded reason %q", stage, fixture.discardedReason)
			}
			return
		}
	}
	t.Errorf("discarded validation stage missing: %#v", discarded["stage_outcomes"])
}
