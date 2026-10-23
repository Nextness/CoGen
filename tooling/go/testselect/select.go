// Package testselect discovers domain-owned suites and invokes the standard test runners.
package testselect

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Options narrows ordinary offline suites by ownership, layer, and runner.
type Options struct {
	Domain, Layer, Component, Package, Runner, Test, TestFile string
	Browser, Workers                                          string
	Race, Headed, Debug                                       bool
}

// Selection contains canonical repository-relative packages and test files.
type Selection struct {
	GoPackages   []string
	NodeFiles    []string
	BrowserFiles []string
}

// validRelative rejects noncanonical or escaping selector paths.
func validRelative(value string) bool {
	return value != "" && !filepath.IsAbs(value) && !strings.Contains(value, "\\") && filepath.ToSlash(filepath.Clean(value)) == value && value != ".." && !strings.HasPrefix(value, "../") && value != "."
}

// Select discovers runnable suites without installing dependencies or starting services.
func Select(root string, options Options) (Selection, error) {
	var selected Selection
	if options.Domain == "" {
		options.Domain = "all"
	}
	if options.Layer == "" {
		options.Layer = "all"
	}
	if options.Runner == "" {
		options.Runner = "all"
	}
	if options.Runner == "go" && options.Domain == "all" && options.Component != "" {
		options.Domain = "backend"
	}
	domains := map[string]bool{"all": true, "backend": true, "frontend": true, "tooling": true, "shared": true, "contracts": true}
	layers := map[string]bool{"all": true, "unit": true, "functional": true, "integration": true}
	runners := map[string]bool{"all": true, "go": true, "node": true, "browser": true}
	if !domains[options.Domain] || !layers[options.Layer] || !runners[options.Runner] {
		return selected, fmt.Errorf("unknown domain, layer, or runner: %q/%q/%q", options.Domain, options.Layer, options.Runner)
	}
	if (options.Headed || options.Debug) && options.Runner != "browser" {
		return selected, fmt.Errorf("headed and debug controls require browser-only selection")
	}
	if options.Component != "" && (options.Domain == "all" || !validRelative(options.Component)) {
		return selected, fmt.Errorf("COMPONENT requires an explicit domain and a canonical relative component path")
	}
	if options.Package != "" && (options.Runner != "go" || options.Component != "") {
		return selected, fmt.Errorf("PACKAGE is a test-go override and cannot be combined with COMPONENT")
	}
	if options.Package != "" {
		value := strings.TrimPrefix(options.Package, "./")
		if !validRelative(value) || !strings.HasPrefix(value, "tests/") || strings.HasPrefix(value, "tests/e2e/") || strings.HasPrefix(value, "tests/support/") {
			return selected, fmt.Errorf("PACKAGE must name a canonical ordinary test package, such as ./tests/backend/server/integration")
		}
		options.Domain = strings.Split(value, "/")[1]
		if !domains[options.Domain] || options.Domain == "all" {
			return selected, fmt.Errorf("invalid test package domain")
		}
	}
	packages := make(map[string]bool)
	for _, domain := range []string{"backend", "frontend", "tooling", "shared", "contracts"} {
		if options.Domain != "all" && domain != options.Domain {
			continue
		}
		base := filepath.Join(root, "tests", domain)
		if options.Component != "" {
			base = filepath.Join(base, filepath.FromSlash(options.Component))
		}
		if options.Package != "" {
			base = filepath.Join(root, strings.TrimPrefix(options.Package, "./"))
		}
		info, err := os.Stat(base)
		if os.IsNotExist(err) && options.Domain == "all" {
			continue
		}
		if err != nil {
			return selected, fmt.Errorf("test scope %s: %w", base, err)
		}
		if !info.IsDir() {
			return selected, fmt.Errorf("test scope must be a directory: %s", base)
		}
		realBase, err := filepath.EvalSymlinks(base)
		if err != nil {
			return selected, err
		}
		realRoot, err := filepath.EvalSymlinks(filepath.Join(root, "tests", domain))
		if err != nil {
			return selected, err
		}
		contained, err := filepath.Rel(realRoot, realBase)
		if err != nil || contained == ".." || strings.HasPrefix(contained, ".."+string(filepath.Separator)) {
			return selected, fmt.Errorf("test scope escapes its domain")
		}
		err = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if options.Package != "" && path != base {
					return filepath.SkipDir
				}
				if entry.Name() == "testdata" || entry.Name() == "node_modules" || strings.HasSuffix(entry.Name(), "-snapshots") {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			layer := filepath.Base(filepath.Dir(path))
			if layer != "unit" && layer != "functional" && layer != "integration" {
				return nil
			}
			if options.Layer != "all" && layer != options.Layer {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			switch {
			case strings.HasSuffix(path, "_test.go") && (options.Runner == "all" || options.Runner == "go"):
				packages["./"+filepath.ToSlash(filepath.Dir(relative))] = true
			case strings.HasSuffix(path, ".test.ts") && (options.Runner == "all" || options.Runner == "node"):
				selected.NodeFiles = append(selected.NodeFiles, relative)
			case strings.HasSuffix(path, ".spec.ts") && (options.Runner == "all" || options.Runner == "browser"):
				selected.BrowserFiles = append(selected.BrowserFiles, relative)
			}
			return nil
		})
		if err != nil {
			return selected, err
		}
	}
	for name := range packages {
		selected.GoPackages = append(selected.GoPackages, name)
	}
	sort.Strings(selected.GoPackages)
	sort.Strings(selected.NodeFiles)
	sort.Strings(selected.BrowserFiles)
	if options.TestFile != "" {
		if !validRelative(options.TestFile) {
			return selected, fmt.Errorf("TEST_FILE must be a canonical repository-relative browser test path")
		}
		found := false
		for _, name := range selected.BrowserFiles {
			if name == options.TestFile {
				found = true
				break
			}
		}
		if !found {
			return selected, fmt.Errorf("TEST_FILE %q is outside the selected browser suites", options.TestFile)
		}
		selected.BrowserFiles = []string{options.TestFile}
		if len(selected.GoPackages) > 0 || len(selected.NodeFiles) > 0 {
			return selected, fmt.Errorf("TEST_FILE requires a browser-only selection")
		}
	}
	if options.Test != "" && (len(selected.NodeFiles) > 0 || len(selected.BrowserFiles) > 0) {
		return selected, fmt.Errorf("TEST is a Go -run expression; select Go suites with test-go or a Go-only domain/component")
	}
	if options.Race && (len(selected.NodeFiles) > 0 || len(selected.BrowserFiles) > 0) {
		return selected, fmt.Errorf("race mode requires Go suites")
	}
	if len(selected.GoPackages)+len(selected.NodeFiles)+len(selected.BrowserFiles) == 0 {
		return selected, fmt.Errorf("no runnable tests for domain=%s component=%s layer=%s runner=%s", options.Domain, options.Component, options.Layer, options.Runner)
	}
	return selected, nil
}

