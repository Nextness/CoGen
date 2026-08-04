// Package pdfstore owns the companion SQLite database used for validated PDF
// bytes and durable audit delivery.
package pdfstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"analysis/database"
	"analysis/manifest"
)

const (
	DefaultStoreFilename = "corpus.pdf.db"
	StatusNotAvailable   = "not_available"
	StatusAvailable      = "available"
)

// Store is the writable companion PDF database.
type Store struct {
	DB  *sql.DB
	now func() time.Time
}

// Document describes one normalized article's PDF inventory state.
type Document struct {
	DOI           string
	Status        string
	ContentHash   string
	InventoriedAt string
	UpdatedAt     string
}

// AddResult reports the content identity, byte size, and insertion outcome of a manual PDF add.
type AddResult struct {
	ContentHash string
	ByteSize    int
	Added       bool
}

// Open creates or opens the PDF store and applies its independent migration
// chain selected by the database registry.
func Open(path, registryPath string) (*Store, error) {
	db, err := database.OpenConfigured(path, registryPath, database.StoreCorpusPDF)
	if err != nil {
		return nil, fmt.Errorf("open PDF store: %w", err)
	}
	return &Store{DB: db, now: time.Now}, nil
}

// Close releases resources owned by the receiver.
func (s *Store) Close() error { return s.DB.Close() }

// timestamp formats a UTC time for persisted PDF metadata.
func timestamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// newCorrelationID returns a cryptographically random hexadecimal audit correlation identifier.
func newCorrelationID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

// Document returns PDF inventory metadata for a normalized DOI, or nil when it is unregistered.
func (s *Store) Document(ctx context.Context, doi string) (*Document, error) {
	doi = database.NormalizeDOI(doi)
	var document Document
	var contentHash, inventoriedAt sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT doi, status, content_hash,
		inventoried_at, updated_at
		FROM pdf_documents WHERE doi=?`, doi).Scan(
		&document.DOI, &document.Status, &contentHash,
		&inventoriedAt, &document.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	document.ContentHash = contentHash.String
	document.InventoriedAt = inventoriedAt.String
	return &document, nil
}

// Register creates the not-available inventory row for one normalized work.
// Re-registering a DOI preserves its current state and emits no duplicate
// audit event.
func (s *Store) Register(ctx context.Context, doi string, workID, pipelineRunID int64) (bool, error) {
	doi = database.NormalizeDOI(doi)
	if doi == "" {
		return false, fmt.Errorf("DOI is required")
	}
	if workID <= 0 {
		return false, fmt.Errorf("work ID must be positive")
	}
	if pipelineRunID <= 0 {
		return false, fmt.Errorf("pipeline run ID must be positive")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	now := timestamp(s.now())
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO pdf_documents
		(doi, status, updated_at) VALUES (?, 'not_available', ?)`, doi, now)
	if err != nil {
		return false, fmt.Errorf("register PDF inventory document: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read PDF inventory registration result: %w", err)
	}
	if affected == 0 {
		return false, nil
	}

	correlationID, err := newCorrelationID()
	if err != nil {
		return false, err
	}
	metadata, err := json.Marshal(map[string]any{
		"doi": doi, "status": StatusNotAvailable,
	})
	if err != nil {
		return false, err
	}
	if err := insertOutbox(ctx, tx, OutboxEvent{
		Actor: "pipeline", PipelineRunID: pipelineRunID,
		EntityType: "work", EntityID: strconv.FormatInt(workID, 10),
		Action: string(manifest.AuditPDFInventoryRegistered), MetadataJSON: string(metadata),
		CorrelationID: correlationID,
	}, now); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit PDF inventory registration: %w", err)
	}
	return true, nil
}

