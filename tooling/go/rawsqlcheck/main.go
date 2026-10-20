// Command rawsqlcheck reports and enforces raw SQL ownership in maintained Go
// sources.
//
// The check parses every Go file under backend/ outside reserved internal/sql
// directories, detects database/sql and driver Query, QueryRow, Exec, and
// prepared-statement calls, and requires every call to live in a classified
// location. Platform migrations and connection pragmas, the viewer query-budget
// driver wrapper, dynamic table introspection, sanitized export maintenance,
// the cross-database audit outbox, retained *_dynamic.go builder readers, and
// direct integrity tests are approved. Unclassified calls fail with the file,
// line, and expected remediation instead of banning necessary low-level SQL.
package rawsqlcheck

import (
	"analysis/internal/projectlayout"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// rawSQLMethods lists the database/sql and driver call names that execute SQL.
var rawSQLMethods = map[string]bool{
	"Query":           true,
	"QueryContext":    true,
	"QueryRow":        true,
	"QueryRowContext": true,
	"Exec":            true,
	"ExecContext":     true,
	"Prepare":         true,
	"PrepareContext":  true,
}

// category identifies the approved ownership class of one raw SQL location.
type category string

// Approved raw SQL ownership categories.
const (
	categoryPlatform      category = "platform"
	categoryPlumbing      category = "plumbing"
	categoryIntrospection category = "introspection"
	categoryExport        category = "export"
	categoryOutbox        category = "outbox"
	categoryDynamic       category = "dynamic"
	categoryTest          category = "test"
	categoryGenerated     category = "generated"
)

// categoryDispositions explains each category in the ownership report.
var categoryDispositions = map[category]string{
	categoryPlatform:      "approved: runtime migrations, connection pragmas, and schema version",
	categoryPlumbing:      "approved: viewer query-budget driver wrapper",
	categoryIntrospection: "approved: dynamic table and column discovery",
	categoryExport:        "approved: sanitized export maintenance",
	categoryOutbox:        "approved: cross-database audit outbox savepoints",
	categoryDynamic:       "approved: retained *_dynamic.go builder readers",
	categoryTest:          "approved: direct integrity tests",
	categoryGenerated:     "excluded: generated internal/sql output",
}

// categoryOrder fixes the report order of every category.
var categoryOrder = []category{
	categoryPlatform,
	categoryPlumbing,
	categoryIntrospection,
	categoryExport,
	categoryOutbox,
	categoryDynamic,
	categoryTest,
	categoryGenerated,
}

// approvedLocation declares one maintained file allowed to contain raw SQL.
// Dynamic locations name the sibling *_dynamic.go builder that owns their
// closed SQL fragments.
type approvedLocation struct {
	Path     string
	Category category
	Builder  string
	Reason   string
}

// approvedLocations is the reviewed raw SQL ownership table. Every maintained
// file with a raw SQL call must appear here with a category and reason, and
// every entry must keep at least one call, or the check fails with the file
// and its remediation. Test files are classified as direct integrity tests
// without an entry because bypassing the family DALs is their subject.
var approvedLocations = []approvedLocation{
	{Path: "internal/migrations/runner.go", Category: categoryPlatform, Reason: "configured SQLite migration transactions and tracking history."},
	{
		Path:     "tests/support/go/testfixture/rows.go",
		Category: categoryTest,
		Reason:   "integration/e2e-only fixture seeds intentionally create historical runs and artifacts without blobs; production uses guarded family writers.",
	},
	{
		Path:     "backend/database/connection.go",
		Category: categoryPlatform,
		Reason:   "SQLite URI pragmas and the query_only probe are connection platform behavior.",
	},
	{
		Path:     "backend/database/migrations.go",
		Category: categoryPlatform,
		Reason:   "the runtime migration registry, write lock, and up-SQL execution are intentionally independent from sqlc.",
	},
	{
		Path:     "backend/server/query_budget.go",
		Category: categoryPlumbing,
		Reason:   "the driver wrapper must observe every statement to enforce the viewer query budget.",
	},
	{
		Path:     "backend/server/introspection.go",
		Category: categoryIntrospection,
		Reason:   "the table browser selects discovered, allowlisted table and column identifiers that cannot be generated.",
	},
	{
		Path:     "backend/server/server.go",
		Category: categoryIntrospection,
		Reason:   "companion schema verification inspects table and column existence before serving.",
	},
	{
		Path:     "tooling/go/prepare-osf/main.go",
		Category: categoryExport,
		Reason:   "secure delete, VACUUM INTO, integrity checks, and schema-version reads are export maintenance orchestration.",
	},
	{
		Path:     "backend/pdfstore/audit.go",
		Category: categoryOutbox,
		Reason:   "dynamic savepoint names isolate each cross-database audit outbox event.",
	},
	{
		Path:     "backend/database/artifact/viewer.go",
		Category: categoryDynamic,
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded artifact evidence uses viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/audit/evidence.go",
		Category: categoryDynamic,
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded audit evidence uses viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/author/viewer.go",
		Category: categoryDynamic,
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded identity evidence uses viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/cache/viewer.go",
		Category: categoryDynamic,
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded cache-use evidence uses viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/source/viewer.go",
		Category: categoryDynamic,
		Builder:  "viewer_dynamic.go",
		Reason:   "bounded corpus records use viewer_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/work/corpus.go",
		Category: categoryDynamic,
		Builder:  "corpus_dynamic.go",
		Reason:   "bounded corpus reads use corpus_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/work/evaluation.go",
		Category: categoryDynamic,
		Builder:  "evaluation_dynamic.go",
		Reason:   "bounded evaluation reads use evaluation_dynamic.go fragments with bound values.",
	},
	{
		Path:     "backend/database/work/graph.go",
		Category: categoryDynamic,
		Builder:  "graph_dynamic.go",
		Reason:   "bounded graph reads use graph_dynamic.go fragments with bound values.",
	},
}

// callSite records one raw SQL call found in maintained source.
type callSite struct {
	Path     string
	Line     int
	Method   string
	Category category
	Reason   string
}

// builderFailure records one dynamic location whose declared builder sibling is absent.
type builderFailure struct {
	Location string
	Builder  string
}

// scanReport is one complete raw SQL ownership scan.
type scanReport struct {
	Calls           []callSite
	Unclassified    []callSite
	Missing         []string
	Unused          []string
	InvalidBuilders []builderFailure
}

// generatedGoPath reports whether a repository-relative Go path lives under a reserved internal/sql directory.
func generatedGoPath(relativePath string) bool {
	segments := strings.Split(relativePath, "/")
	for index := 0; index+1 < len(segments); index++ {
		if segments[index] == "internal" && segments[index+1] == "sql" {
			return true
		}
	}
	return false
}

// isURLQueryReceiver reports whether a Query receiver is an HTTP request URL or a URL value.
func isURLQueryReceiver(receiver ast.Expr) bool {
	switch value := receiver.(type) {
	case *ast.SelectorExpr:
		return value.Sel.Name == "URL"
	case *ast.Ident:
		return value.Name == "URL"
	}
	return false
}

// scanRepository parses maintained Go sources under root and classifies every raw SQL call.
func scanRepository(root string, locations []approvedLocation) (scanReport, error) {
	var report scanReport
	approved := make(map[string]approvedLocation, len(locations))
	for _, location := range locations {
		if location.Path == "" || location.Category == "" || strings.TrimSpace(location.Reason) == "" {
			return report, fmt.Errorf("approved raw SQL location %q requires a path, category, and reason", location.Path)
		}
		if _, known := categoryDispositions[location.Category]; !known {
			return report, fmt.Errorf("approved raw SQL location %s has unknown category %q", location.Path, location.Category)
		}
		if strings.HasSuffix(location.Path, "_test.go") {
			return report, fmt.Errorf("approved raw SQL location %s is a test file; direct integrity tests are classified automatically", location.Path)
		}
		if location.Category == categoryDynamic && location.Builder == "" {
			return report, fmt.Errorf("dynamic raw SQL location %s requires its builder filename", location.Path)
		}
		if _, duplicate := approved[location.Path]; duplicate {
			return report, fmt.Errorf("approved raw SQL location %s is declared more than once", location.Path)
		}
		approved[location.Path] = location
	}
	callCounts := make(map[string]int)
	files := token.NewFileSet()
	err := projectlayout.WalkGo(root, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if generatedGoPath(relative) {
			return nil
		}
		file, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", relative, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !rawSQLMethods[selector.Sel.Name] {
				return true
			}
			if selector.Sel.Name == "Query" && isURLQueryReceiver(selector.X) {
				return true
			}
			site := callSite{
				Path:   relative,
				Line:   files.Position(call.Pos()).Line,
				Method: selector.Sel.Name,
			}
			location, classified := approved[relative]
			switch {
			case strings.HasSuffix(relative, "_test.go") || projectlayout.IsTestPath(relative):
				site.Category = categoryTest
				site.Reason = categoryDispositions[categoryTest]
			case classified:
				site.Category = location.Category
				site.Reason = location.Reason
			default:
				report.Unclassified = append(report.Unclassified, site)
				return true
			}
			callCounts[relative]++
			report.Calls = append(report.Calls, site)
			return true
		})
		return nil
	})
	if err != nil {
		return report, err
	}
	for _, location := range locations {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(location.Path))); err != nil {
			report.Missing = append(report.Missing, location.Path)
			continue
		}
		if callCounts[location.Path] == 0 {
			report.Unused = append(report.Unused, location.Path)
		}
		if location.Category == categoryDynamic {
			builder := filepath.ToSlash(filepath.Join(filepath.Dir(location.Path), location.Builder))
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(builder))); err != nil {
				report.InvalidBuilders = append(report.InvalidBuilders, builderFailure{Location: location.Path, Builder: builder})
			}
		}
	}
	sort.Slice(report.Calls, func(left, right int) bool {
		if report.Calls[left].Path != report.Calls[right].Path {
			return report.Calls[left].Path < report.Calls[right].Path
		}
		return report.Calls[left].Line < report.Calls[right].Line
	})
	sort.Slice(report.Unclassified, func(left, right int) bool {
		if report.Unclassified[left].Path != report.Unclassified[right].Path {
			return report.Unclassified[left].Path < report.Unclassified[right].Path
		}
		return report.Unclassified[left].Line < report.Unclassified[right].Line
	})
	sort.Strings(report.Missing)
	sort.Strings(report.Unused)
	sort.Slice(report.InvalidBuilders, func(left, right int) bool {
		return report.InvalidBuilders[left].Location < report.InvalidBuilders[right].Location
	})
	return report, nil
}

