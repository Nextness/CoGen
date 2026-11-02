// history_unit_test.go verifies configured migration identity and recorded
// history validation without database or filesystem state.

package database_test

import (
	"strings"
	"testing"

	"analysis/internal/migrations"
)

// TestValidateEntries verifies migration identities and supersedes aliases are unambiguous.
func TestValidateEntries(t *testing.T) {
	valid := []migrations.Entry{
		{Filename: "V00001_first.sql"},
		{Filename: "V00002_second.sql", Supersedes: []string{"V00002_old_second.sql"}},
	}
	if err := migrations.ValidateEntries(valid); err != nil {
		t.Fatalf("valid chain: %v", err)
	}
	for _, testCase := range []struct {
		name    string
		entries []migrations.Entry
		want    string
	}{
		{name: "empty", want: "empty"},
		{name: "invalid filename", entries: []migrations.Entry{{Filename: "first.sql"}}, want: "V00001_description.sql"},
		{name: "duplicate filename", entries: []migrations.Entry{{Filename: "V00001_first.sql"}, {Filename: "V00001_first.sql"}}, want: "more than once"},
		{name: "self supersedes", entries: []migrations.Entry{{Filename: "V00001_first.sql", Supersedes: []string{"V00001_first.sql"}}}, want: "supersede itself"},
		{name: "duplicate alias", entries: []migrations.Entry{{Filename: "V00001_first.sql", Supersedes: []string{"V00000_old.sql"}}, {Filename: "V00002_second.sql", Supersedes: []string{"V00000_old.sql"}}}, want: "already supersedes"},
		{name: "alias is configured", entries: []migrations.Entry{{Filename: "V00001_first.sql"}, {Filename: "V00002_second.sql", Supersedes: []string{"V00001_first.sql"}}}, want: "also a configured migration"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := migrations.ValidateEntries(testCase.entries)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want %q", err, testCase.want)
			}
		})
	}
}

// TestVerifyHistory verifies only complete ordered configured history is accepted.
func TestVerifyHistory(t *testing.T) {
	entries := []migrations.Entry{
		{Filename: "V00001_first.sql"},
		{Filename: "V00002_second.sql", Supersedes: []string{"V00002_old_second.sql"}},
		{Filename: "V00003_third.sql"},
	}
	for _, testCase := range []struct {
		name    string
		applied []string
		want    int
		wantErr string
	}{
		{name: "empty", want: -1},
		{name: "complete", applied: []string{"V00001_first.sql", "V00002_second.sql", "V00003_third.sql"}, want: 2},
		{name: "prefix", applied: []string{"V00001_first.sql"}, want: 0},
		{name: "alias prefix", applied: []string{"V00001_first.sql", "V00002_old_second.sql"}, want: 1},
		{name: "alias adoption", applied: []string{"V00001_first.sql", "V00002_old_second.sql", "V00002_second.sql", "V00003_third.sql"}, want: 2},
		{name: "unknown", applied: []string{"V00004_future.sql"}, wantErr: "unknown applied migration"},
		{name: "out of order", applied: []string{"V00001_first.sql", "V00003_third.sql", "V00002_second.sql"}, wantErr: "out of configured order"},
		{name: "missing middle", applied: []string{"V00001_first.sql", "V00003_third.sql"}, wantErr: "missing applied migration"},
		{name: "missing first", applied: []string{"V00002_second.sql"}, wantErr: "missing applied migration"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := migrations.VerifyHistory(entries, testCase.applied)
			if testCase.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("error = %v, want %q", err, testCase.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != testCase.want {
				t.Fatalf("latest = %d, want %d", got, testCase.want)
			}
		})
	}
}
