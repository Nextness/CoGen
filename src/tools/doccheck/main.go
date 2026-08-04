// Command doccheck validates stable, machine-verifiable documentation contracts.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	inlineLink         = regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)
	referenceLink      = regexp.MustCompile(`^[[:space:]]*\[[^\]]+\]:[[:space:]]*(\S+)`)
	migrationName      = regexp.MustCompile(`^V([0-9]{5})_[^/\\]+\.sql$`)
	filenameSetting    = regexp.MustCompile(`(?m)^[[:space:]]*filename[[:space:]]*=[[:space:]]*"([^"]+)"[[:space:]]*[,;]`)
	migrationDocs      = []string{"AGENTS.md", "docs/ARCHITECTURE.md"}
	obsoleteReferences = []string{
		"docs/viewer-design.md",
		"docs/frontend-architecture.md",
		"docs/development.md",
		"docs/viewer-usage.md",
		"docs/css-class-reference.md",
		"architecture-docs",
		"workspace_store.go",
		"TODO.md Phase B",
	}
	registryChains = []chainRegistry{
		{label: "metadata", setting: "corpus_metadata_config"},
		{label: "PDF", setting: "corpus_pdf_config"},
	}
)

// chainRegistry maps a documented migration chain to its expected directory and boundary.
type chainRegistry struct {
	label   string
	setting string
}

// migrationBoundary records the first and last migration filenames in a configured chain.
type migrationBoundary struct {
	first int
	last  int
}

// assignmentValue returns one quoted SOMETHING assignment or a descriptive error.
func assignmentValue(text, name, source string) (string, error) {
	pattern := regexp.MustCompile(`(?m)^[[:space:]]*` + regexp.QuoteMeta(name) + `[[:space:]]*=[[:space:]]*"([^"]+)"[[:space:]]*[,;]`)
	matches := pattern.FindAllStringSubmatch(text, -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("%s: expected exactly one %s string assignment, found %d", source, name, len(matches))
	}
	return matches[0][1], nil
}

// maintainedMarkdownFiles returns root guides and docs Markdown in deterministic order.
func maintainedMarkdownFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if relative == "docs/ref" {
				return filepath.SkipDir
			}
			if relative != "." && relative != "docs" && !strings.HasPrefix(relative, "docs/") {
				return filepath.SkipDir
			}
			return nil
		}
		if !item.Type().IsRegular() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, relative)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list repository Markdown: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

// destinationPath returns the local file portion of a Markdown destination.
func destinationPath(raw string) (string, bool) {
	destination := strings.TrimSpace(raw)
	if strings.HasPrefix(destination, "<") {
		end := strings.Index(destination, ">")
		if end < 0 {
			return destination, true
		}
		destination = destination[1:end]
	} else if fields := strings.Fields(destination); len(fields) > 0 {
		destination = fields[0]
	} else {
		return "", false
	}
	if destination == "" || strings.HasPrefix(destination, "#") || strings.HasPrefix(destination, "/") {
		return "", false
	}
	parsed, err := url.Parse(destination)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Path == "" {
		return "", false
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return parsed.Path, true
	}
	return path, true
}

// withoutInlineCode replaces complete backtick-delimited code spans while preserving surrounding Markdown.
func withoutInlineCode(line string) string {
	var output strings.Builder
	for cursor := 0; cursor < len(line); {
		if line[cursor] != '`' {
			output.WriteByte(line[cursor])
			cursor++
			continue
		}
		delimiterEnd := cursor
		for delimiterEnd < len(line) && line[delimiterEnd] == '`' {
			delimiterEnd++
		}
		delimiter := line[cursor:delimiterEnd]
		closingOffset := strings.Index(line[delimiterEnd:], delimiter)
		if closingOffset < 0 {
			output.WriteByte(line[cursor])
			cursor++
			continue
		}
		closing := delimiterEnd + closingOffset + len(delimiter)
		output.WriteString(strings.Repeat(" ", closing-cursor))
		cursor = closing
	}
	return output.String()
}

