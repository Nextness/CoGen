// Package database provides a SQLite-backed storage layer for the corpus pipeline.
//
// Repository conflict handling preserves idempotency while content-addressed
// artifacts verify that a repeated identity has identical metadata and bytes.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"analysis/backend/database/artifact"
	"analysis/backend/database/audit"
	"analysis/backend/database/author"
	"analysis/backend/database/cache"
	"analysis/backend/database/pdfbinding"
	"analysis/backend/database/review"
	"analysis/backend/database/run"
	"analysis/backend/database/search"
	"analysis/backend/database/source"
	"analysis/backend/database/work"
	"analysis/backend/logging"
)

var lg = logging.Logger("database")

// Database wraps a SQLite connection and exposes the per-table-family stores.
type Database struct {
	db          *sql.DB
	Run         *run.Store
	Search      *search.Store
	Source      *source.Store
	Artifact    *artifact.Store
	Cache       *cache.Store
	Audit       *audit.Store
	Work        *work.Store
	Author      *author.Store
	Review      *review.Store
	PDFBindings *pdfbinding.Store

	dbPath string
}

// Open opens (or creates) the SQLite database at dbPath, runs pending
// migrations, and initialises repositories. Call Close when done.
func Open(dbPath, configPath string) (*Database, error) {
	conn, err := OpenConfigured(dbPath, configPath, StoreCorpusMetadata)
	if err != nil {
		return nil, err
	}

	d := New(conn)
	d.dbPath = dbPath

	lg.Debug("database open successful", "database_path", dbPath)
	return d, nil
}

// MigrateExisting applies the configured metadata migration chain to an existing file and never runs a workspace.
func MigrateExisting(dbPath, configPath string) error {
	if strings.TrimSpace(dbPath) == "" {
		return fmt.Errorf("database path is required")
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("database does not exist")
		}
		return fmt.Errorf("inspect database: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("database path is a directory")
	}
	db, err := Open(dbPath, configPath)
	if err != nil {
		return err
	}
	return db.Close()
}

// New binds metadata family stores to a caller-configured connection without opening or migrating it.
// The caller must configure SQLite and apply the metadata migrations before use, normally through OpenConfigured.
// The returned Database owns the supplied connection; Close closes it.
func New(conn *sql.DB) *Database {
	d := &Database{db: conn}
	d.Run = run.New(d.db)
	d.Search = search.New(d.db)
	d.Source = source.New(d.db)
	d.Artifact = artifact.New(d.db)
	d.Cache = cache.New(d.db)
	d.Audit = audit.New(d.db)
	d.Work = work.New(d.db)
	d.Author = author.New(d.db)
	d.Review = review.New(d.db)
	d.PDFBindings = pdfbinding.New(d.db)
	return d
}
