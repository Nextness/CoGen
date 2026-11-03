package codefacts

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"analysis/tooling/go/doccheck"
	"analysis/tooling/go/importcheck"
	"analysis/tooling/go/rawsqlcheck/ownership"
	"analysis/tooling/go/testselect"
)

// FormatVersion identifies the portable facts envelope schema.
const FormatVersion = 1

// Producer identifies the portable facts exporter.
const Producer = "code-facts"

// Workspace identifies the repository, worktree, and captured input.
type Workspace struct {
	RepositoryID     string `json:"repository_id"`
	WorktreeID       string `json:"worktree_id"`
	InputFingerprint string `json:"input_fingerprint"`
}

// Declaration is one syntactic source declaration owned by doccheck.
type Declaration struct {
	Module      string `json:"module"`
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	Description string `json:"description"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	SourceHash  string `json:"source_hash"`
	Evidence    string `json:"evidence"`
}

// Import is one maintained Go import edge owned by importcheck.
type Import struct {
	Importer string `json:"importer"`
	Target   string `json:"target"`
	Test     bool   `json:"test"`
}

// SQLCall is one raw SQL call site owned by rawsqlcheck/ownership.
type SQLCall struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Method   string `json:"method"`
	Category string `json:"category"`
	Reason   string `json:"reason"`
}

// SQLGenerationMapping is one sqlc generation unit owned by importcheck.
type SQLGenerationMapping struct {
	Schema  string   `json:"schema"`
	Queries []string `json:"queries"`
	Output  string   `json:"output"`
}

// Suite is one ordinary test suite file owned by testselect.
type Suite struct {
	Path   string `json:"path"`
	Runner string `json:"runner"`
}

// ReviewMetadata is one acknowledged document owned by doccheck.
type ReviewMetadata struct {
	Document   string   `json:"document"`
	Hash       string   `json:"hash"`
	Dependents []string `json:"dependents"`
}

// PolicyFact describes the capture policy used for one envelope.
type PolicyFact struct {
	Roots              []string `json:"roots"`
	ExcludeDirs        []string `json:"exclude_dirs"`
	ExcludeInternalSQL bool     `json:"exclude_internal_sql"`
}

// Envelope is the versioned portable fact export.
type Envelope struct {
	FormatVersion         int                    `json:"format_version"`
	Producer              string                 `json:"producer"`
	Workspace             Workspace              `json:"workspace"`
	Policy                PolicyFact             `json:"policy"`
	Files                 []FileFact             `json:"files"`
	Declarations          []Declaration          `json:"declarations"`
	Imports               []Import               `json:"imports"`
	SQLCalls              []SQLCall              `json:"sql_calls"`
	SQLGenerationMappings []SQLGenerationMapping `json:"sql_generation_mappings"`
	Suites                []Suite                `json:"suites"`
	ReviewMetadata        []ReviewMetadata       `json:"review_metadata"`
}

// Options carries the explicit workspace identity and capture policy.
type Options struct {
	RepositoryID string
	WorktreeID   string
	Policy       Policy
}

// Build derives the portable facts envelope from a frozen read-only snapshot.
// Owner extractors read only the captured bytes; the live tree is never reread.
func Build(fsys fs.FS, options Options) (Envelope, error) {
	policy := options.Policy
	if len(policy.Roots) == 0 {
		policy = DefaultPolicy
	}
	module, err := readModulePath(fsys)
	if err != nil {
		return Envelope{}, err
	}
	facts, err := FileFacts(fsys, policy)
	if err != nil {
		return Envelope{}, err
	}
	declarations, err := buildDeclarations(fsys, module)
	if err != nil {
		return Envelope{}, err
	}
	imports, err := buildImports(fsys)
	if err != nil {
		return Envelope{}, err
	}
	sqlCalls, err := buildSQLCalls(fsys)
	if err != nil {
		return Envelope{}, err
	}
	mappings, err := buildSQLGenerationMappings(fsys)
	if err != nil {
		return Envelope{}, err
	}
	suites, err := buildSuites(fsys)
	if err != nil {
		return Envelope{}, err
	}
	reviewMetadata, err := buildReviewMetadata(fsys)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		FormatVersion: FormatVersion,
		Producer:      Producer,
		Workspace: Workspace{
			RepositoryID:     options.RepositoryID,
			WorktreeID:       options.WorktreeID,
			InputFingerprint: Fingerprint(facts),
		},
		Policy: PolicyFact{
			Roots:              append([]string(nil), policy.Roots...),
			ExcludeDirs:        append([]string(nil), policy.ExcludeDirs...),
			ExcludeInternalSQL: policy.ExcludeInternalSQL,
		},
		Files:                 facts,
		Declarations:          declarations,
		Imports:               imports,
		SQLCalls:              sqlCalls,
		SQLGenerationMappings: mappings,
		Suites:                suites,
		ReviewMetadata:        reviewMetadata,
	}, nil
}

// readModulePath returns the module path declared by go.mod in a read-only snapshot.
func readModulePath(fsys fs.FS) (string, error) {
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

// buildDeclarations converts doccheck's catalog DTO into portable declarations.
func buildDeclarations(fsys fs.FS, module string) ([]Declaration, error) {
	entries, err := doccheck.SnapshotCatalog(fsys)
	if err != nil {
		return nil, err
	}
	declarations := make([]Declaration, 0, len(entries))
	for _, entry := range entries {
		declarations = append(declarations, Declaration{
			Module:      module,
			Path:        entry.Path,
			Kind:        entry.Kind,
			Name:        entry.Name,
			Signature:   entry.Signature,
			Description: entry.Description,
			StartLine:   entry.StartLine,
			EndLine:     entry.EndLine,
			SourceHash:  entry.SourceHash,
			Evidence:    "syntactic",
		})
	}
	return declarations, nil
}

// buildImports converts importcheck's import DTO into portable imports.
func buildImports(fsys fs.FS) ([]Import, error) {
	edges, err := importcheck.SnapshotImportsFS(fsys)
	if err != nil {
		return nil, err
	}
	imports := make([]Import, 0, len(edges))
	for _, edge := range edges {
		imports = append(imports, Import{Importer: edge.Importer, Target: edge.Target, Test: edge.Test})
	}
	sort.Slice(imports, func(left, right int) bool {
		if imports[left].Importer == imports[right].Importer {
			if imports[left].Target == imports[right].Target {
				return !imports[left].Test && imports[right].Test
			}
			return imports[left].Target < imports[right].Target
		}
		return imports[left].Importer < imports[right].Importer
	})
	return imports, nil
}

// buildSQLCalls converts ownership's raw SQL call DTO into portable calls.
func buildSQLCalls(fsys fs.FS) ([]SQLCall, error) {
	calls, err := ownership.SnapshotCalls(fsys)
	if err != nil {
		return nil, err
	}
	result := make([]SQLCall, 0, len(calls))
	for _, call := range calls {
		result = append(result, SQLCall{
			Path:     call.Path,
			Line:     call.Line,
			Method:   call.Method,
			Category: call.Category,
			Reason:   call.Reason,
		})
	}
	return result, nil
}

// buildSQLGenerationMappings converts importcheck's sqlc DTO into portable mappings.
func buildSQLGenerationMappings(fsys fs.FS) ([]SQLGenerationMapping, error) {
	units, err := importcheck.SnapshotSQLCUnitsFS(fsys)
	if err != nil {
		return nil, err
	}
	mappings := make([]SQLGenerationMapping, 0, len(units))
	for _, unit := range units {
		mappings = append(mappings, SQLGenerationMapping{
			Schema:  unit.Schema,
			Queries: append([]string(nil), unit.Queries...),
			Output:  unit.Output,
		})
	}
	return mappings, nil
}

// buildSuites converts testselect's suite DTO into portable suites.
func buildSuites(fsys fs.FS) ([]Suite, error) {
	suites, err := testselect.SnapshotSuitesFS(fsys)
	if err != nil {
		return nil, err
	}
	result := make([]Suite, 0, len(suites))
	for _, suite := range suites {
		result = append(result, Suite{Path: suite.Path, Runner: suite.Runner})
	}
	return result, nil
}

// buildReviewMetadata converts doccheck's review DTO into portable metadata.
func buildReviewMetadata(fsys fs.FS) ([]ReviewMetadata, error) {
	metadata, err := doccheck.SnapshotReviewMetadata(fsys)
	if err != nil {
		return nil, err
	}
	result := make([]ReviewMetadata, 0, len(metadata))
	for _, item := range metadata {
		result = append(result, ReviewMetadata{
			Document:   item.Document,
			Hash:       item.Hash,
			Dependents: append([]string(nil), item.Dependents...),
		})
	}
	return result, nil
}
