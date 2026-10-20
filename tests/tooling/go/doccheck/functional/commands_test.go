// commands_test.go supplies fixture construction and public command invocations for documentation scenarios.
package doccheck_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/tooling/go/doccheck"
)

// catalogCommand invokes the public catalog subcommand.
func catalogCommand(args []string, stdout, stderr io.Writer) int {
	return doccheck.Run(append([]string{"catalog"}, args...), stdout, stderr)
}

// stateCommand invokes the public documentation-state subcommand.
func stateCommand(args []string, stdout, stderr io.Writer) int {
	return doccheck.Run(append([]string{"state"}, args...), stdout, stderr)
}

// catalogText regenerates a synthetic catalog through the command and returns the published Markdown.
func catalogText(root string) (string, error) {
	path := filepath.Join(root, "docs/PROJECT_CATALOG.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte("# Catalog\n\n<!-- BEGIN GENERATED PROJECT CATALOG -->\n<!-- END GENERATED PROJECT CATALOG -->\n"), 0644); err != nil {
			return "", err
		}
	}
	var stdout, stderr bytes.Buffer
	if code := catalogCommand([]string{"update", "--root", root}, &stdout, &stderr); code != 0 {
		return "", fmt.Errorf("catalog exit %d: %s", code, stderr.String())
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

// catalogCheck verifies a catalog without changing it.
func catalogCheck(root string) error {
	var output bytes.Buffer
	if code := catalogCommand([]string{"check", "--root", root}, &output, &output); code != 0 {
		return fmt.Errorf("catalog exit %d: %s", code, output.String())
	}
	return nil
}

// catalogDescriptionCheck generates current rows before checking source descriptions through public dispatch.
func catalogDescriptionCheck(root string) error {
	if _, err := catalogText(root); err != nil {
		return err
	}
	return catalogCheck(root)
}

// stateFailures returns individual diagnostics from the non-mutating public state check.
func stateFailures(root string) ([]string, error) {
	var stdout, stderr bytes.Buffer
	code := stateCommand([]string{"check", "--root", root}, &stdout, &stderr)
	if code == 0 {
		return nil, nil
	}
	return strings.Split(strings.TrimSpace(stderr.String()), "\n"), nil
}

// stateDocumentFailures checks a controlled state document against its declared source files.
func stateDocumentFailures(t *testing.T, document string) (int, []string) {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "docs/A.md", "A\n")
	writeTestFile(t, root, "docs/B.md", "B\n")
	writeTestFile(t, root, "docs/DOC-STATE.md", document)
	failures, err := stateFailures(root)
	if err != nil {
		t.Fatal(err)
	}
	return 0, failures
}

// checkDocuments initializes unrelated contracts, then exercises the complete non-mutating documentation check.
func checkDocuments(t *testing.T, root string) []string {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, "config/database.something")); os.IsNotExist(err) {
		writeMigrationRepository(t, root, 2, 1)
	}
	if _, err := catalogText(root); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "docs/DOC-STATE.md")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		writeTestFile(t, root, "docs/DOC-STATE.md", "# State\n\n<!-- BEGIN DOCUMENT REVIEW DEPENDENCIES -->\n| Source document | Review dependents |\n|---|---|\n<!-- END DOCUMENT REVIEW DEPENDENCIES -->\n\n<!-- BEGIN GENERATED DOCUMENT STATE -->\n<!-- END GENERATED DOCUMENT STATE -->\n")
	}
	var output bytes.Buffer
	if code := stateCommand([]string{"update", "--root", root}, &output, &output); code != 0 {
		t.Fatalf("state setup=%d: %s", code, output.String())
	}
	output.Reset()
	code := doccheck.Run([]string{"check", "--root", root}, io.Discard, &output)
	if code == 0 {
		return nil
	}
	if code != 1 {
		t.Fatalf("check could not run: %d: %s", code, output.String())
	}
	var failures []string
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		if !strings.HasPrefix(line, "documentation consistency check failed with ") {
			failures = append(failures, line)
		}
	}
	return failures
}
