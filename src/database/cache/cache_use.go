// cache_use.go provides the handwritten run-cache-use and replay data access
// layer over the generated queries in this family's private internal/sql
// package.
package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	generated "analysis/database/cache/internal/sql"
	"analysis/manifest"
)

// AppendUse validates and inserts one run-scoped cache-use evidence record.
func (s *Store) AppendUse(ctx context.Context, use *Use) (int64, error) {
	if err := validateUse(use); err != nil {
		return 0, err
	}
	result, err := s.queries.InsertRunCacheUse(ctx, generated.InsertRunCacheUseParams{
		PipelineRunID: use.PipelineRunID,
		CacheEntryID:  use.CacheEntryID,
		CacheLayer:    use.CacheLayer,
		Outcome:       use.Outcome,
	})
	if err != nil {
		return 0, fmt.Errorf("create run cache use: %w", err)
	}
	return result.LastInsertId()
}

// ListUsesByRun returns cache-use evidence for a run in insertion order. An
// absent run returns a non-nil empty slice to preserve the current API shape.
func (s *Store) ListUsesByRun(ctx context.Context, runID int64) ([]*Use, error) {
	rows, err := s.queries.ListRunCacheUsesByRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("list run cache uses: %w", err)
	}
	uses := make([]*Use, 0, len(rows))
	for _, row := range rows {
		uses = append(uses, useFromGenerated(row))
	}
	return uses, nil
}

// FindEntry returns the latest entry with this exact versioned key recorded for
// a run and cache layer. It supports active-run and named-prior-run reads.
func (s *Store) FindEntry(ctx context.Context, runID int64, layer string, key Key) (*Entry, error) {
	row, err := s.queries.FindRunCacheEntry(ctx, generated.FindRunCacheEntryParams{
		PipelineRunID:      runID,
		CacheLayer:         layer,
		Provider:           key.Provider,
		Namespace:          key.Namespace,
		RequestFingerprint: key.RequestFingerprint,
		ExtractorVersion:   key.ExtractorVersion,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find run cache entry: %w", err)
	}
	return entryFromGenerated(row), nil
}

// FindAnyEntry returns the latest exact cache key used by a prior run. Named
// prior-run policy reads use this because their source layer is provenance, not
// a restriction on how the source run originally obtained the response.
func (s *Store) FindAnyEntry(ctx context.Context, runID int64, key Key) (*Entry, error) {
	row, err := s.queries.FindAnyRunCacheEntry(ctx, generated.FindAnyRunCacheEntryParams{
		PipelineRunID:           runID,
		Provider:                key.Provider,
		Namespace:               key.Namespace,
		RequestFingerprint:      key.RequestFingerprint,
		ExtractorVersion:        key.ExtractorVersion,
		ExtractorVersionPattern: key.ExtractorVersion + ":run:%",
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find prior-run cache entry: %w", err)
	}
	return entryFromGenerated(row), nil
}

// validateUse enforces run, entry, layer, and outcome invariants before persistence.
func validateUse(use *Use) error {
	if use == nil || use.PipelineRunID == 0 || use.CacheEntryID == 0 || strings.TrimSpace(use.CacheLayer) == "" {
		return fmt.Errorf("pipeline run, cache entry, and cache layer are required")
	}
	return manifest.ValidateCacheOutcome(use.Outcome)
}

// useFromGenerated maps one generated run-cache-use row into an application use.
func useFromGenerated(row generated.RunCacheUse) *Use {
	return &Use{
		ID:            row.ID,
		PipelineRunID: row.PipelineRunID,
		CacheEntryID:  row.CacheEntryID,
		CacheLayer:    row.CacheLayer,
		Outcome:       row.Outcome,
		UsedAt:        row.UsedAt,
	}
}
