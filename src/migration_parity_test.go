// migration_parity_test.go compares frozen pre-sqlc implementations with the
// working source on cloned production-migrated fixtures. It is opt-in because
// the baseline Git object must be present locally; it never fetches history.
//go:build e2e

package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// migrationBaseline pins the last pre-migration implementation, not an adapter.
const migrationBaseline = "f06e5f90eb09b187988a952660cf2323248ad51e"

// TestE2EDatabaseMigrationParity runs identical scenarios through independently
// compiled implementations and compares exact API values and persisted graphs.
func TestE2EDatabaseMigrationParity(t *testing.T) {
	root := e2eRepositoryRoot(t)
	evidenceRoot := filepath.Join(root, "build", "e2e", "migration-parity")
	if err := os.MkdirAll(evidenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	evidence, err := os.MkdirTemp(evidenceRoot, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("migration parity evidence: %s", evidence)
	sources := t.TempDir()
	baselineRoot := filepath.Join(sources, "baseline")
	archive := migrationCommand(t, root, "git", "archive", migrationBaseline, "src", "config", "migrations")
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		destination := filepath.Join(baselineRoot, filepath.FromSlash(header.Name))
		if !filepath.IsLocal(header.Name) {
			t.Fatal("unsafe archive entry")
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	currentRoot := filepath.Join(sources, "current")
	files := migrationCommand(t, root, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "src", "config", "migrations")
	names := strings.Split(string(files), "\x00")
	sort.Strings(names)
	sourceHash := sha256.New()
	for _, name := range names {
		if name == "" {
			continue
		}
		migrationCopy(t, filepath.Join(root, name), filepath.Join(currentRoot, name))
		data, err := os.ReadFile(filepath.Join(currentRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(sourceHash, "%s\x00", name)
		sourceHash.Write(data)
	}
	binaries := map[string]string{}
	for _, version := range []string{"baseline", "current"} {
		sourceRoot := baselineRoot
		if version == "current" {
			sourceRoot = currentRoot
		}
		packageRoot := filepath.Join(sourceRoot, "src", "migrationparity")
		migrationCopy(t, filepath.Join(root, "src/testdata/migration-parity/scenario_test.go"), filepath.Join(packageRoot, "main.go"))
		migrationCopy(t, filepath.Join(root, "src/testdata/migration-parity", version+"_test.go"), filepath.Join(packageRoot, "binding.go"))
		binary := filepath.Join(evidence, version)
		migrationCommand(t, filepath.Join(sourceRoot, "src"), "go", "build", "-buildvcs=false", "-tags=integration", "-o", binary, "./migrationparity")
		binaries[version] = binary
	}
	seed := filepath.Join(evidence, "seed")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	migrationCommand(t, baselineRoot, binaries["baseline"], seed, "seed")
	cases := []string{"run", "run-concurrency", "artifact", "cache", "review", "review-rollback", "review-concurrency", "pdf", "pdf-savepoint", "pdf-crash"}
	for _, scenario := range cases {
		t.Run(scenario, func(t *testing.T) {
			results := map[string]any{}
			for _, version := range []string{"baseline", "current"} {
				directory := filepath.Join(evidence, scenario, version)
				for _, filename := range []string{"metadata.db", "corpus.pdf.db"} {
					migrationCopy(t, filepath.Join(seed, filename), filepath.Join(directory, filename))
				}
				cwd := baselineRoot
				if version == "current" {
					cwd = currentRoot
				}
				output := migrationCommand(t, cwd, binaries[version], directory, scenario)
				if err := os.WriteFile(filepath.Join(directory, "execution.log"), output, 0o644); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(filepath.Join(directory, "result.json"))
				if err != nil {
					t.Fatal(err)
				}
				var decoded any
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Fatal(err)
				}
				results[version] = decoded
			}
			if !reflect.DeepEqual(results["baseline"], results["current"]) {
				t.Fatalf("%s parity differs; compare %s/{baseline,current}/result.json", scenario, filepath.Join(evidence, scenario))
			}
		})
	}
	head := strings.TrimSpace(string(migrationCommand(t, root, "git", "rev-parse", "HEAD")))
	manifest := fmt.Sprintf("Baseline: %s\nCurrent commit: %s\nCurrent source SHA-256: %x\nScenarios: %s\n", migrationBaseline, head, sourceHash.Sum(nil), strings.Join(cases, ", "))
	if err := os.WriteFile(filepath.Join(evidence, "comparison.txt"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

// migrationCommand executes an offline build or isolated parity runner and
// reports its output on failure without shell interpretation.
func migrationCommand(t *testing.T, directory, name string, args ...string) []byte {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
	return output
}

// migrationCopy copies one authoritative source or closed fixture database
// into a target-owned directory without changing its source.
func migrationCopy(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
