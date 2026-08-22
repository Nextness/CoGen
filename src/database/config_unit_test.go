// Unit tests for database migration configuration validation.
//go:build unit

package database

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
		{filename: "V00026_review_anchor_labels.txt", want: false},
	} {
		t.Run(test.filename, func(t *testing.T) {
			if got := validMigrationFilename(test.filename); got != test.want {
				t.Fatalf("validMigrationFilename(%q) = %t, want %t", test.filename, got, test.want)
			}
		})
	}
}