// execute runs one standard command from the repository root and preserves its exit code.
func execute(root, name string, args []string, stdout, stderr io.Writer) int {
	command := exec.Command(name, args...)
	command.Dir = root
	command.Stdout = stdout
	command.Stderr = stderr
	command.Stdin = os.Stdin
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// runGo observes actual test execution so a focused expression cannot silently match nothing.
func runGo(root string, options Options, packages []string, stdout, stderr io.Writer) int {
	args := []string{"test", "-count=1"}
	if options.Race {
		args = append(args, "-race")
	}
	if options.Test == "" {
		return execute(root, "go", append(args, packages...), stdout, stderr)
	}
	args = append(args, "-json", "-run", options.Test)
	args = append(args, packages...)
	command := exec.Command("go", args...)
	command.Dir = root
	command.Stderr = stderr
	pipe, err := command.StdoutPipe()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = command.Start(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	matched := 0
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		var event struct{ Action, Test, Output string }
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			fmt.Fprintln(stdout, scanner.Text())
			continue
		}
		if event.Action == "run" && event.Test != "" {
			matched++
		}
		if event.Output != "" {
			fmt.Fprint(stdout, event.Output)
		}
	}
	scanErr := scanner.Err()
	err = command.Wait()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	if scanErr != nil {
		fmt.Fprintln(stderr, scanErr)
		return 1
	}
	if matched == 0 {
		fmt.Fprintf(stderr, "TEST %q matched no tests\n", options.Test)
		return 1
	}
	return 0
}

