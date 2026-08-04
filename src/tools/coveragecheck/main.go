// Command coveragecheck enforces the repository's local Go coverage policy.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"analysis/something"
)

// policy defines repository-wide and per-file coverage thresholds.
type policy struct {
	Version         int
	CoverageMode    string
	ExcludePackages []string
	PackageMinimums map[string]float64
	FileMinimums    map[string]fileRule
}

// fileRule defines a path pattern and minimum coverage threshold.
type fileRule struct {
	Minimum   float64
	Rationale string
}

// coverage accumulates covered and total statements for one profile scope.
type coverage struct {
	Covered    int
	Statements int
}

// percent returns a covered-line percentage, treating an empty total as complete.
func (c coverage) percent() float64 {
	if c.Statements == 0 {
		return 0
	}
	return float64(c.Covered) * 100 / float64(c.Statements)
}

// main dispatches the analysis command selected by process arguments and exits on command failure.
func main() {
	profilePath := flag.String("profile", "", "path to a Go coverage profile")
	policyPath := flag.String("policy", "", "path to the coverage policy .something file")
	flag.Parse()
	if *profilePath == "" || *policyPath == "" {
		fmt.Fprintln(os.Stderr, "usage: coveragecheck --profile <coverage.out> --policy <coverage_policy.something>")
		os.Exit(2)
	}
	if err := check(*profilePath, *policyPath, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "coverage policy failed:", err)
		os.Exit(1)
	}
}

// check checks check against the current invariants.
func check(profilePath, policyPath string, output io.Writer) error {
	p, err := readPolicy(policyPath)
	if err != nil {
		return err
	}
	profile, err := os.Open(profilePath)
	if err != nil {
		return fmt.Errorf("open coverage profile: %w", err)
	}
	defer profile.Close()
	packages, files, mode, err := readProfile(profile)
	if err != nil {
		return err
	}
	if mode != p.CoverageMode {
		return fmt.Errorf("coverage mode = %q, want %q", mode, p.CoverageMode)
	}

	excluded := make(map[string]bool, len(p.ExcludePackages))
	for _, pkg := range p.ExcludePackages {
		excluded[pkg] = true
	}
	var failures []string
	for pkg := range packages {
		if _, tracked := p.PackageMinimums[pkg]; !tracked && !excluded[pkg] {
			failures = append(failures, fmt.Sprintf("untracked package %q in profile; add a minimum or explicit exclusion", pkg))
		}
	}

	fmt.Fprintln(output, "Go coverage policy")
	writeCoverageTable(output, "Packages", p.PackageMinimums, packages, &failures)
	fileMinimums := make(map[string]float64, len(p.FileMinimums))
	for file, rule := range p.FileMinimums {
		fileMinimums[file] = rule.Minimum
	}
	writeCoverageTable(output, "High-risk files", fileMinimums, files, &failures)

	total := coverage{}
	for pkg, current := range packages {
		if !excluded[pkg] {
			total.Covered += current.Covered
			total.Statements += current.Statements
		}
	}
	fmt.Fprintf(output, "\nTracked total: %.1f%% (%d/%d statements)\n", total.percent(), total.Covered, total.Statements)
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

// readPolicy reads policy from the supplied source.
func readPolicy(policyPath string) (policy, error) {
	values, err := something.LoadSomethingFile(policyPath)
	if err != nil {
		return policy{}, fmt.Errorf("load coverage policy: %w", err)
	}
	policyMap, err := something.GetStructOnce(values, "coverage_policy")
	if err != nil {
		return policy{}, fmt.Errorf("missing coverage_policy setting: %w", err)
	}

	version, err := something.GetIntegerOnce(policyMap, "version")
	if err != nil {
		return policy{}, fmt.Errorf("invalid version: %w", err)
	}
	coverageModeOrdinal, err := something.GetEnumOnce(policyMap, "coverage_mode")
	if err != nil {
		return policy{}, fmt.Errorf("invalid coverage_mode: %w", err)
	}
	excludeList, err := something.GetListOnce(policyMap, "exclude_packages")
	if err != nil {
		return policy{}, fmt.Errorf("invalid exclude_packages: %w", err)
	}

	var p policy
	p.Version = version
	// Enum ordinal 0 = ATOMIC
	if coverageModeOrdinal == 0 {
		p.CoverageMode = "atomic"
	} else {
		return policy{}, fmt.Errorf("unknown coverage_mode ordinal %d", coverageModeOrdinal)
	}
	p.ExcludePackages = make([]string, len(excludeList))
	for i, v := range excludeList {
		p.ExcludePackages[i] = v.(string)
	}

	pkgMinMap, err := something.GetMappingOnce(policyMap, "package_minimums")
	if err != nil {
		return policy{}, fmt.Errorf("invalid package_minimums: %w", err)
	}
	p.PackageMinimums = make(map[string]float64, len(pkgMinMap))
	for pkg, entry := range pkgMinMap {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			return policy{}, fmt.Errorf("invalid entry for package %q", pkg)
		}
		minVal, err := something.GetFloatOnce(entryMap, "minimum")
		if err != nil {
			return policy{}, fmt.Errorf("invalid minimum for package %q: %w", pkg, err)
		}
		if pkg == "" || minVal < 0 || minVal > 100 {
			return policy{}, fmt.Errorf("invalid package minimum for %q", pkg)
		}
		p.PackageMinimums[pkg] = minVal
	}

	fileMinMap, err := something.GetMappingOnce(policyMap, "file_minimums")
	if err != nil {
		return policy{}, fmt.Errorf("invalid file_minimums: %w", err)
	}
	p.FileMinimums = make(map[string]fileRule, len(fileMinMap))
	for file, entry := range fileMinMap {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			return policy{}, fmt.Errorf("invalid entry for file %q", file)
		}
		minVal, err := something.GetFloatOnce(entryMap, "minimum")
		if err != nil {
			return policy{}, fmt.Errorf("invalid minimum for file %q: %w", file, err)
		}
		rationale, _ := something.GetStringOnce(entryMap, "rationale")
		if file == "" || minVal < 0 || minVal > 100 || rationale == "" {
			return policy{}, fmt.Errorf("invalid file minimum for %q", file)
		}
		p.FileMinimums[file] = fileRule{Minimum: minVal, Rationale: rationale}
	}

	if p.Version != 1 || p.CoverageMode == "" || len(p.PackageMinimums) == 0 {
		return policy{}, errors.New("coverage policy requires version 1, coverage_mode, and package_minimums")
	}
	return p, nil
}

