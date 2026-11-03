package ownership

import (
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

// generatedSnapshotPath reports whether a repository-relative Go path lives under a reserved internal/sql directory.
func generatedSnapshotPath(relativePath string) bool {
	segments := strings.Split(relativePath, "/")
	for index := 0; index+1 < len(segments); index++ {
		if segments[index] == "internal" && segments[index+1] == "sql" {
			return true
		}
	}
	return false
}

// SnapshotCalls returns raw SQL call sites from a read-only source snapshot.
// It does not read approved-location configuration or classify categories.
func SnapshotCalls(fsys fs.FS) ([]Call, error) {
	files := token.NewFileSet()
	var calls []Call
	err := projectlayout.WalkGoFS(fsys, func(filePath string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || path.Ext(filePath) != ".go" {
			return nil
		}
		if generatedSnapshotPath(filePath) {
			return nil
		}
		data, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", filePath, err)
		}
		file, err := parser.ParseFile(files, filePath, data, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", filePath, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || !rawSQLMethods[selector.Sel.Name] {
				return true
			}
			if selector.Sel.Name == "Query" && isURLQueryReceiver(selector.X) {
				return true
			}
			calls = append(calls, Call{
				Path:     filePath,
				Line:     files.Position(node.Pos()).Line,
				Method:   selector.Sel.Name,
				Category: "unclassified",
				Reason:   "portable source snapshot",
			})
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(calls, func(left, right int) bool {
		if calls[left].Path == calls[right].Path {
			return calls[left].Line < calls[right].Line
		}
		return calls[left].Path < calls[right].Path
	})
	return calls, nil
}
