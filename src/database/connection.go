// connection.go provides SQLite connection construction, pragma configuration,
// and lifecycle for writable and existing-only modes.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"analysis/internal/sqliteuri"

	_ "modernc.org/sqlite"
)

// OpenExisting opens an existing metadata database for narrowly scoped review
// writes. It never creates directories, changes journal mode, or runs migrations.
func OpenExisting(dbPath string) (*Database, error) {
	return openExistingWithDriver(dbPath, "sqlite")
}

// OpenExistingWithDriver opens an existing metadata database through a caller-provided registered SQL driver.
// It exists so the viewer can enforce request-scoped query budgets without changing pipeline connections.
func OpenExistingWithDriver(dbPath, driverName string) (*Database, error) {
	if strings.TrimSpace(driverName) == "" {
		return nil, fmt.Errorf("database driver is required")
	}
	return openExistingWithDriver(dbPath, driverName)
}

// openExistingWithDriver contains the existing-only connection contract shared by the default and instrumented viewer drivers.
func openExistingWithDriver(dbPath, driverName string) (*Database, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("database path is required")
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("database does not exist")
		}
		return nil, fmt.Errorf("inspect database: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("database path is a directory")
	}
	absolute, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	uri := sqliteuri.File(absolute, map[string][]string{
		"mode":    {"rw"},
		"_txlock": {"immediate"},
		"_pragma": {"busy_timeout(5000)", "foreign_keys(1)"},
	})
	conn, err := sql.Open(driverName, uri)
	if err != nil {
		return nil, fmt.Errorf("open existing sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open existing sqlite: %w", err)
	}
	d := &Database{DB: conn, dbPath: absolute}
	d.initRepositories()
	return d, nil
}

// OpenConfigured opens a writable SQLite database, configures its connection
// pool, and applies the migration chain selected from the database registry.
// It is used by the metadata repositories and the independently owned PDF
// store.
func OpenConfigured(dbPath, registryPath string, kind StoreKind) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			lg.Debug("database directory creation failed", "directory", dir, "error", err)
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	// modernc applies _pragma values to every pooled connection. Foreign-key
	// enforcement is connection-local in SQLite, so configuring it only with a
	// one-time PRAGMA would leave later pooled connections unprotected.
	absolute, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	uri := sqliteuri.File(absolute, map[string][]string{
		"_txlock": {"immediate"},
		"_pragma": {"busy_timeout(5000)", "foreign_keys(1)"},
	})
	conn, err := sql.Open("sqlite", uri)
	if err != nil {
		lg.Debug("database connection open failed", "database_path", dbPath, "error", err)
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Pragmas
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if err := configurePragma(conn, p); err != nil {
			conn.Close()
			lg.Debug("database pragma configuration failed",
				"database_path", dbPath, "pragma", p, "error", err)
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	lg.Debug("database pragma configuration successful",
		"database_path", dbPath, "pragmas", len(pragmas))

	migrationConfig, err := ResolveMigrationConfig(registryPath, kind)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("resolve migrations: %w", err)
	}
	d := &Database{DB: conn, dbPath: dbPath, migrations: migrationConfig.MigrationsDir}
	if err := d.runMigrations(migrationConfig.ConfigPath); err != nil {
		conn.Close()
		lg.Debug("database migration run failed",
			"database_path", dbPath, "config", migrationConfig.ConfigPath, "error", err)
		return nil, fmt.Errorf("migrations: %w", err)
	}
	return conn, nil
}

// configurePragma retries startup-only locking around journal-mode changes.
// The connection URI covers normal busy handling, but two processes enabling
// WAL on an uninitialised database can still race before either has completed
// its first pragma sequence.
func configurePragma(db *sql.DB, pragma string) error {
	const maxAttempts = 50
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if _, err := db.Exec(pragma); err == nil {
			return nil
		} else if !sqliteBusy(err) {
			return err
		} else {
			lastErr = err
		}
		time.Sleep(time.Duration(attempt+1) * time.Millisecond)
	}
	return fmt.Errorf("configure pragma after %d attempts: %w", maxAttempts, lastErr)
}

// sqliteBusy reports whether an error represents SQLite busy or locked contention.
func sqliteBusy(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "database is locked") || strings.Contains(message, "SQLITE_BUSY")
}

// Close closes the database connection.
func (d *Database) Close() error {
	err := d.DB.Close()
	if err != nil {
		lg.Debug("database close failed", "database_path", d.dbPath, "error", err)
		return err
	}
	lg.Debug("database close successful", "database_path", d.dbPath)
	return nil
}
