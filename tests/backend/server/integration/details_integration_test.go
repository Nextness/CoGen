// details_integration_test.go tests the article, author, and reference
// detail endpoints.
//

package server_test

import subject "analysis/backend/server"
import testhelpers "analysis/tests/support/go/backend/server"

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

// TestAPIDetailsArticleAuthorReference verifies api details article author reference.
func TestAPIDetailsArticleAuthorReference(t *testing.T) {
	path, runID, revisionID, mentionID := testhelpers.ViewerFixture(t)
	viewer, err := subject.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	for _, path := range []string{
		"/api/articles/" + subject.stringID(revisionID) + "?run_id=" + subject.stringID(runID),
		"/api/authors/1?run_id=" + subject.stringID(runID),
		"/api/references/" + subject.stringID(mentionID) + "?run_id=" + subject.stringID(runID),
	} {
		response := testhelpers.ViewerRequest(t, handler, path)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{
		"/api/articles/not-a-number",
	} {
		response := testhelpers.ViewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s: status=%d", path, response.Code)
		}
	}
}

// TestAPIDetailsRejectCrossRunRecords verifies crafted identifiers cannot escape the selected run.
func TestAPIDetailsRejectCrossRunRecords(t *testing.T) {
	path, runID, revisionID, mentionID := testhelpers.ViewerFixture(t)
	viewer, err := subject.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()

	for _, path := range []string{
		"/api/articles/" + subject.stringID(revisionID) + "?run_id=" + subject.stringID(runID+1),
		"/api/authors/1?run_id=" + subject.stringID(runID+1),
		"/api/references/" + subject.stringID(mentionID) + "?run_id=" + subject.stringID(runID+1),
	} {
		response := testhelpers.ViewerRequest(t, handler, path)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{
		"/api/articles/" + subject.stringID(revisionID),
		"/api/authors/1",
		"/api/references/" + subject.stringID(mentionID),
	} {
		response := testhelpers.ViewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s without run: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

// TestArticleDetailCollectionsTraverseBeyondOneHundred verifies endpoint-bound cursor paging and ownership.
func TestArticleDetailCollectionsTraverseBeyondOneHundred(t *testing.T) {
	path, runID, revisionID, _ := testhelpers.ViewerFixture(t)
	viewer, err := subject.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	for index := 0; index < 103; index++ {
		author, err := viewer.writeDB.Conn.Exec("INSERT INTO author_occurrences (citation_name) VALUES (?)", "Paged author "+subject.stringID(int64(index)))
		if err != nil {
			t.Fatal(err)
		}
		authorID, _ := author.LastInsertId()
		if _, err := viewer.writeDB.Conn.Exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order) VALUES (?, ?, ?)", revisionID, authorID, index+10); err != nil {
			t.Fatal(err)
		}
	}
	handler := viewer.Handler()
	seen := make(map[int64]bool)
	cursor := ""
	for {
		requestPath := "/api/articles/" + subject.stringID(revisionID) + "/collections/authors?run_id=" + subject.stringID(runID)
		if cursor != "" {
			requestPath += "&cursor=" + url.QueryEscape(cursor)
		}
		response := testhelpers.ViewerRequest(t, handler, requestPath)
		if response.Code != http.StatusOK {
			t.Fatalf("article authors request=%s status=%d body=%s", requestPath, response.Code, response.Body.String())
		}
		var payload struct {
			Items      []map[string]any `json:"items"`
			Total      int              `json:"total"`
			HasMore    bool             `json:"has_more"`
			NextCursor string           `json:"next_cursor"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Total != 106 {
			t.Fatalf("article author total=%d, want 106", payload.Total)
		}
		for _, item := range payload.Items {
			id := int64(item["id"].(float64))
			if seen[id] {
				t.Fatalf("author %d appeared on multiple pages", id)
			}
			seen[id] = true
		}
		if !payload.HasMore {
			break
		}
		if payload.NextCursor == "" {
			t.Fatal("article authors reported more without a cursor")
		}
		cursor = payload.NextCursor
	}
	if len(seen) != 106 {
		t.Fatalf("traversed %d article authors, want 106", len(seen))
	}
	badCursor := testhelpers.ViewerRequest(t, handler, "/api/articles/"+subject.stringID(revisionID)+"/collections/references?run_id="+subject.stringID(runID)+"&cursor="+url.QueryEscape(cursor))
	if badCursor.Code != http.StatusBadRequest {
		t.Fatalf("cross-collection cursor status=%d body=%s", badCursor.Code, badCursor.Body.String())
	}
	crossRun := testhelpers.ViewerRequest(t, handler, "/api/articles/"+subject.stringID(revisionID)+"/collections/authors?run_id="+subject.stringID(runID+1))
	if crossRun.Code != http.StatusNotFound {
		t.Fatalf("cross-run collection status=%d body=%s", crossRun.Code, crossRun.Body.String())
	}
}

// TestAPIAuthorDetailIncludesRunScopedIdentityCandidates verifies candidate evidence moved from the corpus table into author detail.
func TestAPIAuthorDetailIncludesRunScopedIdentityCandidates(t *testing.T) {
	path, runID, _, _ := testhelpers.ViewerFixture(t)
	viewer, err := subject.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	status, body := testhelpers.RequestJSON(t, viewer.Handler(), "/api/authors/2?run_id="+subject.stringID(runID))
	if status != http.StatusOK {
		t.Fatalf("author detail: status=%d body=%v", status, body)
	}
	evidence := body["identity_evidence"].(map[string]any)["items"].([]any)
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

	if status, invalid := testhelpers.RequestJSON(t, viewer.Handler(), "/api/authors/2?run_id=invalid"); status != http.StatusBadRequest {
		t.Fatalf("invalid run filter: status=%d body=%v", status, invalid)
	}
}

// TestAuthorDetailCollectionsValidateScopeAndCursor verifies each bounded author subresource route.
func TestAuthorDetailCollectionsValidateScopeAndCursor(t *testing.T) {
	path, runID, _, _ := testhelpers.ViewerFixture(t)
	viewer, err := subject.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	for _, kind := range []string{"articles", "audit", "identity"} {
		response := testhelpers.ViewerRequest(t, handler, "/api/authors/2/collections/"+kind+"?run_id="+subject.stringID(runID)+"&limit=1")
		if response.Code != http.StatusOK {
			t.Errorf("author %s collection: status=%d body=%s", kind, response.Code, response.Body.String())
		}
	}
	for _, requestPath := range []string{
		"/api/authors/2/collections/articles",
		"/api/authors/not-a-number/collections/articles?run_id=" + subject.stringID(runID),
		"/api/authors/999999/collections/articles?run_id=" + subject.stringID(runID),
		"/api/authors/2/collections/articles?run_id=" + subject.stringID(runID) + "&unknown=1",
		"/api/authors/2/collections/articles?run_id=" + subject.stringID(runID) + "&cursor=invalid",
		"/api/authors/2/collections/unknown?run_id=" + subject.stringID(runID),
	} {
		response := testhelpers.ViewerRequest(t, handler, requestPath)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
			t.Errorf("GET %s: status=%d body=%s", requestPath, response.Code, response.Body.String())
		}
	}
}

// TestAPIReferenceResolutionUsesFinalRevision verifies api reference resolution uses final revision.
func TestAPIReferenceResolutionUsesFinalRevision(t *testing.T) {
	fixture := testhelpers.ViewerReferenceResolutionFixture(t)
	viewer, err := subject.Open(fixture.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()

	code, article := testhelpers.RequestJSON(t, handler, "/api/articles/"+subject.stringID(fixture.CitingRevisionID)+"?run_id="+subject.stringID(fixture.RunID))
	if code != http.StatusOK {
		t.Fatalf("article response: code=%d body=%v", code, article)
	}
	references := article["references"].(map[string]any)["items"].([]any)
	if len(references) != 2 {
		t.Fatalf("article references = %#v, want exactly the two stored mentions", references)
	}
	for _, raw := range references {
		reference := raw.(map[string]any)
		switch int64(reference["id"].(float64)) {
		case fixture.ResolvedMentionID:
			if reference["resolved_revision_id"] != float64(fixture.NormalizedTargetID) {
				t.Errorf("resolved revision = %#v, want %d", reference["resolved_revision_id"], fixture.NormalizedTargetID)
			}
			if reference["resolved_title"] != fixture.NormalizedTargetTitle {
				t.Errorf("resolved title = %#v, want %q", reference["resolved_title"], fixture.NormalizedTargetTitle)
			}
		case fixture.ExternalMentionID:
			if reference["resolved_revision_id"] != nil || reference["resolved_title"] != nil {
				t.Errorf("external reference target = %#v, want no resolved target", reference)
			}
		default:
			t.Errorf("unexpected reference = %#v", reference)
		}
	}

	code, detail := testhelpers.RequestJSON(t, handler, "/api/references/"+subject.stringID(fixture.ResolvedMentionID)+"?run_id="+subject.stringID(fixture.RunID))
	if code != http.StatusOK {
		t.Fatalf("reference response: code=%d body=%v", code, detail)
	}
	reference := detail["reference"].(map[string]any)
	if reference["resolved_revision_id"] != float64(fixture.NormalizedTargetID) {
		t.Errorf("reference detail resolved revision = %#v, want %d", reference["resolved_revision_id"], fixture.NormalizedTargetID)
	}
	if reference["resolved_title"] != fixture.NormalizedTargetTitle {
		t.Errorf("reference detail resolved title = %#v, want %q", reference["resolved_title"], fixture.NormalizedTargetTitle)
	}
}

// TestAPIArticleDetailIncludesRunScopedActivityHistory verifies api article detail includes run scoped activity history.
func TestAPIArticleDetailIncludesRunScopedActivityHistory(t *testing.T) {
	fixture := testhelpers.ViewerArticleActivityFixture(t)
	viewer, err := subject.Open(fixture.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()

	code, article := testhelpers.RequestJSON(t, handler, "/api/articles/"+subject.stringID(fixture.NormalizedRevisionID)+"?run_id="+subject.stringID(fixture.RunID))
	if code != http.StatusOK {
		t.Fatalf("article response: code=%d body=%v", code, article)
	}
	stages := article["stage_outcomes"].(map[string]any)["items"].([]any)
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
	for _, raw := range article["audit_events"].(map[string]any)["items"].([]any) {
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

	enrichment := article["enrichment_summary"].(map[string]any)
	providers := enrichment["providers"].([]any)
	fields := enrichment["fields"].([]any)
	if len(providers) != 2 || len(fields) != 2 {
		t.Errorf("enrichment summary providers=%v fields=%v", providers, fields)
	}

	code, discarded := testhelpers.RequestJSON(t, handler, "/api/articles/"+subject.stringID(fixture.DiscardedRevisionID)+"?run_id="+subject.stringID(fixture.RunID))
	if code != http.StatusOK {
		t.Fatalf("discarded response: code=%d body=%v", code, discarded)
	}
	for _, raw := range discarded["stage_outcomes"].(map[string]any)["items"].([]any) {
		stage := raw.(map[string]any)
		if stage["stage_name"] == "validate" {
			if stage["outcome"] != "discarded" || stage["reason"] != fixture.DiscardedReason {
				t.Errorf("discarded validation stage = %#v, want discarded reason %q", stage, fixture.DiscardedReason)
			}
			return
		}
	}
	t.Errorf("discarded validation stage missing: %#v", discarded["stage_outcomes"])
}
