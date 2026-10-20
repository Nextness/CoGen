// main_unit_test.go tests raw SQL ownership scanning and classification with
// synthetic repository trees.

package rawsqlcheck_test

import subject "analysis/tooling/go/rawsqlcheck"

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSourceFile creates one maintained source file beneath root.
func writeSourceFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestScanClassifiesApprovedAndTestCalls verifies approved and test locations are classified without failures.
func TestScanClassifiesApprovedAndTestCalls(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/database/platform.go", "package database\n\nfunc configure(db handle) {\n\tdb.Exec(\"PRAGMA foreign_keys=ON\")\n}\n")
	writeSourceFile(t, root, "backend/database/family_test.go", "package database\n\nfunc TestFamily(t *testing.T) {\n\tdb.RawDBForTest().QueryRow(\"SELECT 1\")\n}\n")
	locations := []subject.approvedLocation{{Path: "backend/database/platform.go", Category: subject.categoryPlatform, Reason: "platform behavior."}}
	report, err := subject.scanRepository(root, locations)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Calls) != 2 || len(report.Unclassified) != 0 {
		t.Fatalf("calls=%d unclassified=%d, want 2 and 0", len(report.Calls), len(report.Unclassified))
	}
	if failures := report.failures(); len(failures) != 0 {
		t.Fatalf("failures = %v, want none", failures)
	}
	if report.Calls[0].Category != subject.categoryTest && report.Calls[0].Category != subject.categoryPlatform {
		t.Fatalf("unexpected category %q", report.Calls[0].Category)
	}
}

