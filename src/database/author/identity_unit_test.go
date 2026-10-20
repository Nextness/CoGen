// Unit tests for author identity status constants.
//go:build unit

package author

import (
	"testing"
)

// TestIdentityStatusConstantsAreValid verifies author identity status constants are valid.
func TestIdentityStatusConstantsAreValid(t *testing.T) {
	statuses := []string{
		IdentityStatusORCIDUnclear,
		IdentityStatusNoORCIDCandidate,
		IdentityStatusProviderFailed,
		IdentityStatusConfirmed,
		IdentityStatusRejected,
	}
	for _, status := range statuses {
		if status == "" {
			t.Fatal("empty status constant")
		}
		if !validIdentityStatus(status) {
			t.Fatalf("status %q is not accepted by validIdentityStatus", status)
		}
	}
	if validIdentityStatus("bogus") {
		t.Fatal("expected an unknown status to be rejected")
	}
}