// readProfile reads profile from the supplied source.
func readProfile(input io.Reader) (map[string]coverage, map[string]coverage, string, error) {
	scanner := bufio.NewScanner(input)
	lineNumber := 0
	mode := ""
	type block struct {
		file       string
		statements int
		hits       int
	}
	blocks := map[string]block{}
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if lineNumber == 1 {
			const prefix = "mode: "
			if !strings.HasPrefix(line, prefix) {
				return nil, nil, "", errors.New("coverage profile is missing mode header")
			}
			mode = strings.TrimPrefix(line, prefix)
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, nil, "", fmt.Errorf("coverage profile line %d is malformed", lineNumber)
		}
		colon := strings.LastIndex(fields[0], ":")
		if colon <= 0 {
			return nil, nil, "", fmt.Errorf("coverage profile line %d has no source file", lineNumber)
		}
		statements, err := strconv.Atoi(fields[1])
		if err != nil || statements < 0 {
			return nil, nil, "", fmt.Errorf("coverage profile line %d has invalid statement count", lineNumber)
		}
		hits, err := strconv.Atoi(fields[2])
		if err != nil || hits < 0 {
			return nil, nil, "", fmt.Errorf("coverage profile line %d has invalid hit count", lineNumber)
		}
		key := fields[0] + " " + fields[1]
		current, exists := blocks[key]
		if !exists || hits > current.hits {
			blocks[key] = block{file: fields[0][:colon], statements: statements, hits: hits}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, "", fmt.Errorf("read coverage profile: %w", err)
	}
	if lineNumber == 0 {
		return nil, nil, "", errors.New("coverage profile is empty")
	}

	packages := map[string]coverage{}
	files := map[string]coverage{}
	for _, block := range blocks {
		fileCoverage := files[block.file]
		fileCoverage.Statements += block.statements
		if block.hits > 0 {
			fileCoverage.Covered += block.statements
		}
		files[block.file] = fileCoverage

		pkg := path.Dir(block.file)
		packageCoverage := packages[pkg]
		packageCoverage.Statements += block.statements
		if block.hits > 0 {
			packageCoverage.Covered += block.statements
		}
		packages[pkg] = packageCoverage
	}
	return packages, files, mode, nil
}

// writeCoverageTable writes coverage table to the supplied destination.
func writeCoverageTable(output io.Writer, title string, minimums map[string]float64, actual map[string]coverage, failures *[]string) {
	keys := make([]string, 0, len(minimums))
	for key := range minimums {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprintf(output, "\n%s\n%-42s %9s %9s %9s  %s\n", title, "scope", "current", "required", "delta", "result")
	for _, key := range keys {
		current, found := actual[key]
		if !found {
			fmt.Fprintf(output, "%-42s %9s %8.1f%% %9s  FAIL\n", key, "missing", minimums[key], "missing")
			*failures = append(*failures, fmt.Sprintf("%s is absent from profile", key))
			continue
		}
		delta := current.percent() - minimums[key]
		result := "PASS"
		if delta < 0 {
			result = "FAIL"
			*failures = append(*failures, fmt.Sprintf("%s coverage %.1f%% is below %.1f%%", key, current.percent(), minimums[key]))
		}
		fmt.Fprintf(output, "%-42s %8.1f%% %8.1f%% %+8.1f%%  %s\n", key, current.percent(), minimums[key], delta, result)
	}
}
