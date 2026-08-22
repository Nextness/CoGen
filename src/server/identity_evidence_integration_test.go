// identity_evidence_integration_test.go tests the identity evidence endpoint.
//
//go:build integration

package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestRunScopedIdentityEvidence verifies run scoped identity evidence.
func TestRunScopedIdentityEvidence(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	base := "/api/runs/" + stringID(runID)
	identityEvidence := viewerRequest(t, handler, base+"/identity-evidence?page=1&per_page=20&sort=id&order=asc")
	if identityEvidence.Code != http.StatusOK || !strings.Contains(identityEvidence.Body.String(), "orcid_is_unclear") || !strings.Contains(identityEvidence.Body.String(), "provider_failed") || !strings.Contains(identityEvidence.Body.String(), "\"provider_failed\":1") || !strings.Contains(identityEvidence.Body.String(), "Charles Babbage") || !strings.Contains(identityEvidence.Body.String(), "0000-0001-2345-6789") {
		t.Fatalf("identity evidence response: status=%d body=%s", identityEvidence.Code, identityEvidence.Body.String())
	}
}

// TestIdentityCandidatePages verifies bounded previews, stable cursor traversal,
// run ownership, and collection-bound cursor validation.
func TestIdentityCandidatePages(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	var resolutionID int64
	if err := viewer.writeDB.DB.QueryRow(`SELECT id FROM author_identity_resolutions
		WHERE pipeline_run_id=? AND status='orcid_is_unclear'`, runID).Scan(&resolutionID); err != nil {
		t.Fatal(err)
	}
	for rank := 3; rank <= 105; rank++ {
		orcid := "0000-0000-0000-" + stringID(int64(1000+rank))
		if _, err := viewer.writeDB.DB.Exec(`INSERT INTO author_identity_candidates
			(identity_resolution_id, candidate_orcid, query_url, provider_rank)
			VALUES (?, ?, 'https://orcid.example/search', ?)`, resolutionID, orcid, rank); err != nil {
			t.Fatal(err)
		}
	}
	handler := viewer.Handler()
	evidence := viewerRequest(t, handler, "/api/runs/"+stringID(runID)+"/identity-evidence?page=1&per_page=20&sort=id&order=asc")
	if evidence.Code != http.StatusOK {
		t.Fatalf("identity evidence status=%d body=%s", evidence.Code, evidence.Body.String())
	}
	var evidenceEnvelope struct {
		Rows []struct {
			ResolutionID        int64            `json:"resolution_id"`
			CandidateCount      int64            `json:"candidate_count"`
			Candidates          []map[string]any `json:"candidates"`
			CandidatesTruncated bool             `json:"candidates_truncated"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(evidence.Body.Bytes(), &evidenceEnvelope); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, row := range evidenceEnvelope.Rows {
		if row.ResolutionID != resolutionID {
			continue
		}
		found = true
		if row.CandidateCount != 105 || len(row.Candidates) != identityCandidatePreviewLimit || !row.CandidatesTruncated {
			t.Fatalf("candidate preview: count=%d preview=%d truncated=%v", row.CandidateCount, len(row.Candidates), row.CandidatesTruncated)
		}
	}
	if !found {
		t.Fatal("identity resolution missing from evidence response")
	}

	seen := make(map[int64]bool)
	cursor := ""
	for {
		requestPath := "/api/identity-resolutions/" + stringID(resolutionID) + "/candidates?run_id=" + stringID(runID) + "&limit=20"
		if cursor != "" {
			requestPath += "&cursor=" + url.QueryEscape(cursor)
		}
		response := viewerRequest(t, handler, requestPath)
		if response.Code != http.StatusOK {
			t.Fatalf("candidate page status=%d body=%s", response.Code, response.Body.String())
		}
		var envelope struct {
			Items      []map[string]any `json:"items"`
			HasMore    bool             `json:"has_more"`
			NextCursor string           `json:"next_cursor"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		for _, item := range envelope.Items {
			id := int64(item["id"].(float64))
			if seen[id] {
				t.Fatalf("candidate %d appeared on multiple pages", id)
			}
			seen[id] = true
		}
		if !envelope.HasMore {
			break
		}
		if envelope.NextCursor == "" {
			t.Fatal("candidate page reported more rows without a cursor")
		}
		cursor = envelope.NextCursor
	}
	if len(seen) != 105 {
		t.Fatalf("traversed %d candidates, want 105", len(seen))
	}

	wrongRun := viewerRequest(t, handler, "/api/identity-resolutions/"+stringID(resolutionID)+"/candidates?run_id="+stringID(runID+1))
	if wrongRun.Code != http.StatusNotFound {
		t.Fatalf("wrong-run candidate status=%d body=%s", wrongRun.Code, wrongRun.Body.String())
	}
	badCursor := viewerRequest(t, handler, "/api/identity-resolutions/"+stringID(resolutionID)+"/candidates?run_id="+stringID(runID)+"&cursor=not-a-cursor")
	if badCursor.Code != http.StatusBadRequest {
		t.Fatalf("invalid candidate cursor status=%d body=%s", badCursor.Code, badCursor.Body.String())
	}
}
