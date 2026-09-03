// migration_sql_unit_test.go verifies migration SQL marker validation.
//go:build unit

package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExtractUpSQLRejectsInvalidMarkerLayouts verifies malformed migration files are rejected before execution.
func TestExtractUpSQLRejectsInvalidMarkerLayouts(t *testing.T) {
	for name, content := range map[string]string{
		"missing up":       "-- ==DOWN==\nDROP TABLE test;",
		"missing down":     "-- ==UP==\nCREATE TABLE test (id INTEGER);",
		"reversed":         "-- ==DOWN==\n-- ==UP==\nCREATE TABLE test (id INTEGER);",
		"duplicated":       "-- ==UP==\nCREATE TABLE test (id INTEGER);\n-- ==UP==\n-- ==DOWN==",
		"empty up section": "-- ==UP==\n-- ==DOWN==",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "V00001_test.sql")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := extractUpSQL(path); err == nil || !strings.Contains(err.Error(), "marker") && !strings.Contains(err.Error(), "section") {
				t.Fatalf("extractUpSQL error = %v", err)
			}
		})
	}
}
