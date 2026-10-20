// Package testselect_test verifies suite ownership, filtering, and invalid selections.
package testselect_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"analysis/tooling/go/testselect"
)

// writeSuite creates a discoverable test input without requiring an installed runner.
func writeSuite(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// selectionTree provides one suite per runner, along with ignored E2E/support inputs.
func selectionTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{
		"tests/backend/database/run/unit/run_test.go",
		"tests/backend/database/run/integration/run_test.go",
		"tests/backend/server/integration/server_test.go",
		"tests/frontend/views/corpus/functional/corpus.test.ts",
		"tests/frontend/viewer/integration/viewer.spec.ts",
		"tests/contracts/http/integration/api-shapes.spec.ts",
		"tests/tooling/go/doccheck/unit/docs_test.go",
		"tests/shared/pathpolicy/unit/path_test.go",
		"tests/e2e/live/live_test.go",
		"tests/support/go/helper_test.go",
		"tests/frontend/viewer/integration/testdata/ignored.test.ts",
		"tests/frontend/jsx-runtime/unit/jsx-types.test.tsx",
	} {
		writeSuite(t, root, name)
	}
	return root
}

// TestSelectKeepsDomainComponentAndLayerOwnership verifies narrow requests do not pull in unrelated runners.
func TestSelectKeepsDomainComponentAndLayerOwnership(t *testing.T) {
	root := selectionTree(t)
	got, err := testselect.Select(root, testselect.Options{Domain: "backend", Component: "database/run", Layer: "unit"})
	if err != nil {
		t.Fatal(err)
	}
	want := testselect.Selection{GoPackages: []string{"./tests/backend/database/run/unit"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selection=%+v want %+v", got, want)
	}
	got, err = testselect.Select(root, testselect.Options{Runner: "go", Component: "database/run", Layer: "integration"})
	if err != nil || !reflect.DeepEqual(got.GoPackages, []string{"./tests/backend/database/run/integration"}) {
		t.Fatalf("Go component default=%+v err=%v", got, err)
	}
}

// TestSelectAllExcludesE2ESupportFixturesAndCompileOnlyAssertions verifies default ordinary discovery has one owner per suite.
func TestSelectAllExcludesE2ESupportFixturesAndCompileOnlyAssertions(t *testing.T) {
	got, err := testselect.Select(selectionTree(t), testselect.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.GoPackages) != 5 || len(got.NodeFiles) != 1 || len(got.BrowserFiles) != 2 {
		t.Fatalf("ordinary selection=%+v", got)
	}
}

// TestSelectRejectsInvalidOrIneffectiveRequests verifies malformed filters cannot produce a green empty run.
func TestSelectRejectsInvalidOrIneffectiveRequests(t *testing.T) {
	root := selectionTree(t)
	for name, options := range map[string]testselect.Options{
		"unknown domain":            {Domain: "unknown"},
		"unknown layer":             {Layer: "contract"},
		"unknown runner":            {Runner: "custom"},
		"component without domain":  {Component: "database/run"},
		"escape":                    {Domain: "backend", Component: "../frontend"},
		"absolute":                  {Domain: "backend", Component: "/tmp"},
		"missing component":         {Domain: "backend", Component: "missing"},
		"missing layer":             {Domain: "backend", Component: "database/run", Layer: "functional"},
		"old source package":        {Runner: "go", Package: "./backend/server"},
		"package and component":     {Runner: "go", Package: "./tests/backend/server/integration", Component: "server"},
		"e2e package":               {Runner: "go", Package: "./tests/e2e/live"},
		"test regex across runners": {Test: "Test.*"},
		"browser outside scope":     {Domain: "frontend", Runner: "browser", TestFile: "tests/contracts/http/integration/api-shapes.spec.ts"},
		"browser escape":            {Domain: "frontend", Runner: "browser", TestFile: "../viewer.spec.ts"},
		"race across runners":       {Race: true},
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := testselect.Select(root, options); err == nil {
				t.Fatalf("accepted ineffective selection %+v", got)
			}
		})
	}
}

// TestSelectRejectsSymlinkEscape verifies an apparently local component cannot redirect discovery outside its domain.
func TestSelectRejectsSymlinkEscape(t *testing.T) {
	root := selectionTree(t)
	if err := os.Symlink(filepath.Join(root, "tests/frontend"), filepath.Join(root, "tests/backend/escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := testselect.Select(root, testselect.Options{Domain: "backend", Component: "escape"}); err == nil {
		t.Fatal("accepted a component escaping its domain")
	}
}
