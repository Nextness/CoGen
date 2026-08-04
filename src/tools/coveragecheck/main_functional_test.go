// main_functional_test.go tests the coverage policy checker through the
// full check function with temporary files.
//go:build functional

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckReportsTrackedCoverageAndRejectsRegression verifies check reports tracked coverage and rejects regression.
func TestCheckReportsTrackedCoverageAndRejectsRegression(t *testing.T) {
	tempDir := t.TempDir()
	profilePath := filepath.Join(tempDir, "coverage.out")
	policyPath := filepath.Join(tempDir, "policy.something")
	profile := "mode: atomic\nanalysis/main.go:1.1,1.2 2 1\nanalysis/main.go:2.1,2.2 2 0\n"
	policy := "" +
		"available_coverage_modes: enum = { ATOMIC; }\n" +
		"minimums_coverage_config: setup = { minimum: float; rationale?: string = \"\"; description?: string = \"\"; }\n" +
		"coverage_policy_config: setup = { version: integer; coverage_mode: available_coverage_modes; exclude_packages: []string; package_minimums: mapping(string, minimums_coverage_config); file_minimums: mapping(string, minimums_coverage_config); }\n" +
		"coverage_policy: coverage_policy_config = { version = 1, coverage_mode = .ATOMIC, exclude_packages = []string{}, package_minimums = mapping(string, minimums_coverage_config) { [\"analysis\"] => { minimum = 50.0 } }, file_minimums = mapping(string, minimums_coverage_config) { [\"analysis/main.go\"] => { minimum = 50.0, rationale = \"fixture\" } } };\n"
	if err := os.WriteFile(profilePath, []byte(profile), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(policy), 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := check(profilePath, policyPath, &output); err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, expected := range []string{"Packages", "High-risk files", "Tracked total", "50.0%"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("report missing %q: %s", expected, output.String())
		}
	}

	policy = "" +
		"available_coverage_modes: enum = { ATOMIC; }\n" +
		"minimums_coverage_config: setup = { minimum: float; rationale?: string = \"\"; description?: string = \"\"; }\n" +
		"coverage_policy_config: setup = { version: integer; coverage_mode: available_coverage_modes; exclude_packages: []string; package_minimums: mapping(string, minimums_coverage_config); file_minimums: mapping(string, minimums_coverage_config); }\n" +
		"coverage_policy: coverage_policy_config = { version = 1, coverage_mode = .ATOMIC, exclude_packages = []string{}, package_minimums = mapping(string, minimums_coverage_config) { [\"analysis\"] => { minimum = 100.0 } }, file_minimums = mapping(string, minimums_coverage_config) { [\"analysis/main.go\"] => { minimum = 50.0, rationale = \"fixture\" } } };\n"
	if err := os.WriteFile(policyPath, []byte(policy), 0644); err != nil {
		t.Fatal(err)
	}
	if err := check(profilePath, policyPath, &output); err == nil || !strings.Contains(err.Error(), "below") {
		t.Fatalf("coverage regression error = %v", err)
	}
}
