// Package database provides a SQLite-backed storage layer for the corpus pipeline.
//
// All tables use INSERT OR IGNORE semantics for key conflicts: if a row with
// the same unique key already exists, the insert is silently skipped. This
// makes the pipeline idempotent - you can re-run without deleting the database.
package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"analysis/logging"

	_ "modernc.org/sqlite"
)

var lg = logging.Logger("database")

// Database wraps a SQLite connection and exposes per-table repositories.
type Database struct {
	DB                  *sql.DB
	PipelineRuns        *PipelineRunRepository
	Searches            *SearchRepository
	Revisions           *SearchRevisionRepository
	Plans               *ExecutionPlanRepository
	RunSources          *RunSourceRepository
	SourceRecords       *SourceRecordRepository
	Artifacts           *ArtifactRepository
	RunSteps            *RunStepRepository
	Metrics             *MetricsRepository
	AuditEvents         *AuditEventRepository
	Works               *WorkRepository
	WorkIdentifiers     *WorkIdentifierRepository
	WorkRevisions       *WorkRevisionRepository
	RunWorkStages       *RunWorkStageRepository
	People              *PersonRepository
	AuthorOccs          *AuthorOccurrenceRepository
	Authorships         *AuthorshipRepository
	IdentityResolutions *AuthorIdentityResolutionRepository
	IdentityCandidates  *AuthorIdentityCandidateRepository
	ReferenceMentions   *ReferenceMentionRepository
	CacheEntries        *CacheEntryRepository
	RunCacheUses        *RunCacheUseRepository
	ArtifactBlobs       *ArtifactBlobRepository
	RunArtifacts        *RunArtifactRepository
	SourceFilterCounts  *SourceFilterCountRepository

	dbPath     string
	migrations string // migration SQL directory
}

// Open opens (or creates) the SQLite database at dbPath, runs pending
// migrations, and initialises repositories. Call Close when done.
func Open(dbPath, configPath string) (*Database, error) {
	conn, err := OpenConfigured(dbPath, configPath, StoreCorpusMetadata)
	if err != nil {
		return nil, err
	}

	d := &Database{
		DB:     conn,
		dbPath: dbPath,
	}
	d.initRepositories()

	lg.Debug("database open successful", "database_path", dbPath)
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
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
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

// initRepositories binds every repository facade to the opened database.
func (d *Database) initRepositories() {
	d.PipelineRuns = &PipelineRunRepository{db: d}
	d.Searches = &SearchRepository{db: d}
	d.Revisions = &SearchRevisionRepository{db: d}
	d.Plans = &ExecutionPlanRepository{db: d}
	d.RunSources = &RunSourceRepository{db: d}
	d.SourceRecords = &SourceRecordRepository{db: d}
	d.Artifacts = &ArtifactRepository{db: d}
	d.RunSteps = &RunStepRepository{db: d}
	d.Metrics = &MetricsRepository{db: d}
	d.AuditEvents = &AuditEventRepository{db: d}
	d.Works = &WorkRepository{db: d}
	d.WorkIdentifiers = &WorkIdentifierRepository{db: d}
	d.WorkRevisions = &WorkRevisionRepository{db: d}
	d.RunWorkStages = &RunWorkStageRepository{db: d}
	d.People = &PersonRepository{db: d}
	d.AuthorOccs = &AuthorOccurrenceRepository{db: d}
	d.Authorships = &AuthorshipRepository{db: d}
	d.IdentityResolutions = &AuthorIdentityResolutionRepository{db: d}
	d.IdentityCandidates = &AuthorIdentityCandidateRepository{db: d}
	d.ReferenceMentions = &ReferenceMentionRepository{db: d}
	d.CacheEntries = &CacheEntryRepository{db: d}
	d.RunCacheUses = &RunCacheUseRepository{db: d}
	d.ArtifactBlobs = &ArtifactBlobRepository{db: d}
	d.RunArtifacts = &RunArtifactRepository{db: d}
	d.SourceFilterCounts = &SourceFilterCountRepository{db: d}
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
	skippedCount := 0
	for _, entry := range entries {
		fn := entry.filename
		sqlPath := filepath.Join(d.migrations, fn)
		upSQL, err := extractUpSQL(sqlPath)
		if err != nil {
			lg.Debug("migration SQL extraction failed", "file", fn, "error", err)
			return fmt.Errorf("extract SQL from %s: %w", fn, err)
		}
		if upSQL == "" {
			skippedCount++
			lg.Debug("migration validation failed", "file", fn, "reason", "missing_up_section")
			continue
		}
		cs, err := fileChecksum(sqlPath)
		if err != nil {
			lg.Debug("migration checksum failed", "file", fn, "error", err)
			return fmt.Errorf("checksum %s: %w", fn, err)
		}

		wasApplied := false
		if err := d.withMigrationLock(ctx, func(conn *sql.Conn) error {
			var count int
			if err := conn.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE filename=?", migrationsTable), fn).Scan(&count); err != nil {
				return fmt.Errorf("query applied migration %s: %w", fn, err)
			}
			if count > 0 {
				wasApplied = true
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
		appliedCount++
		lg.Debug("migration application successful", "file", fn)
	}

	lg.Info("migration run successful",
		"config", configPath,
		"configured", len(entries),
		"applied", appliedCount,
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

// migrationEntry stores one configured migration filename and its descriptive linkage fields.
type migrationEntry struct {
	filename string
	previous string
	upgrade  string
}

// loadMigrationChain evaluates the database registry and returns its migrations in declaration order.
func loadMigrationChain(configPath string) ([]migrationEntry, error) {
	cfg, err := loadSomethingConfig(configPath)
	if err != nil {
		return nil, err
	}

	entries, err := getMigrationStructs(cfg)
	if err != nil {
		return nil, err
	}

	// Sort by iteration counter (already in order from getMigrationStructs)
	return entries, nil
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

	upStart := strings.Index(content, upMarker)
	if upStart < 0 {
		return "", nil
	}

	sectionStart := upStart + len(upMarker)
	downStart := strings.Index(content, downMarker)

	var upSQL string
	if downStart >= 0 {
		upSQL = strings.TrimSpace(content[sectionStart:downStart])
	} else {
		upSQL = strings.TrimSpace(content[sectionStart:])
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

// timestamp returns the current UTC time in the repository's persisted format.
func timestamp() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}
