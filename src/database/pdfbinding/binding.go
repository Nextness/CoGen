// binding.go provides the metadata-side companion PDF binding store, path
// validation, and default binding resolution.
package pdfbinding

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	generated "analysis/database/pdfbinding/internal/sql"
	"analysis/internal/pathpolicy"
)

// DefaultStoreFilename is the companion store bound beside the metadata
// database on first inventory use.
const DefaultStoreFilename = "corpus.pdf.db"

// Store binds the generated pdfbinding queries to one already configured
// metadata connection.
type Store struct {
	db      *sql.DB
	queries *generated.Queries
}

// New returns a pdfbinding store over an already configured metadata
// connection. It only binds the generated queries to db; it does not build a
// SQLite URI, alter pragmas, open or close the connection, load migration
// configuration, or run migrations.
func New(db *sql.DB) *Store {
	return &Store{db: db, queries: generated.New(db)}
}

// Binding returns the singleton relative companion path and whether a binding exists.
func (s *Store) Binding(ctx context.Context) (string, bool, error) {
	relativePath, err := s.queries.GetPDFStoreBinding(ctx)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read PDF store binding: %w", err)
	}
	return relativePath, true, nil
}

// EnsureBinding records a portable bundle-relative companion path. Existing
// bindings are preserved so older corpus bundles remain usable.
func (s *Store) EnsureBinding(ctx context.Context, relativePath string) error {
	cleanPath, err := validateRelativeStorePath(relativePath)
	if err != nil {
		return err
	}
	existingPath, found, err := s.Binding(ctx)
	if err != nil {
		return err
	}
	if !found {
		digest := sha256.Sum256([]byte("pdf-inventory-store\x00" + cleanPath))
		if err := s.queries.InsertPDFStoreBinding(ctx, generated.InsertPDFStoreBindingParams{
			RelativePath:      cleanPath,
			ConfiguredAt:      timestamp(time.Now()),
			ConfigFingerprint: hex.EncodeToString(digest[:]),
		}); err != nil {
			return fmt.Errorf("create PDF store binding: %w", err)
		}
		return nil
	}
	if existingPath != cleanPath {
		return fmt.Errorf("metadata corpus is already bound to PDF store %q, not %q", existingPath, cleanPath)
	}
	return nil
}

// ResolveBinding returns the existing companion path or binds the default
// corpus.pdf.db beside the metadata database on first inventory use.
func (s *Store) ResolveBinding(ctx context.Context, metadataPath string) (string, error) {
	relativePath, found, err := s.Binding(ctx)
	if err != nil {
		return "", err
	}
	if !found {
		relativePath = DefaultStoreFilename
		if err := s.EnsureBinding(ctx, relativePath); err != nil {
			return "", err
		}
	}
	return resolveStorePath(metadataPath, relativePath)
}

// BeginTx starts one metadata transaction for a delivered-event batch.
func (s *Store) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, nil)
}

// timestamp formats a UTC time for persisted binding metadata.
func timestamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// resolveStorePath resolves store path from the supplied context.
func resolveStorePath(metadataPath, relativePath string) (string, error) {
	if metadataPath == "" {
		return "", fmt.Errorf("metadata database path is required")
	}
	cleanPath, err := validateRelativeStorePath(relativePath)
	if err != nil {
		return "", err
	}
	metadataAbsolute, err := filepath.Abs(metadataPath)
	if err != nil {
		return "", fmt.Errorf("resolve metadata database path: %w", err)
	}
	metadataDir := filepath.Dir(metadataAbsolute)
	storePath, err := pathpolicy.ResolveExistingComponentsWithin(metadataDir, cleanPath)
	if err != nil {
		return "", fmt.Errorf("resolve PDF store path: %w", err)
	}
	metadataResolved, err := pathpolicy.ResolveExistingComponentsWithin(metadataDir, metadataAbsolute)
	if err != nil {
		return "", fmt.Errorf("resolve metadata database path: %w", err)
	}
	if storePath == metadataResolved {
		return "", fmt.Errorf("PDF store path must differ from the metadata database path")
	}
	return storePath, nil
}

// validateRelativeStorePath rejects absolute or escaping companion-store paths and returns a clean relative path.
func validateRelativeStorePath(relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("PDF store path must be relative")
	}
	cleanPath := filepath.Clean(strings.TrimSpace(relativePath))
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("PDF store path must stay within the metadata database directory")
	}
	return cleanPath, nil
}
