package testselect

import (
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Suite is a narrow read-only suite inventory DTO owned by testselect.
type Suite struct {
	Path   string
	Runner string
}

// SnapshotSuitesFS lists ordinary suite files from a read-only source snapshot.
func SnapshotSuitesFS(fsys fs.FS) ([]Suite, error) {
	var suites []Suite
	if _, err := fs.Stat(fsys, "tests"); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return suites, nil
		}
		return nil, err
	}
	err := fs.WalkDir(fsys, "tests", func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" || entry.Name() == "node_modules" || strings.HasSuffix(entry.Name(), "-snapshots") {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		layer := path.Base(path.Dir(filePath))
		if layer != "unit" && layer != "functional" && layer != "integration" {
			return nil
		}
		switch {
		case strings.HasSuffix(filePath, "_test.go"):
			suites = append(suites, Suite{Path: filePath, Runner: "go"})
		case strings.HasSuffix(filePath, ".test.ts"):
			suites = append(suites, Suite{Path: filePath, Runner: "node"})
		case strings.HasSuffix(filePath, ".spec.ts"):
			suites = append(suites, Suite{Path: filePath, Runner: "browser"})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(suites, func(left, right int) bool {
		if suites[left].Path == suites[right].Path {
			return suites[left].Runner < suites[right].Runner
		}
		return suites[left].Path < suites[right].Path
	})
	return suites, nil
}
