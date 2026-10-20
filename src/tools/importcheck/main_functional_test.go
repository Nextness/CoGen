// main_functional_test.go tests import graph enforcement against the real
// repository tree.
//go:build functional

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepositoryImportGraphPasses verifies the committed import and transaction graph satisfies every rule.
func TestRepositoryImportGraphPasses(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	var stdout, stderr bytes.Buffer
	if status := run([]string{"check", "--root", root}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, want 0; stderr=%s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "import graph check passed") {
		t.Fatalf("stdout = %q, want a passing check summary", stdout.String())
	}
}

// TestRepositoryReportListsFamiliesUnitsAndInvariants verifies the report covers every maintained layer.
func TestRepositoryReportListsFamiliesUnitsAndInvariants(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	var stdout, stderr bytes.Buffer
	if status := run([]string{"report", "--root", root}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, want 0; stderr=%s", status, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Import and transaction graph report",
		"analysis/database/work",
		"../src/database/work",
		"../src/pdfstore",
		"../src/tools/prepare-osf",
		"verified invariants",
		"sibling families have no production imports",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("report does not mention %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "violations") {
		t.Fatalf("report contains violations:\n%s", output)
	}
}
