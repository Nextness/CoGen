// cache.go provides the handwritten cache-entry data access layer over the
// generated queries in this family's private internal/sql package.
package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	generated "analysis/database/cache/internal/sql"
)

// Store binds the generated cache queries to one already configured connection.
type Store struct {
	queries *generated.Queries
}

// New returns a cache family store over an already configured connection. It
// only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations.
func New(db *sql.DB) *Store {
	return &Store{queries: generated.New(db)}
}

// AppendEntry validates and appends one immutable response version; existing
// run uses retain the exact response they observed.
func (s *Store) AppendEntry(ctx context.Context, entry *Entry) (int64, error) {
	if err := validateEntry(entry); err != nil {
		return 0, err
	}
	result, err := s.queries.InsertCacheEntry(ctx, generated.InsertCacheEntryParams{
		Provider:           entry.Provider,
		Namespace:          entry.Namespace,
		RequestFingerprint: entry.RequestFingerprint,
		ResponseStatus:     int64(entry.ResponseStatus),
		PayloadArtifactID:  nullableInt64(entry.PayloadArtifactID),
		FetchedAt:          entry.FetchedAt,
		ExpiresAt:          nullableString(entry.ExpiresAt),
		ExtractorVersion:   entry.ExtractorVersion,
	})
	if err != nil {
		return 0, fmt.Errorf("upsert cache entry: %w", err)
	}
	return result.LastInsertId()
}

// LatestEntry returns the latest response for an exact request and extractor,
// regardless of expiry. Policy execution decides whether an expired entry is
// stale or may be reused.
func (s *Store) LatestEntry(ctx context.Context, key Key) (*Entry, error) {
	row, err := s.queries.LatestCacheEntry(ctx, generated.LatestCacheEntryParams{
		Provider:           key.Provider,
		Namespace:          key.Namespace,
		RequestFingerprint: key.RequestFingerprint,
		ExtractorVersion:   key.ExtractorVersion,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cache entry: %w", err)
	}
	return entryFromGenerated(row), nil
}

// GlobalEntry returns an entry only after a run explicitly published it to the
// global layer. Entries written only to active_run remain private to that run.
func (s *Store) GlobalEntry(ctx context.Context, key Key) (*Entry, error) {
	row, err := s.queries.GlobalCacheEntry(ctx, generated.GlobalCacheEntryParams{
		Provider:           key.Provider,
		Namespace:          key.Namespace,
		RequestFingerprint: key.RequestFingerprint,
		ExtractorVersion:   key.ExtractorVersion,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cache entry: %w", err)
	}
	return entryFromGenerated(row), nil
}

// validateEntry enforces provider, namespace, fingerprint, status, and payload
// invariants before persistence. Fields are checked in a fixed order so the
// reported missing field is deterministic.
func validateEntry(entry *Entry) error {
	if entry == nil {
		return fmt.Errorf("cache entry is required")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"provider", entry.Provider},
		{"namespace", entry.Namespace},
		{"request fingerprint", entry.RequestFingerprint},
		{"extractor version", entry.ExtractorVersion},
		{"fetched at", entry.FetchedAt},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("cache entry %s is required", field.name)
		}
	}
	if entry.ResponseStatus < 100 || entry.ResponseStatus > 599 {
		return fmt.Errorf("cache entry response status %d is invalid", entry.ResponseStatus)
	}
	return nil
}

// entryFromGenerated maps one generated cache row into an application entry.
func entryFromGenerated(row generated.CacheEntry) *Entry {
	entry := &Entry{
		ID:                 row.ID,
		Provider:           row.Provider,
		Namespace:          row.Namespace,
		RequestFingerprint: row.RequestFingerprint,
		ResponseStatus:     int(row.ResponseStatus),
		FetchedAt:          row.FetchedAt,
		ExtractorVersion:   row.ExtractorVersion,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
	if row.PayloadArtifactID.Valid {
		entry.PayloadArtifactID = &row.PayloadArtifactID.Int64
	}
	if row.ExpiresAt.Valid {
		entry.ExpiresAt = row.ExpiresAt.String
	}
	return entry
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
