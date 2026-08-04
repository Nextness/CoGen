// main_integration_test.go tests the something-json CLI tool with real
// .something config files.
//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"analysis/something"
)

// TestLoadRealConfig verifies load real config.
func TestLoadRealConfig(t *testing.T) {
	candidates := []string{
		filepath.Join("..", "..", "..", "config", "workspace.something"),
		filepath.Join("..", "..", "..", "config", "database.something"),
	}
	var configPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			configPath, _ = filepath.Abs(c)
			break
		}
	}
	if configPath == "" {
		t.Skip("no real .something config file found")
	}

	result, err := something.LoadSomethingFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty JSON output")
	}
}
