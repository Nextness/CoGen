// Command importcheck reports and enforces the final package import and
// transaction graph.
//
// The check parses every maintained Go package under backend/, classifies each
// package as a metadata family, the composition root, the companion PDF store,
// a consumer, or a shared low-level package, and verifies the approved
// dependency direction. Family production code may depend only on its own
// subtree and shared internal, logging, manifest, or notes packages, so
// sibling families cannot call one another inside transactions. Test fixtures
// may compose the composition root and sibling families. Generated internal/sql
// output is imported only inside its owning package tree, and every sqlc
// generation unit keeps its schema, queries, and generated output under one
// owner directory.
package importcheck

import (
	"analysis/internal/projectlayout"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// layer identifies one package's position in the approved dependency direction.
type layer string

// Approved package layers.
const (
	layerFamily    layer = "family"
	layerRoot      layer = "root"
	layerCompanion layer = "companion"
	layerConsumer  layer = "consumer"
	layerShared    layer = "shared"
	layerExternal  layer = "external"
)

// layerOrder fixes the report order of every maintained layer.
var layerOrder = []layer{layerFamily, layerRoot, layerCompanion, layerConsumer, layerShared}

// importEdge records one import from one maintained package.
type importEdge struct {
	Importer string
	Target   string
	Test     bool
}

// packageGraph is one parsed module import graph.
type packageGraph struct {
	Module   string
	Families map[string]bool
	Packages map[string]bool
	Edges    []importEdge
}

// sqlcUnit is one generation unit parsed from config/sqlc.yaml.
type sqlcUnit struct {
	Schema  string
	Queries []string
	Output  string
}

// scanReport is one complete import and transaction graph verification.
type scanReport struct {
	Graph       packageGraph
	Units       []sqlcUnit
	Violations  []string
	UnitFailure []string
}

// readModulePath returns the module path declared by one go.mod file.
func readModulePath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("%s does not declare a module path", path)
}

// loadGraph parses every maintained Go package under backend/ and returns its import graph.
func loadGraph(root string) (packageGraph, error) {
	module, err := readModulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return packageGraph{}, err
	}
	graph := packageGraph{
		Module:   module,
		Families: make(map[string]bool),
		Packages: make(map[string]bool),
	}
	type packageImports struct {
		production map[string]bool
		test       map[string]bool
	}
	imports := make(map[string]*packageImports)
	sourceRoot := root
	files := token.NewFileSet()
	err = projectlayout.WalkGo(root, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() {
			if item.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		directory := filepath.ToSlash(filepath.Dir(relative))
		packagePath := module
		if directory != "." {
			packagePath = module + "/" + directory
		}
		file, err := parser.ParseFile(files, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", filepath.ToSlash(relative), err)
		}
		entry := imports[packagePath]
		if entry == nil {
			entry = &packageImports{production: make(map[string]bool), test: make(map[string]bool)}
			imports[packagePath] = entry
		}
		for _, spec := range file.Imports {
			target, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("parse import in %s: %w", relative, err)
			}
			if strings.HasSuffix(path, "_test.go") || projectlayout.IsTestPath(relative) {
				entry.test[target] = true
			} else {
				entry.production[target] = true
			}
		}
		return nil
	})
	if err != nil {
		return graph, err
	}
	for packagePath, entry := range imports {
		graph.Packages[packagePath] = true
		prefix := module + "/backend/database/"
		if strings.HasPrefix(packagePath, prefix) {
			name := strings.SplitN(strings.TrimPrefix(packagePath, prefix), "/", 2)[0]
			graph.Families[name] = true
		}
		for target := range entry.production {
			graph.Edges = append(graph.Edges, importEdge{Importer: packagePath, Target: target})
		}
		for target := range entry.test {
			if !entry.production[target] {
				graph.Edges = append(graph.Edges, importEdge{Importer: packagePath, Target: target, Test: true})
			}
		}
	}
	sort.Slice(graph.Edges, func(left, right int) bool {
		if graph.Edges[left].Importer != graph.Edges[right].Importer {
			return graph.Edges[left].Importer < graph.Edges[right].Importer
		}
		if graph.Edges[left].Target != graph.Edges[right].Target {
			return graph.Edges[left].Target < graph.Edges[right].Target
		}
		return !graph.Edges[left].Test && graph.Edges[right].Test
	})
	if len(graph.Families) == 0 {
		return graph, fmt.Errorf("no metadata families found under backend/database")
	}
	return graph, nil
}

