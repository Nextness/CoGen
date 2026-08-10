//go:build unit

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCollectGoDeclarationsIncludesFunctionsMethodsTypesAndSourceDescriptions verifies collect go declarations includes functions methods types and source descriptions.
func TestCollectGoDeclarationsIncludesFunctionsMethodsTypesAndSourceDescriptions(t *testing.T) {
	root := t.TempDir()
	source := `package sample

// Record stores one value.
type Record struct { Value string }

type Reader interface { Read() error }
type Alias = Record

// Read returns the current value.
func (record *Record) Read(input string) (string, error) { return record.Value, nil }

func helper() {}
`
	writeTestFile(t, root, "src/sample.go", source)
	entries, err := collectGoDeclarations(root)
	if err != nil {
		t.Fatal(err)
	}
	output := renderCatalogEntries(entries)
	for _, expected := range []string{"`Record`](../src/sample.go#L4) | struct", "Record stores one value.", "`Reader`](../src/sample.go#L6) | interface", "`Alias`](../src/sample.go#L7) | type alias", "`(*Record).Read`](../src/sample.go#L10) | method", "Read returns the current value.", "`helper`](../src/sample.go#L12) | function", noDescription} {
		if !strings.Contains(output, expected) {
			t.Errorf("Go catalog missing %q:\n%s", expected, output)
		}
	}
}

// TestCollectGoDeclarationsClassifiesTests verifies collect go declarations classifies tests.
func TestCollectGoDeclarationsClassifiesTests(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "src/sample_unit_test.go", "package sample\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\nfunc helper(t *testing.T) {}\n")
	entries, err := collectGoDeclarations(root)
	if err != nil {
		t.Fatal(err)
	}
	output := renderCatalogEntries(entries)
	if !strings.Contains(output, "`TestValue`](../src/sample_unit_test.go#L3) | test") || !strings.Contains(output, "`helper`](../src/sample_unit_test.go#L4) | function") {
		t.Fatalf("Go test catalog classifications are missing:\n%s", output)
	}
}

// TestCollectJavaScriptDeclarationsIncludesJSDocClassesMethodsAndTests verifies collect java script declarations includes js doc classes methods and tests.
func TestCollectJavaScriptDeclarationsIncludesJSDocClassesMethodsAndTests(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "frontend/src/example.js", `/**
 * Renders a value.
 * @param {string} value - Value to render.
 * @returns {string} Rendered value.
 */
export function renderValue(value) { return value; }

/** Represents a widget. */
export class Widget {
  /** Updates the widget. */
  async update(value) { return value; }
}
`)
	writeTestFile(t, root, "frontend/tests/unit/example.test.js", "test('renders a value', () => {})\n")
	declarations, tests, err := collectJavaScriptDeclarations(root)
	if err != nil {
		t.Fatal(err)
	}
	output := renderCatalogEntries(declarations)
	for _, expected := range []string{"`renderValue`]", "Renders a value.", "@param {string} value", "@returns {string}", "`Widget`]", "| class |", "`Widget.update`]", "| method |"} {
		if !strings.Contains(output, expected) {
			t.Errorf("JavaScript catalog missing %q:\n%s", expected, output)
		}
	}
	testOutput := renderCatalogEntries(tests)
	if !strings.Contains(testOutput, "renders a value") || strings.Contains(testOutput, noDescription) {
		t.Fatalf("JavaScript test catalog is incomplete:\n%s", testOutput)
	}
}