// markdownDestinations returns inline and reference-style destinations outside fenced code blocks.
func markdownDestinations(text string) map[int][]string {
	destinations := make(map[int][]string)
	fence := ""
	for index, line := range strings.Split(text, "\n") {
		lineNumber := index + 1
		trimmed := strings.TrimLeft(line, " \t")
		marker := ""
		if len(trimmed) >= 3 && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			marker = trimmed[:3]
		}
		if marker != "" {
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		visible := withoutInlineCode(line)
		for _, match := range inlineLink.FindAllStringSubmatch(visible, -1) {
			destinations[lineNumber] = append(destinations[lineNumber], match[1])
		}
		if match := referenceLink.FindStringSubmatch(visible); match != nil {
			destinations[lineNumber] = append(destinations[lineNumber], match[1])
		}
	}
	return destinations
}

// pathWithin reports whether target is contained by root after path cleaning.
func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// checkMarkdownLinks reports missing, unreadable, or repository-escaping local Markdown links.
func checkMarkdownLinks(root string, markdownFiles []string) []string {
	var failures []string
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return []string{fmt.Sprintf("resolve repository root: %v", err)}
	}
	physicalRoot, err := filepath.EvalSymlinks(resolvedRoot)
	if err != nil {
		return []string{fmt.Sprintf("resolve repository root symbolic links: %v", err)}
	}
	for _, relativePath := range markdownFiles {
		source := filepath.Join(resolvedRoot, relativePath)
		data, err := os.ReadFile(source)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: cannot read tracked Markdown: %v", filepath.ToSlash(relativePath), err))
			continue
		}
		destinations := markdownDestinations(string(data))
		lineNumbers := make([]int, 0, len(destinations))
		for lineNumber := range destinations {
			lineNumbers = append(lineNumbers, lineNumber)
		}
		sort.Ints(lineNumbers)
		for _, lineNumber := range lineNumbers {
			for _, rawDestination := range destinations[lineNumber] {
				localPath, local := destinationPath(rawDestination)
				if !local {
					continue
				}
				target := filepath.Clean(filepath.Join(filepath.Dir(source), filepath.FromSlash(localPath)))
				if !pathWithin(resolvedRoot, target) {
					failures = append(failures, fmt.Sprintf("%s:%d: local link escapes the repository: %s", filepath.ToSlash(relativePath), lineNumber, rawDestination))
					continue
				}
				if _, err := os.Stat(target); err != nil {
					failures = append(failures, fmt.Sprintf("%s:%d: local link target does not exist: %s", filepath.ToSlash(relativePath), lineNumber, rawDestination))
					continue
				}
				resolvedTarget, err := filepath.EvalSymlinks(target)
				if err != nil || !pathWithin(physicalRoot, resolvedTarget) {
					failures = append(failures, fmt.Sprintf("%s:%d: local link escapes the repository through a symbolic link: %s", filepath.ToSlash(relativePath), lineNumber, rawDestination))
				}
			}
		}
	}
	return failures
}

// checkObsoleteReferences reports exact historical references that are invalid in current documentation.
func checkObsoleteReferences(root string, markdownFiles []string) []string {
	var failures []string
	for _, relativePath := range markdownFiles {
		if filepath.ToSlash(relativePath) == "PLAN.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, relativePath))
		if err != nil {
			continue
		}
		for index, line := range strings.Split(string(data), "\n") {
			for _, obsolete := range obsoleteReferences {
				if strings.Contains(line, obsolete) {
					failures = append(failures, fmt.Sprintf("%s:%d: obsolete reference: %s", filepath.ToSlash(relativePath), index+1, obsolete))
				}
			}
		}
	}
	return failures
}

// migrationVersions parses ordered migration versions and reports malformed or duplicate declarations.
func migrationVersions(filenames []string, source string) ([]int, []string) {
	var versions []int
	var failures []string
	seen := make(map[string]bool)
	for _, filename := range filenames {
		if seen[filename] {
			failures = append(failures, fmt.Sprintf("%s: migration filename is configured more than once: %s", source, filename))
			continue
		}
		seen[filename] = true
		match := migrationName.FindStringSubmatch(filename)
		if match == nil {
			failures = append(failures, fmt.Sprintf("%s: invalid migration filename: %s", source, filename))
			continue
		}
		version, err := strconv.Atoi(match[1])
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: invalid migration version: %s", source, filename))
			continue
		}
		versions = append(versions, version)
	}
	for index, version := range versions {
		if version != index+1 {
			failures = append(failures, fmt.Sprintf("%s: migration versions must be ordered and contiguous from V00001, found %v", source, versions))
			break
		}
	}
	return versions, failures
}

