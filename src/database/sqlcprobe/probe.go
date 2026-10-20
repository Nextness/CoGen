// probe.go provides the handwritten wrapper that exercises the pinned sqlc
// generator features against a production-migrated database. It is validation
// scaffolding rather than a production table-family API.
package sqlcprobe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/sqlcprobe/internal/sql"
)

// Record is one probe cache-entry projection mapped from generated rows.
type Record struct {
	ID                 int64
	Provider           string
	Namespace          string
	RequestFingerprint string
	ResponseStatus     int
	PayloadArtifactID  *int64
	FetchedAt          string
	ExpiresAt          string
	ExtractorVersion   string
}

// Entry is one probe cache-entry write with nullable payload and expiry input.
type Entry struct {
	Provider           string
	Namespace          string
	RequestFingerprint string
	ResponseStatus     int
	PayloadArtifactID  *int64
	ExpiresAt          string
	FetchedAt          string
	ExtractorVersion   string
}

// InsertResult reports the identifier and affected-row count of one generated insert.
type InsertResult struct {
	ID           int64
	RowsAffected int64
}

// Probe exercises generated query features against one configured connection.
type Probe struct {
	db      *sql.DB
	queries *generated.Queries
}

// New returns a probe over an already configured connection.
func New(db *sql.DB) *Probe {
	return &Probe{db: db, queries: generated.New(db)}
}

// InsertEntry inserts one probe entry and inspects the generated insert result.
func (p *Probe) InsertEntry(ctx context.Context, entry Entry) (InsertResult, error) {
	result, err := p.queries.InsertProbeEntry(ctx, insertParams(entry))
	if err != nil {
		return InsertResult{}, fmt.Errorf("insert probe entry: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return InsertResult{}, fmt.Errorf("probe insert id: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return InsertResult{}, fmt.Errorf("probe insert rows: %w", err)
	}
	return InsertResult{ID: id, RowsAffected: affected}, nil
}

// LatestEntry returns the latest probe entry for an exact provider and namespace.
func (p *Probe) LatestEntry(ctx context.Context, provider, namespace string) (*Record, error) {
	row, err := p.queries.LatestProbeEntry(ctx, generated.LatestProbeEntryParams{Provider: provider, Namespace: namespace})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest probe entry: %w", err)
	}
	record := recordFromGenerated(row)
	return &record, nil
}

// ListEntries returns probe entries for a provider in explicit identifier order.
func (p *Probe) ListEntries(ctx context.Context, provider string) ([]Record, error) {
	rows, err := p.queries.ListProbeEntries(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("list probe entries: %w", err)
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		records = append(records, recordFromGenerated(row))
	}
	return records, nil
}

// InsertPair commits two probe entries in one generated transaction.
func (p *Probe) InsertPair(ctx context.Context, first, second Entry) error {
	transaction, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin probe transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	queries := p.queries.WithTx(transaction)
	if _, err := queries.InsertProbeEntry(ctx, insertParams(first)); err != nil {
		return fmt.Errorf("insert first probe entry: %w", err)
	}
	if _, err := queries.InsertProbeEntry(ctx, insertParams(second)); err != nil {
		return fmt.Errorf("insert second probe entry: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit probe transaction: %w", err)
	}
	return nil
}

// insertParams maps one probe entry into generated insert parameters.
func insertParams(entry Entry) generated.InsertProbeEntryParams {
	return generated.InsertProbeEntryParams{
		Provider:           entry.Provider,
		Namespace:          entry.Namespace,
		RequestFingerprint: entry.RequestFingerprint,
		ResponseStatus:     int64(entry.ResponseStatus),
		PayloadArtifactID:  nullableInt64(entry.PayloadArtifactID),
		FetchedAt:          entry.FetchedAt,
		ExpiresAt:          nullableString(entry.ExpiresAt),
		ExtractorVersion:   entry.ExtractorVersion,
	}
}

// recordFromGenerated maps one generated probe row into an application record.
func recordFromGenerated(row generated.CacheEntry) Record {
	record := Record{
		ID:                 row.ID,
		Provider:           row.Provider,
		Namespace:          row.Namespace,
		RequestFingerprint: row.RequestFingerprint,
		ResponseStatus:     int(row.ResponseStatus),
		FetchedAt:          row.FetchedAt,
		ExtractorVersion:   row.ExtractorVersion,
	}
	if row.PayloadArtifactID.Valid {
		record.PayloadArtifactID = &row.PayloadArtifactID.Int64
	}
	if row.ExpiresAt.Valid {
		record.ExpiresAt = row.ExpiresAt.String
	}
	return record
}

// nullableInt64 maps an optional artifact identifier into a generated null wrapper.
func nullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

// nullableString maps optional text into a generated null wrapper.
func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