// failures returns the deterministic ownership failures for one scan.
func (r scanReport) failures() []string {
	var failures []string
	for _, site := range r.Unclassified {
		failures = append(failures, fmt.Sprintf("%s:%d: unclassified raw SQL call %s; use the owning table-family DAL, or add %s to approvedLocations in tooling/go/rawsqlcheck/main.go with a category and reason", site.Path, site.Line, site.Method, site.Path))
	}
	for _, path := range r.Missing {
		failures = append(failures, fmt.Sprintf("approved raw SQL location %s does not exist; remove it from approvedLocations in tooling/go/rawsqlcheck/main.go", path))
	}
	for _, path := range r.Unused {
		failures = append(failures, fmt.Sprintf("approved raw SQL location %s has no raw SQL calls; remove it from approvedLocations in tooling/go/rawsqlcheck/main.go", path))
	}
	for _, failure := range r.InvalidBuilders {
		failures = append(failures, fmt.Sprintf("dynamic raw SQL location %s requires an existing builder sibling %s", failure.Location, failure.Builder))
	}
	return failures
}

// callsForCategory returns the calls classified under one category.
func (r scanReport) callsForCategory(value category) []callSite {
	var calls []callSite
	for _, site := range r.Calls {
		if site.Category == value {
			calls = append(calls, site)
		}
	}
	return calls
}

