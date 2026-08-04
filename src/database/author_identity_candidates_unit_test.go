// Unit tests for author identity status constants.
//go:build unit

package database

import (
	"testing"
)

// TestAuthorIdentityStatusConstantsAreValid verifies author identity status constants are valid.
func TestAuthorIdentityStatusConstantsAreValid(t *testing.T) {
	statuses := []string{
		AuthorIdentityStatusORCIDUnclear,
		AuthorIdentityStatusNoORCIDCandidate,
		AuthorIdentityStatusProviderFailed,
		AuthorIdentityStatusConfirmed,
		AuthorIdentityStatusRejected,
	}
	for _, s := range statuses {
		if s == "" {
			t.Fatal("empty status constant")
		}
	}
}