// layerOf classifies one package path into its approved layer.
func (g packageGraph) layerOf(path string) layer {
	if path == g.Module || strings.HasPrefix(path, g.Module+"/backend/cli") || strings.HasPrefix(path, g.Module+"/backend/cmd/") || strings.HasPrefix(path, g.Module+"/tests/") {
		return layerConsumer
	}
	if !strings.HasPrefix(path, g.Module+"/") {
		return layerExternal
	}
	relative := strings.TrimPrefix(path, g.Module+"/")
	switch {
	case relative == "backend/database" || relative == "internal/migrations":
		return layerRoot
	case strings.HasPrefix(relative, "backend/database/"):
		return layerFamily
	case relative == "backend/pdfstore" || strings.HasPrefix(relative, "backend/pdfstore/"):
		return layerCompanion
	case relative == "backend/workspace" || relative == "backend/server" || strings.HasPrefix(relative, "tooling/go/"):
		return layerConsumer
	default:
		return layerShared
	}
}

// familyRoot returns the metadata family package root that owns one family package path.
func (g packageGraph) familyRoot(path string) string {
	relative := strings.TrimPrefix(path, g.Module+"/backend/database/")
	name := strings.SplitN(relative, "/", 2)[0]
	return g.Module + "/backend/database/" + name
}

// familyShared reports whether one target is a shared low-level package that families may depend on.
func (g packageGraph) familyShared(target string) bool {
	if !strings.HasPrefix(target, g.Module+"/") {
		return false
	}
	relative := strings.TrimPrefix(target, g.Module+"/")
	return relative == "backend/logging" || relative == "backend/manifest" || relative == "backend/notes" || (strings.HasPrefix(relative, "internal/") && !withinSubtree(g.Module+"/internal/migrations", target))
}

// generatedOwner returns the package tree that owns one generated internal/sql path.
func generatedOwner(path string) (string, bool) {
	segments := strings.Split(path, "/")
	for index := 0; index+1 < len(segments); index++ {
		if segments[index] == "internal" && segments[index+1] == "sql" {
			return strings.Join(segments[:index], "/"), true
		}
	}
	return "", false
}

// withinSubtree reports whether target is the subtree root or one of its descendants.
func withinSubtree(root, target string) bool {
	return target == root || strings.HasPrefix(target, root+"/")
}

// verifyFamilyEdge verifies one family import edge and returns its violations.
func (g packageGraph) verifyFamilyEdge(edge importEdge) []string {
	root := g.familyRoot(edge.Importer)
	if withinSubtree(root, edge.Target) || g.familyShared(edge.Target) {
		return nil
	}
	targetLayer := g.layerOf(edge.Target)
	if edge.Test {
		if edge.Target == g.Module+"/backend/database" || targetLayer == layerFamily {
			return nil
		}
		return []string{fmt.Sprintf("%s imports %s; family tests may compose only their own subtree, shared low-level packages, the composition root, and sibling families", edge.Importer, edge.Target)}
	}
	if targetLayer == layerFamily {
		return []string{fmt.Sprintf("%s imports sibling family %s; sibling families must not call one another inside transactions", edge.Importer, edge.Target)}
	}
	return []string{fmt.Sprintf("%s imports %s; family production code may only depend on its own subtree and shared internal, logging, manifest, or notes packages", edge.Importer, edge.Target)}
}