// callsForPath returns the calls recorded for one repository-relative path.
func (r scanReport) callsForPath(path string) []callSite {
	var calls []callSite
	for _, site := range r.Calls {
		if site.Path == path {
			calls = append(calls, site)
		}
	}
	return calls
}

// fileCount returns the number of distinct files among the supplied calls.
func fileCount(calls []callSite) int {
	files := make(map[string]bool)
	for _, site := range calls {
		files[site.Path] = true
	}
	return len(files)
}

// writeReport renders the ownership report for one scan.
func writeReport(output io.Writer, report scanReport) {
	fmt.Fprintln(output, "Raw SQL ownership report")
	fmt.Fprintln(output)
	fmt.Fprintf(output, "%-15s %6s %6s  %s\n", "category", "files", "calls", "disposition")
	for _, value := range categoryOrder {
		calls := report.callsForCategory(value)
		fmt.Fprintf(output, "%-15s %6d %6d  %s\n", value, fileCount(calls), len(calls), categoryDispositions[value])
	}
	fmt.Fprintf(output, "%-15s %6d %6d  %s\n", "unclassified", fileCount(report.Unclassified), len(report.Unclassified), "use the owning table-family DAL or classify the file")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Approved raw SQL locations")
	for _, location := range approvedLocations {
		calls := report.callsForPath(location.Path)
		fmt.Fprintf(output, "%-15s %-55s %4d  %s\n", location.Category, location.Path, len(calls), location.Reason)
	}
	tests := report.callsForCategory(categoryTest)
	fmt.Fprintf(output, "%-15s %-55s %4d  %s\n", categoryTest, "direct integrity test files", len(tests), categoryDispositions[categoryTest])
	if len(report.Unclassified) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Unclassified raw SQL calls")
		for _, site := range report.Unclassified {
			fmt.Fprintf(output, "%s:%d: %s\n", site.Path, site.Line, site.Method)
		}
	}
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