// Add inventories a validated local PDF for a normalized work. The DOI must
// already have been registered by the pipeline. An available record is
// immutable; adding the same DOI again reports it as unchanged.
func (s *Store) Add(ctx context.Context, doi string, workID int64, data []byte) (AddResult, error) {
	doi = database.NormalizeDOI(doi)
	if doi == "" {
		return AddResult{}, fmt.Errorf("DOI is required")
	}
	if workID <= 0 {
		return AddResult{}, fmt.Errorf("work ID must be positive")
	}
	hash, err := ValidatePDF(data, DefaultMaxPDFBytes)
	if err != nil {
		return AddResult{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return AddResult{}, err
	}
	defer tx.Rollback()

	var status string
	var existingHash sql.NullString
	var existingSize sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT d.status, d.content_hash, b.byte_size
		FROM pdf_documents d
		LEFT JOIN pdf_blobs b ON b.content_hash=d.content_hash
		WHERE d.doi=?`, doi).Scan(&status, &existingHash, &existingSize)
	if err == sql.ErrNoRows {
		return AddResult{}, fmt.Errorf("DOI %q is not registered in the normalized PDF inventory", doi)
	}
	if err != nil {
		return AddResult{}, fmt.Errorf("read existing PDF document: %w", err)
	}
	if status == StatusAvailable {
		if !existingHash.Valid || !existingSize.Valid {
			return AddResult{}, fmt.Errorf("available PDF inventory document %q has no stored blob", doi)
		}
		return AddResult{ContentHash: existingHash.String, ByteSize: int(existingSize.Int64), Added: false}, nil
	}
	if status != StatusNotAvailable {
		return AddResult{}, fmt.Errorf("PDF inventory document %q has unsupported status %q", doi, status)
	}

	now := timestamp(s.now())
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO pdf_blobs
		(content_hash, byte_size, data, created_at) VALUES (?, ?, ?, ?)`, hash, len(data), data, now); err != nil {
		return AddResult{}, fmt.Errorf("insert PDF blob: %w", err)
	}
	var storedSize int
	if err := tx.QueryRowContext(ctx, "SELECT byte_size FROM pdf_blobs WHERE content_hash=?", hash).Scan(&storedSize); err != nil {
		return AddResult{}, fmt.Errorf("verify PDF blob: %w", err)
	}
	if storedSize != len(data) {
		return AddResult{}, fmt.Errorf("existing PDF blob size does not match its content hash")
	}
	result, err := tx.ExecContext(ctx, `UPDATE pdf_documents
		SET status='available', content_hash=?, inventoried_at=?, updated_at=?
		WHERE doi=? AND status='not_available'`, hash, now, now, doi)
	if err != nil {
		return AddResult{}, fmt.Errorf("store PDF document: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		if err := tx.QueryRowContext(ctx, `SELECT d.content_hash, b.byte_size
			FROM pdf_documents d
			JOIN pdf_blobs b ON b.content_hash=d.content_hash
			WHERE d.doi=? AND d.status='available'`, doi).Scan(&existingHash, &existingSize); err != nil {
			return AddResult{}, fmt.Errorf("read concurrently stored PDF document: %w", err)
		}
		return AddResult{ContentHash: existingHash.String, ByteSize: int(existingSize.Int64), Added: false}, nil
	}

	correlationID, err := newCorrelationID()
	if err != nil {
		return AddResult{}, err
	}
	metadata, err := json.Marshal(map[string]any{
		"source": "manual", "doi": doi, "status": StatusAvailable,
		"byte_size": len(data), "content_hash": hash, "inventoried_at": now,
	})
	if err != nil {
		return AddResult{}, err
	}
	if err := insertOutbox(ctx, tx, OutboxEvent{
		Actor: "user", EntityType: "work", EntityID: strconv.FormatInt(workID, 10),
		Action: string(manifest.AuditPDFDocumentInventoried), MetadataJSON: string(metadata), CorrelationID: correlationID,
	}, now); err != nil {
		return AddResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return AddResult{}, fmt.Errorf("commit PDF document: %w", err)
	}
	return AddResult{ContentHash: hash, ByteSize: len(data), Added: true}, nil
}

// BindStore records a portable bundle-relative companion path. Existing
// bindings are preserved so older corpus bundles remain usable.
func BindStore(ctx context.Context, metadata *sql.DB, relativePath string) error {
	cleanPath, err := validateRelativeStorePath(relativePath)
	if err != nil {
		return err
	}
	var existingPath string
	err = metadata.QueryRowContext(ctx, "SELECT relative_path FROM pdf_store_binding WHERE id=1").Scan(&existingPath)
	if err == sql.ErrNoRows {
		digest := sha256.Sum256([]byte("pdf-inventory-store\x00" + cleanPath))
		_, err = metadata.ExecContext(ctx, `INSERT INTO pdf_store_binding
			(id, relative_path, configured_at, config_fingerprint) VALUES (1, ?, ?, ?)`,
			cleanPath, timestamp(time.Now()), hex.EncodeToString(digest[:]))
		if err != nil {
			return fmt.Errorf("create PDF store binding: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read PDF store binding: %w", err)
	}
	if existingPath != cleanPath {
		return fmt.Errorf("metadata corpus is already bound to PDF store %q, not %q", existingPath, cleanPath)
	}
	return nil
}

// BoundStorePath returns the existing companion path or binds the default
// corpus.pdf.db beside the metadata database on first inventory use.
func BoundStorePath(ctx context.Context, metadata *sql.DB, metadataPath string) (string, error) {
	var relativePath string
	err := metadata.QueryRowContext(ctx, "SELECT relative_path FROM pdf_store_binding WHERE id=1").Scan(&relativePath)
	if err == sql.ErrNoRows {
		relativePath = DefaultStoreFilename
		if err := BindStore(ctx, metadata, relativePath); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", fmt.Errorf("read PDF store binding: %w", err)
	}
	return resolveStorePath(metadataPath, relativePath)
}

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
	storeAbsolute := filepath.Clean(filepath.Join(filepath.Dir(metadataAbsolute), cleanPath))
	if storeAbsolute == filepath.Clean(metadataAbsolute) {
		return "", fmt.Errorf("PDF store path must differ from the metadata database path")
	}
	return storeAbsolute, nil
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
