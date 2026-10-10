// hierarchy.go provides the viewer's Home discovery reads over the generated
// statements. It owns the bounded legacy search projection and the workspace
// totals shown before a search is selected.
package search

import (
	"context"
	"fmt"

	generated "analysis/database/search/internal/sql"
)

// HierarchyTotals counts the planned and completed workspace identities shown
// by the viewer's Home summary.
type HierarchyTotals struct {
	Searches      int64
	Revisions     int64
	Plans         int64
	Runs          int64
	CompletedRuns int64
}

// LegacySearchRow is one row of the deprecated bounded search discovery
// projection. Revision fields stay zero when a search has no revision.
type LegacySearchRow struct {
	ID                   int64
	SearchID             string
	CreatedAt            string
	RevisionID           *int64
	RevisionLabel        string
	ConfigArtifactHash   string
	ResolvedManifestHash string
	RevisionCreatedAt    string
}

// HierarchyTotals returns the workspace identity totals for the Home summary.
func (s *Store) HierarchyTotals(ctx context.Context) (*HierarchyTotals, error) {
	row, err := s.queries.GetHierarchyTotals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get hierarchy totals: %w", err)
	}
	return &HierarchyTotals{
		Searches:      row.Searches,
		Revisions:     row.Revisions,
		Plans:         row.Plans,
		Runs:          row.Runs,
		CompletedRuns: row.CompletedRuns,
	}, nil
}

// ListLegacySearches returns up to limit searches with up to limit newest
// revisions each, ordered by search ID and revision ID descending.
func (s *Store) ListLegacySearches(ctx context.Context, limit int) ([]*LegacySearchRow, error) {
	rows, err := s.queries.ListLegacySearches(ctx, generated.ListLegacySearchesParams{
		RevisionLimit: int64(limit),
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list legacy searches: %w", err)
	}
	items := make([]*LegacySearchRow, 0, len(rows))
	for _, row := range rows {
		item := &LegacySearchRow{
			ID:                   row.ID,
			SearchID:             row.SearchID,
			CreatedAt:            row.CreatedAt,
			RevisionLabel:        row.RevisionLabel.String,
			ConfigArtifactHash:   row.ConfigArtifactHash.String,
			ResolvedManifestHash: row.ResolvedManifestHash.String,
			RevisionCreatedAt:    row.RevisionCreatedAt.String,
		}
		if row.RevisionID.Valid {
			revisionID := row.RevisionID.Int64
			item.RevisionID = &revisionID
		}
		items = append(items, item)
	}
	return items, nil
}

// ListLegacyPlans returns up to limit execution plans for one revision in
// descending ID order.
func (s *Store) ListLegacyPlans(ctx context.Context, searchRevisionID int64, limit int) ([]*Plan, error) {
	rows, err := s.queries.ListLegacyPlans(ctx, generated.ListLegacyPlansParams{
		SearchRevisionID: searchRevisionID,
		Limit:            int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list legacy execution plans: %w", err)
	}
	plans := make([]*Plan, 0, len(rows))
	for _, row := range rows {
		plans = append(plans, planFromGenerated(row))
	}
	return plans, nil
}
