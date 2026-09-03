// main_unit_test.go tests the coverage policy checker profile parsing
// and input validation in isolation.
//go:build unit

package main

import (
	"strings"
	"testing"
)

// TestReadProfileMergesRepeatedBlocksByHighestHitCount verifies read profile merges repeated blocks by highest hit count.
func TestReadProfileMergesRepeatedBlocksByHighestHitCount(t *testing.T) {
	profile := "mode: atomic\n" +
		"analysis/main.go:1.1,1.2 2 0\n" +
		"analysis/main.go:1.1,1.2 2 3\n" +
		"analysis/article/article.go:1.1,1.2 1 0\n"
	packages, files, mode, err := readProfile(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	if mode != "atomic" {
		t.Fatalf("mode = %q, want atomic", mode)
	}
	if got := files["analysis/main.go"]; got != (coverage{Covered: 2, Statements: 2}) {
		t.Fatalf("main file coverage = %+v", got)
	}
	if got := packages["analysis/article"]; got != (coverage{Statements: 1}) {
		t.Fatalf("article package coverage = %+v", got)
	}
}

// TestReadProfileRejectsMalformedInput verifies read profile rejects malformed input.
func TestReadProfileRejectsMalformedInput(t *testing.T) {
	for _, profile := range []string{
		"",
		"mode: set\ninvalid\n",
		"mode: atomic\nanalysis/main.go:1.1,1.2 not-a-number 0\n",
		"mode: atomic\nanalysis/main.go:1.1,1.2 -1 0\n",
	} {
		if _, _, _, err := readProfile(strings.NewReader(profile)); err == nil {
			t.Fatalf("readProfile(%q) succeeded", profile)
		}
	}
}

// TestCoveragePercentTreatsEmptyScopeAsComplete verifies the documented empty
// scope policy is represented by the returned percentage.
func TestCoveragePercentTreatsEmptyScopeAsComplete(t *testing.T) {
	if got := (coverage{}).percent(); got != 100 {
		t.Fatalf("empty coverage percent = %v, want 100", got)
	}
}
