// identity_evidence_integration_test.go tests the identity evidence endpoint.
//
//go:build integration

package server

import (
	"net/http"
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