// verifyGraph returns the deterministic import direction violations for one graph.
func verifyGraph(graph packageGraph) []string {
	var violations []string
	for _, edge := range graph.Edges {
		if owner, generated := generatedOwner(edge.Target); generated {
			if !withinSubtree(owner, edge.Importer) {
				violations = append(violations, fmt.Sprintf("%s imports generated output %s; generated types stay inside the %s package tree", edge.Importer, edge.Target, owner))
			}
			continue
		}
		if !strings.HasPrefix(edge.Target, graph.Module+"/") && edge.Target != graph.Module {
			continue
		}
		if !edge.Test && strings.HasPrefix(edge.Target, graph.Module+"/tests/") && !strings.HasPrefix(edge.Importer, graph.Module+"/tests/") {
			violations = append(violations, fmt.Sprintf("%s imports %s; production must not import test code", edge.Importer, edge.Target))
			continue
		}
		if edge.Test && strings.HasPrefix(edge.Target, graph.Module+"/tests/") {
			continue
		}
		targetLayer := graph.layerOf(edge.Target)
		switch graph.layerOf(edge.Importer) {
		case layerFamily:
			violations = append(violations, graph.verifyFamilyEdge(edge)...)
		case layerRoot:
			if targetLayer == layerConsumer || targetLayer == layerCompanion {
				violations = append(violations, fmt.Sprintf("%s imports %s; the composition root must not depend on consumers or the companion store", edge.Importer, edge.Target))
			}
		case layerCompanion:
			if targetLayer == layerConsumer {
				violations = append(violations, fmt.Sprintf("%s imports %s; the companion store must not depend on consumers", edge.Importer, edge.Target))
			}
		case layerShared:
			if targetLayer != layerShared {
				violations = append(violations, fmt.Sprintf("%s imports %s; shared low-level packages must not depend on families, the composition root, the companion store, or consumers", edge.Importer, edge.Target))
			}
		}
	}
	sort.Strings(violations)
	return violations
}

// unitOwner returns the owner directory shared by one sqlc schema, query, or output path.
func unitOwner(path string) string {
	cleaned := filepath.ToSlash(path)
	for _, marker := range []string{"/schema/", "/internal/sql"} {
		if index := strings.Index(cleaned, marker); index >= 0 {
			return cleaned[:index]
		}
	}
	return ""
}

