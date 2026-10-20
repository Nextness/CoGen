// main_functional_test.go tests the coverage policy checker through the
// full check function with temporary files.

package coveragecheck_test

import subject "analysis/tooling/go/coveragecheck"

import (
	"bytes"
	"fmt"
	"io"
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
	if err := runCheck(profilePath, policyPath, &output); err != nil {
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
	if err := runCheck(profilePath, policyPath, &output); err == nil || !strings.Contains(err.Error(), "below") {
		t.Fatalf("coverage regression error = %v", err)
	}
}

// TestReadPolicyRejectsNonStringExcludedPackage verifies malformed policy lists
// return a descriptive error instead of panicking during conversion.
func TestReadPolicyRejectsNonStringExcludedPackage(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "policy.something")
	policy := `
CoverageMode: enum = { atomic; }
coverage_policy: scope = {
    version := 1;
    coverage_mode := CoverageMode.atomic;
    exclude_packages := []integer{1};
    package_minimums := mapping(string, integer){};
    file_minimums := mapping(string, integer){};
}
`
	if err := os.WriteFile(policyPath, []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runPolicy(policyPath); err == nil || !strings.Contains(err.Error(), "exclude_packages[0]") {
		t.Fatalf("readPolicy() error = %v, want indexed exclude_packages error", err)
	}
}

// runCheck captures the public command's diagnostics as an error for scenario assertions.
func runCheck(profile, policy string, output io.Writer) error {
	var diagnostic bytes.Buffer
	if code := subject.Run([]string{"--profile", profile, "--policy", policy}, output, &diagnostic); code != 0 {
		return fmt.Errorf("exit %d: %s", code, diagnostic.String())
	}
	return nil
}

// runPolicy exercises policy validation before opening a deliberately absent profile.
func runPolicy(policy string) (int, error) {
	return 0, runCheck("unused-coverage-profile", policy, io.Discard)
}

// runProfile writes a controlled profile and policy and executes the real command.
func runProfile(t *testing.T, profile string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "coverage.out")
	policyPath := filepath.Join(dir, "policy.something")
	policy := `Mode: enum={ATOMIC;}
 coverage_policy: scope = {
 version:=1;coverage_mode:=Mode.ATOMIC;exclude_packages:=[]string{};
 package_minimums:=mapping(string, float){["analysis/backend"]=>0.0,["analysis/backend/article"]=>0.0};
 file_minimums:=mapping(string, float){};
 }`
	// Use real rule objects because the command validates the maintained policy shape.
	policy = strings.ReplaceAll(policy, `mapping(string, float){["analysis/backend"]=>0.0,["analysis/backend/article"]=>0.0}`, `mapping(string, Rule){["analysis/backend"]=>{minimum=0.0},["analysis/backend/article"]=>{minimum=0.0}}`)
	policy = strings.ReplaceAll(policy, `mapping(string, float){}`, `mapping(string, Rule){["analysis/backend/main.go"]=>{minimum=0.0,rationale="fixture"}}`)
	policy = `Rule: setup={minimum:float;rationale?:string="";} ` + policy
	if err := os.WriteFile(profilePath, []byte(profile), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := runCheck(profilePath, policyPath, &output)
	return output.String(), err
}