// TestScanExcludesGeneratedOutputAndNearMisses verifies reserved internal/sql output is excluded while near-miss paths remain scanned.
func TestScanExcludesGeneratedOutputAndNearMisses(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/database/cache/internal/sql/db.go", "package sql\n\nfunc generated(q handle) {\n\tq.QueryContext(\"SELECT 1\")\n}\n")
	writeSourceFile(t, root, "backend/database/cache/internal/sql/nested/models.go", "package nested\n\nfunc nestedGenerated(q handle) {\n\tq.QueryRow(\"SELECT 1\")\n}\n")
	writeSourceFile(t, root, "backend/database/cache/internal/sqltest/db.go", "package sqltest\n\nfunc nearMiss(q handle) {\n\tq.QueryContext(\"SELECT 1\")\n}\n")
	report, err := subject.scanRepository(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Calls) != 0 {
		t.Fatalf("generated calls were classified: %+v", report.Calls)
	}
	if len(report.Unclassified) != 1 || report.Unclassified[0].Path != "backend/database/cache/internal/sqltest/db.go" {
		t.Fatalf("near miss = %+v, want one unclassified internal/sqltest call", report.Unclassified)
	}
}

// TestScanExcludesRequestURLQuery verifies HTTP request URL parsing is not treated as SQL.
func TestScanExcludesRequestURLQuery(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/server/handler.go", "package server\n\nfunc handle(r *request) string {\n\treturn r.URL.Query().Get(\"q\")\n}\n")
	report, err := subject.scanRepository(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Calls) != 0 || len(report.Unclassified) != 0 {
		t.Fatalf("URL query was classified: calls=%+v unclassified=%+v", report.Calls, report.Unclassified)
	}
}

// TestScanDetectsEveryRawSQLMethod verifies Query, QueryRow, Exec, and prepared-statement calls are all detected.
func TestScanDetectsEveryRawSQLMethod(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/server/mixed.go", "package server\n\nfunc mixed(db handle) {\n\tdb.Query(\"SELECT 1\")\n\tdb.QueryContext(nil, \"SELECT 1\")\n\tdb.QueryRow(\"SELECT 1\")\n\tdb.QueryRowContext(nil, \"SELECT 1\")\n\tdb.Exec(\"UPDATE t SET x=1\")\n\tdb.ExecContext(nil, \"UPDATE t SET x=1\")\n\tdb.Prepare(\"SELECT 1\")\n\tdb.PrepareContext(nil, \"SELECT 1\")\n}\n")
	report, err := subject.scanRepository(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Unclassified) != 8 {
		t.Fatalf("unclassified=%d, want 8: %+v", len(report.Unclassified), report.Unclassified)
	}
}

// TestCheckFailsWithFileMethodAndRemediation verifies an unclassified call fails with actionable output.
func TestCheckFailsWithFileMethodAndRemediation(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/database/foo.go", "package database\n\nfunc load(db handle) {\n\tdb.QueryRowContext(nil, \"SELECT 1\")\n}\n")
	var stdout, stderr bytes.Buffer
	if status := subject.checkLocations(root, nil, &stdout, &stderr); status != 1 {
		t.Fatalf("status = %d, want 1; stderr=%s", status, stderr.String())
	}
	for _, want := range []string{"backend/database/foo.go:4", "QueryRowContext", "approvedLocations", "failed with 1 error(s)"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr %q does not contain %q", stderr.String(), want)
		}
	}
}

// TestCheckPassesForClassifiedRepository verifies a fully classified tree exits successfully.
func TestCheckPassesForClassifiedRepository(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/database/platform.go", "package database\n\nfunc configure(db handle) {\n\tdb.Exec(\"PRAGMA foreign_keys=ON\")\n}\n")
	writeSourceFile(t, root, "backend/database/family_test.go", "package database\n\nfunc TestFamily(t *testing.T) {\n\tdb.RawDBForTest().QueryRow(\"SELECT 1\")\n}\n")
	locations := []subject.approvedLocation{{Path: "backend/database/platform.go", Category: subject.categoryPlatform, Reason: "platform behavior."}}
	var stdout, stderr bytes.Buffer
	if status := subject.checkLocations(root, locations, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, want 0; stderr=%s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "passed: 2 classified calls") {
		t.Fatalf("stdout %q does not report the classified call count", stdout.String())
	}
}

// TestScanRejectsStaleApprovedLocations verifies missing and unused approved entries fail with remediation.
func TestScanRejectsStaleApprovedLocations(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/database/platform.go", "package database\n\nfunc configure(db handle) {\n\tdb.Exec(\"PRAGMA foreign_keys=ON\")\n}\n")
	locations := []subject.approvedLocation{
		{Path: "backend/database/platform.go", Category: subject.categoryPlatform, Reason: "platform behavior."},
		{Path: "backend/database/missing.go", Category: subject.categoryPlatform, Reason: "removed file."},
		{Path: "backend/database/unused.go", Category: subject.categoryPlatform, Reason: "no calls."},
	}
	writeSourceFile(t, root, "backend/database/unused.go", "package database\n\nfunc idle() {}\n")
	report, err := subject.scanRepository(root, locations)
	if err != nil {
		t.Fatal(err)
	}
	failures := report.failures()
	if len(failures) != 2 {
		t.Fatalf("failures = %v, want missing and unused entries", failures)
	}
	for _, want := range []string{"backend/database/missing.go does not exist", "backend/database/unused.go has no raw SQL calls"} {
		if !strings.Contains(strings.Join(failures, "\n"), want) {
			t.Fatalf("failures %v do not contain %q", failures, want)
		}
	}
}

// TestDynamicLocationRequiresBuilderSibling verifies dynamic entries resolve their declared builder file.
func TestDynamicLocationRequiresBuilderSibling(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/database/work/corpus.go", "package work\n\nfunc corpus(db handle) {\n\tdb.QueryRowContext(nil, \"SELECT 1\")\n}\n")
	locations := []subject.approvedLocation{{Path: "backend/database/work/corpus.go", Category: subject.categoryDynamic, Builder: "corpus_dynamic.go", Reason: "dynamic fragments."}}
	report, err := subject.scanRepository(root, locations)
	if err != nil {
		t.Fatal(err)
	}
	if failures := report.failures(); len(failures) != 1 || !strings.Contains(failures[0], "corpus_dynamic.go") {
		t.Fatalf("failures = %v, want one missing builder failure", failures)
	}
	writeSourceFile(t, root, "backend/database/work/corpus_dynamic.go", "package work\n\nfunc corpusFilterClauses() string { return \"\" }\n")
	report, err = subject.scanRepository(root, locations)
	if err != nil {
		t.Fatal(err)
	}
	if failures := report.failures(); len(failures) != 0 {
		t.Fatalf("failures = %v, want none after the builder exists", failures)
	}
}

// TestScanRejectsIncompleteOrDuplicateLocations verifies malformed ownership tables are rejected.
func TestScanRejectsIncompleteOrDuplicateLocations(t *testing.T) {
	root := t.TempDir()
	if _, err := subject.scanRepository(root, []subject.approvedLocation{{Path: "backend/database/foo.go"}}); err == nil {
		t.Fatal("expected a missing category and reason to be rejected")
	}
	if _, err := subject.scanRepository(root, []subject.approvedLocation{{Path: "backend/database/foo.go", Category: subject.categoryDynamic, Reason: "dynamic."}}); err == nil {
		t.Fatal("expected a dynamic entry without a builder to be rejected")
	}
	if _, err := subject.scanRepository(root, []subject.approvedLocation{{Path: "backend/database/foo.go", Category: "unknown", Reason: "unknown."}}); err == nil {
		t.Fatal("expected an unknown category to be rejected")
	}
	if _, err := subject.scanRepository(root, []subject.approvedLocation{{Path: "backend/database/family_test.go", Category: subject.categoryPlatform, Reason: "test."}}); err == nil {
		t.Fatal("expected a test-file location to be rejected")
	}
	duplicate := []subject.approvedLocation{
		{Path: "backend/database/foo.go", Category: subject.categoryPlatform, Reason: "first."},
		{Path: "backend/database/foo.go", Category: subject.categoryPlatform, Reason: "second."},
	}
	if _, err := subject.scanRepository(root, duplicate); err == nil {
		t.Fatal("expected a duplicate path to be rejected")
	}
}

// TestCheckReportsParseFailure verifies malformed Go source exits with the could-not-run status.
func TestCheckReportsParseFailure(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/database/broken.go", "package database\n\nfunc broken( {\n")
	var stdout, stderr bytes.Buffer
	if status := subject.Run([]string{"check", "--root", root}, &stdout, &stderr); status != 2 {
		t.Fatalf("status = %d, want 2; stderr=%s", status, stderr.String())
	}
	if !strings.Contains(stderr.String(), "could not run") {
		t.Fatalf("stderr %q does not report the parse failure", stderr.String())
	}
}

// TestReportPrintsCategoriesAndUnclassifiedCalls verifies the ownership report summarizes classifications.
func TestReportPrintsCategoriesAndUnclassifiedCalls(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "backend/database/foo.go", "package database\n\nfunc load(db handle) {\n\tdb.QueryRowContext(nil, \"SELECT 1\")\n}\n")
	var stdout, stderr bytes.Buffer
	if status := subject.Run([]string{"report", "--root", root}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, want 0; stderr=%s", status, stderr.String())
	}
	for _, want := range []string{"Raw SQL ownership report", "unclassified", "backend/database/foo.go:4: QueryRowContext"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report %q does not contain %q", stdout.String(), want)
		}
	}
}

// TestRunRejectsUnknownCommandAndExtraArguments verifies the command hierarchy rejects invalid usage.
func TestRunRejectsUnknownCommandAndExtraArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := subject.Run([]string{"inspect"}, &stdout, &stderr); status != 2 {
		t.Fatalf("unknown command status = %d, want 2", status)
	}
	stdout.Reset()
	stderr.Reset()
	if status := subject.Run([]string{"check", "extra"}, &stdout, &stderr); status != 2 {
		t.Fatalf("extra argument status = %d, want 2", status)
	}
}
