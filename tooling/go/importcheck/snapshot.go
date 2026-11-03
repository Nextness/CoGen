package importcheck

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"analysis/internal/projectlayout"
)

// ImportEdge is a narrow read-only import relationship DTO owned by importcheck.
type ImportEdge struct {
	Importer string
	Target   string
	Test     bool
}

// readModulePathFS returns the module path declared by go.mod in a read-only snapshot.
func readModulePathFS(fsys fs.FS) (string, error) {
	data, err := fs.ReadFile(fsys, "go.mod")
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("go.mod does not declare a module path")
}

// SnapshotImportsFS returns maintained Go import edges from a read-only source snapshot.
func SnapshotImportsFS(fsys fs.FS) ([]ImportEdge, error) {
	module, err := readModulePathFS(fsys)
	if err != nil {
		return nil, err
	}
	files := token.NewFileSet()
	var edges []ImportEdge
	err = projectlayout.WalkGoFS(fsys, func(filePath string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || path.Ext(filePath) != ".go" {
			return nil
		}
		data, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", filePath, err)
		}
		file, err := parser.ParseFile(files, filePath, data, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", filePath, err)
		}
		directory := path.Dir(filePath)
		importer := module
		if directory != "." {
			importer = module + "/" + directory
		}
		for _, spec := range file.Imports {
			target, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("parse import in %s: %w", filePath, err)
			}
			edges = append(edges, ImportEdge{
				Importer: importer,
				Target:   target,
				Test:     strings.HasSuffix(filePath, "_test.go"),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(edges, func(left, right int) bool {
		if edges[left].Importer == edges[right].Importer {
			if edges[left].Target == edges[right].Target {
				return !edges[left].Test && edges[right].Test
			}
			return edges[left].Target < edges[right].Target
		}
		return edges[left].Importer < edges[right].Importer
	})
	return edges, nil
}

// SQLCUnit is a narrow read-only DTO for one sqlc generation unit.
type SQLCUnit struct {
	Schema  string
	Queries []string
	Output  string
}

// SnapshotSQLCUnitsFS parses config/sqlc.yaml from a read-only source snapshot.
// A missing configuration yields no units.
func SnapshotSQLCUnitsFS(fsys fs.FS) ([]SQLCUnit, error) {
	data, err := fs.ReadFile(fsys, "config/sqlc.yaml")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config/sqlc.yaml: %w", err)
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
	units := make([]SQLCUnit, 0, len(config.SQL))
	for _, entry := range config.SQL {
		units = append(units, SQLCUnit{
			Schema:  entry.Schema,
			Queries: append([]string(nil), entry.Queries...),
			Output:  entry.Gen.Go.Out,
		})
	}
	return units, nil
}
