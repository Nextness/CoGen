// main_unit_test.go tests import graph loading, layer classification, and rule
// enforcement with synthetic repository trees.
//go:build unit

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile creates one maintained source file beneath root.
func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeSQLCFixture writes one minimal valid sqlc unit for the work family.
func writeSQLCFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "config/sqlc.yaml", `version: "2"
sql:
  - engine: "sqlite"
    schema: "../src/database/work/schema/definitions.sql"
    queries:
      - "../src/database/work/schema/work.sql"
    gen:
      go:
        package: "sql"
        out: "../src/database/work/internal/sql"
`)
	writeTestFile(t, root, "src/database/work/schema/definitions.sql", "CREATE TABLE work (id INTEGER);\n")
	writeTestFile(t, root, "src/database/work/schema/work.sql", "-- name: GetWork :one\nSELECT 1;\n")
	writeTestFile(t, root, "src/database/work/internal/sql/db.go", "package sql\n")
}

// writeModuleFixture writes one minimal module with two families.
func writeModuleFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "src/go.mod", "module analysis\n\ngo 1.25.0\n")
	writeTestFile(t, root, "src/database/work/work.go", "package work\n")
	writeTestFile(t, root, "src/database/run/run.go", "package run\n")
}

// graphFixture returns one synthetic graph for rule tests.
func graphFixture(edges ...importEdge) packageGraph {
	return packageGraph{
		Module:   "analysis",
		Families: map[string]bool{"work": true, "run": true},
		Packages: map[string]bool{"analysis/database/work": true, "analysis/database/run": true},
		Edges:    edges,
	}
}

// TestLoadGraphClassifiesPackagesAndEdges verifies family detection and production/test edge separation.
func TestLoadGraphClassifiesPackagesAndEdges(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root)
	writeTestFile(t, root, "src/database/work/work.go", "package work\n\nimport (\n\t\"analysis/manifest\"\n\t\"context\"\n)\n")
	writeTestFile(t, root, "src/database/work/fixture_test.go", "package work_test\n\nimport (\n\t\"analysis/database\"\n\t\"analysis/database/run\"\n)\n")
	writeTestFile(t, root, "src/server/server.go", "package server\n\nimport \"analysis/database/work\"\n")
	graph, err := loadGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if !graph.Families["work"] || !graph.Families["run"] || len(graph.Families) != 2 {
		t.Fatalf("families = %v, want work and run", graph.Families)
	}
	if graph.layerOf("analysis/database/work") != layerFamily ||
		graph.layerOf("analysis/database") != layerRoot ||
		graph.layerOf("analysis/pdfstore") != layerCompanion ||
		graph.layerOf("analysis/server") != layerConsumer ||
		graph.layerOf("analysis/manifest") != layerShared ||
		graph.layerOf("context") != layerExternal {
		t.Fatal("layer classification does not match the approved graph")
	}
	var productionTargets, testTargets []string
	for _, edge := range graph.Edges {
		if edge.Importer != "analysis/database/work" {
			continue
		}
		if edge.Test {
			testTargets = append(testTargets, edge.Target)
		} else {
			productionTargets = append(productionTargets, edge.Target)
		}
	}
	if strings.Join(sortedUnique(productionTargets), ",") != "analysis/manifest,context" {
		t.Fatalf("production targets = %v", productionTargets)
	}
	if strings.Join(sortedUnique(testTargets), ",") != "analysis/database,analysis/database/run" {
		t.Fatalf("test targets = %v", testTargets)
	}
}

// TestLoadGraphRejectsMissingFamilies verifies a tree without metadata families cannot be verified.
func TestLoadGraphRejectsMissingFamilies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "src/go.mod", "module analysis\n")
	writeTestFile(t, root, "src/server/server.go", "package server\n")
	if _, err := loadGraph(root); err == nil {
		t.Fatal("expected a missing family tree to be rejected")
	}
}

// TestVerifyGraphRejectsFamilyProductionImports verifies production family isolation for every forbidden target.
func TestVerifyGraphRejectsFamilyProductionImports(t *testing.T) {
	graph := graphFixture(
		importEdge{Importer: "analysis/database/work", Target: "analysis/database/run"},
		importEdge{Importer: "analysis/database/work", Target: "analysis/database"},
		importEdge{Importer: "analysis/database/work", Target: "analysis/server"},
		importEdge{Importer: "analysis/database/work", Target: "analysis/enrich"},
	)
	violations := verifyGraph(graph)
	if len(violations) != 4 {
		t.Fatalf("violations = %v, want four", violations)
	}
	if !strings.Contains(strings.Join(violations, "\n"), "sibling families must not call one another inside transactions") {
		t.Fatalf("violations %v do not state the transaction rule", violations)
	}
}

