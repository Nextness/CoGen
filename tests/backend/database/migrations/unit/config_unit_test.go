// Unit tests for database migration configuration validation.

package database_test

import "analysis/internal/migrations"

import "testing"

// TestValidMigrationFilename verifies canonical and malformed migration identities are distinguished without accepting paths or empty descriptions.
func TestValidMigrationFilename(t *testing.T) {
	for _, test := range []struct {
		filename string
		want     bool
	}{
		{filename: "V00026_review_anchor_labels.sql", want: true},
		{filename: "V00025_work_revision_term_matches.sql", want: true},
		{filename: "V00026_.sql", want: false},
		{filename: "V0026_review_anchor_labels.sql", want: false},
		{filename: "V0002x_review_anchor_labels.sql", want: false},
		{filename: "../V00026_review_anchor_labels.sql", want: false},
		{filename: "V00001_../outside.sql", want: false},
		{filename: "V00001_nested/name.sql", want: false},
		{filename: "V00001_nested\\name.sql", want: false},
		{filename: "V00001_..sql", want: false},
		{filename: "V00026_review_anchor_labels.txt", want: false},
	} {
		t.Run(test.filename, func(t *testing.T) {
			if got := migrations.ValidFilename(test.filename); got != test.want {
				t.Fatalf("validMigrationFilename(%q) = %t, want %t", test.filename, got, test.want)
			}
		})
	}
}

// TestGetMigrationStructsRejectsMissingOrNonStringFilename verifies every configured iteration has a usable filename.
func TestGetMigrationStructsRejectsMissingOrNonStringFilename(t *testing.T) {
	for name, entry := range map[string]map[string]any{
		"missing":    {},
		"empty":      {"filename": ""},
		"non-string": {"filename": int64(1)},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := map[string]any{
				"iteration_0000000000_db_migration": entry,
			}
			if _, err := migrations.ParseChain(cfg); err == nil {
				t.Fatal("getMigrationStructs() unexpectedly succeeded")
			}
		})
	}
}