// checkMigrationChains validates configured migration files and returns version boundaries by chain.
func checkMigrationChains(root string) ([]string, map[string]migrationBoundary) {
	var failures []string
	boundaries := make(map[string]migrationBoundary)
	registryPath := filepath.Join("config", "database.something")
	registryData, err := os.ReadFile(filepath.Join(root, registryPath))
	if err != nil {
		return []string{fmt.Sprintf("%s: cannot read migration registry: %v", filepath.ToSlash(registryPath), err)}, boundaries
	}
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return []string{fmt.Sprintf("resolve repository root: %v", err)}, boundaries
	}
	physicalRoot, err := filepath.EvalSymlinks(resolvedRoot)
	if err != nil {
		return []string{fmt.Sprintf("resolve repository root symbolic links: %v", err)}, boundaries
	}
	for _, registered := range registryChains {
		chainName, err := assignmentValue(string(registryData), registered.setting, filepath.ToSlash(registryPath))
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		chainPath := filepath.Join(filepath.Dir(registryPath), filepath.FromSlash(chainName))
		chainSource := filepath.Clean(filepath.Join(resolvedRoot, chainPath))
		if !pathWithin(resolvedRoot, chainSource) {
			failures = append(failures, fmt.Sprintf("%s: configured migration chain escapes the repository: %s", filepath.ToSlash(registryPath), chainName))
			continue
		}
		chainSource, err = filepath.EvalSymlinks(chainSource)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: cannot load migration chain: %v", filepath.ToSlash(chainPath), err))
			continue
		}
		if !pathWithin(physicalRoot, chainSource) {
			failures = append(failures, fmt.Sprintf("%s: configured migration chain escapes the repository through a symbolic link: %s", filepath.ToSlash(registryPath), chainName))
			continue
		}
		chainData, err := os.ReadFile(chainSource)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: cannot load migration chain: %v", filepath.ToSlash(chainPath), err))
			continue
		}
		migrationsDirValue, err := assignmentValue(string(chainData), "migrations_dir", filepath.ToSlash(chainPath))
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: cannot load migration chain: %v", filepath.ToSlash(chainPath), err))
			continue
		}
		matches := filenameSetting.FindAllStringSubmatch(string(chainData), -1)
		filenames := make([]string, 0, len(matches))
		for _, match := range matches {
			filenames = append(filenames, match[1])
		}
		if len(filenames) == 0 {
			failures = append(failures, fmt.Sprintf("%s: no migration filenames are configured", filepath.ToSlash(chainPath)))
			continue
		}
		versions, versionFailures := migrationVersions(filenames, filepath.ToSlash(chainPath))
		failures = append(failures, versionFailures...)
		migrationsDir := filepath.Clean(filepath.Join(filepath.Dir(chainSource), filepath.FromSlash(migrationsDirValue)))
		if !pathWithin(physicalRoot, migrationsDir) {
			failures = append(failures, fmt.Sprintf("%s: migrations_dir escapes the repository: %s", filepath.ToSlash(chainPath), migrationsDirValue))
			continue
		}
		migrationsDir, err = filepath.EvalSymlinks(migrationsDir)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: migrations_dir does not exist: %s", filepath.ToSlash(chainPath), migrationsDirValue))
			continue
		}
		if !pathWithin(physicalRoot, migrationsDir) {
			failures = append(failures, fmt.Sprintf("%s: migrations_dir escapes the repository through a symbolic link: %s", filepath.ToSlash(chainPath), migrationsDirValue))
			continue
		}
		entries, err := os.ReadDir(migrationsDir)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: migrations_dir does not exist: %s", filepath.ToSlash(chainPath), migrationsDirValue))
			continue
		}
		configured := make(map[string]bool, len(filenames))
		for _, filename := range filenames {
			configured[filename] = true
		}
		present := make(map[string]bool)
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
				present[entry.Name()] = true
			}
		}
		for _, filename := range sortedDifference(configured, present) {
			failures = append(failures, fmt.Sprintf("%s: configured migration file does not exist: %s", filepath.ToSlash(chainPath), filename))
		}
		for _, filename := range sortedDifference(present, configured) {
			failures = append(failures, fmt.Sprintf("%s: migration file is not configured: %s", filepath.ToSlash(chainPath), filename))
		}
		if len(versions) > 0 && len(versionFailures) == 0 {
			boundaries[registered.label] = migrationBoundary{first: versions[0], last: versions[len(versions)-1]}
		}
	}
	return failures, boundaries
}

