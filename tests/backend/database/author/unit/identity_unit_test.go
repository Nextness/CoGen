// Unit tests for author identity status constants.

package author_test

import subject "analysis/backend/database/author"

import (
	"analysis/tests/support/go/testfixture"
	"context"
	"strings"
	"testing"
)

// TestIdentityStatusConstantsAreValid verifies author identity status constants are valid.
func TestIdentityStatusConstantsAreValid(t *testing.T) {
	statuses := []string{
		subject.IdentityStatusORCIDUnclear,
		subject.IdentityStatusNoORCIDCandidate,
		subject.IdentityStatusProviderFailed,
		subject.IdentityStatusConfirmed,
		subject.IdentityStatusRejected,
	}
	for _, status := range statuses {
		if status == "" {
			t.Fatal("empty status constant")
		}
		if !acceptsIdentityStatus(status) {
			t.Fatalf("status %q is not accepted by validIdentityStatus", status)
		}
	}
	if acceptsIdentityStatus("bogus") {
		t.Fatal("expected an unknown status to be rejected")
	}
}

// acceptsIdentityStatus verifies status acceptance before a deliberately closed storage boundary.
func acceptsIdentityStatus(status string) bool {
	_, err := subject.New(testfixture.ClosedDB()).CreateIdentityResolution(context.Background(), &subject.IdentityResolution{PipelineRunID: 1, AuthorOccurrenceID: 1, Status: status, Provider: "orcid", QueriedCitationName: "Author", ResolvedAt: "2026-01-01T00:00:00Z"})
	return err != nil && strings.Contains(err.Error(), "database is closed")
}
