// Package rawsqlcheck dispatches repository SQL ownership checks and reports.
package rawsqlcheck

import (
	"analysis/tooling/go/rawsqlcheck/ownership"
	"flag"
	"fmt"
	"io"
	"strings"
)

// approvedLocations is the reviewed raw SQL ownership table. Every maintained
// file with a raw SQL call must appear here with a category and reason, and
// every entry must keep at least one call, or the check fails with the file
// and its remediation. Test files are classified as direct integrity tests
// without an entry because bypassing the family DALs is their subject.
var approvedLocations = []ownership.Location{
	{Path: "internal/migrations/runner.go", Category: "platform", Reason: "configured SQLite migration transactions and tracking history."},
	{Path: "internal/migrations/history.go", Category: "platform", Reason: "read-only configured migration history and tracking-table existence for bundle readiness."},
	{
		Path:     "tests/support/go/testfixture/rows.go",
		Category: "test",
		Reason:   "integration/e2e-only fixture seeds intentionally create historical runs and artifacts without blobs; production uses guarded family writers.",
	},
	{
		Path:     "backend/database/connection.go",
		Category: "platform",
		Reason:   "SQLite URI pragmas and the query_only probe are connection platform behavior.",
	},
	{
		Path:     "backend/database/migrations.go",
		Category: "platform",
		Reason:   "the runtime migration registry, write lock, and up-SQL execution are intentionally independent from sqlc.",
	},
	{
		Path:     "internal/querybudget/driver.go",
		Category: "plumbing",
		Reason:   "the driver wrapper must observe every statement to enforce the viewer query budget.",
	},
	{
		Path:     "backend/server/introspection.go",
		Category: "introspection",
		Reason:   "the table browser selects discovered, allowlisted table and column identifiers that cannot be generated.",
	},
	{
		Path:     "backend/server/server.go",
		Category: "introspection",
		Reason:   "companion schema verification inspects table and column existence before serving.",
	},
	{
		Path:     "tooling/go/prepare-osf/main.go",
		Category: "export",
		Reason:   "secure delete, VACUUM INTO, integrity checks, and schema-version reads are export maintenance orchestration.",
	},
	{
		Path:     "backend/pdfstore/audit.go",
		Category: "outbox",
		Reason:   "dynamic savepoint names isolate each cross-database audit outbox event.",
	},
	{
		Path:     "backend/database/artifact/viewer.go",
		Category: "dynamic",
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded artifact evidence uses viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/audit/evidence.go",
		Category: "dynamic",
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded audit evidence uses viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/author/viewer.go",
		Category: "dynamic",
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded identity evidence uses viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/cache/viewer.go",
		Category: "dynamic",
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded cache-use evidence uses viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/source/viewer.go",
		Category: "dynamic",
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded corpus records use viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/work/corpus.go",
		Category: "dynamic",
		Builder:  "corpus_dynamic.go",
		Reason:   "bounded corpus reads use corpus_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/work/evaluation.go",
		Category: "dynamic",
		Builder:  "evaluation_dynamic.go",
		Reason:   "bounded evaluation reads use evaluation_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/work/graph.go",
		Category: "dynamic",
		Builder:  "graph_dynamic.go",
		Reason:   "bounded graph reads use graph_dynamic.go fragments with bound values.",
	},
}

// parseRoot parses the shared --root flag for one subcommand.
func parseRoot(name string, args []string, stderr io.Writer) (string, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		return "", false
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "%s accepts only --root\n", name)
		return "", false
	}
	return *root, true
}

// runCheck parses check arguments, scans the repository, and returns a process exit status.
func runCheck(args []string, stdout, stderr io.Writer) int {
	root, ok := parseRoot("check", args, stderr)
	if !ok {
		return 2
	}
	return ownership.Check(root, approvedLocations, stdout, stderr)
}

// runReport parses report arguments, prints the ownership report, and returns a process exit status.
func runReport(args []string, stdout, stderr io.Writer) int {
	root, ok := parseRoot("report", args, stderr)
	if !ok {
		return 2
	}
	report, err := ownership.Analyze(root, approvedLocations)
	if err != nil {
		fmt.Fprintln(stderr, "raw SQL ownership report could not run:", err)
		return 2
	}
	ownership.WriteReport(stdout, report)
	return 0
}

// Run parses the command hierarchy and returns a process exit status.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runCheck(nil, stdout, stderr)
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "report":
		return runReport(args[1:], stdout, stderr)
	default:
		if strings.HasPrefix(args[0], "-") {
			return runCheck(args, stdout, stderr)
		}
		fmt.Fprintf(stderr, "unknown rawsqlcheck command %q; use check or report\n", args[0])
		return 2
	}
}