// TestCollectJavaScriptDeclarationsIncludesTypeScriptSyntax verifies collect java script declarations catalogs type script generics and modifiers.
func TestCollectJavaScriptDeclarationsIncludesTypeScriptSyntax(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "frontend/src/example.ts", `/** Finds values by key. */
export function findValue<T>(items, key: string): T | undefined { return undefined; }

/** Represents a generic holder. */
export class Holder<T> {
  /** Stores one value. */
  private store(value: T): void {}

  /** Readies content. */
  async prepare(url: string): Promise<void> {}

  /** Runs a callback with nested parameter types. */
  run(callback: (item: T) => void): void {}
}
`)
	declarations, _, err := collectJavaScriptDeclarations(root)
	if err != nil {
		t.Fatal(err)
	}
	output := renderCatalogEntries(declarations)
	for _, expected := range []string{"`findValue`]", "`Holder`]", "`Holder.store`]", "`Holder.prepare`]", "`Holder.run`]", "| function |", "| class |", "| method |"} {
		if !strings.Contains(output, expected) {
			t.Errorf("TypeScript catalog missing %q:\n%s", expected, output)
		}
	}
	for _, wanted := range []string{"findValue", "Holder.store", "Holder.prepare", "Holder.run"} {
		found := false
		for _, entry := range declarations {
			if entry.name == wanted {
				found = true
				if entry.description == noDescription {
					t.Errorf("TypeScript declaration %q lacks a source description", wanted)
				}
			}
		}
		if !found {
			t.Errorf("TypeScript declaration %q was not cataloged", wanted)
		}
	}
}

// TestCollectJavaScriptDeclarationsExcludesDeclarationFiles verifies collect java script declarations skips dot d ts files.
func TestCollectJavaScriptDeclarationsExcludesDeclarationFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "frontend/src/vendor.d.ts", "export function omittedAmbient(): void;\ndeclare module '*omitted' { export const value: number; }\n")
	writeTestFile(t, root, "frontend/src/example.ts", "export function included(): void {}\n")
	declarations, _, err := collectJavaScriptDeclarations(root)
	if err != nil {
		t.Fatal(err)
	}
	output := renderCatalogEntries(declarations)
	if strings.Contains(output, "omittedAmbient") || strings.Contains(output, "omitted") {
		t.Fatalf("declaration file content leaked into catalog:\n%s", output)
	}
	if !strings.Contains(output, "included") {
		t.Fatalf("declaration file exclusion hid real declarations:\n%s", output)
	}
}

// TestCollectJavaScriptDeclarationsExcludesDist verifies collect java script declarations skips the assembled output directory.
func TestCollectJavaScriptDeclarationsExcludesDist(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "frontend/dist/app.js", "function omittedDist() {}\n")
	writeTestFile(t, root, "frontend/src/example.js", "export function supported() {}\n")
	declarations, _, err := collectJavaScriptDeclarations(root)
	if err != nil {
		t.Fatal(err)
	}
	output := renderCatalogEntries(declarations)
	if strings.Contains(output, "omitted") || !strings.Contains(output, "supported") {
		t.Fatalf("dist exclusions are incorrect:\n%s", output)
	}
}

// TestCollectJavaScriptDeclarationsExcludesVendorAndRejectsUnsupportedSyntax verifies collect java script declarations excludes vendor and rejects unsupported syntax.
func TestCollectJavaScriptDeclarationsExcludesVendorAndRejectsUnsupportedSyntax(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "frontend/vendor/generated.js", "function omitted() {}\n")
	writeTestFile(t, root, "frontend/node_modules/package/index.js", "function omittedDependency() {}\n")
	writeTestFile(t, root, "frontend/src/example.js", "export function supported() {}\n")
	declarations, _, err := collectJavaScriptDeclarations(root)
	if err != nil {
		t.Fatal(err)
	}
	output := renderCatalogEntries(declarations)
	if strings.Contains(output, "omitted") || !strings.Contains(output, "supported") {
		t.Fatalf("JavaScript exclusions are incorrect:\n%s", output)
	}
	writeTestFile(t, root, "frontend/src/example.js", "export function broken(\n")
	if _, _, err := collectJavaScriptDeclarations(root); err == nil || !strings.Contains(err.Error(), "unsupported JavaScript") {
		t.Fatalf("unsupported JavaScript error = %v", err)
	}
	writeTestFile(t, root, "frontend/src/example.js", "export function* generated() {}\n")
	if _, _, err := collectJavaScriptDeclarations(root); err == nil || !strings.Contains(err.Error(), "unsupported JavaScript") {
		t.Fatalf("unsupported JavaScript generator error = %v", err)
	}
}

