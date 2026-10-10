// plans.go provides the handwritten execution-plan data access layer over the
// generated queries in this family's private internal/sql package.
package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/search/internal/sql"
)

// FindOrCreatePlan inserts a new execution plan and returns the plan ID. An
// identical plan (same search revision and execution fingerprint) with the
// same manifest and input policy returns the existing ID. A different
// manifest or input for a reserved fingerprint returns an error because the
// fingerprint is reserved for a different resolved configuration.
func (s *Store) FindOrCreatePlan(ctx context.Context, input PlanInput) (int64, error) {
	return s.findOrCreatePlan(ctx, input)
}

// FindOrCreatePlanWithInputManifest inserts or reuses a plan linked to the
// frozen input manifest used to calculate its execution fingerprint. The
// input manifest hash is required.
func (s *Store) FindOrCreatePlanWithInputManifest(ctx context.Context, input PlanInput) (int64, error) {
	if input.InputManifestHash == "" {
		return 0, fmt.Errorf("create execution plan: input manifest hash is required")
	}
	return s.findOrCreatePlan(ctx, input)
}

// findOrCreatePlan inserts or reuses an execution plan with the supplied
// manifest and enrichment policy. The input policy is only compared when the
// caller supplied an input manifest hash.
func (s *Store) findOrCreatePlan(ctx context.Context, input PlanInput) (int64, error) {
	result, err := s.queries.InsertPlanIgnore(ctx, generated.InsertPlanIgnoreParams{
		SearchRevisionID:     input.SearchRevisionID,
		ExecutionFingerprint: input.ExecutionFingerprint,
		ResolvedManifestHash: input.ResolvedManifestHash,
		InputManifestHash:    input.InputManifestHash,
		EnrichmentEnabled:    boolToInt64(input.EnrichmentEnabled),
	})
	if err != nil {
		return 0, fmt.Errorf("create execution plan: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("create execution plan: read affected rows: %w", err)
	}
	if rowsAffected > 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("create execution plan: read inserted id: %w", err)
		}
		return id, nil
	}

	existing, err := s.GetPlanByFingerprint(ctx, input.SearchRevisionID, input.ExecutionFingerprint)
	if err != nil {
		return 0, err
	}
	if existing == nil {
		return 0, fmt.Errorf("create execution plan: insert skipped but existing row not found")
	}
	if existing.ResolvedManifestHash != input.ResolvedManifestHash ||
		(input.InputManifestHash != "" && existing.InputManifestHash != input.InputManifestHash) ||
		(input.InputManifestHash != "" && existing.EnrichmentEnabled != input.EnrichmentEnabled) {
		return 0, fmt.Errorf("execution fingerprint %q already exists for search revision %d with different manifest or input", input.ExecutionFingerprint, input.SearchRevisionID)
	}
	return existing.ID, nil
}

// GetPlanByID returns an execution plan by its primary key, or nil if not found.
func (s *Store) GetPlanByID(ctx context.Context, id int64) (*Plan, error) {
	row, err := s.queries.GetPlanByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get execution plan: %w", err)
	}
	return planFromGenerated(row), nil
}

// GetPlanByFingerprint returns an execution plan matching the given search
// revision and fingerprint, or nil if not found.
func (s *Store) GetPlanByFingerprint(ctx context.Context, searchRevisionID int64, fingerprint string) (*Plan, error) {
	row, err := s.queries.GetPlanByFingerprint(ctx, generated.GetPlanByFingerprintParams{
		SearchRevisionID:     searchRevisionID,
		ExecutionFingerprint: fingerprint,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get execution plan: %w", err)
	}
	return planFromGenerated(row), nil
}

// ListPlansBySearchRevision returns all execution plans for a given search
// revision, ordered by ID. An absent revision returns a nil slice to preserve
// the current API shape.
func (s *Store) ListPlansBySearchRevision(ctx context.Context, searchRevisionID int64) ([]*Plan, error) {
	rows, err := s.queries.ListPlansBySearchRevision(ctx, searchRevisionID)
	if err != nil {
		return nil, fmt.Errorf("list execution plans: %w", err)
	}
	var plans []*Plan
	for _, row := range rows {
		plans = append(plans, planFromGenerated(row))
	}
	return plans, nil
}

// planFromGenerated maps one generated plan row into an application plan and
// makes the integer enrichment policy explicit as a boolean.
func planFromGenerated(row generated.ExecutionPlan) *Plan {
	return &Plan{
		ID:                   row.ID,
		SearchRevisionID:     row.SearchRevisionID,
		ExecutionFingerprint: row.ExecutionFingerprint,
		ResolvedManifestHash: row.ResolvedManifestHash,
		InputManifestHash:    row.InputManifestHash,
		EnrichmentEnabled:    row.EnrichmentEnabled != 0,
		CreatedAt:            row.CreatedAt,
	}
}

// boolToInt64 maps the application enrichment flag into SQLite integer form.
func boolToInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
