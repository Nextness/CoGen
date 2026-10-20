// migrations.go applies the configured migration chain under a cross-process
// lock and reports the applied schema version.
package migrations

import (
	"analysis/backend/logging"
	"analysis/internal/filehash"
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
)

var lg = logging.Logger("database")

const migrationsTable = "schema_migrations"

// Apply applies unapplied configured migrations in declaration order and records their checksums.
func Apply(db *sql.DB, migrationsDir, configPath string) error {
	ctx := context.Background()
	if err := withMigrationLock(db, ctx, func(conn *sql.Conn) error {
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
	entries, err := LoadChain(configPath)
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
		fn := entry.Filename
		sqlPath := filepath.Join(migrationsDir, fn)
		upSQL, err := ReadUpSQL(sqlPath)
		if err != nil {
			lg.Debug("migration SQL extraction failed", "file", fn, "error", err)
			return fmt.Errorf("extract SQL from %s: %w", fn, err)
		}
		cs, err := filehash.SHA256(sqlPath)
		if err != nil {
			lg.Debug("migration checksum failed", "file", fn, "error", err)
			return fmt.Errorf("checksum %s: %w", fn, err)
		}

		wasApplied := false
		wasAdopted := false
		if err := withMigrationLock(db, ctx, func(conn *sql.Conn) error {
			var appliedFilename string
			filenames := append([]string{fn}, entry.Supersedes...)
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
			lg.Info("migration adoption successful", "file", fn, "supersedes", entry.Supersedes)
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
func withMigrationLock(db *sql.DB, ctx context.Context, action func(*sql.Conn) error) (err error) {
	conn, err := db.Conn(ctx)
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