// TestCatalogCheckIsNonMutatingAndUpdateChangesOnlyMarkers verifies catalog check is non mutating and update changes only markers.
func TestCatalogCheckIsNonMutatingAndUpdateChangesOnlyMarkers(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "src/sample.go", "package sample\nfunc value() {}\n")
	writeTestFile(t, root, "frontend/src/example.js", "")
	writeTestFile(t, root, "frontend/example.js", "")
	document := "# Catalog\n\nMaintained introduction.\n\n" + catalogBegin + "\n\nstale\n\n" + catalogEnd + "\n\nMaintained ending.\n"
	writeTestFile(t, root, catalogDocument, document)
	if err := checkCatalog(root); err == nil {
		t.Fatal("stale catalog passed check")
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(catalogDocument)))
	if err != nil || string(data) != document {
		t.Fatalf("checkCatalog mutated the document: %v", err)
	}
	if status := runCatalogCommand([]string{"update", "--root", root}, &strings.Builder{}, &strings.Builder{}); status != 0 {
		t.Fatalf("catalog update status = %d", status)
	}
	updated, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(catalogDocument)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "Maintained introduction.") || !strings.Contains(string(updated), "Maintained ending.") || strings.Contains(string(updated), "stale") {
		t.Fatalf("catalog update changed maintained content:\n%s", updated)
	}
}

// TestCheckCatalogDescriptionsRejectsMissingComments verifies check catalog descriptions rejects missing comments.
func TestCheckCatalogDescriptionsRejectsMissingComments(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "src/sample.go", "package sample\nfunc undocumented() {}\n")
	writeTestFile(t, root, "frontend/src/example.js", "/** Documents documented. */\nexport function documented() {}\n")
	writeTestFile(t, root, "frontend/example.js", "")
	err := checkCatalogDescriptions(root)
	if err == nil || !strings.Contains(err.Error(), "src/sample.go:2 undocumented") {
		t.Fatalf("missing source description error = %v", err)
	}
	writeTestFile(t, root, "src/sample.go", "package sample\n// documented records its source description.\nfunc documented() {}\n")
	writeTestFile(t, root, "frontend/src/example.js", "export function undocumentedJavaScript() {}\n")
	err = checkCatalogDescriptions(root)
	if err == nil || !strings.Contains(err.Error(), "frontend/src/example.js:1 undocumentedJavaScript") {
		t.Fatalf("missing JavaScript source description error = %v", err)
	}
}

// TestCheckCatalogDescriptionsAcceptsMaintainedComments verifies check catalog descriptions accepts maintained comments.
func TestCheckCatalogDescriptionsAcceptsMaintainedComments(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "src/sample.go", "package sample\n// documented records its source description.\nfunc documented() {}\n")
	writeTestFile(t, root, "frontend/src/example.js", "/** Documents rendered output. */\nexport function rendered() {}\n")
	writeTestFile(t, root, "frontend/tests/unit/example.test.js", "test('keeps its title as the description', () => {})\n")
	if err := checkCatalogDescriptions(root); err != nil {
		t.Fatal(err)
	}
}

// TestCheckCatalogDescriptionsRejectsMisnamedGoComments verifies check catalog descriptions rejects misnamed go comments.
func TestCheckCatalogDescriptionsRejectsMisnamedGoComments(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "src/sample.go", "package sample\n// AnotherName does not identify the declaration.\nfunc documented() {}\n")
	writeTestFile(t, root, "frontend/src/example.js", "")
	writeTestFile(t, root, "frontend/example.js", "")
	err := checkCatalogDescriptions(root)
	if err == nil || !strings.Contains(err.Error(), "comments do not begin with the declared symbol") {
		t.Fatalf("misnamed source description error = %v", err)
	}
}
