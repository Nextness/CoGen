package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	catalogDocument = "docs/PROJECT_CATALOG.md"
	catalogBegin    = "<!-- BEGIN GENERATED PROJECT CATALOG -->"
	catalogEnd      = "<!-- END GENERATED PROJECT CATALOG -->"
	noDescription   = "No source description"
)

// catalogEntry is one source-derived declaration row in the project catalog.
type catalogEntry struct {
	file        string
	kind        string
	name        string
	signature   string
	description string
	start       int
	end         int
}

// markdownCell collapses whitespace and escapes Markdown table delimiters.
func markdownCell(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	return strings.ReplaceAll(value, "|", `\|`)
}

// markdownCode wraps one table-safe value in a code span that tolerates struct tags.
func markdownCode(value string) string {
	value = markdownCell(value)
	delimiter := "`"
	if strings.Contains(value, "`") {
		delimiter = "``"
	}
	return delimiter + value + delimiter
}

// nodeText formats one Go AST node as compact source text.
func nodeText(files *token.FileSet, node ast.Node) string {
	var output strings.Builder
	if err := format.Node(&output, files, node); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(output.String()), " ")
}

// sourceDescription returns only an attached source comment or the explicit fallback.
func sourceDescription(group *ast.CommentGroup) string {
	if group == nil || strings.TrimSpace(group.Text()) == "" {
		return noDescription
	}
	return strings.Join(strings.Fields(group.Text()), " ")
}

// goDeclarationKind identifies the catalog kind of a Go type declaration.
func goDeclarationKind(spec *ast.TypeSpec) string {
	if spec.Assign.IsValid() {
		return "type alias"
	}
	switch spec.Type.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	default:
		return "type"
	}
}

// collectGoDeclarations walks src and returns named functions, methods, types, and tests.
func collectGoDeclarations(root string) ([]catalogEntry, error) {
	files := token.NewFileSet()
	var entries []catalogEntry
	sourceRoot := filepath.Join(root, "src")
	err := filepath.WalkDir(sourceRoot, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(files, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		isTestFile := strings.HasSuffix(path, "_test.go")
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				name := value.Name.Name
				kind := "function"
				if value.Recv != nil {
					name = "(" + nodeText(files, value.Recv.List[0].Type) + ")." + name
					kind = "method"
				} else if isTestFile && strings.HasPrefix(value.Name.Name, "Test") {
					kind = "test"
				} else if isTestFile && strings.HasPrefix(value.Name.Name, "Benchmark") {
					kind = "benchmark"
				}
				signature := "func " + name + strings.TrimPrefix(nodeText(files, value.Type), "func")
				entries = append(entries, catalogEntry{file: relativePath, kind: kind, name: name, signature: signature, description: sourceDescription(value.Doc), start: files.Position(value.Pos()).Line, end: files.Position(value.End()).Line})
			case *ast.GenDecl:
				if value.Tok != token.TYPE {
					continue
				}
				for _, rawSpec := range value.Specs {
					spec, ok := rawSpec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					doc := spec.Doc
					if doc == nil && len(value.Specs) == 1 {
						doc = value.Doc
					}
					entries = append(entries, catalogEntry{file: relativePath, kind: goDeclarationKind(spec), name: spec.Name.Name, signature: "type " + nodeText(files, spec), description: sourceDescription(doc), start: files.Position(spec.Pos()).Line, end: files.Position(spec.End()).Line})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].file == entries[right].file {
			return entries[left].start < entries[right].start
		}
		return entries[left].file < entries[right].file
	})
	return entries, nil
}

// renderCatalogEntries renders declarations in per-file Markdown tables.
func renderCatalogEntries(entries []catalogEntry) string {
	var output strings.Builder
	currentFile := ""
	for _, entry := range entries {
		if entry.file != currentFile {
			if currentFile != "" {
				output.WriteByte('\n')
			}
			currentFile = entry.file
			fmt.Fprintf(&output, "### [`%s`](../%s)\n\n", currentFile, currentFile)
			output.WriteString("| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |\n")
			output.WriteString("|---|---|---:|---|---|\n")
		}
		lines := fmt.Sprintf("%d", entry.start)
		if entry.end > entry.start {
			lines = fmt.Sprintf("%d-%d", entry.start, entry.end)
		}
		symbol := fmt.Sprintf("[%s](../%s#L%d)", markdownCode(entry.name), entry.file, entry.start)
		fmt.Fprintf(&output, "| %s | %s | %s | %s | %s |\n", symbol, entry.kind, lines, markdownCode(entry.signature), markdownCell(entry.description))
	}
	return strings.TrimSpace(output.String())
}

