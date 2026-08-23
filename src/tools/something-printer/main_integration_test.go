// main_integration_test.go tests the something-printer CLI tool with real
// .something config files.
//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/something"
)

// findRealConfig locates a real .something config file relative to the repo
// root that currently evaluates, so a mid-edit production config does not fail
// the printer integration tests.
func findRealConfig(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "config", "workspace.something"),
		filepath.Join("..", "..", "..", "config", "database.something"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		abs, err := filepath.Abs(c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := something.LoadSomethingFile(abs); err != nil {
			continue
		}
		return abs
	}
	t.Skip("no evaluable real .something config file found")
	return ""
}

// TestRunRealConfigJSON verifies json style with a real config.
func TestRunRealConfigJSON(t *testing.T) {
	configPath := findRealConfig(t)
	var stdout, stderr bytes.Buffer
	code := run("something-printer", []string{"--json", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "{") {
		t.Errorf("expected JSON object output, got: %.120s", stdout.String())
	}
}

// TestRunRealConfigSomething verifies something style with a real config.
func TestRunRealConfigSomething(t *testing.T) {
	configPath := findRealConfig(t)
	var stdout, stderr bytes.Buffer
	code := run("something-printer", []string{"--something", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), " = ") {
		t.Errorf("expected SOMETHING assignments, got: %.120s", stdout.String())
	}
}

// TestRunRealConfigYAML verifies yaml style with a real config.
func TestRunRealConfigYAML(t *testing.T) {
	configPath := findRealConfig(t)
	var stdout, stderr bytes.Buffer
	code := run("something-printer", []string{"--yaml", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatal("expected non-empty YAML output")
	}
}
