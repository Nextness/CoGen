// context.go provides review context initialization, parent candidate
// selection, and the immutable lineage checks that freeze matching parent
// heads without copying version bodies.
package review

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	generated "analysis/database/review/internal/sql"
	"analysis/manifest"
)

// GetContextByRun returns the one initialized review context for a run, if present.
func (s *Store) GetContextByRun(ctx context.Context, runID int64) (*Context, error) {
	return getReviewContext(ctx, s.queries, runID)
}

// getReviewContext reads the optional immutable context associated with one run.
func getReviewContext(ctx context.Context, queries *generated.Queries, runID int64) (*Context, error) {
	row, err := queries.GetReviewContextByRun(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get review context: %w", err)
	}
	return &Context{
		ID:              row.ID,
		PipelineRunID:   row.PipelineRunID,
		ParentContextID: nullInt64Pointer(row.ParentContextID),
		CreatedAt:       row.CreatedAt,
	}, nil
}

// ProposeParent selects the latest initialized context from the same plan, then the same search.
func (s *Store) ProposeParent(ctx context.Context, runID int64) (*ContextCandidate, error) {
	target, err := s.reviewTarget(ctx, s.queries, runID)
	if err != nil {
		return nil, err
	}
	if target.Status != string(manifest.AttemptCompleted) || target.Visibility == string(manifest.RunTrashed) {
		return nil, reviewLifecycle("run is not reviewable")
	}
	for _, samePlan := range []bool{true, false} {
		candidate, err := s.firstParentCandidate(ctx, s.queries, runID, target, samePlan)
		if err != nil {
			return nil, err
		}
		if candidate != nil {
			return candidate, nil
		}
	}
	return nil, nil
}

// reviewTargetRecord holds lineage fields required to validate or compare a target run.
type reviewTargetRecord struct {
	RunID, PlanID, SearchDBID     int64
	StartedAt, Status, Visibility string
}

// reviewTarget loads one planned run and its stable search lineage.
func (s *Store) reviewTarget(ctx context.Context, queries *generated.Queries, runID int64) (reviewTargetRecord, error) {
	row, err := queries.GetReviewTarget(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return reviewTargetRecord{}, reviewNotFound("pipeline run not found or has no execution plan")
	}
	if err != nil {
		return reviewTargetRecord{}, fmt.Errorf("load review target: %w", err)
	}
	return reviewTargetRecord{
		RunID:      row.RunID,
		PlanID:     row.ExecutionPlanID,
		SearchDBID: row.SearchDbID,
		StartedAt:  row.StartedAt,
		Status:     row.Status,
		Visibility: row.VisibilityState,
	}, nil
}