// Run selects and executes ordinary suites with the standard Go, Node, and Playwright runners.
func Run(args []string, stdout, stderr io.Writer) int {
	var options Options
	flags := flag.NewFlagSet("testselect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.Domain, "domain", "all", "test domain")
	flags.StringVar(&options.Layer, "layer", "all", "test layer")
	flags.StringVar(&options.Component, "component", "", "relative component")
	flags.StringVar(&options.Package, "package", "", "canonical Go test package")
	flags.StringVar(&options.Runner, "runner", "all", "all, go, node, or browser")
	flags.StringVar(&options.Test, "test", "", "Go -run expression")
	flags.StringVar(&options.TestFile, "test-file", "", "repository-relative browser spec")
	flags.StringVar(&options.Browser, "browser", "chromium", "browser project, or all")
	flags.StringVar(&options.Workers, "workers", "", "browser workers")
	flags.BoolVar(&options.Race, "race", false, "Go race detector")
	flags.BoolVar(&options.Headed, "headed", false, "headed browser")
	flags.BoolVar(&options.Debug, "debug", false, "browser debugger")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	selected, err := Select(root, options)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "Test scope: domain=%s component=%s layer=%s; %d Go packages, %d Node files, %d browser files\n", options.Domain, options.Component, options.Layer, len(selected.GoPackages), len(selected.NodeFiles), len(selected.BrowserFiles))
	if len(selected.GoPackages) > 0 {
		if code := runGo(root, options, selected.GoPackages, stdout, stderr); code != 0 {
			return code
		}
	}
	if len(selected.NodeFiles) > 0 {
		if code := execute(root, "node", append([]string{"--import", "./tests/support/node/tsx-loader.ts", "--test"}, selected.NodeFiles...), stdout, stderr); code != 0 {
			return code
		}
	}
	if len(selected.BrowserFiles) > 0 {
		if code := execute(root, "make", []string{"build", "frontend-build"}, stdout, stderr); code != 0 {
			return code
		}
		defaultFixture := filepath.Join(root, "build/fixtures/viewer/workspace.fixture.db")
		fixture := os.Getenv("FIXTURE_DB")
		if fixture == "" {
			fixture = defaultFixture
		}
		if !filepath.IsAbs(fixture) {
			fixture = filepath.Join(root, fixture)
		}
		companion := strings.TrimSuffix(fixture, ".db") + ".pdf.db"
		if strings.HasSuffix(fixture, ".metadata.db") {
			companion = strings.TrimSuffix(fixture, ".metadata.db") + ".pdf.db"
		}
		if explicit := os.Getenv("FIXTURE_PDF_DB"); explicit != "" {
			companion = explicit
		}
		for _, filename := range []string{fixture, companion} {
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				// Only generate the disposable default pair automatically; custom inputs belong to the caller.
				if filepath.Clean(fixture) != defaultFixture || os.Getenv("FIXTURE_PDF_DB") != "" {
					fmt.Fprintf(stderr, "fixture input %s is missing; run make fixture FIXTURE_DB=<metadata path> or provide an existing pair\n", filename)
					return 1
				}
				if code := execute(root, "make", []string{"fixture"}, stdout, stderr); code != 0 {
					return code
				}
				break
			} else if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
		browserArgs := []string{"tests/support/node/run-playwright.ts"}
		if options.Headed {
			browserArgs = append(browserArgs, "--headed")
		}
		if options.Debug {
			browserArgs = append(browserArgs, "--debug")
		}
		if options.Browser != "all" {
			browserArgs = append(browserArgs, "--project="+options.Browser)
		}
		if options.Workers != "" {
			browserArgs = append(browserArgs, "--workers="+options.Workers)
		}
		if code := execute(root, "node", append(browserArgs, selected.BrowserFiles...), stdout, stderr); code != 0 {
			return code
		}
	}
	return 0
}
