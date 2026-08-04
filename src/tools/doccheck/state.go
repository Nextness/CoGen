package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	stateDocument   = "docs/DOC-STATE.md"
	dependencyBegin = "<!-- BEGIN DOCUMENT REVIEW DEPENDENCIES -->"
	dependencyEnd   = "<!-- END DOCUMENT REVIEW DEPENDENCIES -->"
	stateBegin      = "<!-- BEGIN GENERATED DOCUMENT STATE -->"
	stateEnd        = "<!-- END GENERATED DOCUMENT STATE -->"
)

var markdownPathLink = regexp.MustCompile(`\[([^]]+)\]\(([^)]+)\)`)

// documentState stores one tracked documentation path, digest, and review dependents.
type documentState struct {
	path string
	hash string
}

// documentationFiles returns every maintained regular file under docs.
func documentationFiles(root string) ([]string, error) {
	docsRoot := filepath.Join(root, "docs")
	var paths []string
	err := filepath.WalkDir(docsRoot, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if item.IsDir() {
			if relative == "docs/ref" {
				return filepath.SkipDir
			}
			return nil
		}
		if !item.Type().IsRegular() || relative == stateDocument {
			return nil
		}
		paths = append(paths, relative)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list maintained documentation: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

// hashDocumentation hashes maintained documentation in deterministic path order.
func hashDocumentation(root string) ([]documentState, error) {
	paths, err := documentationFiles(root)
	if err != nil {
		return nil, err
	}
	states := make([]documentState, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", path, err)
		}
		hash := sha256.Sum256(data)
		states = append(states, documentState{path: path, hash: hex.EncodeToString(hash[:])})
	}
	return states, nil
}

// markedContent returns content strictly between one marker pair.
func markedContent(document, begin, end, label string) (string, error) {
	if strings.Count(document, begin) != 1 || strings.Count(document, end) != 1 {
		return "", fmt.Errorf("%s must contain exactly one %s marker pair", stateDocument, label)
	}
	_, remainder, _ := strings.Cut(document, begin)
	content, _, _ := strings.Cut(remainder, end)
	return content, nil
}

// linkedPaths parses path labels from one Markdown table cell.
func linkedPaths(cell string) ([]string, error) {
	matches := markdownPathLink.FindAllStringSubmatch(cell, -1)
	if len(matches) == 0 {
		if strings.TrimSpace(cell) == "None" {
			return nil, nil
		}
		return nil, fmt.Errorf("expected Markdown path link or None")
	}
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		label := strings.TrimSpace(match[1])
		destination := strings.TrimSpace(match[2])
		cleanLabel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(label)))
		if cleanLabel != label || !strings.HasPrefix(label, "docs/") || label == stateDocument || strings.HasPrefix(label, "docs/ref/") {
			return nil, fmt.Errorf("excluded or escaping documentation path %q", label)
		}
		expectedDestination := strings.TrimPrefix(label, "docs/")
		if destination != expectedDestination {
			return nil, fmt.Errorf("documentation link for %s must target %s", label, expectedDestination)
		}
		paths = append(paths, label)
	}
	return paths, nil
}

// parseDependencies reads the manually maintained review-dependency table.
func parseDependencies(document string) (map[string][]string, []string) {
	content, err := markedContent(document, dependencyBegin, dependencyEnd, "dependency")
	if err != nil {
		return nil, []string{err.Error()}
	}
	dependencies := make(map[string][]string)
	var failures []string
	for index, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "| Source document |") || strings.HasPrefix(trimmed, "|---") {
			continue
		}
		parts := strings.Split(trimmed, "|")
		if len(parts) != 4 {
			failures = append(failures, fmt.Sprintf("%s dependency row %d is malformed", stateDocument, index+1))
			continue
		}
		sources, sourceErr := linkedPaths(parts[1])
		dependents, dependentErr := linkedPaths(parts[2])
		if sourceErr != nil || len(sources) != 1 {
			failures = append(failures, fmt.Sprintf("%s dependency row %d has an invalid source: %v", stateDocument, index+1, sourceErr))
			continue
		}
		if dependentErr != nil {
			failures = append(failures, fmt.Sprintf("%s dependency row %d has invalid dependents: %v", stateDocument, index+1, dependentErr))
			continue
		}
		if _, duplicate := dependencies[sources[0]]; duplicate {
			failures = append(failures, fmt.Sprintf("%s dependency source is duplicated: %s", stateDocument, sources[0]))
			continue
		}
		dependencies[sources[0]] = dependents
	}
	return dependencies, failures
}

// parseStoredState reads and validates the generated state table.
func parseStoredState(document string) (map[string]string, []string) {
	content, err := markedContent(document, stateBegin, stateEnd, "generated state")
	if err != nil {
		return nil, []string{err.Error()}
	}
	stored := make(map[string]string)
	var failures []string
	for index, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "| Document |") || strings.HasPrefix(trimmed, "|---") {
			continue
		}
		parts := strings.Split(trimmed, "|")
		if len(parts) != 5 {
			failures = append(failures, fmt.Sprintf("%s state row %d is malformed", stateDocument, index+1))
			continue
		}
		paths, pathErr := linkedPaths(parts[1])
		hashCell := strings.TrimSpace(parts[2])
		hash := ""
		if len(hashCell) >= 2 && strings.HasPrefix(hashCell, "`") && strings.HasSuffix(hashCell, "`") {
			hash = strings.TrimSuffix(strings.TrimPrefix(hashCell, "`"), "`")
		}
		if pathErr != nil || len(paths) != 1 {
			failures = append(failures, fmt.Sprintf("%s state row %d has an invalid path: %v", stateDocument, index+1, pathErr))
			continue
		}
		if len(hash) != sha256.Size*2 {
			failures = append(failures, fmt.Sprintf("%s state row for %s has a malformed SHA-256", stateDocument, paths[0]))
			continue
		}
		if _, err := hex.DecodeString(hash); err != nil {
			failures = append(failures, fmt.Sprintf("%s state row for %s has a malformed SHA-256", stateDocument, paths[0]))
			continue
		}
		if _, duplicate := stored[paths[0]]; duplicate {
			failures = append(failures, fmt.Sprintf("%s state path is duplicated: %s", stateDocument, paths[0]))
			continue
		}
		dependents, dependentErr := linkedPaths(parts[3])
		if dependentErr != nil {
			failures = append(failures, fmt.Sprintf("%s state row for %s has invalid review dependents: %v", stateDocument, paths[0], dependentErr))
			continue
		}
		_ = dependents
		stored[paths[0]] = hash
	}
	return stored, failures
}

