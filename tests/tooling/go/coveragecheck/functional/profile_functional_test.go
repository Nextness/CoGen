// profile_functional_test.go verifies parsing and aggregation through the public coverage command.
package coveragecheck_test

import (
	"strings"
	"testing"
)

// TestReadProfileMergesRepeatedBlocksByHighestHitCount verifies duplicate blocks count once, retaining their highest hit count.
func TestReadProfileMergesRepeatedBlocksByHighestHitCount(t *testing.T) {
	output, err := runProfile(t, "mode: atomic\nanalysis/backend/main.go:1.1,1.2 2 0\nanalysis/backend/main.go:1.1,1.2 2 3\nanalysis/backend/article/article.go:1.1,1.2 1 0\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Tracked total: 66.7% (2/3 statements)") {
		t.Fatalf("aggregation=%s", output)
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "analysis/backend/main.go") && !strings.Contains(line, "100.0%") {
			t.Fatalf("file coverage=%s", line)
		}
		if strings.HasPrefix(line, "analysis/backend/article ") && !strings.Contains(line, "0.0%") {
			t.Fatalf("package coverage=%s", line)
		}
	}
}

// TestReadProfileRejectsMalformedInput verifies malformed profiles fail through the actual command diagnostics.
func TestReadProfileRejectsMalformedInput(t *testing.T) {
	for _, profile := range []string{"", "missing header\n", "mode: atomic\ninvalid\n", "mode: atomic\nanalysis/main.go:1.1,1.2 not-a-number 0\n", "mode: atomic\nanalysis/main.go:1.1,1.2 -1 0\n", "mode: atomic\nanalysis/main.go:1.1,1.2 1 -1\n", "mode: atomic\nno-file 1 0\n"} {
		if _, err := runProfile(t, profile); err == nil || !strings.Contains(err.Error(), "coverage profile") {
			t.Fatalf("profile=%q error=%v", profile, err)
		}
	}
}

// TestCoveragePercentTreatsEmptyScopeAsComplete verifies zero-statement scopes retain complete coverage.
func TestCoveragePercentTreatsEmptyScopeAsComplete(t *testing.T) {
	output, err := runProfile(t, "mode: atomic\nanalysis/backend/main.go:1.1,1.2 0 0\nanalysis/backend/article/article.go:1.1,1.2 0 0\n")
	if err != nil || !strings.Contains(output, "Tracked total: 100.0% (0/0 statements)") {
		t.Fatalf("output=%s error=%v", output, err)
	}
}
