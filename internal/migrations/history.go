// history.go validates configured migration inputs and recorded migration
// history before either store changes.
package migrations

import (
	"analysis/internal/filehash"
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
)

// ValidateEntries verifies that configured migration identities and supersedes
// aliases are unique and unambiguous within one chain.
func ValidateEntries(entries []Entry) error {
	if len(entries) == 0 {
		return fmt.Errorf("migration chain is empty")
	}
	canonical := make(map[string]bool, len(entries))
	aliases := make(map[string]string)
	for _, entry := range entries {
		if !ValidFilename(entry.Filename) {
			return fmt.Errorf("migration filename %q must be in V00001_description.sql form", entry.Filename)
		}
		if canonical[entry.Filename] {
			return fmt.Errorf("migration %s is configured more than once", entry.Filename)
		}
		canonical[entry.Filename] = true
		for _, alias := range entry.Supersedes {
			if !ValidFilename(alias) {
				return fmt.Errorf("migration %s supersedes %q, which must be in V00001_description.sql form", entry.Filename, alias)
			}
			if alias == entry.Filename {
				return fmt.Errorf("migration %s cannot supersede itself", entry.Filename)
			}
			if owner, exists := aliases[alias]; exists {
				return fmt.Errorf("migration %s supersedes %q, which %s already supersedes", entry.Filename, alias, owner)
			}
			aliases[alias] = entry.Filename
		}
	}
	for _, entry := range entries {
		if owner, exists := aliases[entry.Filename]; exists {
			return fmt.Errorf("supersedes alias %q of %s is also a configured migration", entry.Filename, owner)
		}
	}
	return nil
}

// ValidateInputs verifies every configured migration file is readable, has
// exactly one ordered UP and DOWN marker pair, and contains a non-empty UP
// section before any migration runs.
func ValidateInputs(entries []Entry, migrationsDir string) error {
	if err := ValidateEntries(entries); err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(migrationsDir, entry.Filename)
		if _, err := ReadUpSQL(path); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Filename, err)
		}
		if _, err := filehash.SHA256(path); err != nil {
			return fmt.Errorf("migration %s checksum: %w", entry.Filename, err)
		}
	}
	return nil
}

// Applied returns the recorded migration filenames in application order. A
// missing tracking table means no migration has been recorded.
func Applied(ctx context.Context, db *sql.DB) ([]string, error) {
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", migrationsTable).Scan(&exists); err != nil {
		return nil, fmt.Errorf("inspect migration tracking table: %w", err)
	}
	if exists == 0 {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT filename FROM %s ORDER BY rowid", migrationsTable))
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()
	var applied []string
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, fmt.Errorf("read applied migration: %w", err)
		}
		applied = append(applied, filename)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	return applied, nil
}

// VerifyHistory verifies that applied is a complete, ordered prefix of entries
// and returns the index of the latest applied configured migration, or -1 when
// none has been applied. Declared supersedes aliases count as their canonical
// migration, and unknown or out-of-order history is rejected.
func VerifyHistory(entries []Entry, applied []string) (int, error) {
	if err := ValidateEntries(entries); err != nil {
		return -1, err
	}
	canonical := make(map[string]int, len(entries))
	aliases := make(map[string]int)
	for index, entry := range entries {
		canonical[entry.Filename] = index
		for _, alias := range entry.Supersedes {
			aliases[alias] = index
		}
	}
	latest := -1
	seen := make(map[int]bool)
	for _, filename := range applied {
		index, ok := canonical[filename]
		if !ok {
			index, ok = aliases[filename]
		}
		if !ok {
			return -1, fmt.Errorf("unknown applied migration %q", filename)
		}
		if index < latest {
			return -1, fmt.Errorf("applied migration %q is out of configured order", filename)
		}
		latest = index
		seen[index] = true
	}
	for index := 0; index <= latest; index++ {
		if !seen[index] {
			return -1, fmt.Errorf("missing applied migration %s", entries[index].Filename)
		}
	}
	return latest, nil
}
