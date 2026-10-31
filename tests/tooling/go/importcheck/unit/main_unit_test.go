// main_unit_test.go exercises import and sqlc ownership rules through the public checker with synthetic repositories.
package importcheck_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/tooling/go/importcheck"
)

// writeTestFile creates a maintained input beneath one synthetic root.
func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// sqlcFixture is a complete owned generation unit accepted by the actual configuration parser.
const sqlcFixture = `version: "2"
sql:
  - engine: "sqlite"
    schema: "../backend/database/work/schema/definitions.sql"
    queries:
      - "../backend/database/work/schema/work.sql"
    gen:
      go:
        package: "sql"
        out: "../backend/database/work/internal/sql"
`

// repositoryFixture writes a root module, two families, and one valid generation unit.
func repositoryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range map[string]string{
		"go.mod":                                       "module analysis\n\ngo 1.25.0\n",
		"backend/database/work/work.go":                "package work\n",
		"backend/database/run/run.go":                  "package run\n",
		"config/sqlc.yaml":                             sqlcFixture,
		"backend/database/work/schema/definitions.sql": "CREATE TABLE work (id INTEGER);\n",
		"backend/database/work/schema/work.sql":        "-- name: GetWork :one\nSELECT 1;\n",
		"backend/database/work/internal/sql/db.go":     "package sql\n",
	} {
		writeTestFile(t, root, path, content)
	}
	return root
}

// invoke captures the public command's streams and process status.
func invoke(root, command string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := importcheck.Run([]string{command, "--root", root}, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestImportRulesThroughSourceTrees verifies every family, platform, companion, shared, generated, and test boundary.
func TestImportRulesThroughSourceTrees(t *testing.T) {
	for _, test := range []struct{ name, path, target, want string }{
		{"sibling family", "backend/database/work/edge.go", "analysis/backend/database/run", "sibling families must not call one another inside transactions"},
		{"family composition root", "backend/database/work/edge.go", "analysis/backend/database", "imports"},
		{"family server", "backend/database/work/edge.go", "analysis/backend/server", "imports"},
		{"family provider", "backend/database/work/edge.go", "analysis/backend/enrich", "imports"},
		{"family own generated", "backend/database/work/edge.go", "analysis/backend/database/work/internal/sql", ""},
		{"family text limit", "backend/database/work/edge.go", "analysis/internal/textlimit", ""},
		{"family logging", "backend/database/work/edge.go", "analysis/backend/logging", ""},
		{"family manifest", "backend/database/work/edge.go", "analysis/backend/manifest", ""},
		{"family notes", "backend/database/work/edge.go", "analysis/backend/notes", ""},
		{"family standard library", "backend/database/work/edge.go", "database/sql", ""},
		{"family test root", "backend/database/work/edge_test.go", "analysis/backend/database", ""},
		{"family test sibling", "backend/database/work/edge_test.go", "analysis/backend/database/run", ""},
		{"family test own generated", "backend/database/work/edge_test.go", "analysis/backend/database/work/internal/sql", ""},
		{"family test consumer", "backend/database/work/edge_test.go", "analysis/backend/server", "imports"},
		{"family test companion", "backend/database/work/edge_test.go", "analysis/backend/pdfstore", "imports"},
		{"generated leak", "backend/database/run/edge.go", "analysis/backend/database/work/internal/sql", "generated types stay inside"},
		{"root consumer", "backend/database/edge.go", "analysis/backend/server", "imports"},
		{"companion orchestration", "backend/pdfstore/edge.go", "analysis/backend/workspace", "imports"},
		{"family bundle", "backend/database/work/edge.go", "analysis/backend/bundle", "imports"},
		{"companion bundle", "backend/pdfstore/edge.go", "analysis/backend/bundle", "imports"},
		{"root bundle", "backend/database/edge.go", "analysis/backend/bundle", "imports"},
		{"bundle workspace", "backend/bundle/edge.go", "analysis/backend/workspace", "the bundle consumer must not depend on workspace or server orchestration"},
		{"bundle server", "backend/bundle/edge.go", "analysis/backend/server", "the bundle consumer must not depend on workspace or server orchestration"},
		{"bundle family", "backend/bundle/edge.go", "analysis/backend/database/work", ""},
		{"bundle companion", "backend/bundle/edge.go", "analysis/backend/pdfstore", ""},
		{"bundle root", "backend/bundle/edge.go", "analysis/backend/database", ""},
		{"shared family", "backend/manifest/edge.go", "analysis/backend/database/work", "imports"},
		{"external tests compose", "tests/backend/database/work/integration/edge_test.go", "analysis/backend/database/run", ""},
		{"external tests generated leak", "tests/backend/database/work/integration/edge_test.go", "analysis/backend/database/work/internal/sql", "generated types stay inside"},
		{"production test dependency", "backend/workspace/edge.go", "analysis/tests/support/go/testfixture", "must not import test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := repositoryFixture(t)
			writeTestFile(t, root, test.path, "package fixture\nimport "+`"`+test.target+`"`+"\n")
			code, output, diagnostic := invoke(root, "check")
			if test.want == "" {
				if code != 0 {
					t.Fatalf("code=%d: %s", code, diagnostic)
				}
			} else if code != 1 || !strings.Contains(diagnostic, test.want) {
				t.Fatalf("code=%d output=%s diagnostic=%s, want %q", code, output, diagnostic, test.want)
			}
		})
	}
}

// TestGraphReportClassifiesProductionAndFixtureEdges verifies both graph views and their invariant descriptions remain visible.
func TestGraphReportClassifiesProductionAndFixtureEdges(t *testing.T) {
	root := repositoryFixture(t)
	writeTestFile(t, root, "backend/database/work/edge.go", "package work\nimport(\"analysis/backend/manifest\";\"context\")\n")
	writeTestFile(t, root, "backend/database/work/edge_test.go", "package work_test\nimport \"analysis/backend/database/run\"\n")
	code, output, diagnostic := invoke(root, "report")
	if code != 0 {
		t.Fatalf("code=%d: %s", code, diagnostic)
	}
	for _, want := range []string{"family production imports", "analysis/backend/manifest", "family test fixture composition", "analysis/backend/database/run", "sqlc units", "verified invariants", "sibling families have no production imports"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in %s", want, output)
		}
	}
}