// generatedCatalog composes the source-derived catalog sections.
func generatedCatalog(root string) (string, error) {
	goEntries, err := collectGoDeclarations(root)
	if err != nil {
		return "", err
	}
	javascriptEntries, javascriptTests, err := collectJavaScriptDeclarations(root)
	if err != nil {
		return "", err
	}
	sections := []string{
		"## Go declarations",
		renderCatalogEntries(goEntries),
		"## JavaScript declarations",
		renderCatalogEntries(javascriptEntries),
		"## JavaScript test cases",
		renderCatalogEntries(javascriptTests),
	}
	return strings.Join(sections, "\n\n"), nil
}

// checkCatalogDescriptions rejects maintained declarations without attached source documentation.
func checkCatalogDescriptions(root string) error {
	goEntries, err := collectGoDeclarations(root)
	if err != nil {
		return err
	}
	javascriptEntries, javascriptTests, err := collectJavaScriptDeclarations(root)
	if err != nil {
		return err
	}
	entries := append(goEntries, javascriptEntries...)
	entries = append(entries, javascriptTests...)
	missing := make([]string, 0)
	for _, entry := range entries {
		if entry.description == noDescription {
			missing = append(missing, fmt.Sprintf("%s:%d %s", entry.file, entry.start, entry.name))
		}
	}
	if len(missing) == 0 {
		invalid := make([]string, 0)
		for _, entry := range goEntries {
			name := entry.name
			if dot := strings.LastIndex(name, "."); dot >= 0 {
				name = name[dot+1:]
			}
			if !strings.HasPrefix(entry.description, name) {
				invalid = append(invalid, fmt.Sprintf("%s:%d %s", entry.file, entry.start, entry.name))
			}
		}
		if len(invalid) == 0 {
			return nil
		}
		return fmt.Errorf("%d Go declaration comments do not begin with the declared symbol: %s", len(invalid), strings.Join(invalid, ", "))
	}
	const reportedLimit = 20
	reported := missing
	if len(reported) > reportedLimit {
		reported = reported[:reportedLimit]
	}
	message := strings.Join(reported, ", ")
	if len(missing) > len(reported) {
		message += fmt.Sprintf(", and %d more", len(missing)-len(reported))
	}
	return fmt.Errorf("%d maintained declarations have no source description: %s", len(missing), message)
}

// replaceGeneratedRegion replaces exactly one marked block without changing maintained prose.
func replaceGeneratedRegion(document, begin, end, generated, label string) (string, error) {
	if strings.Count(document, begin) != 1 || strings.Count(document, end) != 1 {
		return "", fmt.Errorf("%s must contain exactly one generated marker pair", label)
	}
	prefix, remainder, _ := strings.Cut(document, begin)
	_, suffix, _ := strings.Cut(remainder, end)
	return prefix + begin + "\n\n" + generated + "\n\n" + end + suffix, nil
}

// catalogContent returns current and generated catalog document content.
func catalogContent(root string) (string, string, string, error) {
	path := filepath.Join(root, filepath.FromSlash(catalogDocument))
	current, err := os.ReadFile(path)
	if err != nil {
		return "", "", path, fmt.Errorf("%s: cannot read project catalog: %w", catalogDocument, err)
	}
	generated, err := generatedCatalog(root)
	if err != nil {
		return "", "", path, fmt.Errorf("generate project catalog: %w", err)
	}
	updated, err := replaceGeneratedRegion(string(current), catalogBegin, catalogEnd, generated, catalogDocument)
	if err != nil {
		return "", "", path, err
	}
	return string(current), updated, path, nil
}

// checkCatalog verifies that PROJECT_CATALOG.md matches source without writing it.
func checkCatalog(root string) error {
	current, updated, _, err := catalogContent(root)
	if err != nil {
		return err
	}
	if current != updated {
		return fmt.Errorf("%s: source declaration catalog is stale; run make docs-catalog-update", catalogDocument)
	}
	return checkCatalogDescriptions(root)
}

// runCatalogCommand checks or updates the source catalog.
func runCatalogCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "check" && args[0] != "update") {
		fmt.Fprintln(stderr, "catalog requires check or update")
		return 2
	}
	action := args[0]
	flags := flag.NewFlagSet("doccheck catalog "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "catalog %s accepts only --root\n", action)
		return 2
	}
	current, updated, path, err := catalogContent(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if action == "check" {
		if current != updated {
			fmt.Fprintf(stderr, "%s is stale; run make docs-catalog-update\n", catalogDocument)
			return 1
		}
		if err := checkCatalogDescriptions(*root); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "Project catalog is current.")
		return 0
	}
	if current == updated {
		fmt.Fprintln(stdout, "Project catalog is already current.")
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(stderr, "stat project catalog:", err)
		return 1
	}
	if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
		fmt.Fprintln(stderr, "write project catalog:", err)
		return 1
	}
	fmt.Fprintln(stdout, "Updated docs/PROJECT_CATALOG.md.")
	return 0
}
