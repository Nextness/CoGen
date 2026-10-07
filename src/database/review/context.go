// context.go provides review context initialization, parent candidate
// selection, and the immutable lineage checks that freeze matching parent
// heads without copying version bodies.
package review

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"analysis/manifest"
)

// GetContextByRun returns the one initialized review context for a run, if present.
func (s *Store) GetContextByRun(ctx context.Context, runID int64) (*Context, error) {
	return getReviewContext(ctx, s.db, runID)
}

// getReviewContext reads the optional immutable context associated with one run.
func getReviewContext(ctx context.Context, q queryRower, runID int64) (*Context, error) {
	var item Context
	var parent sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT id, pipeline_run_id, parent_context_id, created_at
		FROM review_contexts WHERE pipeline_run_id=?`, runID).Scan(&item.ID, &item.PipelineRunID, &parent, &item.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get review context: %w", err)
	}
	if parent.Valid {
		item.ParentContextID = &parent.Int64
	}
	return &item, nil
}

// ProposeParent selects the latest initialized context from the same plan, then the same search.
func (s *Store) ProposeParent(ctx context.Context, runID int64) (*ContextCandidate, error) {
	target, err := s.reviewTarget(ctx, s.db, runID)
	if err != nil {
		return nil, err
	}
	if target.Status != string(manifest.AttemptCompleted) || target.Visibility == string(manifest.RunTrashed) {
		return nil, reviewLifecycle("run is not reviewable")
	}
	for _, samePlan := range []bool{true, false} {
		candidate, err := s.firstParentCandidate(ctx, runID, target, samePlan)
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
func (s *Store) reviewTarget(ctx context.Context, q queryRower, runID int64) (reviewTargetRecord, error) {
	var target reviewTargetRecord
	err := q.QueryRowContext(ctx, `SELECT pr.id, pr.execution_plan_id, s.id, pr.started_at, pr.status, pr.visibility_state
		FROM pipeline_runs pr
		JOIN execution_plans ep ON ep.id=pr.execution_plan_id
		JOIN search_revisions sr ON sr.id=ep.search_revision_id
		JOIN searches s ON s.id=sr.search_id
		WHERE pr.id=?`, runID).Scan(&target.RunID, &target.PlanID, &target.SearchDBID, &target.StartedAt, &target.Status, &target.Visibility)
	if err == sql.ErrNoRows {
		return target, reviewNotFound("pipeline run not found or has no execution plan")
	}
	if err != nil {
		return target, fmt.Errorf("load review target: %w", err)
	}
	return target, nil
}

// firstParentCandidate returns the newest eligible same-plan or same-search context.
func (s *Store) firstParentCandidate(ctx context.Context, runID int64, target reviewTargetRecord, samePlan bool) (*ContextCandidate, error) {
	clause, value := "s.id=?", target.SearchDBID
	if samePlan {
		clause, value = "ep.id=?", target.PlanID
	}
	row := s.db.QueryRowContext(ctx, `SELECT rc.id, pr.id, s.search_id, sr.revision_label, ep.id,
		COALESCE(pr.attempt_number, 0), pr.started_at,
		(SELECT COUNT(*) FROM review_context_work_heads parent_head
		 WHERE parent_head.review_context_id=rc.id AND EXISTS (
		   SELECT 1 FROM work_revisions target_wr
		   WHERE target_wr.pipeline_run_id=? AND `+normalizedRevisionPredicate("target_wr")+`
		     AND target_wr.work_id=parent_head.work_id))
		FROM review_contexts rc
		JOIN pipeline_runs pr ON pr.id=rc.pipeline_run_id
		JOIN execution_plans ep ON ep.id=pr.execution_plan_id
		JOIN search_revisions sr ON sr.id=ep.search_revision_id
		JOIN searches s ON s.id=sr.search_id
		WHERE `+clause+` AND pr.status='completed' AND pr.visibility_state!='trashed'
		AND (pr.started_at < ? OR (pr.started_at=? AND pr.id < ?))
		ORDER BY pr.started_at DESC, pr.id DESC LIMIT 1`, runID, value, target.StartedAt, target.StartedAt, runID)
	var item ContextCandidate
	err := row.Scan(&item.ContextID, &item.PipelineRunID, &item.SearchID, &item.SearchRevision,
		&item.ExecutionPlanID, &item.AttemptNumber, &item.StartedAt, &item.InheritedWorkCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("propose review parent: %w", err)
	}
	return &item, nil
}

// ListParentCandidates returns bounded earlier contexts in stable descending run order.
func (s *Store) ListParentCandidates(ctx context.Context, runID int64, scope, cursorStartedAt string, cursorRunID int64, limit int, query string) ([]ContextCandidate, error) {
	if scope != "same_search" && scope != "all" {
		return nil, reviewValidation("candidate scope must be same_search or all")
	}
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("candidate fetch limit must be between 1 and 101")
	}
	target, err := s.reviewTarget(ctx, s.db, runID)
	if err != nil {
		return nil, err
	}
	clauses := []string{"pr.status='completed'", "pr.visibility_state!='trashed'", "(pr.started_at < ? OR (pr.started_at=? AND pr.id < ?))"}
	args := []any{runID, target.StartedAt, target.StartedAt, runID}
	if scope == "same_search" {
		clauses = append(clauses, "s.id=?")
		args = append(args, target.SearchDBID)
	}
	if cursorStartedAt != "" {
		clauses = append(clauses, "(pr.started_at < ? OR (pr.started_at=? AND pr.id < ?))")
		args = append(args, cursorStartedAt, cursorStartedAt, cursorRunID)
	}
	if query = strings.TrimSpace(query); query != "" {
		clauses = append(clauses, "(s.search_id LIKE ? OR sr.revision_label LIKE ?)")
		like := "%" + query + "%"
		args = append(args, like, like)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `SELECT rc.id, pr.id, s.search_id, sr.revision_label, ep.id,
		COALESCE(pr.attempt_number, 0), pr.started_at,
		(SELECT COUNT(*) FROM review_context_work_heads parent_head
		 WHERE parent_head.review_context_id=rc.id AND EXISTS (
		   SELECT 1 FROM work_revisions target_wr
		   WHERE target_wr.pipeline_run_id=? AND `+normalizedRevisionPredicate("target_wr")+`
		     AND target_wr.work_id=parent_head.work_id))
		FROM review_contexts rc
		JOIN pipeline_runs pr ON pr.id=rc.pipeline_run_id
		JOIN execution_plans ep ON ep.id=pr.execution_plan_id
		JOIN search_revisions sr ON sr.id=ep.search_revision_id
		JOIN searches s ON s.id=sr.search_id
		WHERE `+strings.Join(clauses, " AND ")+`
		ORDER BY pr.started_at DESC, pr.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list review parent candidates: %w", err)
	}
	defer rows.Close()
	items := make([]ContextCandidate, 0)
	for rows.Next() {
		var item ContextCandidate
		if err := rows.Scan(&item.ContextID, &item.PipelineRunID, &item.SearchID, &item.SearchRevision,
			&item.ExecutionPlanID, &item.AttemptNumber, &item.StartedAt, &item.InheritedWorkCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateContext initializes one run context and freezes matching parent heads without copying version bodies.
func (s *Store) CreateContext(ctx context.Context, runID int64, parentContextID *int64) (*Context, bool, error) {
	var created *Context
	newlyCreated := false
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		existing, err := getReviewContext(ctx, tx, runID)
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
		target, err := s.reviewTarget(ctx, tx, runID)
		if err != nil {
			return err
		}
		if target.Status != string(manifest.AttemptCompleted) || target.Visibility == string(manifest.RunTrashed) {
			return reviewLifecycle("run must be completed and non-trashed")
		}
		if parentContextID != nil {
			var parentRunID int64
			var startedAt, status, visibility string
			err := tx.QueryRowContext(ctx, `SELECT pr.id, pr.started_at, pr.status, pr.visibility_state
				FROM review_contexts rc JOIN pipeline_runs pr ON pr.id=rc.pipeline_run_id WHERE rc.id=?`, *parentContextID).
				Scan(&parentRunID, &startedAt, &status, &visibility)
			if err == sql.ErrNoRows {
				return reviewNotFound("parent review context not found")
			}
			if err != nil {
				return err
			}
			if status != string(manifest.AttemptCompleted) || visibility == string(manifest.RunTrashed) {
				return reviewLifecycle("parent review context is not eligible")
			}
			if startedAt > target.StartedAt || (startedAt == target.StartedAt && parentRunID >= runID) {
				return reviewValidation("parent review context must belong to an earlier run")
			}
		}
		createdAt := timestamp()
		result, err := tx.ExecContext(ctx, `INSERT INTO review_contexts
			(pipeline_run_id, parent_context_id, created_at) VALUES (?, ?, ?)`, runID, nullablePointer(parentContextID), createdAt)
		if err != nil {
			return fmt.Errorf("insert review context: %w", err)
		}
		contextID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO review_context_work_heads
			(review_context_id, work_id, work_revision_id, review_version_id)
			SELECT ?, latest.work_id, latest.id, parent.review_version_id
			FROM work_revisions latest
			LEFT JOIN review_context_work_heads parent
			  ON parent.review_context_id=? AND parent.work_id=latest.work_id
			WHERE latest.pipeline_run_id=? AND `+normalizedRevisionPredicate("latest"), contextID, nullablePointer(parentContextID), runID)
		if err != nil {
			return fmt.Errorf("initialize review work heads: %w", err)
		}
		if parentContextID != nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO review_context_note_heads
				(review_context_id, note_id, note_version_id)
				SELECT ?, parent.note_id, parent.note_version_id
				FROM review_context_note_heads parent
				JOIN review_notes note ON note.id=parent.note_id
				JOIN review_context_work_heads target ON target.review_context_id=? AND target.work_id=note.work_id
				WHERE parent.review_context_id=?`, contextID, contextID, *parentContextID); err != nil {
				return fmt.Errorf("initialize review note heads: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO review_context_anchor_heads
				(review_context_id, anchor_id, anchor_version_id)
				SELECT ?, parent.anchor_id, parent.anchor_version_id
				FROM review_context_anchor_heads parent
				JOIN review_anchors anchor ON anchor.id=parent.anchor_id
				JOIN review_context_work_heads target ON target.review_context_id=? AND target.work_id=anchor.work_id
				WHERE parent.review_context_id=?`, contextID, contextID, *parentContextID); err != nil {
				return fmt.Errorf("initialize review anchor heads: %w", err)
			}
		}
		metadata := map[string]any{"review_context_id": contextID, "pipeline_run_id": runID, "parent_context_id": parentContextID}
		if err := insertReviewAudit(ctx, tx, runID, "review_context", strconv.FormatInt(contextID, 10), manifest.AuditReviewContextCreated, metadata); err != nil {
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
