// Package projectlayout identifies maintained source roots for repository checks.
package projectlayout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// WalkGo visits maintained Go files across production, tooling, and external tests.
func WalkGo(root string, visit fs.WalkDirFunc) error {
	for _, relative := range []string{"backend", "tooling/go", "internal", "tests"} {
		base := filepath.Join(root, filepath.FromSlash(relative))
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		if err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" || entry.Name() == "node_modules" || strings.HasSuffix(entry.Name(), "-snapshots") {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() || filepath.Ext(path) != ".go" {
				return nil
			}
			return visit(path, entry, nil)
		}); err != nil {
			return err
		}
	}
	return nil
}

// WalkGoFS visits maintained Go files in a read-only filesystem snapshot.
// Paths are relative to fsys rather than an operating-system root.
func WalkGoFS(fsys fs.FS, visit fs.WalkDirFunc) error {
	for _, relative := range []string{"backend", "tooling/go", "internal", "tests"} {
		if _, err := fs.Stat(fsys, relative); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := fs.WalkDir(fsys, relative, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" || entry.Name() == "node_modules" || strings.HasSuffix(entry.Name(), "-snapshots") {
					return fs.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() || filepath.Ext(path) != ".go" {
				return nil
			}
			return visit(path, entry, nil)
		}); err != nil {
			return err
		}
	}
	return nil
}

// IsTestPath reports canonical external test and support ownership.
func IsTestPath(relative string) bool { return strings.HasPrefix(filepath.ToSlash(relative), "tests/") }

// JavaScriptRoots lists product, developer tooling, and testing sources for catalog discovery.
func JavaScriptRoots() []string { return []string{"frontend", "tooling/node", "tests"} }
