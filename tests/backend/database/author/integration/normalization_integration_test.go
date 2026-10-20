// author_normalization_integration_test.go verifies persisted ORCID canonicalization.
package author_test

import (
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
	"context"
	"path/filepath"
	"testing"
)

// TestNormalizeORCIDTrimsAndLowercases verifies the supported insert and lookup preserve canonical identity.
func TestNormalizeORCIDTrimsAndLowercases(t *testing.T) {
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "author.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := db.Author.CreatePersonByORCID(context.Background(), "  0000-0002-1694-233X  ")
	if err != nil {
		t.Fatal(err)
	}
	person, err := db.Author.GetPersonByID(context.Background(), id)
	if err != nil || person == nil {
		t.Fatalf("person=%+v err=%v", person, err)
	}
	if person.ORCID != "0000-0002-1694-233x" {
		t.Fatalf("normalized ORCID=%q", person.ORCID)
	}
	lookup, err := db.Author.GetPersonByORCID(context.Background(), " 0000-0002-1694-233X ")
	if err != nil || lookup == nil || lookup.ID != id {
		t.Fatalf("normalized lookup=%+v err=%v", lookup, err)
	}
}
