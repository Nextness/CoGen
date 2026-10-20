// Package testenv resolves checked-in inputs independently of test working directories.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// repositoryRoot is captured before tests change their working directory.
var repositoryRoot = locateRoot()

// locateRoot finds the module's Makefile from the helper source or initial test directory.
func locateRoot() string {
	_, source, _, _ := runtime.Caller(0)
	cwd, _ := os.Getwd()
	for _, start := range []string{filepath.Dir(source), cwd} {
		for directory := start; directory != "." && directory != ""; directory = filepath.Dir(directory) {
			if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
				if _, err := os.Stat(filepath.Join(directory, "Makefile")); err == nil {
					return directory
				}
			}
			if parent := filepath.Dir(directory); parent == directory {
				break
			}
		}
	}
	panic(fmt.Errorf("cannot locate repository Go module and Makefile"))
}

// Root returns the repository checkout containing the test harness.
func Root() string { return repositoryRoot }

// Path resolves checked-in repository input paths without depending on package depth.
func Path(parts ...string) string {
	return filepath.Join(append([]string{repositoryRoot}, parts...)...)
}