// TestSQLCUnitsRejectOwnershipAndListingErrors verifies the command reads and validates authoritative YAML and query files.
func TestSQLCUnitsRejectOwnershipAndListingErrors(t *testing.T) {
	for _, test := range []struct{ name, config, extraPath, want string }{
		{"wrong owner", strings.ReplaceAll(sqlcFixture, "out: \"../backend/database/work/internal/sql\"", "out: \"../backend/database/run/internal/sql\""), "", "must share the ../backend/database/work owner directory"},
		{"unlisted query", sqlcFixture, "backend/database/work/schema/undeclared.sql", "undeclared.sql is not declared"},
		{"duplicate query", strings.ReplaceAll(sqlcFixture, "    gen:", "      - \"../backend/database/work/schema/work.sql\"\n    gen:"), "", "lists ../backend/database/work/schema/work.sql in both"},
		{"definitions query", strings.ReplaceAll(sqlcFixture, "    gen:", "      - \"../backend/database/work/schema/definitions.sql\"\n    gen:"), "", "must not list ../backend/database/work/schema/definitions.sql as a query file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := repositoryFixture(t)
			writeTestFile(t, root, "config/sqlc.yaml", test.config)
			if test.extraPath != "" {
				writeTestFile(t, root, test.extraPath, "-- name: Extra :one\nSELECT 1;\n")
			}
			code, _, diagnostic := invoke(root, "check")
			if code != 1 || !strings.Contains(diagnostic, test.want) {
				t.Fatalf("code=%d diagnostic=%s, want %q", code, diagnostic, test.want)
			}
		})
	}
}

// TestCheckReportsMissingInputs verifies absent family roots and configuration fail with the could-not-run status.
func TestCheckReportsMissingInputs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module analysis\n")
	writeTestFile(t, root, "backend/server/server.go", "package server\n")
	if code, _, _ := invoke(root, "check"); code != 2 {
		t.Fatalf("missing families code=%d", code)
	}
	root = repositoryFixture(t)
	if err := os.Remove(filepath.Join(root, "config/sqlc.yaml")); err != nil {
		t.Fatal(err)
	}
	if code, _, diagnostic := invoke(root, "check"); code != 2 || !strings.Contains(diagnostic, "config/sqlc.yaml") {
		t.Fatalf("code=%d diagnostic=%s", code, diagnostic)
	}
}

// TestCheckPassesAndReportsActionableFailures verifies success counts and exact remediation survive public dispatch.
func TestCheckPassesAndReportsActionableFailures(t *testing.T) {
	root := repositoryFixture(t)
	if code, output, diagnostic := invoke(root, "check"); code != 0 || !strings.Contains(output, "passed: 3 packages, 2 families, 1 sqlc units") {
		t.Fatalf("code=%d output=%s diagnostic=%s", code, output, diagnostic)
	}
	writeTestFile(t, root, "backend/database/work/edge.go", "package work\nimport \"analysis/backend/database/run\"\n")
	code, _, diagnostic := invoke(root, "check")
	for _, want := range []string{"analysis/backend/database/work imports sibling family analysis/backend/database/run", "must not call one another inside transactions", "failed with 1 error(s)"} {
		if code != 1 || !strings.Contains(diagnostic, want) {
			t.Fatalf("code=%d diagnostic=%s, want %q", code, diagnostic, want)
		}
	}
}

// TestRunRejectsUnknownCommandAndExtraArguments verifies usage mistakes cannot report a passing check.
func TestRunRejectsUnknownCommandAndExtraArguments(t *testing.T) {
	for _, args := range [][]string{{"inspect"}, {"check", "extra"}} {
		var output bytes.Buffer
		if code := importcheck.Run(args, &output, &output); code != 2 {
			t.Fatalf("args=%v code=%d: %s", args, code, output.String())
		}
	}
}
