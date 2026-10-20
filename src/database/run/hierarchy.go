// hierarchy.go provides the viewer's bounded run hierarchy reads over the
// generated statements. It owns the filtered attempt and run pages behind the
// Home sections and the deprecated run discovery projection.
package run

import (
	"context"
	"fmt"
	"strings"

	generated "analysis/database/run/internal/sql"
)

// maxHierarchyListLimit bounds one viewer hierarchy page so a caller cannot
// request an unbounded attempt or run listing.
const maxHierarchyListLimit = 1000

// HierarchyAttemptFilter identifies one bounded page of non-trashed attempts
// for a selected execution plan.
type HierarchyAttemptFilter struct {
	ExecutionPlanID int64
	Query           string
	CursorID        int64
	Limit           int
}

// HierarchyRunFilter identifies one bounded page of filtered run attempts with
// their complete search ancestry.
type HierarchyRunFilter struct {
	Query         string
	Visibility    string
	Status        string
	StartedAfter  string
	StartedBefore string
	CursorID      int64
	Limit         int
}

// LegacyRunFilter identifies one bounded page of the deprecated run discovery
// projection.
type LegacyRunFilter struct {
	SearchRevisionID int64
	PlanID           int64
	IncludeTrashed   bool
	Limit            int
}

// HierarchyAttemptRow is one non-trashed attempt summary for a selected plan.
type HierarchyAttemptRow struct {
	ID              int64
	ExecutionPlanID *int64
	AttemptNumber   *int64
	StartedAt       string
	FinishedAt      *string
	Status          string
	VisibilityState string
}

// HierarchyRunRow is one run attempt with its complete search ancestry. Search
// and revision names stay empty when the run has no execution plan.
type HierarchyRunRow struct {
	ID               int64
	AttemptNumber    *int64
	StartedAt        string
	FinishedAt       *string
	Status           string
	VisibilityState  string
	SearchID         *int64
	SearchName       string
	SearchRevisionID *int64
	RevisionLabel    string
	ExecutionPlanID  *int64
}

// LegacyRunRow is one row of the deprecated bounded run discovery projection.
type LegacyRunRow struct {
	ID               int64
	Step             string
	StartedAt        string
	FinishedAt       *string
	Status           string
	Summary          *string
	SearchQuery      *string
	ExecutionPlanID  *int64
	AttemptNumber    *int64
	VisibilityState  string
	TrashedAt        *string
	TrashReason      *string
	SearchRevisionID *int64
}

// ListHierarchyAttempts returns one bounded page of non-trashed attempts for a
// plan in descending ID order together with a lookahead flag.
func (s *Store) ListHierarchyAttempts(ctx context.Context, filter HierarchyAttemptFilter) ([]*HierarchyAttemptRow, bool, error) {
	if err := validateHierarchyLimit("list hierarchy attempts", filter.Limit); err != nil {
		return nil, false, err
	}
	rows, err := s.queries.ListHierarchyAttempts(ctx, generated.ListHierarchyAttemptsParams{
		ExecutionPlanID: nullableInt64(filter.ExecutionPlanID),
		Query:           filter.Query,
		Pattern:         hierarchyPattern(filter.Query),
		Cursor:          filter.CursorID,
		Limit:           int64(filter.Limit + 1),
	})
	if err != nil {
		return nil, false, fmt.Errorf("list hierarchy attempts: %w", err)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	items := make([]*HierarchyAttemptRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, &HierarchyAttemptRow{
			ID:              row.ID,
			ExecutionPlanID: nullableInt64Pointer(row.ExecutionPlanID),
			AttemptNumber:   nullableInt64Pointer(row.AttemptNumber),
			StartedAt:       row.StartedAt,
			FinishedAt:      nullableStringPointer(row.FinishedAt),
			Status:          row.Status,
			VisibilityState: row.VisibilityState,
		})
	}
	return items, hasMore, nil
}

// ListHierarchyRuns returns one bounded filtered page of run attempts with
// complete search ancestry in descending ID order together with a lookahead
// flag.
func (s *Store) ListHierarchyRuns(ctx context.Context, filter HierarchyRunFilter) ([]*HierarchyRunRow, bool, error) {
	if err := validateHierarchyLimit("list hierarchy runs", filter.Limit); err != nil {
		return nil, false, err
	}
	switch filter.Visibility {
	case "active", "trashed", "all":
	default:
		return nil, false, fmt.Errorf("list hierarchy runs: invalid visibility %q", filter.Visibility)
	}
	rows, err := s.queries.ListHierarchyRuns(ctx, generated.ListHierarchyRunsParams{
		Query:         filter.Query,
		Pattern:       hierarchyPattern(filter.Query),
		Visibility:    filter.Visibility,
		Status:        filter.Status,
		StartedAfter:  filter.StartedAfter,
		StartedBefore: filter.StartedBefore,
		Cursor:        filter.CursorID,
		Limit:         int64(filter.Limit + 1),
	})
	if err != nil {
		return nil, false, fmt.Errorf("list hierarchy runs: %w", err)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	items := make([]*HierarchyRunRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, &HierarchyRunRow{
			ID:               row.ID,
			AttemptNumber:    nullableInt64Pointer(row.AttemptNumber),
			StartedAt:        row.StartedAt,
			FinishedAt:       nullableStringPointer(row.FinishedAt),
			Status:           row.Status,
			VisibilityState:  row.VisibilityState,
			SearchID:         nullableInt64Pointer(row.SearchID),
			SearchName:       row.SearchName.String,
			SearchRevisionID: nullableInt64Pointer(row.SearchRevisionID),
			RevisionLabel:    row.RevisionLabel.String,
			ExecutionPlanID:  nullableInt64Pointer(row.ExecutionPlanID),
		})
	}
	return items, hasMore, nil
}

// ListLegacyRuns returns up to limit runs for the deprecated discovery
// projection in descending ID order, optionally including trashed attempts.
func (s *Store) ListLegacyRuns(ctx context.Context, filter LegacyRunFilter) ([]*LegacyRunRow, error) {
	if err := validateHierarchyLimit("list legacy runs", filter.Limit); err != nil {
		return nil, err
	}
	includeTrashed := int64(0)
	if filter.IncludeTrashed {
		includeTrashed = 1
	}
	rows, err := s.queries.ListLegacyRuns(ctx, generated.ListLegacyRunsParams{
		SearchRevisionID: filter.SearchRevisionID,
		PlanID:           filter.PlanID,
		IncludeTrashed:   includeTrashed,
		Limit:            int64(filter.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list legacy runs: %w", err)
	}
	items := make([]*LegacyRunRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, &LegacyRunRow{
			ID:               row.ID,
			Step:             row.Step,
			StartedAt:        row.StartedAt,
			FinishedAt:       nullableStringPointer(row.FinishedAt),
			Status:           row.Status,
			Summary:          nullableStringPointer(row.Summary),
			SearchQuery:      nullableStringPointer(row.SearchQuery),
			ExecutionPlanID:  nullableInt64Pointer(row.ExecutionPlanID),
			AttemptNumber:    nullableInt64Pointer(row.AttemptNumber),
			VisibilityState:  row.VisibilityState,
			TrashedAt:        nullableStringPointer(row.TrashedAt),
			TrashReason:      nullableStringPointer(row.TrashReason),
			SearchRevisionID: nullableInt64Pointer(row.SearchRevisionID),
		})
	}
	return items, nil
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
