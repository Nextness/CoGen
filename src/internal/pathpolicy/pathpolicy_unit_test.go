// pathpolicy_unit_test.go verifies lexical and symbolic-link containment resolution.
package pathpolicy

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveExistingWithinRejectsEscapes verifies contained paths resolve while lexical and symbolic-link escapes fail.
func TestResolveExistingWithinRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	outside := t.TempDir()
	if err := os.WriteFile(inside, []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if resolved, err := ResolveExistingWithin(root, "inside"); err != nil || resolved != inside {
		t.Fatalf("contained path = %q, %v", resolved, err)
	}
	for _, candidate := range []string{"../outside", "escape"} {
		if _, err := ResolveExistingWithin(root, candidate); err == nil {
			t.Fatalf("ResolveExistingWithin(%q) unexpectedly succeeded", candidate)
		}
	}
}

// TestResolveExistingComponentsWithinAllowsNewFinalFile verifies a missing final file retains resolved containment.
func TestResolveExistingComponentsWithinAllowsNewFinalFile(t *testing.T) {
	root := t.TempDir()
	resolved, err := ResolveExistingComponentsWithin(root, filepath.Join("new", "companion.db"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved != filepath.Join(root, "new", "companion.db") {
		t.Fatalf("resolved path = %q", resolved)
	}
}
