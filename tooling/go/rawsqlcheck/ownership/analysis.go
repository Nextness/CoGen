// Package ownership analyzes handwritten SQL locations against an explicit reviewed inventory.
package ownership

import (
	"analysis/internal/projectlayout"
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
type category = string

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

// Location declares one maintained file allowed to contain raw SQL.
// Dynamic locations name the sibling *_dynamic.go builder that owns their
// closed SQL fragments.
type Location struct {
	Path     string
	Category category
	Builder  string
	Reason   string
}

// Call records one raw SQL call found in maintained source.
type Call struct {
	Path     string
	Line     int
	Method   string
	Category category
	Reason   string
}

// BuilderFailure records one dynamic location whose declared builder sibling is absent.
type BuilderFailure struct {
	Location string
	Builder  string
}

// Report is one complete raw SQL ownership scan.
type Report struct {
	locations       []Location
	Calls           []Call
	Unclassified    []Call
	Missing         []string
	Unused          []string
	InvalidBuilders []BuilderFailure
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

// Analyze parses maintained Go sources under root and classifies every raw SQL call.
func Analyze(root string, locations []Location) (Report, error) {
	report := Report{locations: append([]Location(nil), locations...)}
	approved := make(map[string]Location, len(locations))
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
			site := Call{
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
				report.InvalidBuilders = append(report.InvalidBuilders, BuilderFailure{Location: location.Path, Builder: builder})
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

// Failures returns the deterministic ownership Failures for one scan.
func (r Report) Failures() []string {
	var Failures []string
	for _, site := range r.Unclassified {
		Failures = append(Failures, fmt.Sprintf("%s:%d: unclassified raw SQL call %s; use the owning table-family DAL, or add %s to approvedLocations in tooling/go/rawsqlcheck/main.go with a category and reason", site.Path, site.Line, site.Method, site.Path))
	}
	for _, path := range r.Missing {
		Failures = append(Failures, fmt.Sprintf("approved raw SQL location %s does not exist; remove it from approvedLocations in tooling/go/rawsqlcheck/main.go", path))
	}
	for _, path := range r.Unused {
		Failures = append(Failures, fmt.Sprintf("approved raw SQL location %s has no raw SQL calls; remove it from approvedLocations in tooling/go/rawsqlcheck/main.go", path))
	}
	for _, failure := range r.InvalidBuilders {
		Failures = append(Failures, fmt.Sprintf("dynamic raw SQL location %s requires an existing builder sibling %s", failure.Location, failure.Builder))
	}
	return Failures
}

// callsForCategory returns the calls classified under one category.
func (r Report) callsForCategory(value category) []Call {
	var calls []Call
	for _, site := range r.Calls {
		if site.Category == value {
			calls = append(calls, site)
		}
	}
	return calls
}

// callsForPath returns the calls recorded for one repository-relative path.
func (r Report) callsForPath(path string) []Call {
	var calls []Call
	for _, site := range r.Calls {
		if site.Path == path {
			calls = append(calls, site)
		}
	}
	return calls
}

// fileCount returns the number of distinct files among the supplied calls.
func fileCount(calls []Call) int {
	files := make(map[string]bool)
	for _, site := range calls {
		files[site.Path] = true
	}
	return len(files)
}

// WriteReport renders the ownership report for one scan.
func WriteReport(output io.Writer, report Report) {
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
	for _, location := range report.locations {
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

// Check scans root with the supplied ownership table and reports the result.
func Check(root string, locations []Location, stdout, stderr io.Writer) int {
	report, err := Analyze(root, locations)
	if err != nil {
		fmt.Fprintln(stderr, "raw SQL ownership check could not run:", err)
		return 2
	}
	Failures := report.Failures()
	if len(Failures) > 0 {
		for _, failure := range Failures {
			fmt.Fprintln(stderr, failure)
		}
		fmt.Fprintf(stderr, "raw SQL ownership check failed with %d error(s)\n", len(Failures))
		return 1
	}
	fmt.Fprintf(stdout, "raw SQL ownership check passed: %d classified calls in %d files\n", len(report.Calls), fileCount(report.Calls))
	return 0
}
