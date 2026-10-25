// Package pdfstore owns the companion SQLite database used for validated PDF
// bytes and durable audit delivery.
package pdfstore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"analysis/backend/database"
	"analysis/backend/database/work"
	"analysis/backend/manifest"
	generated "analysis/backend/pdfstore/internal/sql"
)

const (
	StatusNotAvailable = "not_available"
	StatusAvailable    = "available"
)

// Store is one companion PDF database handle bound to an already configured
// connection. The pipeline opens the writable store through Open; the viewer
// binds its statement-budgeted read-only connection through New.
type Store struct {
	DB      *sql.DB
	queries *generated.Queries
	now     func() time.Time
}

// Document describes one normalized article's PDF inventory state.
type Document struct {
	DOI           string
	Status        string
	ContentHash   string
	InventoriedAt string
	UpdatedAt     string
	ChangeCounter int64
}

// AvailableDocument is one available companion document's inventory metadata
// and stored byte size.
type AvailableDocument struct {
	ContentHash   string
	InventoriedAt string
	ByteSize      int64
}

// InventoryEntry is one available companion document's DOI and optional
// inventory timestamp.
type InventoryEntry struct {
	DOI           string
	InventoriedAt *string
}

// New returns a companion-store handle over an already configured connection.
// It only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations. Writable callers must supply a connection configured for
// immediate transactions so conditional save and removal serialize writers
// before validating the current document state.
func New(db *sql.DB) *Store {
	return &Store{DB: db, queries: generated.New(db), now: time.Now}
}

// Open creates or opens the PDF store on an immediate transaction connection
// and applies its independent migration chain selected by the database registry.
func Open(path, registryPath string) (*Store, error) {
	db, err := database.OpenConfigured(path, registryPath, database.StoreCorpusPDF)
	if err != nil {
		return nil, fmt.Errorf("open PDF store: %w", err)
	}
	return New(db), nil
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
	row, err := s.queries.GetPDFDocument(ctx, work.NormalizeDOI(doi))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Document{
		DOI:           row.Doi,
		Status:        row.Status,
		ContentHash:   row.ContentHash.String,
		InventoriedAt: row.InventoriedAt.String,
		UpdatedAt:     row.UpdatedAt,
		ChangeCounter: row.ChangeCounter,
	}, nil
}

// AvailablePDFDOIs returns normalized DOIs with available companion content in
// DOI order.
func (s *Store) AvailablePDFDOIs(ctx context.Context) ([]string, error) {
	rows, err := s.queries.ListAvailablePDFDOIs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list available PDF DOIs: %w", err)
	}
	items := make([]string, 0, len(rows))
	items = append(items, rows...)
	return items, nil
}

// InventoryForDOIs returns the available inventory timestamp for every
// supplied DOI that has stored content, ordered by DOI. An empty request
// returns no rows without querying.
func (s *Store) InventoryForDOIs(ctx context.Context, dois []string) ([]*InventoryEntry, error) {
	if len(dois) == 0 {
		return nil, nil
	}
	rows, err := s.queries.ListAvailableDocumentInventory(ctx, dois)
	if err != nil {
		return nil, fmt.Errorf("list available document inventory: %w", err)
	}
	items := make([]*InventoryEntry, 0, len(rows))
	for _, row := range rows {
		item := &InventoryEntry{DOI: row.Doi}
		if row.InventoriedAt.Valid {
			inventoriedAt := row.InventoriedAt.String
			item.InventoriedAt = &inventoriedAt
		}
		items = append(items, item)
	}
	return items, nil
}

// AvailableDocument returns one available document's inventory metadata, or
// nil when the normalized DOI has no available content.
func (s *Store) AvailableDocument(ctx context.Context, doi string) (*AvailableDocument, error) {
	row, err := s.queries.GetAvailablePDFDocument(ctx, work.NormalizeDOI(doi))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get available PDF document: %w", err)
	}
	return &AvailableDocument{
		ContentHash:   row.ContentHash.String,
		InventoriedAt: row.InventoriedAt.String,
		ByteSize:      row.ByteSize,
	}, nil
}

// BlobData returns stored bytes for one content hash, or nil when the blob is
// absent.
func (s *Store) BlobData(ctx context.Context, contentHash string) ([]byte, error) {
	data, err := s.queries.GetPDFBlobData(ctx, contentHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get PDF blob data: %w", err)
	}
	return data, nil
}

// Register creates the not-available inventory row for one normalized work.
// Re-registering a DOI preserves its current state and emits no duplicate
// audit event.
func (s *Store) Register(ctx context.Context, doi string, workID, pipelineRunID int64) (bool, error) {
	doi = work.NormalizeDOI(doi)
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
	queries := s.queries.WithTx(tx)

	now := timestamp(s.now())
	result, err := queries.InsertPDFDocument(ctx, generated.InsertPDFDocumentParams{
		Doi: doi, UpdatedAt: now,
	})
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
	afterJSON, err := marshalEvidence(documentEvidence{Status: StatusNotAvailable, ChangeCounter: 1})
	if err != nil {
		return false, err
	}
	if err := insertOutbox(ctx, queries, OutboxEvent{
		Actor: "pipeline", PipelineRunID: pipelineRunID,
		EntityType: "work", EntityID: strconv.FormatInt(workID, 10),
		Action: string(manifest.AuditPDFInventoryRegistered), AfterJSON: afterJSON,
		MetadataJSON: string(metadata), CorrelationID: correlationID,
	}, now); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit PDF inventory registration: %w", err)
	}
	return true, nil
}
