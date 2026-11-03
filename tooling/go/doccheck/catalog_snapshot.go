package doccheck

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strings"

	"analysis/internal/projectlayout"
)

// CatalogEntry is a narrow read-only declaration DTO owned by doccheck.
// It contains only source-derived facts and is safe to serialize.
type CatalogEntry struct {
	Path        string
	Kind        string
	Name        string
	Signature   string
	Description string
	StartLine   int
	EndLine     int
	SourceHash  string
}

// SnapshotCatalog derives catalog entries from a read-only source snapshot.
// It covers maintained and optional source; the same captured bytes produce the same ordered entries.
func SnapshotCatalog(fsys fs.FS) ([]CatalogEntry, error) {
	files := token.NewFileSet()
	var entries []CatalogEntry
	visit := func(filePath string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || path.Ext(filePath) != ".go" {
			return nil
		}
		if generatedGoPath(filePath) {
			return nil
		}
		data, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", filePath, err)
		}
		hash := sha256.Sum256(data)
		file, err := parser.ParseFile(files, filePath, data, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", filePath, err)
		}
		isTestFile := strings.HasSuffix(filePath, "_test.go")
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
				entries = append(entries, CatalogEntry{
					Path:        filePath,
					Kind:        kind,
					Name:        name,
					Signature:   signature,
					Description: sourceDescription(value.Doc),
					StartLine:   files.Position(value.Pos()).Line,
					EndLine:     files.Position(value.End()).Line,
					SourceHash:  "sha256:" + hex.EncodeToString(hash[:]),
				})
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
					entries = append(entries, CatalogEntry{
						Path:        filePath,
						Kind:        goDeclarationKind(spec),
						Name:        spec.Name.Name,
						Signature:   "type " + nodeText(files, spec),
						Description: sourceDescription(doc),
						StartLine:   files.Position(spec.Pos()).Line,
						EndLine:     files.Position(spec.End()).Line,
						SourceHash:  "sha256:" + hex.EncodeToString(hash[:]),
					})
				}
			}
		}
		return nil
	}
	if err := projectlayout.WalkGoFS(fsys, visit); err != nil {
		return nil, err
	}
	if err := projectlayout.WalkOptionalGoFS(fsys, visit); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].Path == entries[right].Path {
			if entries[left].StartLine == entries[right].StartLine {
				return entries[left].Name < entries[right].Name
			}
			return entries[left].StartLine < entries[right].StartLine
		}
		return entries[left].Path < entries[right].Path
	})
	return entries, nil
}