// loadSQLCUnits parses the generation units declared by config/sqlc.yaml.
func loadSQLCUnits(root string) ([]sqlcUnit, error) {
	path := filepath.Join(root, "config", "sqlc.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.ToSlash(filepath.Join("config", "sqlc.yaml")), err)
	}
	var config struct {
		SQL []struct {
			Schema  string   `yaml:"schema"`
			Queries []string `yaml:"queries"`
			Gen     struct {
				Go struct {
					Out string `yaml:"out"`
				} `yaml:"go"`
			} `yaml:"gen"`
		} `yaml:"sql"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config/sqlc.yaml: %w", err)
	}
	var units []sqlcUnit
	for _, entry := range config.SQL {
		units = append(units, sqlcUnit{Schema: entry.Schema, Queries: entry.Queries, Output: entry.Gen.Go.Out})
	}
	return units, nil
}

// verifyUnits returns the deterministic ownership failures for the parsed sqlc units.
func verifyUnits(root string, units []sqlcUnit) []string {
	var failures []string
	configured := make(map[string]string)
	for _, unit := range units {
		if unit.Schema == "" || unit.Output == "" || len(unit.Queries) == 0 {
			failures = append(failures, "config/sqlc.yaml contains a generation unit without a schema, queries, and output")
			continue
		}
		owner := unitOwner(unit.Schema)
		if owner == "" {
			failures = append(failures, fmt.Sprintf("config/sqlc.yaml schema %s is not under a family schema directory", unit.Schema))
			continue
		}
		if unitOwner(unit.Output) != owner {
			failures = append(failures, fmt.Sprintf("config/sqlc.yaml output %s must share the %s owner directory", unit.Output, owner))
		}
		seen := make(map[string]bool)
		for _, query := range unit.Queries {
			if unitOwner(query) != owner {
				failures = append(failures, fmt.Sprintf("config/sqlc.yaml query %s must share the %s owner directory", query, owner))
			}
			if filepath.Base(query) == "definitions.sql" {
				failures = append(failures, fmt.Sprintf("config/sqlc.yaml must not list %s as a query file", query))
			}
			if previous, duplicate := configured[query]; duplicate {
				failures = append(failures, fmt.Sprintf("config/sqlc.yaml lists %s in both %s and %s", query, previous, owner))
			}
			configured[query] = owner
			seen[filepath.Base(query)] = true
			if _, err := os.Stat(filepath.Join(root, "config", filepath.FromSlash(query))); err != nil {
				failures = append(failures, fmt.Sprintf("config/sqlc.yaml query %s does not exist", query))
			}
		}
		schemaPath := filepath.Join(root, "config", filepath.FromSlash(unit.Schema))
		if _, err := os.Stat(schemaPath); err != nil {
			failures = append(failures, fmt.Sprintf("config/sqlc.yaml schema %s does not exist", unit.Schema))
			continue
		}
		entries, err := os.ReadDir(filepath.Dir(schemaPath))
		if err != nil {
			failures = append(failures, fmt.Sprintf("config/sqlc.yaml schema directory %s cannot be read: %v", filepath.Dir(unit.Schema), err))
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || filepath.Ext(name) != ".sql" || name == "definitions.sql" || seen[name] {
				continue
			}
			failures = append(failures, fmt.Sprintf("schema query file %s is not declared in config/sqlc.yaml", filepath.ToSlash(filepath.Join(filepath.Dir(unit.Schema), name))))
		}
		outputPath := filepath.Join(root, "config", filepath.FromSlash(unit.Output))
		outputEntries, err := os.ReadDir(outputPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("config/sqlc.yaml output %s cannot be read: %v", unit.Output, err))
			continue
		}
		generated := 0
		for _, entry := range outputEntries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
				generated++
			}
		}
		if generated == 0 {
			failures = append(failures, fmt.Sprintf("config/sqlc.yaml output %s contains no generated Go files", unit.Output))
		}
	}
	sort.Strings(failures)
	return failures
}

// scanRepository loads and verifies one repository root.
func scanRepository(root string) (scanReport, error) {
	graph, err := loadGraph(root)
	if err != nil {
		return scanReport{}, err
	}
	units, err := loadSQLCUnits(root)
	if err != nil {
		return scanReport{}, err
	}
	return scanReport{
		Graph:       graph,
		Units:       units,
		Violations:  verifyGraph(graph),
		UnitFailure: verifyUnits(root, units),
	}, nil
}

// failures returns the deterministic check failures for one scan.
func (r scanReport) failures() []string {
	failures := append([]string{}, r.Violations...)
	failures = append(failures, r.UnitFailure...)
	sort.Strings(failures)
	return failures
}

// familyProductionImports returns each family's sorted production imports of analysis packages.
func (r scanReport) familyProductionImports() map[string][]string {
	result := make(map[string][]string)
	for _, edge := range r.Graph.Edges {
		if edge.Test || r.Graph.layerOf(edge.Importer) != layerFamily {
			continue
		}
		if !strings.HasPrefix(edge.Target, r.Graph.Module+"/") {
			continue
		}
		family := r.Graph.familyRoot(edge.Importer)
		result[family] = append(result[family], edge.Target)
	}
	for family, targets := range result {
		result[family] = sortedUnique(targets)
	}
	return result
}

// familyFixtureImports returns each family's sorted cross-family test imports.
func (r scanReport) familyFixtureImports() map[string][]string {
	result := make(map[string][]string)
	for _, edge := range r.Graph.Edges {
		if !edge.Test || r.Graph.layerOf(edge.Importer) != layerFamily || r.Graph.layerOf(edge.Target) != layerFamily {
			continue
		}
		family := r.Graph.familyRoot(edge.Importer)
		target := r.Graph.familyRoot(edge.Target)
		if family == target {
			continue
		}
		result[family] = append(result[family], target)
	}
	for family, targets := range result {
		result[family] = sortedUnique(targets)
	}
	return result
}

// sortedUnique returns the sorted distinct values of one string slice.
func sortedUnique(values []string) []string {
	seen := make(map[string]bool, len(values))
	var unique []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	sort.Strings(unique)
	return unique
}

// packageCounts returns the number of maintained packages per layer.
func (r scanReport) packageCounts() map[layer]int {
	counts := make(map[layer]int)
	for path := range r.Graph.Packages {
		counts[r.Graph.layerOf(path)]++
	}
	return counts
}

// writeReport renders the import and transaction graph report for one scan.
func writeReport(output io.Writer, report scanReport) {
	fmt.Fprintln(output, "Import and transaction graph report")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "layers")
	counts := report.packageCounts()
	for _, value := range layerOrder {
		fmt.Fprintf(output, "%-10s %3d packages\n", value, counts[value])
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output, "family production imports")
	production := report.familyProductionImports()
	for _, family := range sortedKeys(production) {
		fmt.Fprintf(output, "%-40s -> %s\n", family, strings.Join(production[family], ", "))
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output, "family test fixture composition")
	fixtures := report.familyFixtureImports()
	if len(fixtures) == 0 {
		fmt.Fprintln(output, "(none)")
	}
	for _, family := range sortedKeys(fixtures) {
		fmt.Fprintf(output, "%-40s -> %s\n", family, strings.Join(fixtures[family], ", "))
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output, "sqlc units")
	for _, unit := range report.Units {
		fmt.Fprintf(output, "%-40s queries %2d output %s\n", unitOwner(unit.Schema), len(unit.Queries), unit.Output)
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output, "verified invariants")
	fmt.Fprintln(output, "- family production code imports only its own subtree and shared internal, logging, manifest, or notes packages")
	fmt.Fprintln(output, "- sibling families have no production imports, so no family transaction can call another family's DAL")
	fmt.Fprintln(output, "- generated internal/sql output is imported only inside its owning package tree")
	fmt.Fprintln(output, "- every sqlc generation unit keeps its schema, queries, and output under one owner directory")
	if failures := report.failures(); len(failures) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "violations")
		for _, failure := range failures {
			fmt.Fprintln(output, failure)
		}
	}
}

// sortedKeys returns the sorted keys of one family map.
func sortedKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
	report, err := scanRepository(root)
	if err != nil {
		fmt.Fprintln(stderr, "import graph check could not run:", err)
		return 2
	}
	failures := report.failures()
	if len(failures) > 0 {
		for _, failure := range failures {
			fmt.Fprintln(stderr, failure)
		}
		fmt.Fprintf(stderr, "import graph check failed with %d error(s)\n", len(failures))
		return 1
	}
	fmt.Fprintf(stdout, "import graph check passed: %d packages, %d families, %d sqlc units\n", len(report.Graph.Packages), len(report.Graph.Families), len(report.Units))
	return 0
}

// runReport parses report arguments, prints the graph report, and returns a process exit status.
func runReport(args []string, stdout, stderr io.Writer) int {
	root, ok := parseRoot("report", args, stderr)
	if !ok {
		return 2
	}
	report, err := scanRepository(root)
	if err != nil {
		fmt.Fprintln(stderr, "import graph report could not run:", err)
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
		fmt.Fprintf(stderr, "unknown importcheck command %q; use check or report\n", args[0])
		return 2
	}
}