// TestVerifyGraphAllowsApprovedFamilyImports verifies own-subtree and shared low-level imports pass.
func TestVerifyGraphAllowsApprovedFamilyImports(t *testing.T) {
	graph := graphFixture(
		importEdge{Importer: "analysis/database/work", Target: "analysis/database/work/internal/sql"},
		importEdge{Importer: "analysis/database/work", Target: "analysis/internal/textlimit"},
		importEdge{Importer: "analysis/database/work", Target: "analysis/logging"},
		importEdge{Importer: "analysis/database/work", Target: "analysis/manifest"},
		importEdge{Importer: "analysis/database/work", Target: "analysis/notes"},
		importEdge{Importer: "analysis/database/work", Target: "database/sql"},
	)
	if violations := verifyGraph(graph); len(violations) != 0 {
		t.Fatalf("violations = %v, want none", violations)
	}
}

// TestVerifyGraphAllowsFamilyTestComposition verifies fixtures may compose the root and sibling families.
func TestVerifyGraphAllowsFamilyTestComposition(t *testing.T) {
	graph := graphFixture(
		importEdge{Importer: "analysis/database/work", Target: "analysis/database", Test: true},
		importEdge{Importer: "analysis/database/work", Target: "analysis/database/run", Test: true},
		importEdge{Importer: "analysis/database/work", Target: "analysis/database/work/internal/sql", Test: true},
	)
	if violations := verifyGraph(graph); len(violations) != 0 {
		t.Fatalf("violations = %v, want none", violations)
	}
}

// TestVerifyGraphRejectsFamilyTestConsumers verifies fixtures must not compose consumers or the companion store.
func TestVerifyGraphRejectsFamilyTestConsumers(t *testing.T) {
	graph := graphFixture(
		importEdge{Importer: "analysis/database/work", Target: "analysis/server", Test: true},
		importEdge{Importer: "analysis/database/work", Target: "analysis/pdfstore", Test: true},
	)
	violations := verifyGraph(graph)
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want two", violations)
	}
}

// TestVerifyGraphRejectsGeneratedOutputLeaks verifies generated output stays inside its owning package tree.
func TestVerifyGraphRejectsGeneratedOutputLeaks(t *testing.T) {
	graph := graphFixture(
		importEdge{Importer: "analysis/database/run", Target: "analysis/database/work/internal/sql"},
	)
	violations := verifyGraph(graph)
	if len(violations) != 1 || !strings.Contains(violations[0], "generated types stay inside") {
		t.Fatalf("violations = %v, want one generated output leak", violations)
	}
}

// TestVerifyGraphRejectsRootCompanionAndSharedDependencies verifies the remaining dependency direction rules.
func TestVerifyGraphRejectsRootCompanionAndSharedDependencies(t *testing.T) {
	graph := graphFixture(
		importEdge{Importer: "analysis/database", Target: "analysis/server"},
		importEdge{Importer: "analysis/pdfstore", Target: "analysis/workspace"},
		importEdge{Importer: "analysis/manifest", Target: "analysis/database/work"},
	)
	violations := verifyGraph(graph)
	if len(violations) != 3 {
		t.Fatalf("violations = %v, want three", violations)
	}
}

// TestVerifyUnitsAcceptsOwnedUnits verifies a consistent generation unit passes.
func TestVerifyUnitsAcceptsOwnedUnits(t *testing.T) {
	root := t.TempDir()
	writeSQLCFixture(t, root)
	units, err := loadSQLCUnits(root)
	if err != nil {
		t.Fatal(err)
	}
	if failures := verifyUnits(root, units); len(failures) != 0 {
		t.Fatalf("failures = %v, want none", failures)
	}
}

