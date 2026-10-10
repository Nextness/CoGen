// migrations.go applies the configured migration chain under a cross-process
// lock and reports the applied schema version.
package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SchemaVersion returns the most recently applied migration filename. It is
// recorded in each resolved manifest so plan fingerprints describe the schema
// that interpreted the input.
func (d *Database) SchemaVersion() (string, error) {
	var version sql.NullString
	err := d.DB.QueryRow("SELECT filename FROM schema_migrations ORDER BY rowid DESC LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows || !version.Valid {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get schema version: %w", err)
	}
	return version.String, nil
}

const migrationsTable = "schema_migrations"

// runMigrations applies unapplied configured migrations in declaration order and records their checksums.
func (d *Database) runMigrations(configPath string) error {
	ctx := context.Background()
	if err := d.withMigrationLock(ctx, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			filename   TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now')),
			checksum   TEXT NOT NULL
		)`, migrationsTable))
		return err
	}); err != nil {
		lg.Debug("migration tracking table creation failed", "error", err)
		return fmt.Errorf("create tracking table: %w", err)
	}

	// Load migration chain from database.something
	entries, err := loadMigrationChain(configPath)
	if err != nil {
		lg.Debug("migration chain load failed", "config", configPath, "error", err)
		return fmt.Errorf("load migration chain: %w", err)
	}
	if len(entries) == 0 {
		lg.Debug("migration run successful", "config", configPath, "result", "no_migrations")
		return nil
	}

	appliedCount := 0
	adoptedCount := 0
	skippedCount := 0
	for _, entry := range entries {
		fn := entry.filename
		sqlPath := filepath.Join(d.migrations, fn)
		upSQL, err := extractUpSQL(sqlPath)
		if err != nil {
			lg.Debug("migration SQL extraction failed", "file", fn, "error", err)
			return fmt.Errorf("extract SQL from %s: %w", fn, err)
		}
		cs, err := fileChecksum(sqlPath)
		if err != nil {
			lg.Debug("migration checksum failed", "file", fn, "error", err)
			return fmt.Errorf("checksum %s: %w", fn, err)
		}

		wasApplied := false
		wasAdopted := false
		if err := d.withMigrationLock(ctx, func(conn *sql.Conn) error {
			var appliedFilename string
			filenames := append([]string{fn}, entry.supersedes...)
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(filenames)), ",")
			args := make([]any, len(filenames))
			for index := range filenames {
				args[index] = filenames[index]
			}
			query := fmt.Sprintf("SELECT filename FROM %s WHERE filename IN (%s) ORDER BY CASE filename WHEN ? THEN 0 ELSE 1 END LIMIT 1", migrationsTable, placeholders)
			args = append(args, fn)
			err := conn.QueryRowContext(ctx, query, args...).Scan(&appliedFilename)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("query applied migration %s: %w", fn, err)
			}
			if appliedFilename == fn {
				wasApplied = true
				return nil
			}
			if appliedFilename != "" {
				if _, err := conn.ExecContext(ctx,
					fmt.Sprintf("INSERT INTO %s (filename, checksum) VALUES (?, ?)", migrationsTable), fn, cs,
				); err != nil {
					return fmt.Errorf("record migration %s superseding %s: %w", fn, appliedFilename, err)
				}
				wasAdopted = true
				return nil
			}
			if _, err := conn.ExecContext(ctx, upSQL); err != nil {
				return fmt.Errorf("apply %s: %w", fn, err)
			}
			if _, err := conn.ExecContext(ctx,
				fmt.Sprintf("INSERT INTO %s (filename, checksum) VALUES (?, ?)", migrationsTable), fn, cs,
			); err != nil {
				return fmt.Errorf("record migration %s: %w", fn, err)
			}
			return nil
		}); err != nil {
			lg.Debug("migration application failed", "file", fn, "error", err)
			return err
		}
		if wasApplied {
			skippedCount++
			lg.Debug("migration skip successful", "file", fn, "result", "already_applied")
			continue
		}
		if wasAdopted {
			adoptedCount++
			lg.Info("migration adoption successful", "file", fn, "supersedes", entry.supersedes)
			continue
		}
		appliedCount++
		lg.Debug("migration application successful", "file", fn)
	}

	lg.Info("migration run successful",
		"config", configPath,
		"configured", len(entries),
		"applied", appliedCount,
		"adopted", adoptedCount,
		"skipped", skippedCount)
	return nil
}

// withMigrationLock serializes each migration transaction across independent
// processes. BEGIN IMMEDIATE obtains SQLite's write lock before checking the
// tracking table, preventing two openers from both observing a migration as
// pending and applying it twice.
func (d *Database) withMigrationLock(ctx context.Context, action func(*sql.Conn) error) (err error) {
	conn, err := d.DB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()
	if err := action(conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit migration lock: %w", err)
	}
	committed = true
	return nil
}

const upMarker = "-- ==UP=="
const downMarker = "-- ==DOWN=="

// extractUpSQL returns the SQL between a migration's required UP and DOWN markers.
func extractUpSQL(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	content := string(data)

	if strings.Count(content, upMarker) != 1 || strings.Count(content, downMarker) != 1 {
		return "", fmt.Errorf("migration must contain exactly one %s and one %s marker", upMarker, downMarker)
	}
	upStart := strings.Index(content, upMarker)
	downStart := strings.Index(content, downMarker)
	if downStart <= upStart {
		return "", fmt.Errorf("migration %s marker must precede %s marker", upMarker, downMarker)
	}
	upSQL := strings.TrimSpace(content[upStart+len(upMarker) : downStart])
	if upSQL == "" {
		return "", fmt.Errorf("migration %s section must not be empty", upMarker)
	}
	return upSQL, nil
}

// fileChecksum returns the lowercase hexadecimal SHA-256 digest of a file.
func fileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}
