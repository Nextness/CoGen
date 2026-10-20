// hierarchy.go provides the viewer's Home discovery reads over the generated
// statements. It owns the bounded legacy search projection, the workspace
// totals shown before a search is selected, and the filtered search, revision,
// and plan pages behind the Home sections.
package search

import (
	"context"
	"fmt"
	"strings"

	generated "analysis/database/search/internal/sql"
)

// maxHierarchyListLimit bounds one viewer hierarchy page so a caller cannot
// request an unbounded search, revision, or plan listing.
const maxHierarchyListLimit = 1000

// HierarchySearchFilter identifies one bounded page of search summaries.
type HierarchySearchFilter struct {
	Query    string
	CursorID int64
	Limit    int
}

// HierarchyRevisionFilter identifies one bounded page of revision summaries
// for a selected search.
type HierarchyRevisionFilter struct {
	SearchID int64
	Query    string
	CursorID int64
	Limit    int
}

// HierarchyPlanFilter identifies one bounded page of execution plans for a
// selected search revision.
type HierarchyPlanFilter struct {
	SearchRevisionID int64
	Query            string
	CursorID         int64
	Limit            int
}

// HierarchySearchRow is one search summary with its revision, plan, and run
// counts and newest planned identifiers.
type HierarchySearchRow struct {
	ID               int64
	SearchID         string
	CreatedAt        string
	RevisionCount    int64
	PlanCount        int64
	RunCount         int64
	LatestRunID      *int64
	LatestPlanID     *int64
	LatestRevisionID *int64
}

// HierarchyRevisionRow is one revision summary with its plan and run counts and
// newest planned identifiers.
type HierarchyRevisionRow struct {
	ID           int64
	Label        string
	CreatedAt    string
	PlanCount    int64
	RunCount     int64
	LatestRunID  *int64
	LatestPlanID *int64
}

// HierarchyPlanRow is one execution plan summary.
type HierarchyPlanRow struct {
	ID                   int64
	SearchRevisionID     int64
	ExecutionFingerprint string
	EnrichmentEnabled    bool
	CreatedAt            string
}

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

// ListHierarchySearches returns one bounded page of search summaries in
// descending ID order together with a lookahead flag. The query filter matches
// either the search identifier or one of its revision labels.
func (s *Store) ListHierarchySearches(ctx context.Context, filter HierarchySearchFilter) ([]*HierarchySearchRow, bool, error) {
	if err := validateHierarchyLimit("list hierarchy searches", filter.Limit); err != nil {
		return nil, false, err
	}
	rows, err := s.queries.ListHierarchySearches(ctx, generated.ListHierarchySearchesParams{
		Query:   filter.Query,
		Pattern: hierarchyPattern(filter.Query),
		Cursor:  filter.CursorID,
		Limit:   int64(filter.Limit + 1),
	})
	if err != nil {
		return nil, false, fmt.Errorf("list hierarchy searches: %w", err)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	items := make([]*HierarchySearchRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, &HierarchySearchRow{
			ID:               row.ID,
			SearchID:         row.SearchID,
			CreatedAt:        row.CreatedAt,
			RevisionCount:    row.RevisionCount,
			PlanCount:        row.PlanCount,
			RunCount:         row.RunCount,
			LatestRunID:      nullableIdentifier(row.LatestRunID),
			LatestPlanID:     nullableIdentifier(row.LatestPlanID),
			LatestRevisionID: nullableIdentifier(row.LatestRevisionID),
		})
	}
	return items, hasMore, nil
}

// ListHierarchyRevisions returns one bounded page of revision summaries for a
// selected search in descending ID order together with a lookahead flag.
func (s *Store) ListHierarchyRevisions(ctx context.Context, filter HierarchyRevisionFilter) ([]*HierarchyRevisionRow, bool, error) {
	if err := validateHierarchyLimit("list hierarchy revisions", filter.Limit); err != nil {
		return nil, false, err
	}
	rows, err := s.queries.ListHierarchyRevisions(ctx, generated.ListHierarchyRevisionsParams{
		SearchID: filter.SearchID,
		Query:    filter.Query,
		Pattern:  hierarchyPattern(filter.Query),
		Cursor:   filter.CursorID,
		Limit:    int64(filter.Limit + 1),
	})
	if err != nil {
		return nil, false, fmt.Errorf("list hierarchy revisions: %w", err)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	items := make([]*HierarchyRevisionRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, &HierarchyRevisionRow{
			ID:           row.ID,
			Label:        row.RevisionLabel,
			CreatedAt:    row.CreatedAt,
			PlanCount:    row.PlanCount,
			RunCount:     row.RunCount,
			LatestRunID:  nullableIdentifier(row.LatestRunID),
			LatestPlanID: nullableIdentifier(row.LatestPlanID),
		})
	}
	return items, hasMore, nil
}

// ListHierarchyPlans returns one bounded page of execution plans for a selected
// revision in descending ID order together with a lookahead flag.
func (s *Store) ListHierarchyPlans(ctx context.Context, filter HierarchyPlanFilter) ([]*HierarchyPlanRow, bool, error) {
	if err := validateHierarchyLimit("list hierarchy plans", filter.Limit); err != nil {
		return nil, false, err
	}
	rows, err := s.queries.ListHierarchyPlans(ctx, generated.ListHierarchyPlansParams{
		SearchRevisionID: filter.SearchRevisionID,
		Query:            filter.Query,
		Pattern:          hierarchyPattern(filter.Query),
		Cursor:           filter.CursorID,
		Limit:            int64(filter.Limit + 1),
	})
	if err != nil {
		return nil, false, fmt.Errorf("list hierarchy plans: %w", err)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	items := make([]*HierarchyPlanRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, &HierarchyPlanRow{
			ID:                   row.ID,
			SearchRevisionID:     row.SearchRevisionID,
			ExecutionFingerprint: row.ExecutionFingerprint,
			EnrichmentEnabled:    row.EnrichmentEnabled != 0,
			CreatedAt:            row.CreatedAt,
		})
	}
	return items, hasMore, nil
}

// validateHierarchyLimit bounds one viewer hierarchy page request.
func validateHierarchyLimit(operation string, limit int) error {
	if limit < 1 || limit > maxHierarchyListLimit {
		return fmt.Errorf("%s: limit must be between 1 and %d", operation, maxHierarchyListLimit)
	}
	return nil
}

// hierarchyPattern returns the lowercase LIKE pattern for one optional filter.
func hierarchyPattern(query string) string {
	return "%" + strings.ToLower(query) + "%"
}

// nullableIdentifier converts one generated nullable aggregate value into an
// optional application identifier. SQLite aggregates are weakly typed, so the
// generated value arrives as an empty interface.
func nullableIdentifier(value any) *int64 {
	switch typed := value.(type) {
	case int64:
		return &typed
	case int:
		converted := int64(typed)
		return &converted
	default:
		return nil
	}
}