// TestVerifyUnitsRejectsOwnershipProblems verifies mismatched owners and undeclared query files fail.
func TestVerifyUnitsRejectsOwnershipProblems(t *testing.T) {
	root := t.TempDir()
	writeSQLCFixture(t, root)
	writeTestFile(t, root, "src/database/work/schema/undeclared.sql", "-- name: Undeclared :one\nSELECT 1;\n")
	units, err := loadSQLCUnits(root)
	if err != nil {
		t.Fatal(err)
	}
	units[0].Output = "../src/database/run/internal/sql"
	failures := strings.Join(verifyUnits(root, units), "\n")
	for _, want := range []string{"must share the ../src/database/work owner directory", "undeclared.sql is not declared"} {
		if !strings.Contains(failures, want) {
			t.Fatalf("failures %q do not contain %q", failures, want)
		}
	}
}

// TestVerifyUnitsRejectsDuplicatesAndDefinitionsQueries verifies duplicate and definitions query listings fail.
func TestVerifyUnitsRejectsDuplicatesAndDefinitionsQueries(t *testing.T) {
	root := t.TempDir()
	writeSQLCFixture(t, root)
	units, err := loadSQLCUnits(root)
	if err != nil {
		t.Fatal(err)
	}
	duplicated := append([]sqlcUnit{}, units...)
	duplicated = append(duplicated, sqlcUnit{Schema: units[0].Schema, Queries: []string{units[0].Queries[0]}, Output: units[0].Output})
	failures := strings.Join(verifyUnits(root, duplicated), "\n")
	if !strings.Contains(failures, "lists ../src/database/work/schema/work.sql in both") {
		t.Fatalf("failures %q do not report the duplicate query", failures)
	}
	units[0].Queries = append(units[0].Queries, "../src/database/work/schema/definitions.sql")
	failures = strings.Join(verifyUnits(root, units), "\n")
	if !strings.Contains(failures, "must not list ../src/database/work/schema/definitions.sql as a query file") {
		t.Fatalf("failures %q do not report the definitions query", failures)
	}
}

// TestCheckFailsWithFileAndRemediation verifies a family production violation fails with actionable output.
func TestCheckFailsWithFileAndRemediation(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root)
	writeSQLCFixture(t, root)
	writeTestFile(t, root, "src/database/work/work.go", "package work\n\nimport \"analysis/database/run\"\n")
	var stdout, stderr bytes.Buffer
	if status := run([]string{"check", "--root", root}, &stdout, &stderr); status != 1 {
		t.Fatalf("status = %d, want 1; stderr=%s", status, stderr.String())
	}
	for _, want := range []string{"analysis/database/work imports sibling family analysis/database/run", "must not call one another inside transactions", "failed with 1 error(s)"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr %q does not contain %q", stderr.String(), want)
		}
	}
}

// TestCheckPassesForApprovedGraph verifies a compliant tree exits successfully.
func TestCheckPassesForApprovedGraph(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root)
	writeSQLCFixture(t, root)
	var stdout, stderr bytes.Buffer
	if status := run([]string{"check", "--root", root}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, want 0; stderr=%s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "import graph check passed: 3 packages, 2 families, 1 sqlc units") {
		t.Fatalf("stdout = %q, want a passing check summary", stdout.String())
	}
}

// TestReportSummarizesLayersFamiliesAndInvariants verifies the report content.
func TestReportSummarizesLayersFamiliesAndInvariants(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root)
	writeSQLCFixture(t, root)
	writeTestFile(t, root, "src/database/work/fixture_test.go", "package work_test\n\nimport \"analysis/database/run\"\n")
	var stdout, stderr bytes.Buffer
	if status := run([]string{"report", "--root", root}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, want 0; stderr=%s", status, stderr.String())
	}
	for _, want := range []string{"Import and transaction graph report", "family production imports", "analysis/database/work", "family test fixture composition", "sqlc units", "verified invariants", "sibling families have no production imports"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report %q does not contain %q", stdout.String(), want)
		}
	}
}

// TestScanRepositoryReportsSQLCErrors verifies a missing sqlc config fails the analysis.
func TestScanRepositoryReportsSQLCErrors(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root)
	if _, err := scanRepository(root); err == nil {
		t.Fatal("expected a missing config/sqlc.yaml to fail the scan")
	}
}

// TestRunRejectsUnknownCommandAndExtraArguments verifies the command hierarchy rejects invalid usage.
func TestRunRejectsUnknownCommandAndExtraArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"inspect"}, &stdout, &stderr); status != 2 {
		t.Fatalf("unknown command status = %d, want 2", status)
	}
	stdout.Reset()
	stderr.Reset()
	if status := run([]string{"check", "extra"}, &stdout, &stderr); status != 2 {
		t.Fatalf("extra argument status = %d, want 2", status)
	}
}