// sortedDifference returns deterministic keys present in left and absent from right.
func sortedDifference(left, right map[string]bool) []string {
	var difference []string
	for value := range left {
		if !right[value] {
			difference = append(difference, value)
		}
	}
	sort.Strings(difference)
	return difference
}

// checkDocumentedMigrationBoundaries requires designated guides to state each derived boundary.
func checkDocumentedMigrationBoundaries(root string, boundaries map[string]migrationBoundary) []string {
	var failures []string
	labels := make([]string, 0, len(boundaries))
	for label := range boundaries {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, relativePath := range migrationDocs {
		data, err := os.ReadFile(filepath.Join(root, relativePath))
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: cannot read migration documentation: %v", relativePath, err))
			continue
		}
		for _, label := range labels {
			boundary := boundaries[label]
			first := fmt.Sprintf("V%05d", boundary.first)
			last := fmt.Sprintf("V%05d", boundary.last)
			pattern := regexp.MustCompile(regexp.QuoteMeta(first) + `(-| through )` + regexp.QuoteMeta(last))
			if !pattern.Match(data) {
				failures = append(failures, fmt.Sprintf("%s: missing current %s migration boundary %s-%s", relativePath, label, first, last))
			}
		}
	}
	return failures
}

// checkRepository runs every documentation check and returns failures plus the Markdown count.
func checkRepository(root string) ([]string, int, error) {
	markdownFiles, err := maintainedMarkdownFiles(root)
	if err != nil {
		return nil, 0, err
	}
	failures := checkMarkdownLinks(root, markdownFiles)
	failures = append(failures, checkObsoleteReferences(root, markdownFiles)...)
	failures = append(failures, checkSingleLineMarkdown(root, markdownFiles)...)
	migrationFailures, boundaries := checkMigrationChains(root)
	failures = append(failures, migrationFailures...)
	failures = append(failures, checkDocumentedMigrationBoundaries(root, boundaries)...)
	if err := checkCatalog(root); err != nil {
		failures = append(failures, err.Error())
	}
	stateFailures, err := checkDocumentState(root)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		failures = append(failures, stateFailures...)
	}
	sort.Strings(failures)
	return failures, len(markdownFiles), nil
}

// runCheck parses check arguments, executes every non-mutating check, and returns a process exit status.
func runCheck(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doccheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "check accepts only --root")
		return 2
	}
	failures, markdownCount, err := checkRepository(*root)
	if err != nil {
		fmt.Fprintln(stderr, "documentation consistency check could not run:", err)
		return 2
	}
	if len(failures) > 0 {
		for _, failure := range failures {
			fmt.Fprintln(stderr, failure)
		}
		fmt.Fprintf(stderr, "documentation consistency check failed with %d error(s)\n", len(failures))
		return 1
	}
	fmt.Fprintf(stdout, "documentation consistency check passed for %d Markdown files\n", markdownCount)
	return 0
}

// run parses the command hierarchy and returns a process exit status.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runCheck(nil, stdout, stderr)
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "catalog":
		return runCatalogCommand(args[1:], stdout, stderr)
	case "state":
		return runStateCommand(args[1:], stdout, stderr)
	default:
		if strings.HasPrefix(args[0], "-") {
			return runCheck(args, stdout, stderr)
		}
		fmt.Fprintf(stderr, "unknown doccheck command %q; use check, catalog check, catalog update, state check, or state update\n", args[0])
		return 2
	}
}

// main runs the documentation consistency command.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