// firstParentCandidate returns the newest eligible same-plan or same-search context.
func (s *Store) firstParentCandidate(ctx context.Context, queries *generated.Queries, runID int64, target reviewTargetRecord, samePlan bool) (*ContextCandidate, error) {
	if samePlan {
		row, err := queries.GetReviewParentCandidateByPlan(ctx, generated.GetReviewParentCandidateByPlanParams{
			TargetRunID:     runID,
			PlanID:          target.PlanID,
			TargetStartedAt: target.StartedAt,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("propose review parent: %w", err)
		}
		return contextCandidateFromPlanRow(row), nil
	}
	row, err := queries.GetReviewParentCandidateBySearch(ctx, generated.GetReviewParentCandidateBySearchParams{
		TargetRunID:     runID,
		SearchDbID:      target.SearchDBID,
		TargetStartedAt: target.StartedAt,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("propose review parent: %w", err)
	}
	return contextCandidateFromSearchRow(row), nil
}

// contextCandidateFromPlanRow maps one same-plan generated candidate row.
func contextCandidateFromPlanRow(row generated.GetReviewParentCandidateByPlanRow) *ContextCandidate {
	return &ContextCandidate{
		ContextID:          row.ContextID,
		PipelineRunID:      row.PipelineRunID,
		SearchID:           row.SearchID,
		SearchRevision:     row.RevisionLabel,
		ExecutionPlanID:    row.ExecutionPlanID,
		AttemptNumber:      int(row.AttemptNumber),
		StartedAt:          row.StartedAt,
		InheritedWorkCount: int(row.InheritedWorkCount),
	}
}

// contextCandidateFromSearchRow maps one same-search generated candidate row.
func contextCandidateFromSearchRow(row generated.GetReviewParentCandidateBySearchRow) *ContextCandidate {
	return &ContextCandidate{
		ContextID:          row.ContextID,
		PipelineRunID:      row.PipelineRunID,
		SearchID:           row.SearchID,
		SearchRevision:     row.RevisionLabel,
		ExecutionPlanID:    row.ExecutionPlanID,
		AttemptNumber:      int(row.AttemptNumber),
		StartedAt:          row.StartedAt,
		InheritedWorkCount: int(row.InheritedWorkCount),
	}
}

// ListParentCandidates returns bounded earlier contexts in stable descending run order.
func (s *Store) ListParentCandidates(ctx context.Context, runID int64, scope, cursorStartedAt string, cursorRunID int64, limit int, query string) ([]ContextCandidate, error) {
	if scope != "same_search" && scope != "all" {
		return nil, reviewValidation("candidate scope must be same_search or all")
	}
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("candidate fetch limit must be between 1 and 101")
	}
	target, err := s.reviewTarget(ctx, s.queries, runID)
	if err != nil {
		return nil, err
	}
	params := generated.ListReviewParentCandidatesParams{
		TargetRunID:     runID,
		TargetStartedAt: target.StartedAt,
		SearchDbID:      target.SearchDBID,
		PageLimit:       int64(limit),
	}
	if scope == "all" {
		params.ScopeAll = 1
		params.SearchDbID = 0
	}
	if cursorStartedAt != "" {
		params.HasCursor = 1
		params.CursorStartedAt = cursorStartedAt
		params.CursorRunID = cursorRunID
	}
	if query = strings.TrimSpace(query); query != "" {
		params.HasQuery = 1
		params.QueryPattern = "%" + query + "%"
	}
	rows, err := s.queries.ListReviewParentCandidates(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list review parent candidates: %w", err)
	}
	items := make([]ContextCandidate, 0, len(rows))
	for _, row := range rows {
		items = append(items, ContextCandidate{
			ContextID:          row.ContextID,
			PipelineRunID:      row.PipelineRunID,
			SearchID:           row.SearchID,
			SearchRevision:     row.RevisionLabel,
			ExecutionPlanID:    row.ExecutionPlanID,
			AttemptNumber:      int(row.AttemptNumber),
			StartedAt:          row.StartedAt,
			InheritedWorkCount: int(row.InheritedWorkCount),
		})
	}
	return items, nil
}

// CreateContext initializes one run context and freezes matching parent heads without copying version bodies.
func (s *Store) CreateContext(ctx context.Context, runID int64, parentContextID *int64) (*Context, bool, error) {
	var created *Context
	newlyCreated := false
	err := s.withTx(ctx, func(queries *generated.Queries) error {
		existing, err := getReviewContext(ctx, queries, runID)
		if err != nil {
			return err
		}
		if existing != nil {
			if !sameNullableID(existing.ParentContextID, parentContextID) {
				return &ContextParentConflictError{Requested: parentContextID, Existing: existing.ParentContextID}
			}
			created = existing
			return nil
		}
		target, err := s.reviewTarget(ctx, queries, runID)
		if err != nil {
			return err
		}
		if target.Status != string(manifest.AttemptCompleted) || target.Visibility == string(manifest.RunTrashed) {
			return reviewLifecycle("run must be completed and non-trashed")
		}
		if parentContextID != nil {
			parent, err := queries.GetReviewParentContextRun(ctx, *parentContextID)
			if errors.Is(err, sql.ErrNoRows) {
				return reviewNotFound("parent review context not found")
			}
			if err != nil {
				return err
			}
			if parent.Status != string(manifest.AttemptCompleted) || parent.VisibilityState == string(manifest.RunTrashed) {
				return reviewLifecycle("parent review context is not eligible")
			}
			if parent.StartedAt > target.StartedAt || (parent.StartedAt == target.StartedAt && parent.RunID >= runID) {
				return reviewValidation("parent review context must belong to an earlier run")
			}
		}
		createdAt := timestamp()
		result, err := queries.InsertReviewContext(ctx, generated.InsertReviewContextParams{
			PipelineRunID:   runID,
			ParentContextID: optionalInt64(parentContextID),
			CreatedAt:       createdAt,
		})
		if err != nil {
			return fmt.Errorf("insert review context: %w", err)
		}
		contextID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		if err := queries.InsertReviewContextWorkHeads(ctx, generated.InsertReviewContextWorkHeadsParams{
			ReviewContextID: contextID,
			ParentContextID: optionalInt64(parentContextID),
			PipelineRunID:   runID,
		}); err != nil {
			return fmt.Errorf("initialize review work heads: %w", err)
		}
		if parentContextID != nil {
			if err := queries.InsertReviewContextNoteHeads(ctx, generated.InsertReviewContextNoteHeadsParams{
				ReviewContextID: contextID,
				ParentContextID: *parentContextID,
			}); err != nil {
				return fmt.Errorf("initialize review note heads: %w", err)
			}
			if err := queries.InsertReviewContextAnchorHeads(ctx, generated.InsertReviewContextAnchorHeadsParams{
				ReviewContextID: contextID,
				ParentContextID: *parentContextID,
			}); err != nil {
				return fmt.Errorf("initialize review anchor heads: %w", err)
			}
		}
		metadata := map[string]any{"review_context_id": contextID, "pipeline_run_id": runID, "parent_context_id": parentContextID}
		if err := insertReviewAudit(ctx, queries, runID, "review_context", strconv.FormatInt(contextID, 10), manifest.AuditReviewContextCreated, metadata); err != nil {
			return err
		}
		created = &Context{ID: contextID, PipelineRunID: runID, ParentContextID: parentContextID, CreatedAt: createdAt}
		newlyCreated = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return created, newlyCreated, nil
}