// checkLocations scans root with the supplied ownership table and reports the result.
func checkLocations(root string, locations []approvedLocation, stdout, stderr io.Writer) int {
	report, err := scanRepository(root, locations)
	if err != nil {
		fmt.Fprintln(stderr, "raw SQL ownership check could not run:", err)
		return 2
	}
	failures := report.failures()
	if len(failures) > 0 {
		for _, failure := range failures {
			fmt.Fprintln(stderr, failure)
		}
		fmt.Fprintf(stderr, "raw SQL ownership check failed with %d error(s)\n", len(failures))
		return 1
	}
	fmt.Fprintf(stdout, "raw SQL ownership check passed: %d classified calls in %d files\n", len(report.Calls), fileCount(report.Calls))
	return 0
}

// runCheck parses check arguments, scans the repository, and returns a process exit status.
func runCheck(args []string, stdout, stderr io.Writer) int {
	root, ok := parseRoot("check", args, stderr)
	if !ok {
		return 2
	}
	return checkLocations(root, approvedLocations, stdout, stderr)
}

// runReport parses report arguments, prints the ownership report, and returns a process exit status.
func runReport(args []string, stdout, stderr io.Writer) int {
	root, ok := parseRoot("report", args, stderr)
	if !ok {
		return 2
	}
	report, err := scanRepository(root, approvedLocations)
	if err != nil {
		fmt.Fprintln(stderr, "raw SQL ownership report could not run:", err)
		return 2
	}
	writeReport(stdout, report)
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
