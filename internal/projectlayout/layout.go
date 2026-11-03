// Package projectlayout identifies maintained source roots for repository checks.
package projectlayout

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ModuleRoot describes one maintained Go module outside the root module.
type ModuleRoot struct {
	Root   string
	Module string
}

// OptionalTestModules lists repository-relative optional test modules.
func OptionalTestModules() []string { return []string{"tests/tooling/agentgraph"} }

// OptionalProductionModules lists repository-relative optional production modules.
func OptionalProductionModules() []string { return []string{"tooling/agentgraph"} }

// OptionalGoRoots returns the optional production and test modules with their declared identities.
func OptionalGoRoots(root string) ([]ModuleRoot, error) {
	modules := make([]ModuleRoot, 0, 2)
	for _, relative := range append(OptionalProductionModules(), OptionalTestModules()...) {
		module, err := readModulePath(filepath.Join(root, filepath.FromSlash(relative), "go.mod"))
		if err != nil {
			return nil, err
		}
		modules = append(modules, ModuleRoot{Root: relative, Module: module})
	}
	return modules, nil
}

// OptionalGoRootsFS returns the optional modules from a read-only snapshot.
func OptionalGoRootsFS(fsys fs.FS) ([]ModuleRoot, error) {
	modules := make([]ModuleRoot, 0, 2)
	for _, relative := range append(OptionalProductionModules(), OptionalTestModules()...) {
		data, err := fs.ReadFile(fsys, relative+"/go.mod")
		if err != nil {
			return nil, err
		}
		module, err := parseModulePath(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s/go.mod: %w", relative, err)
		}
		modules = append(modules, ModuleRoot{Root: relative, Module: module})
	}
	return modules, nil
}

// readModulePath returns the module path from a go.mod file.
func readModulePath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return parseModulePath(string(data))
}

// parseModulePath extracts the module directive from go.mod content.
func parseModulePath(data string) (string, error) {
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("go.mod does not declare a module path")
}

// HasGoMod reports whether a directory contains a go.mod file.
func HasGoMod(path string) bool {
	info, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil && !info.IsDir()
}

// HasGoModFS reports whether a directory in fsys contains a go.mod file.
func HasGoModFS(fsys fs.FS, path string) bool {
	info, err := fs.Stat(fsys, path+"/go.mod")
	return err == nil && !info.IsDir()
}

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
				if path != base && HasGoMod(path) {
					return filepath.SkipDir
				}
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

// WalkOptionalGo visits maintained Go files in the declared optional modules.
func WalkOptionalGo(root string, visit fs.WalkDirFunc) error {
	for _, relative := range append(OptionalProductionModules(), OptionalTestModules()...) {
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
				if path != base && HasGoMod(path) {
					return filepath.SkipDir
				}
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
				if path != relative && HasGoModFS(fsys, path) {
					return fs.SkipDir
				}
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

// WalkOptionalGoFS visits maintained Go files in the declared optional modules.
func WalkOptionalGoFS(fsys fs.FS, visit fs.WalkDirFunc) error {
	for _, relative := range append(OptionalProductionModules(), OptionalTestModules()...) {
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
				if path != relative && HasGoModFS(fsys, path) {
					return fs.SkipDir
				}
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