// dependentGuidance formats the documents that require review after one source changes.
func dependentGuidance(path string, dependencies map[string][]string) string {
	dependents := dependencies[path]
	if len(dependents) == 0 {
		return "no listed dependents"
	}
	return "review " + strings.Join(dependents, ", ")
}

// documentStateContent compares exact documentation bytes with the acknowledged state.
func documentStateContent(root string) (string, []documentState, map[string][]string, map[string]string, []string, error) {
	path := filepath.Join(root, filepath.FromSlash(stateDocument))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("%s: cannot read documentation state: %w", stateDocument, err)
	}
	document := string(data)
	dependencies, dependencyFailures := parseDependencies(document)
	stored, storedFailures := parseStoredState(document)
	current, err := hashDocumentation(root)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	failures := append(dependencyFailures, storedFailures...)
	return document, current, dependencies, stored, failures, nil
}

// checkDocumentState reports changed, added, removed, and malformed documentation state.
func checkDocumentState(root string) ([]string, error) {
	document, current, dependencies, stored, failures, err := documentStateContent(root)
	if err != nil {
		return nil, err
	}
	currentPaths := make(map[string]bool, len(current))
	for _, state := range current {
		currentPaths[state.path] = true
		if hash, exists := stored[state.path]; !exists {
			failures = append(failures, fmt.Sprintf("%s: documentation was added; review dependency rules and run make docs-state-update", state.path))
		} else if hash != state.hash {
			failures = append(failures, fmt.Sprintf("%s: documentation changed since acknowledgement; %s; run make docs-state-update after review", state.path, dependentGuidance(state.path, dependencies)))
		}
	}
	for path := range stored {
		if !currentPaths[path] {
			failures = append(failures, fmt.Sprintf("%s: documented file is missing; %s; run make docs-state-update after review", path, dependentGuidance(path, dependencies)))
		}
	}
	for source, dependents := range dependencies {
		if !currentPaths[source] {
			failures = append(failures, fmt.Sprintf("%s: dependency source does not exist: %s", stateDocument, source))
		}
		for _, dependent := range dependents {
			if !currentPaths[dependent] {
				failures = append(failures, fmt.Sprintf("%s: dependency target does not exist: %s", stateDocument, dependent))
			}
		}
	}
	generated, markerErr := markedContent(document, stateBegin, stateEnd, "generated state")
	if markerErr == nil && strings.TrimSpace(generated) != renderStateTable(current, dependencies) {
		failures = append(failures, fmt.Sprintf("%s: generated hashes or review dependents are stale; run make docs-state-update after review", stateDocument))
	}
	sort.Strings(failures)
	return failures, nil
}

// renderStateTable renders current hashes and review dependents.
func renderStateTable(states []documentState, dependencies map[string][]string) string {
	var output strings.Builder
	output.WriteString("| Document | SHA-256 | Review dependents |\n")
	output.WriteString("|---|---|---|\n")
	for _, state := range states {
		dependents := "None"
		if values := dependencies[state.path]; len(values) > 0 {
			links := make([]string, 0, len(values))
			for _, value := range values {
				links = append(links, fmt.Sprintf("[%s](%s)", value, strings.TrimPrefix(value, "docs/")))
			}
			dependents = strings.Join(links, ", ")
		}
		fmt.Fprintf(&output, "| [%s](%s) | `%s` | %s |\n", state.path, strings.TrimPrefix(state.path, "docs/"), state.hash, dependents)
	}
	return strings.TrimSpace(output.String())
}

// runStateCommand checks or explicitly acknowledges the documentation state.
func runStateCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "check" && args[0] != "update") {
		fmt.Fprintln(stderr, "state requires check or update")
		return 2
	}
	action := args[0]
	flags := flag.NewFlagSet("doccheck state "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "state %s accepts only --root\n", action)
		return 2
	}
	if action == "check" {
		failures, err := checkDocumentState(*root)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if len(failures) > 0 {
			for _, failure := range failures {
				fmt.Fprintln(stderr, failure)
			}
			return 1
		}
		fmt.Fprintln(stdout, "Documentation state is current.")
		return 0
	}
	path := filepath.Join(*root, filepath.FromSlash(stateDocument))
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, stateDocument+": cannot read documentation state:", err)
		return 1
	}
	document := string(data)
	dependencies, failures := parseDependencies(document)
	if len(failures) > 0 {
		for _, failure := range failures {
			fmt.Fprintln(stderr, failure)
		}
		return 1
	}
	current, err := hashDocumentation(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	generated := renderStateTable(current, dependencies)
	updated, err := replaceGeneratedRegion(document, stateBegin, stateEnd, generated, stateDocument)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(stderr, "stat documentation state:", err)
		return 1
	}
	if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
		fmt.Fprintln(stderr, "write documentation state:", err)
		return 1
	}
	fmt.Fprintln(stdout, "Updated docs/DOC-STATE.md after documentation review.")
	return 0
}
