package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"analysis/manifest"
	"analysis/notes"
)

var (
	validReviewStatuses    = map[string]bool{"not_evaluated": true, "in_progress": true, "approved": true, "not_approved": true, "removed": true}
	validReviewSubstatuses = map[string]bool{
		"redacted": true, "unrelated": true, "out_of_scope": true, "duplicate": true,
		"retracted": true, "withdrawn": true, "superseded": true, "predatory_low_quality": true,
		"copyright_licensing": true, "not_peer_reviewed": true,
	}
)

const (
	reviewListLimit        = 101
	reviewTextPreviewBytes = 1024
	anchorTextPreviewBytes = 512
)

// boundedText decodes a bounded SQL text projection with its original byte length.
type boundedText struct {
	text  sql.NullString
	bytes int
}

// optional returns the bounded value and whether SQL truncated its original byte sequence.
func (value boundedText) optional() (*string, bool) {
	if !value.text.Valid {
		return nil, false
	}
	return &value.text.String, value.bytes > len([]byte(value.text.String))
}

// ReviewConflictError reports that a context head changed after the caller read it.
type ReviewConflictError struct {
	Expected *int64
	Current  *int64
}

// Error returns a safe optimistic-concurrency diagnostic.
func (e *ReviewConflictError) Error() string { return "review head changed; reload before saving" }

// ReviewError is a safe repository error with a stable client-visible category.
type ReviewError struct {
	Kind    string
	Message string
}

// Error returns the safe repository diagnostic.
func (e *ReviewError) Error() string { return e.Message }

// ReviewContextParentConflictError reports conflicting idempotent initialization choices.
type ReviewContextParentConflictError struct {
	Requested *int64
	Existing  *int64
}

// ReviewAnchorLabelConflictError reports a duplicate human anchor label within one work.
type ReviewAnchorLabelConflictError struct {
	Label string
}

// Error returns a safe work-scoped label diagnostic.
func (e *ReviewAnchorLabelConflictError) Error() string {
	return "anchor label is already used for this article"
}

// Error returns a safe immutable-lineage diagnostic.
func (e *ReviewContextParentConflictError) Error() string {
	return "review context was already initialized with a different parent"
}

// reviewValidation reports invalid caller-controlled review input.
func reviewValidation(message string) error {
	return &ReviewError{Kind: "validation", Message: message}
}

// reviewNotFound reports a missing review-scoped record.
func reviewNotFound(message string) error { return &ReviewError{Kind: "not_found", Message: message} }

// reviewLifecycle reports a valid request rejected by immutable run lifecycle.
func reviewLifecycle(message string) error { return &ReviewError{Kind: "lifecycle", Message: message} }

// NoteSyntaxError reports parser diagnostics that make a note version unsaveable.
type NoteSyntaxError struct{ Errors []notes.SyntaxError }

// Error returns a safe note-language diagnostic.
func (e *NoteSyntaxError) Error() string { return "note contains syntax errors" }

// ReviewContext is one explicitly initialized interpretation context for a completed run.
type ReviewContext struct {
	ID              int64  `json:"id"`
	PipelineRunID   int64  `json:"pipeline_run_id"`
	ParentContextID *int64 `json:"parent_context_id,omitempty"`
	CreatedAt       string `json:"created_at"`
}

// ReviewContextCandidate describes one eligible parent context without materializing inherited state.
type ReviewContextCandidate struct {
	ContextID          int64  `json:"context_id"`
	PipelineRunID      int64  `json:"pipeline_run_id"`
	SearchID           string `json:"search_id"`
	SearchRevision     string `json:"search_revision"`
	ExecutionPlanID    int64  `json:"execution_plan_id"`
	AttemptNumber      int    `json:"attempt_number"`
	StartedAt          string `json:"started_at"`
	InheritedWorkCount int    `json:"inherited_work_count"`
}

// WorkReviewVersion is one immutable complete article-review snapshot.
type WorkReviewVersion struct {
	ID                 int64    `json:"id"`
	WorkID             int64    `json:"work_id"`
	WorkRevisionID     int64    `json:"work_revision_id"`
	CreatedInContextID int64    `json:"created_in_context_id"`
	ParentVersionID    *int64   `json:"parent_version_id,omitempty"`
	Status             string   `json:"status"`
	Substatuses        []string `json:"sub_statuses"`
	Reason             *string  `json:"reason"`
	ReasonTruncated    bool     `json:"reason_truncated"`
	CreatedAt          string   `json:"created_at"`
	ReviewerDisplay    string   `json:"reviewer_display"`
}

// WorkReviewState is the complete current state for one context work head.
type WorkReviewState struct {
	ContextID              int64              `json:"context_id"`
	WorkID                 int64              `json:"work_id"`
	WorkRevisionID         int64              `json:"work_revision_id"`
	Version                *WorkReviewVersion `json:"version,omitempty"`
	InheritedFromContextID *int64             `json:"inherited_from_context_id,omitempty"`
}

// workReviewAuditState is the bounded decision payload stored in audit before/after state.
type workReviewAuditState struct {
	Status      string   `json:"status"`
	Reason      *string  `json:"reason"`
	Substatuses []string `json:"sub_statuses"`
}

// ReviewRepository owns context initialization and immutable review, note, and anchor versions.
type ReviewRepository struct{ db *Database }

// CorpusID returns the opaque corpus identity used to namespace browser-local drafts.
func (r *ReviewRepository) CorpusID(ctx context.Context) (string, error) {
	var id string
	if err := r.db.DB.QueryRowContext(ctx, "SELECT corpus_id FROM review_settings WHERE id=1").Scan(&id); err != nil {
		return "", fmt.Errorf("read review corpus ID: %w", err)
	}
	return id, nil
}

// GetContextByRun returns the one initialized review context for a run, if present.
func (r *ReviewRepository) GetContextByRun(ctx context.Context, runID int64) (*ReviewContext, error) {
	return getReviewContext(ctx, r.db.DB, runID)
}

// queryRower is the shared single-row query boundary for database and transaction callers.
type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// reviewQuerier is the shared query boundary for review reads that need single and multiple rows.
type reviewQuerier interface {
	queryRower
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// getReviewContext reads the optional immutable context associated with one run.
func getReviewContext(ctx context.Context, q queryRower, runID int64) (*ReviewContext, error) {
	var item ReviewContext
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
func (r *ReviewRepository) ProposeParent(ctx context.Context, runID int64) (*ReviewContextCandidate, error) {
	target, err := r.reviewTarget(ctx, r.db.DB, runID)
	if err != nil {
		return nil, err
	}
	if target.Status != string(manifest.AttemptCompleted) || target.Visibility == string(manifest.RunTrashed) {
		return nil, reviewLifecycle("run is not reviewable")
	}
	for _, samePlan := range []bool{true, false} {
		candidate, err := r.firstParentCandidate(ctx, runID, target, samePlan)
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
func (r *ReviewRepository) reviewTarget(ctx context.Context, q queryRower, runID int64) (reviewTargetRecord, error) {
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
func (r *ReviewRepository) firstParentCandidate(ctx context.Context, runID int64, target reviewTargetRecord, samePlan bool) (*ReviewContextCandidate, error) {
	clause, value := "s.id=?", target.SearchDBID
	if samePlan {
		clause, value = "ep.id=?", target.PlanID
	}
	row := r.db.DB.QueryRowContext(ctx, `SELECT rc.id, pr.id, s.search_id, sr.revision_label, ep.id,
		COALESCE(pr.attempt_number, 0), pr.started_at,
		(SELECT COUNT(*) FROM review_context_work_heads parent_head
		 WHERE parent_head.review_context_id=rc.id AND EXISTS (
		   SELECT 1 FROM work_revisions target_wr
		   WHERE target_wr.pipeline_run_id=? AND `+CurrentNormalizedRevisionPredicate("target_wr")+`
		     AND target_wr.work_id=parent_head.work_id))
		FROM review_contexts rc
		JOIN pipeline_runs pr ON pr.id=rc.pipeline_run_id
		JOIN execution_plans ep ON ep.id=pr.execution_plan_id
		JOIN search_revisions sr ON sr.id=ep.search_revision_id
		JOIN searches s ON s.id=sr.search_id
		WHERE `+clause+` AND pr.status='completed' AND pr.visibility_state!='trashed'
		AND (pr.started_at < ? OR (pr.started_at=? AND pr.id < ?))
		ORDER BY pr.started_at DESC, pr.id DESC LIMIT 1`, runID, value, target.StartedAt, target.StartedAt, runID)
	var item ReviewContextCandidate
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
func (r *ReviewRepository) ListParentCandidates(ctx context.Context, runID int64, scope, cursorStartedAt string, cursorRunID int64, limit int, query string) ([]ReviewContextCandidate, error) {
	if scope != "same_search" && scope != "all" {
		return nil, reviewValidation("candidate scope must be same_search or all")
	}
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("candidate fetch limit must be between 1 and 101")
	}
	target, err := r.reviewTarget(ctx, r.db.DB, runID)
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
	rows, err := r.db.DB.QueryContext(ctx, `SELECT rc.id, pr.id, s.search_id, sr.revision_label, ep.id,
		COALESCE(pr.attempt_number, 0), pr.started_at,
		(SELECT COUNT(*) FROM review_context_work_heads parent_head
		 WHERE parent_head.review_context_id=rc.id AND EXISTS (
		   SELECT 1 FROM work_revisions target_wr
		   WHERE target_wr.pipeline_run_id=? AND `+CurrentNormalizedRevisionPredicate("target_wr")+`
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
	items := make([]ReviewContextCandidate, 0)
	for rows.Next() {
		var item ReviewContextCandidate
		if err := rows.Scan(&item.ContextID, &item.PipelineRunID, &item.SearchID, &item.SearchRevision,
			&item.ExecutionPlanID, &item.AttemptNumber, &item.StartedAt, &item.InheritedWorkCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateContext initializes one run context and freezes matching parent heads without copying version bodies.
func (r *ReviewRepository) CreateContext(ctx context.Context, runID int64, parentContextID *int64) (*ReviewContext, bool, error) {
	var created *ReviewContext
	newlyCreated := false
	err := r.db.withTx(ctx, func(tx *sql.Tx) error {
		existing, err := getReviewContext(ctx, tx, runID)
		if err != nil {
			return err
		}
		if existing != nil {
			if !sameNullableID(existing.ParentContextID, parentContextID) {
				return &ReviewContextParentConflictError{Requested: parentContextID, Existing: existing.ParentContextID}
			}
			created = existing
			return nil
		}
		target, err := r.reviewTarget(ctx, tx, runID)
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
			(pipeline_run_id, parent_context_id, created_at) VALUES (?, ?, ?)`, runID, nullableInt64(parentContextID), createdAt)
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
			WHERE latest.pipeline_run_id=? AND `+CurrentNormalizedRevisionPredicate("latest"), contextID, nullableInt64(parentContextID), runID)
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
		created = &ReviewContext{ID: contextID, PipelineRunID: runID, ParentContextID: parentContextID, CreatedAt: createdAt}
		newlyCreated = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return created, newlyCreated, nil
}

// GetWorkReview returns the complete current state for one context work revision.
func (r *ReviewRepository) GetWorkReview(ctx context.Context, contextID, workRevisionID int64) (*WorkReviewState, error) {
	var state WorkReviewState
	var versionID sql.NullInt64
	err := r.db.DB.QueryRowContext(ctx, `SELECT review_context_id, work_id, work_revision_id, review_version_id
		FROM review_context_work_heads WHERE review_context_id=? AND work_revision_id=?`, contextID, workRevisionID).
		Scan(&state.ContextID, &state.WorkID, &state.WorkRevisionID, &versionID)
	if err == sql.ErrNoRows {
		return nil, reviewNotFound("work revision does not belong to review context")
	}
	if err != nil {
		return nil, err
	}
	if versionID.Valid {
		version, err := r.getWorkReviewVersion(ctx, r.db.DB, versionID.Int64)
		if err != nil {
			return nil, err
		}
		state.Version = version
		if version.CreatedInContextID != contextID {
			value := version.CreatedInContextID
			state.InheritedFromContextID = &value
		}
	}
	return &state, nil
}

// AppendWorkReview appends a complete immutable state and compare-and-swaps only the selected context head.
func (r *ReviewRepository) AppendWorkReview(ctx context.Context, contextID, workRevisionID int64, expectedVersionID *int64, status string, substatuses []string, reason *string) (*WorkReviewState, bool, error) {
	canonical, normalizedReason, err := validateReviewState(status, substatuses, reason)
	if err != nil {
		return nil, false, err
	}
	var result *WorkReviewState
	changed := false
	err = r.db.withTx(ctx, func(tx *sql.Tx) error {
		runID, workID, current, err := r.mutableWorkHead(ctx, tx, contextID, workRevisionID)
		if err != nil {
			return err
		}
		if !sameNullableID(expectedVersionID, current) {
			return &ReviewConflictError{Expected: expectedVersionID, Current: current}
		}
		if current == nil && status == "not_evaluated" && len(canonical) == 0 && normalizedReason == nil {
			result = &WorkReviewState{ContextID: contextID, WorkID: workID, WorkRevisionID: workRevisionID}
			return nil
		}
		beforeState := workReviewAuditState{Status: "not_evaluated", Substatuses: []string{}}
		if current != nil {
			previous, err := r.getWorkReviewVersion(ctx, tx, *current)
			if err != nil {
				return err
			}
			beforeState = workReviewAuditState{Status: previous.Status, Reason: previous.Reason, Substatuses: previous.Substatuses}
			if previous.Status == status && optionalStringEqual(previous.Reason, normalizedReason) && stringSlicesEqual(previous.Substatuses, canonical) {
				result = &WorkReviewState{ContextID: contextID, WorkID: workID, WorkRevisionID: workRevisionID, Version: previous}
				if previous.CreatedInContextID != contextID {
					value := previous.CreatedInContextID
					result.InheritedFromContextID = &value
				}
				return nil
			}
		}
		inserted, err := tx.ExecContext(ctx, `INSERT INTO work_review_versions
			(work_id, work_revision_id, created_in_context_id, parent_version_id, status, reason, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, workID, workRevisionID, contextID, nullableInt64(current), status, nullableString(normalizedReason), timestamp())
		if err != nil {
			return fmt.Errorf("insert work review version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		for _, substatus := range canonical {
			if _, err := tx.ExecContext(ctx, `INSERT INTO work_review_version_substatuses
				(review_version_id, sub_status) VALUES (?, ?)`, versionID, substatus); err != nil {
				return fmt.Errorf("insert work review sub-status: %w", err)
			}
		}
		updated, err := tx.ExecContext(ctx, `UPDATE review_context_work_heads SET review_version_id=?
			WHERE review_context_id=? AND work_revision_id=?
			AND ((review_version_id IS NULL AND ? IS NULL) OR review_version_id=?)`,
			versionID, contextID, workRevisionID, nullableInt64(current), nullableInt64(current))
		if err != nil {
			return err
		}
		if count, _ := updated.RowsAffected(); count != 1 {
			return &ReviewConflictError{Expected: expectedVersionID, Current: current}
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "parent_version_id": current, "new_version_id": versionID}
		afterState := workReviewAuditState{Status: status, Reason: normalizedReason, Substatuses: canonical}
		if err := insertReviewChangeAudit(ctx, tx, runID, "work_review_version", strconv.FormatInt(versionID, 10), manifest.AuditWorkReviewVersionCreated, beforeState, afterState, metadata); err != nil {
			return err
		}
		version, err := r.getWorkReviewVersion(ctx, tx, versionID)
		if err != nil {
			return err
		}
		result = &WorkReviewState{ContextID: contextID, WorkID: workID, WorkRevisionID: workRevisionID, Version: version}
		changed = true
		return nil
	})
	return result, changed, err
}

// mutableWorkHead validates an editable context work and returns its run, work, and current version IDs.
func (r *ReviewRepository) mutableWorkHead(ctx context.Context, tx *sql.Tx, contextID, workRevisionID int64) (int64, int64, *int64, error) {
	var runID, workID int64
	var current sql.NullInt64
	var status, visibility string
	err := tx.QueryRowContext(ctx, `SELECT rc.pipeline_run_id, head.work_id, head.review_version_id, pr.status, pr.visibility_state
		FROM review_context_work_heads head
		JOIN review_contexts rc ON rc.id=head.review_context_id
		JOIN pipeline_runs pr ON pr.id=rc.pipeline_run_id
		JOIN work_revisions wr ON wr.id=head.work_revision_id AND wr.pipeline_run_id=rc.pipeline_run_id AND wr.work_id=head.work_id
		WHERE head.review_context_id=? AND head.work_revision_id=?`, contextID, workRevisionID).
		Scan(&runID, &workID, &current, &status, &visibility)
	if err == sql.ErrNoRows {
		return 0, 0, nil, reviewNotFound("work revision does not belong to review context")
	}
	if err != nil {
		return 0, 0, nil, err
	}
	if status != string(manifest.AttemptCompleted) || visibility == string(manifest.RunTrashed) {
		return 0, 0, nil, reviewLifecycle("review context run is read-only")
	}
	return runID, workID, nullInt64Pointer(current), nil
}

// ListWorkReviewVersions follows only the selected head's ancestor chain.
func (r *ReviewRepository) ListWorkReviewVersions(ctx context.Context, contextID, workRevisionID, cursor int64, limit int) ([]WorkReviewVersion, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("version fetch limit must be between 1 and 101")
	}
	rows, err := r.db.DB.QueryContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT review_version_id FROM review_context_work_heads
		 WHERE review_context_id=? AND work_revision_id=? AND review_version_id IS NOT NULL
		UNION ALL
		SELECT version.parent_version_id FROM work_review_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	)
	SELECT version.id, version.work_id, version.work_revision_id, version.created_in_context_id,
		version.parent_version_id, version.status, substr(version.reason, 1, `+strconv.Itoa(reviewTextPreviewBytes)+`),
		COALESCE(length(CAST(version.reason AS BLOB)), 0), version.created_at, reviewer.username, reviewer.email,
		COALESCE((SELECT json_group_array(sub_status) FROM (
			SELECT sub_status FROM work_review_version_substatuses WHERE review_version_id=version.id ORDER BY sub_status)), '[]')
	FROM ancestry JOIN work_review_versions version ON version.id=ancestry.id
	JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
	LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
	WHERE (?=0 OR version.id<?) ORDER BY version.id DESC LIMIT ?`, contextID, workRevisionID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	versions := make([]WorkReviewVersion, 0)
	for rows.Next() {
		var item WorkReviewVersion
		var parent sql.NullInt64
		var reason boundedText
		var username, email sql.NullString
		var substatusesJSON string
		if err := rows.Scan(&item.ID, &item.WorkID, &item.WorkRevisionID, &item.CreatedInContextID,
			&parent, &item.Status, &reason.text, &reason.bytes, &item.CreatedAt, &username, &email, &substatusesJSON); err != nil {
			return nil, err
		}
		item.ParentVersionID = nullInt64Pointer(parent)
		item.Reason, item.ReasonTruncated = reason.optional()
		item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
		if err := json.Unmarshal([]byte(substatusesJSON), &item.Substatuses); err != nil {
			return nil, err
		}
		versions = append(versions, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return versions, nil
}

// GetWorkReviewVersion returns one full version only when it belongs to the selected head ancestry.
func (r *ReviewRepository) GetWorkReviewVersion(ctx context.Context, contextID, workRevisionID, versionID int64) (*WorkReviewVersion, error) {
	var exists bool
	if err := r.db.DB.QueryRowContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT review_version_id FROM review_context_work_heads
		 WHERE review_context_id=? AND work_revision_id=? AND review_version_id IS NOT NULL
		UNION ALL
		SELECT version.parent_version_id FROM work_review_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	) SELECT EXISTS(SELECT 1 FROM ancestry WHERE id=?)`, contextID, workRevisionID, versionID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return r.getWorkReviewVersion(ctx, r.db.DB, versionID)
}

// getWorkReviewVersion reads one immutable version with canonical sub-statuses and reviewer attribution.
func (r *ReviewRepository) getWorkReviewVersion(ctx context.Context, q reviewQuerier, id int64) (*WorkReviewVersion, error) {
	var item WorkReviewVersion
	var parent sql.NullInt64
	var reason sql.NullString
	var username, email sql.NullString
	err := q.QueryRowContext(ctx, `SELECT version.id, version.work_id, version.work_revision_id,
		version.created_in_context_id, version.parent_version_id, version.status, version.reason, version.created_at,
		reviewer.username, reviewer.email
		FROM work_review_versions version
		JOIN review_contexts context ON context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=context.pipeline_run_id
		WHERE version.id=?`, id).Scan(&item.ID, &item.WorkID, &item.WorkRevisionID, &item.CreatedInContextID,
		&parent, &item.Status, &reason, &item.CreatedAt, &username, &email)
	if err != nil {
		return nil, err
	}
	item.ParentVersionID = nullInt64Pointer(parent)
	if reason.Valid {
		item.Reason = &reason.String
	}
	item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
	rows, err := q.QueryContext(ctx, `SELECT sub_status FROM work_review_version_substatuses
		WHERE review_version_id=? ORDER BY sub_status`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	item.Substatuses = make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		item.Substatuses = append(item.Substatuses, value)
	}
	return &item, rows.Err()
}

// validateReviewState normalizes one complete review state and enforces vocabulary compatibility.
func validateReviewState(status string, substatuses []string, reason *string) ([]string, *string, error) {
	if !validReviewStatuses[status] {
		return nil, nil, reviewValidation(fmt.Sprintf("invalid review status %q", status))
	}
	seen := make(map[string]bool, len(substatuses))
	canonical := append([]string(nil), substatuses...)
	for _, value := range canonical {
		if !validReviewSubstatuses[value] {
			return nil, nil, reviewValidation(fmt.Sprintf("invalid review sub-status %q", value))
		}
		if seen[value] {
			return nil, nil, reviewValidation(fmt.Sprintf("duplicate review sub-status %q", value))
		}
		seen[value] = true
	}
	if len(canonical) > 0 && status != "not_approved" && status != "removed" {
		return nil, nil, reviewValidation("sub-statuses are allowed only for not_approved or removed")
	}
	sort.Strings(canonical)
	var normalized *string
	if reason != nil && strings.TrimSpace(*reason) != "" {
		value := strings.TrimSpace(*reason)
		if utf8.RuneCountInString(value) > 32768 {
			return nil, nil, reviewValidation("review reason exceeds 32768 characters")
		}
		normalized = &value
	}
	return canonical, normalized, nil
}

// insertReviewAudit appends identifier-only review evidence within the caller's head-move transaction.
func insertReviewAudit(ctx context.Context, tx *sql.Tx, runID int64, entityType, entityID string, action manifest.AuditAction, metadata any) error {
	return insertReviewChangeAudit(ctx, tx, runID, entityType, entityID, action, nil, nil, metadata)
}

// insertReviewChangeAudit appends identifier metadata and optional bounded decision-state changes.
func insertReviewChangeAudit(ctx context.Context, tx *sql.Tx, runID int64, entityType, entityID string, action manifest.AuditAction, before, after, metadata any) error {
	if err := manifest.ValidateAuditAction(string(action)); err != nil {
		return err
	}
	encodedMetadata, err := marshalReviewAuditValue("metadata", metadata)
	if err != nil {
		return err
	}
	encodedBefore, err := marshalReviewAuditValue("before state", before)
	if err != nil {
		return err
	}
	encodedAfter, err := marshalReviewAuditValue("after state", after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events
		(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, before_json, after_json, metadata_json)
		VALUES (?, 'reviewer', ?, ?, ?, ?, ?, ?, ?)`, timestamp(), runID, entityType, entityID, string(action), encodedBefore, encodedAfter, encodedMetadata)
	if err != nil {
		return fmt.Errorf("insert review audit event: %w", err)
	}
	return nil
}

// marshalReviewAuditValue returns a nullable JSON payload for one review audit field.
func marshalReviewAuditValue(name string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal review audit %s: %w", name, err)
	}
	return string(encoded), nil
}

// nullableInt64 converts an optional integer into a SQL parameter.
func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

// nullableString converts an optional string into a SQL parameter.
func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

// nullInt64Pointer converts a scanned nullable integer into an optional value.
func nullInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

// optionalStringEqual compares nullable normalized text.
func optionalStringEqual(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

// stringSlicesEqual compares canonical ordered string sets.
func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// reviewerDisplay exposes only the optional username and never places reviewer email in portable API responses.
func reviewerDisplay(username, _ string) string {
	username = strings.TrimSpace(username)
	if username != "" {
		return username
	}
	return "Anonymous or redacted"
}

// ReviewNoteVersion is one immutable active note snapshot or deletion tombstone.
type ReviewNoteVersion struct {
	ID                 int64        `json:"id"`
	NoteID             int64        `json:"note_id"`
	ParentVersionID    *int64       `json:"parent_version_id,omitempty"`
	CreatedInContextID int64        `json:"created_in_context_id"`
	State              string       `json:"state"`
	Body               *string      `json:"body"`
	BodyBytes          int          `json:"body_bytes"`
	BodyTruncated      bool         `json:"body_truncated"`
	Title              string       `json:"title"`
	Excerpt            string       `json:"excerpt"`
	LinkCount          int          `json:"link_count"`
	CreatedAt          string       `json:"created_at"`
	ReviewerDisplay    string       `json:"reviewer_display"`
	Links              []ReviewLink `json:"links"`
}

// ReviewNote is one logical note with the selected context's current immutable head.
type ReviewNote struct {
	ID                     int64             `json:"id"`
	WorkID                 int64             `json:"work_id"`
	WorkRevisionID         int64             `json:"work_revision_id"`
	CreatedAt              string            `json:"created_at"`
	Version                ReviewNoteVersion `json:"version"`
	InheritedFromContextID *int64            `json:"inherited_from_context_id,omitempty"`
}

// ReviewLink is a version-scoped custom link plus its context-sensitive resolution.
type ReviewLink struct {
	Ordinal        int     `json:"ordinal"`
	TargetType     string  `json:"target_type"`
	RawTarget      string  `json:"raw_target"`
	DisplayText    *string `json:"display_text,omitempty"`
	UTF16Position  int     `json:"utf16_position"`
	UTF16Length    int     `json:"utf16_length"`
	Resolved       bool    `json:"resolved"`
	WorkRevisionID *int64  `json:"work_revision_id,omitempty"`
	NoteID         *int64  `json:"note_id,omitempty"`
	AnchorID       *string `json:"anchor_id,omitempty"`
	Page           *int    `json:"page,omitempty"`
	URL            *string `json:"url,omitempty"`
}

// CreateNote creates one logical note, immutable first version, head, links, and audit atomically.
func (r *ReviewRepository) CreateNote(ctx context.Context, contextID, workRevisionID int64, body string) (*ReviewNote, error) {
	document := notes.Parse(body)
	if len(document.Errors) != 0 {
		return nil, &NoteSyntaxError{Errors: document.Errors}
	}
	if strings.TrimSpace(body) == "" {
		return nil, reviewValidation("active note body must not be blank")
	}
	var note *ReviewNote
	err := r.db.withTx(ctx, func(tx *sql.Tx) error {
		runID, workID, _, err := r.mutableWorkHead(ctx, tx, contextID, workRevisionID)
		if err != nil {
			return err
		}
		createdAt := timestamp()
		inserted, err := tx.ExecContext(ctx, `INSERT INTO review_notes (work_id, created_at) VALUES (?, ?)`, workID, createdAt)
		if err != nil {
			return fmt.Errorf("insert review note: %w", err)
		}
		noteID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		versionResult, err := tx.ExecContext(ctx, `INSERT INTO review_note_versions
			(note_id, created_in_context_id, state, body, created_at) VALUES (?, ?, 'active', ?, ?)`, noteID, contextID, body, createdAt)
		if err != nil {
			return fmt.Errorf("insert review note version: %w", err)
		}
		versionID, err := versionResult.LastInsertId()
		if err != nil {
			return err
		}
		if err := insertNoteLinks(ctx, tx, versionID, document.Links); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_context_note_heads
			(review_context_id, note_id, note_version_id) VALUES (?, ?, ?)`, contextID, noteID, versionID); err != nil {
			return fmt.Errorf("insert review note head: %w", err)
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, tx, runID, "review_note_version", strconv.FormatInt(versionID, 10), manifest.AuditReviewNoteCreated, metadata); err != nil {
			return err
		}
		version, err := r.getNoteVersion(ctx, tx, contextID, versionID)
		if err != nil {
			return err
		}
		note = &ReviewNote{ID: noteID, WorkID: workID, WorkRevisionID: workRevisionID, CreatedAt: createdAt, Version: *version}
		return nil
	})
	return note, err
}

// AppendNoteVersion appends an active edit or deletion tombstone with optimistic concurrency.
func (r *ReviewRepository) AppendNoteVersion(ctx context.Context, contextID, noteID int64, expectedVersionID int64, state, body string) (*ReviewNote, bool, error) {
	if state != "active" && state != "deleted" {
		return nil, false, reviewValidation("note state must be active or deleted")
	}
	var document notes.Document
	if state == "active" {
		document = notes.Parse(body)
		if len(document.Errors) != 0 {
			return nil, false, &NoteSyntaxError{Errors: document.Errors}
		}
		if strings.TrimSpace(body) == "" {
			return nil, false, reviewValidation("active note body must not be blank")
		}
	} else if body != "" {
		return nil, false, reviewValidation("deleted note version must not contain a body")
	}
	var note *ReviewNote
	changed := false
	err := r.db.withTx(ctx, func(tx *sql.Tx) error {
		var runID, workID, workRevisionID, currentID int64
		var createdAt, runStatus, visibility string
		err := tx.QueryRowContext(ctx, `SELECT rc.pipeline_run_id, logical.work_id, work_head.work_revision_id,
			head.note_version_id, logical.created_at, pr.status, pr.visibility_state
			FROM review_context_note_heads head
			JOIN review_contexts rc ON rc.id=head.review_context_id
			JOIN pipeline_runs pr ON pr.id=rc.pipeline_run_id
			JOIN review_notes logical ON logical.id=head.note_id
			JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
			WHERE head.review_context_id=? AND head.note_id=?`, contextID, noteID).
			Scan(&runID, &workID, &workRevisionID, &currentID, &createdAt, &runStatus, &visibility)
		if err == sql.ErrNoRows {
			return reviewNotFound("note does not belong to review context")
		}
		if err != nil {
			return err
		}
		if runStatus != string(manifest.AttemptCompleted) || visibility == string(manifest.RunTrashed) {
			return reviewLifecycle("review context run is read-only")
		}
		if currentID != expectedVersionID {
			current := currentID
			expected := expectedVersionID
			return &ReviewConflictError{Expected: &expected, Current: &current}
		}
		current, err := r.getNoteVersion(ctx, tx, contextID, currentID)
		if err != nil {
			return err
		}
		if current.State == state && ((state == "deleted") || (current.Body != nil && *current.Body == body)) {
			note = &ReviewNote{ID: noteID, WorkID: workID, WorkRevisionID: workRevisionID, CreatedAt: createdAt, Version: *current}
			if current.CreatedInContextID != contextID {
				value := current.CreatedInContextID
				note.InheritedFromContextID = &value
			}
			return nil
		}
		createdVersionAt := timestamp()
		var bodyValue any
		if state == "active" {
			bodyValue = body
		}
		inserted, err := tx.ExecContext(ctx, `INSERT INTO review_note_versions
			(note_id, parent_version_id, created_in_context_id, state, body, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`, noteID, currentID, contextID, state, bodyValue, createdVersionAt)
		if err != nil {
			return fmt.Errorf("insert review note version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		if state == "active" {
			if err := insertNoteLinks(ctx, tx, versionID, document.Links); err != nil {
				return err
			}
		}
		updated, err := tx.ExecContext(ctx, `UPDATE review_context_note_heads SET note_version_id=?
			WHERE review_context_id=? AND note_id=? AND note_version_id=?`, versionID, contextID, noteID, currentID)
		if err != nil {
			return err
		}
		if count, _ := updated.RowsAffected(); count != 1 {
			current := currentID
			expected := expectedVersionID
			return &ReviewConflictError{Expected: &expected, Current: &current}
		}
		action := manifest.AuditReviewNoteVersionCreated
		if state == "deleted" {
			action = manifest.AuditReviewNoteTombstoned
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "parent_version_id": currentID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, tx, runID, "review_note_version", strconv.FormatInt(versionID, 10), action, metadata); err != nil {
			return err
		}
		version, err := r.getNoteVersion(ctx, tx, contextID, versionID)
		if err != nil {
			return err
		}
		note = &ReviewNote{ID: noteID, WorkID: workID, WorkRevisionID: workRevisionID, CreatedAt: createdAt, Version: *version}
		changed = true
		return nil
	})
	return note, changed, err
}

// GetNote returns an explicitly addressed current note head, including tombstones.
func (r *ReviewRepository) GetNote(ctx context.Context, contextID, noteID int64) (*ReviewNote, error) {
	return r.getNote(ctx, r.db.DB, contextID, noteID)
}

// getNote reads one selected context head and resolves inherited attribution.
func (r *ReviewRepository) getNote(ctx context.Context, q reviewQuerier, contextID, noteID int64) (*ReviewNote, error) {
	var note ReviewNote
	var versionID int64
	err := q.QueryRowContext(ctx, `SELECT logical.id, logical.work_id, work_head.work_revision_id,
		logical.created_at, head.note_version_id
		FROM review_context_note_heads head
		JOIN review_notes logical ON logical.id=head.note_id
		JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
		WHERE head.review_context_id=? AND head.note_id=?`, contextID, noteID).
		Scan(&note.ID, &note.WorkID, &note.WorkRevisionID, &note.CreatedAt, &versionID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	version, err := r.getNoteVersion(ctx, q, contextID, versionID)
	if err != nil {
		return nil, err
	}
	note.Version = *version
	if version.CreatedInContextID != contextID {
		value := version.CreatedInContextID
		note.InheritedFromContextID = &value
	}
	return &note, nil
}

// ListNotes returns bounded current note heads for one context work, excluding tombstones unless requested.
func (r *ReviewRepository) ListNotes(ctx context.Context, contextID, workRevisionID, cursor int64, limit int, includeDeleted bool) ([]ReviewNote, error) {
	state := "active"
	if includeDeleted {
		state = "all"
	}
	return r.ListNotesFiltered(ctx, contextID, &workRevisionID, cursor, limit, state, "")
}

// ListNotesFiltered returns bounded current note-head summaries for one work or the complete run context.
func (r *ReviewRepository) ListNotesFiltered(ctx context.Context, contextID int64, workRevisionID *int64, cursor int64, limit int, state, query string) ([]ReviewNote, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("note fetch limit must be between 1 and 101")
	}
	clauses := []string{"head.review_context_id=?", "(?=0 OR head.note_id<?)"}
	args := []any{contextID, cursor, cursor}
	switch state {
	case "active":
		clauses = append(clauses, "version.state='active'")
	case "removed":
		clauses = append(clauses, "version.state='deleted'")
	case "all":
	default:
		return nil, reviewValidation("note state must be active, removed, or all")
	}
	if workRevisionID != nil {
		clauses = append(clauses, "work_head.work_revision_id=?")
		args = append(args, *workRevisionID)
	}
	if query = strings.TrimSpace(query); query != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
		clauses = append(clauses, `version.body LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escaped+"%")
	}
	args = append(args, limit)
	rows, err := r.db.DB.QueryContext(ctx, `SELECT logical.id, logical.work_id, work_head.work_revision_id, logical.created_at,
		version.id, version.note_id, version.parent_version_id, version.created_in_context_id, version.state,
		substr(version.body, 1, `+strconv.Itoa(reviewTextPreviewBytes)+`), COALESCE(length(CAST(version.body AS BLOB)), 0), version.created_at,
		reviewer.username, reviewer.email,
		(SELECT COUNT(*) FROM review_note_links link WHERE link.note_version_id=version.id)
		FROM review_context_note_heads head
		JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id
		JOIN review_notes logical ON logical.id=head.note_id AND logical.work_id=work_head.work_id
		JOIN review_note_versions version ON version.id=head.note_version_id
		JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
		WHERE `+strings.Join(clauses, " AND ")+`
		ORDER BY head.note_id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	return scanReviewNoteSummaries(rows, contextID)
}

// ListNoteVersions follows only the selected context note head's ancestors.
func (r *ReviewRepository) ListNoteVersions(ctx context.Context, contextID, noteID, cursor int64, limit int) ([]ReviewNoteVersion, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("note version fetch limit must be between 1 and 101")
	}
	rows, err := r.db.DB.QueryContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT note_version_id FROM review_context_note_heads WHERE review_context_id=? AND note_id=?
		UNION ALL
		SELECT version.parent_version_id FROM review_note_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	)
	SELECT version.id, version.note_id, version.parent_version_id, version.created_in_context_id, version.state,
		substr(version.body, 1, `+strconv.Itoa(reviewTextPreviewBytes)+`), COALESCE(length(CAST(version.body AS BLOB)), 0), version.created_at,
		reviewer.username, reviewer.email,
		(SELECT COUNT(*) FROM review_note_links link WHERE link.note_version_id=version.id)
	FROM ancestry JOIN review_note_versions version ON version.id=ancestry.id
	JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
	LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
	WHERE (?=0 OR version.id<?) ORDER BY version.id DESC LIMIT ?`, contextID, noteID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	versions := make([]ReviewNoteVersion, 0)
	for rows.Next() {
		var item ReviewNoteVersion
		var parent sql.NullInt64
		var body boundedText
		var username, email sql.NullString
		if err := rows.Scan(&item.ID, &item.NoteID, &parent, &item.CreatedInContextID, &item.State,
			&body.text, &body.bytes, &item.CreatedAt, &username, &email, &item.LinkCount); err != nil {
			return nil, err
		}
		item.ParentVersionID = nullInt64Pointer(parent)
		item.Body, item.BodyTruncated = body.optional()
		item.BodyBytes = body.bytes
		if item.Body != nil {
			item.Title, item.Excerpt = noteSummary(*item.Body)
		}
		item.Links = []ReviewLink{}
		item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
		versions = append(versions, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return versions, nil
}

// GetNoteVersion returns one full body and link set only when it belongs to the selected head ancestry.
func (r *ReviewRepository) GetNoteVersion(ctx context.Context, contextID, noteID, versionID int64) (*ReviewNoteVersion, error) {
	var exists bool
	if err := r.db.DB.QueryRowContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT note_version_id FROM review_context_note_heads WHERE review_context_id=? AND note_id=?
		UNION ALL
		SELECT version.parent_version_id FROM review_note_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	) SELECT EXISTS(SELECT 1 FROM ancestry WHERE id=?)`, contextID, noteID, versionID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return r.getNoteVersion(ctx, r.db.DB, contextID, versionID)
}

// getNoteVersion reads one immutable note version and resolves its version-scoped links.
func (r *ReviewRepository) getNoteVersion(ctx context.Context, q reviewQuerier, contextID, versionID int64) (*ReviewNoteVersion, error) {
	var item ReviewNoteVersion
	var parent sql.NullInt64
	var body, username, email sql.NullString
	err := q.QueryRowContext(ctx, `SELECT version.id, version.note_id, version.parent_version_id,
		version.created_in_context_id, version.state, version.body, version.created_at,
		reviewer.username, reviewer.email
		FROM review_note_versions version
		JOIN review_contexts context ON context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=context.pipeline_run_id
		WHERE version.id=?`, versionID).Scan(&item.ID, &item.NoteID, &parent, &item.CreatedInContextID,
		&item.State, &body, &item.CreatedAt, &username, &email)
	if err != nil {
		return nil, err
	}
	item.ParentVersionID = nullInt64Pointer(parent)
	if body.Valid {
		item.Body = &body.String
		item.BodyBytes = len([]byte(body.String))
		item.Title, item.Excerpt = noteSummary(body.String)
	}
	item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
	links, err := r.linksForVersion(ctx, q, contextID, item.NoteID, versionID)
	if err != nil {
		return nil, err
	}
	item.Links = links
	item.LinkCount = len(links)
	return &item, nil
}

// insertNoteLinks stores parser output against the exact immutable note version.
func insertNoteLinks(ctx context.Context, tx *sql.Tx, versionID int64, links []notes.Link) error {
	for index, link := range links {
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_note_links
			(note_version_id, ordinal, target_type, raw_target, display_text, utf16_position, utf16_length, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, versionID, index+1, link.TargetType, link.RawTarget,
			nullableString(link.DisplayText), link.Position, link.Length, timestamp()); err != nil {
			return fmt.Errorf("insert review note link: %w", err)
		}
	}
	return nil
}

// linksForVersion reads links in source order and resolves them against the selected context.
func (r *ReviewRepository) linksForVersion(ctx context.Context, q reviewQuerier, contextID, noteID, versionID int64) ([]ReviewLink, error) {
	rows, err := q.QueryContext(ctx, `SELECT ordinal, target_type, raw_target, display_text, utf16_position, utf16_length
		FROM review_note_links WHERE note_version_id=? ORDER BY ordinal`, versionID)
	if err != nil {
		return nil, err
	}
	links := make([]ReviewLink, 0)
	for rows.Next() {
		var link ReviewLink
		var display sql.NullString
		if err := rows.Scan(&link.Ordinal, &link.TargetType, &link.RawTarget, &display, &link.UTF16Position, &link.UTF16Length); err != nil {
			return nil, err
		}
		if display.Valid {
			link.DisplayText = &display.String
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range links {
		if err := r.resolveLink(ctx, q, contextID, noteID, &links[index]); err != nil {
			return nil, err
		}
	}
	return links, nil
}

// resolveLink enriches a syntactically valid link without rewriting persisted link identity.
func (r *ReviewRepository) resolveLink(ctx context.Context, q queryRower, contextID, sourceNoteID int64, link *ReviewLink) error {
	switch link.TargetType {
	case "note":
		noteID, _ := strconv.ParseInt(link.RawTarget, 10, 64)
		var revisionID int64
		err := q.QueryRowContext(ctx, `SELECT work_head.work_revision_id
			FROM review_context_note_heads note_head
			JOIN review_notes logical ON logical.id=note_head.note_id
			JOIN review_context_work_heads work_head ON work_head.review_context_id=note_head.review_context_id AND work_head.work_id=logical.work_id
			WHERE note_head.review_context_id=? AND note_head.note_id=?`, contextID, noteID).Scan(&revisionID)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		link.Resolved, link.WorkRevisionID, link.NoteID = true, &revisionID, &noteID
	case "article":
		var revisionID int64
		err := q.QueryRowContext(ctx, `SELECT head.work_revision_id FROM review_context_work_heads head
			JOIN works work ON work.id=head.work_id WHERE head.review_context_id=? AND work.doi=?`, contextID, link.RawTarget).Scan(&revisionID)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		link.Resolved, link.WorkRevisionID = true, &revisionID
	case "pdf_page":
		page, _ := strconv.Atoi(link.RawTarget)
		var revisionID int64
		err := q.QueryRowContext(ctx, `SELECT work_head.work_revision_id
			FROM review_notes logical JOIN review_context_work_heads work_head
			 ON work_head.review_context_id=? AND work_head.work_id=logical.work_id
			WHERE logical.id=?`, contextID, sourceNoteID).Scan(&revisionID)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		link.Resolved, link.WorkRevisionID, link.Page = true, &revisionID, &page
	case "anchor":
		var revisionID int64
		var page int
		err := q.QueryRowContext(ctx, `SELECT work_head.work_revision_id, version.page
			FROM review_context_anchor_heads anchor_head
			JOIN review_anchors logical ON logical.id=anchor_head.anchor_id
			JOIN review_anchor_versions version ON version.id=anchor_head.anchor_version_id AND version.state='active'
			JOIN review_context_work_heads work_head ON work_head.review_context_id=anchor_head.review_context_id AND work_head.work_id=logical.work_id
			WHERE anchor_head.review_context_id=? AND anchor_head.anchor_id=?`, contextID, link.RawTarget).Scan(&revisionID, &page)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		anchorID := link.RawTarget
		link.Resolved, link.WorkRevisionID, link.AnchorID, link.Page = true, &revisionID, &anchorID, &page
	case "ext":
		parsed, err := url.Parse(link.RawTarget)
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			value := parsed.String()
			link.Resolved, link.URL = true, &value
		}
	}
	return nil
}

// ListBacklinks returns links from current note heads only.
func (r *ReviewRepository) ListBacklinks(ctx context.Context, contextID int64, targetType, targetID string, sourceWorkID, cursor int64, limit int) ([]ReviewNote, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("backlink fetch limit must be between 1 and 101")
	}
	rows, err := r.db.DB.QueryContext(ctx, `SELECT logical.id, logical.work_id, work_head.work_revision_id, logical.created_at,
		version.id, version.note_id, version.parent_version_id, version.created_in_context_id, version.state,
		substr(version.body, 1, `+strconv.Itoa(reviewTextPreviewBytes)+`), COALESCE(length(CAST(version.body AS BLOB)), 0), version.created_at,
		reviewer.username, reviewer.email,
		(SELECT COUNT(*) FROM review_note_links all_links WHERE all_links.note_version_id=version.id)
		FROM review_context_note_heads head
		JOIN review_note_versions version ON version.id=head.note_version_id AND version.state='active'
		JOIN review_notes logical ON logical.id=head.note_id
		JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
		JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
		JOIN review_note_links link ON link.note_version_id=head.note_version_id
		WHERE head.review_context_id=? AND link.target_type=? AND link.raw_target=?
		AND (?=0 OR logical.work_id=?)
		AND (?=0 OR head.note_id<?)
		GROUP BY logical.id, version.id
		ORDER BY head.note_id DESC LIMIT ?`, contextID, targetType, targetID, sourceWorkID, sourceWorkID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	return scanReviewNoteSummaries(rows, contextID)
}

// scanReviewNoteSummaries reads bounded list projections without loading full bodies or resolving every link.
func scanReviewNoteSummaries(rows *sql.Rows, contextID int64) ([]ReviewNote, error) {
	defer rows.Close()
	items := make([]ReviewNote, 0)
	for rows.Next() {
		var item ReviewNote
		var version ReviewNoteVersion
		var parent sql.NullInt64
		var body boundedText
		var username, email sql.NullString
		if err := rows.Scan(&item.ID, &item.WorkID, &item.WorkRevisionID, &item.CreatedAt,
			&version.ID, &version.NoteID, &parent, &version.CreatedInContextID, &version.State,
			&body.text, &body.bytes, &version.CreatedAt, &username, &email, &version.LinkCount); err != nil {
			return nil, err
		}
		version.ParentVersionID = nullInt64Pointer(parent)
		version.Body, version.BodyTruncated = body.optional()
		version.BodyBytes = body.bytes
		if version.Body != nil {
			version.Title, version.Excerpt = noteSummary(*version.Body)
		}
		version.Links = []ReviewLink{}
		version.ReviewerDisplay = reviewerDisplay(username.String, email.String)
		item.Version = version
		if version.CreatedInContextID != contextID {
			value := version.CreatedInContextID
			item.InheritedFromContextID = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// noteSummary derives a safe title and excerpt from one stored body or bounded prefix.
func noteSummary(body string) (string, string) {
	lines := strings.Split(body, "\n")
	title := "Untitled note"
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		trimmed := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if trimmed != "" {
			title = trimmed
			break
		}
	}
	return truncateRunes(title, 80), truncateRunes(strings.Join(strings.Fields(body), " "), 180)
}

// truncateRunes returns a Unicode-safe bounded label with an explicit truncation marker.
func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

// AnchorRectangle is one normalized highlight rectangle on a single PDF page.
type AnchorRectangle struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ReviewAnchorVersion is one immutable active highlight snapshot or deletion tombstone.
type ReviewAnchorVersion struct {
	ID                    int64             `json:"id"`
	AnchorID              string            `json:"anchor_id"`
	ParentVersionID       *int64            `json:"parent_version_id,omitempty"`
	CreatedInContextID    int64             `json:"created_in_context_id"`
	WorkRevisionID        int64             `json:"work_revision_id"`
	PDFContentHash        string            `json:"pdf_content_hash"`
	State                 string            `json:"state"`
	Page                  *int              `json:"page,omitempty"`
	SelectedText          *string           `json:"selected_text,omitempty"`
	SelectedTextTruncated bool              `json:"selected_text_truncated"`
	Rectangles            []AnchorRectangle `json:"rectangles,omitempty"`
	CreatedAt             string            `json:"created_at"`
	ReviewerDisplay       string            `json:"reviewer_display"`
}

// ReviewAnchor is one stable corpus-wide anchor with the selected context's current head.
type ReviewAnchor struct {
	ID                     string              `json:"id"`
	Label                  string              `json:"label"`
	WorkID                 int64               `json:"work_id"`
	CreatedAt              string              `json:"created_at"`
	Version                ReviewAnchorVersion `json:"version"`
	InheritedFromContextID *int64              `json:"inherited_from_context_id,omitempty"`
}

// CreateAnchor creates one generated logical anchor with a work-scoped label and immutable first version atomically.
func (r *ReviewRepository) CreateAnchor(ctx context.Context, contextID, workRevisionID int64, label, contentHash string, page int, selectedText string, rectangles []AnchorRectangle) (*ReviewAnchor, error) {
	if !notes.ValidAnchorID(label) {
		return nil, reviewValidation("anchor label has an invalid format")
	}
	anchorID, err := newAnchorID()
	if err != nil {
		return nil, fmt.Errorf("generate review anchor identity: %w", err)
	}
	if err := validateAnchorVersion(anchorID, contentHash, "active", page, selectedText, rectangles); err != nil {
		return nil, err
	}
	var anchor *ReviewAnchor
	err = r.db.withTx(ctx, func(tx *sql.Tx) error {
		runID, workID, _, err := r.mutableWorkHead(ctx, tx, contextID, workRevisionID)
		if err != nil {
			return err
		}
		var labelExists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM review_anchors WHERE work_id=? AND label=?)`, workID, label).Scan(&labelExists); err != nil {
			return err
		}
		if labelExists {
			return &ReviewAnchorLabelConflictError{Label: label}
		}
		createdAt := timestamp()
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_anchors (id, work_id, label, created_at) VALUES (?, ?, ?, ?)`, anchorID, workID, label, createdAt); err != nil {
			return fmt.Errorf("insert review anchor: %w", err)
		}
		rectanglesJSON, _ := json.Marshal(rectangles)
		inserted, err := tx.ExecContext(ctx, `INSERT INTO review_anchor_versions
			(anchor_id, created_in_context_id, work_revision_id, pdf_content_hash, state, page, selected_text, rectangles_json, created_at)
			VALUES (?, ?, ?, ?, 'active', ?, ?, ?, ?)`, anchorID, contextID, workRevisionID, contentHash, page, selectedText, string(rectanglesJSON), createdAt)
		if err != nil {
			return fmt.Errorf("insert review anchor version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_context_anchor_heads
			(review_context_id, anchor_id, anchor_version_id) VALUES (?, ?, ?)`, contextID, anchorID, versionID); err != nil {
			return fmt.Errorf("insert review anchor head: %w", err)
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, tx, runID, "review_anchor_version", strconv.FormatInt(versionID, 10), manifest.AuditReviewAnchorCreated, metadata); err != nil {
			return err
		}
		version, err := r.getAnchorVersion(ctx, tx, versionID)
		if err != nil {
			return err
		}
		anchor = &ReviewAnchor{ID: anchorID, Label: label, WorkID: workID, CreatedAt: createdAt, Version: *version}
		return nil
	})
	return anchor, err
}

// AppendAnchorVersion appends an active replacement or tombstone using optimistic concurrency.
func (r *ReviewRepository) AppendAnchorVersion(ctx context.Context, contextID int64, anchorID string, expectedVersionID int64, state, contentHash string, page int, selectedText string, rectangles []AnchorRectangle) (*ReviewAnchor, bool, error) {
	if state != "active" && state != "deleted" {
		return nil, false, reviewValidation("anchor state must be active or deleted")
	}
	if err := validateAnchorVersion(anchorID, contentHash, state, page, selectedText, rectangles); err != nil {
		return nil, false, err
	}
	var anchor *ReviewAnchor
	changed := false
	err := r.db.withTx(ctx, func(tx *sql.Tx) error {
		var runID, workID, workRevisionID, currentID int64
		var label, createdAt, runStatus, visibility string
		err := tx.QueryRowContext(ctx, `SELECT rc.pipeline_run_id, logical.work_id, work_head.work_revision_id,
			head.anchor_version_id, COALESCE(logical.label, logical.id), logical.created_at, pr.status, pr.visibility_state
			FROM review_context_anchor_heads head
			JOIN review_contexts rc ON rc.id=head.review_context_id
			JOIN pipeline_runs pr ON pr.id=rc.pipeline_run_id
			JOIN review_anchors logical ON logical.id=head.anchor_id
			JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
			WHERE head.review_context_id=? AND head.anchor_id=?`, contextID, anchorID).
			Scan(&runID, &workID, &workRevisionID, &currentID, &label, &createdAt, &runStatus, &visibility)
		if err == sql.ErrNoRows {
			return reviewNotFound("anchor does not belong to review context")
		}
		if err != nil {
			return err
		}
		if runStatus != string(manifest.AttemptCompleted) || visibility == string(manifest.RunTrashed) {
			return reviewLifecycle("review context run is read-only")
		}
		if currentID != expectedVersionID {
			current, expected := currentID, expectedVersionID
			return &ReviewConflictError{Expected: &expected, Current: &current}
		}
		current, err := r.getAnchorVersion(ctx, tx, currentID)
		if err != nil {
			return err
		}
		if anchorsEqual(current, state, contentHash, page, selectedText, rectangles) {
			anchor = &ReviewAnchor{ID: anchorID, Label: label, WorkID: workID, CreatedAt: createdAt, Version: *current}
			if current.CreatedInContextID != contextID {
				value := current.CreatedInContextID
				anchor.InheritedFromContextID = &value
			}
			return nil
		}
		createdVersionAt := timestamp()
		var pageValue, textValue, rectanglesValue any
		if state == "active" {
			encoded, _ := json.Marshal(rectangles)
			pageValue, textValue, rectanglesValue = page, selectedText, string(encoded)
		}
		inserted, err := tx.ExecContext(ctx, `INSERT INTO review_anchor_versions
			(anchor_id, parent_version_id, created_in_context_id, work_revision_id, pdf_content_hash, state, page, selected_text, rectangles_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, anchorID, currentID, contextID, workRevisionID, contentHash,
			state, pageValue, textValue, rectanglesValue, createdVersionAt)
		if err != nil {
			return fmt.Errorf("insert review anchor version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		updated, err := tx.ExecContext(ctx, `UPDATE review_context_anchor_heads SET anchor_version_id=?
			WHERE review_context_id=? AND anchor_id=? AND anchor_version_id=?`, versionID, contextID, anchorID, currentID)
		if err != nil {
			return err
		}
		if count, _ := updated.RowsAffected(); count != 1 {
			currentValue, expected := currentID, expectedVersionID
			return &ReviewConflictError{Expected: &expected, Current: &currentValue}
		}
		action := manifest.AuditReviewAnchorVersionCreated
		if state == "deleted" {
			action = manifest.AuditReviewAnchorTombstoned
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "parent_version_id": currentID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, tx, runID, "review_anchor_version", strconv.FormatInt(versionID, 10), action, metadata); err != nil {
			return err
		}
		version, err := r.getAnchorVersion(ctx, tx, versionID)
		if err != nil {
			return err
		}
		anchor = &ReviewAnchor{ID: anchorID, Label: label, WorkID: workID, CreatedAt: createdAt, Version: *version}
		changed = true
		return nil
	})
	return anchor, changed, err
}

// GetAnchor returns one selected-context logical anchor and its current head.
func (r *ReviewRepository) GetAnchor(ctx context.Context, contextID int64, anchorID string) (*ReviewAnchor, error) {
	return r.getAnchor(ctx, contextID, anchorID)
}

// ListAnchors returns bounded active current anchors for one context work.
func (r *ReviewRepository) ListAnchors(ctx context.Context, contextID, workRevisionID int64, cursor string, limit int) ([]ReviewAnchor, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("anchor fetch limit must be between 1 and 101")
	}
	rows, err := r.db.DB.QueryContext(ctx, `SELECT logical.id, COALESCE(logical.label, logical.id), logical.work_id, logical.created_at,
		version.id, version.anchor_id, version.parent_version_id, version.created_in_context_id,
		version.work_revision_id, version.pdf_content_hash, version.state, version.page,
		substr(version.selected_text, 1, `+strconv.Itoa(anchorTextPreviewBytes)+`), COALESCE(length(CAST(version.selected_text AS BLOB)), 0),
		version.rectangles_json, version.created_at, reviewer.username, reviewer.email
		FROM review_context_anchor_heads head
		JOIN review_anchors logical ON logical.id=head.anchor_id
		JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
		JOIN review_anchor_versions version ON version.id=head.anchor_version_id AND version.state='active'
		JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
		WHERE head.review_context_id=? AND work_head.work_revision_id=? AND (?='' OR head.anchor_id>?)
		ORDER BY head.anchor_id LIMIT ?`, contextID, workRevisionID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	items := make([]ReviewAnchor, 0)
	for rows.Next() {
		item, err := scanReviewAnchor(rows, contextID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return items, nil
}

// getAnchor reads one selected context anchor head with inherited attribution.
func (r *ReviewRepository) getAnchor(ctx context.Context, contextID int64, anchorID string) (*ReviewAnchor, error) {
	var item ReviewAnchor
	var versionID int64
	err := r.db.DB.QueryRowContext(ctx, `SELECT logical.id, COALESCE(logical.label, logical.id), logical.work_id, logical.created_at, head.anchor_version_id
		FROM review_context_anchor_heads head JOIN review_anchors logical ON logical.id=head.anchor_id
		WHERE head.review_context_id=? AND head.anchor_id=?`, contextID, anchorID).
		Scan(&item.ID, &item.Label, &item.WorkID, &item.CreatedAt, &versionID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	version, err := r.getAnchorVersion(ctx, r.db.DB, versionID)
	if err != nil {
		return nil, err
	}
	item.Version = *version
	if version.CreatedInContextID != contextID {
		value := version.CreatedInContextID
		item.InheritedFromContextID = &value
	}
	return &item, nil
}

// ListAnchorVersions follows only the selected context anchor head's ancestors.
func (r *ReviewRepository) ListAnchorVersions(ctx context.Context, contextID int64, anchorID string, cursor int64, limit int) ([]ReviewAnchorVersion, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("anchor version fetch limit must be between 1 and 101")
	}
	rows, err := r.db.DB.QueryContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT anchor_version_id FROM review_context_anchor_heads WHERE review_context_id=? AND anchor_id=?
		UNION ALL
		SELECT version.parent_version_id FROM review_anchor_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	)
	SELECT version.id, version.anchor_id, version.parent_version_id, version.created_in_context_id,
		version.work_revision_id, version.pdf_content_hash, version.state, version.page,
		substr(version.selected_text, 1, `+strconv.Itoa(anchorTextPreviewBytes)+`), COALESCE(length(CAST(version.selected_text AS BLOB)), 0),
		version.rectangles_json, version.created_at, reviewer.username, reviewer.email
	FROM ancestry JOIN review_anchor_versions version ON version.id=ancestry.id
	JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
	LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
	WHERE (?=0 OR version.id<?) ORDER BY version.id DESC LIMIT ?`, contextID, anchorID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	items := make([]ReviewAnchorVersion, 0)
	for rows.Next() {
		item, err := scanReviewAnchorVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return items, nil
}

// GetAnchorVersion returns one full geometry version only when it belongs to the selected head ancestry.
func (r *ReviewRepository) GetAnchorVersion(ctx context.Context, contextID int64, anchorID string, versionID int64) (*ReviewAnchorVersion, error) {
	var exists bool
	if err := r.db.DB.QueryRowContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT anchor_version_id FROM review_context_anchor_heads WHERE review_context_id=? AND anchor_id=?
		UNION ALL
		SELECT version.parent_version_id FROM review_anchor_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	) SELECT EXISTS(SELECT 1 FROM ancestry WHERE id=?)`, contextID, anchorID, versionID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return r.getAnchorVersion(ctx, r.db.DB, versionID)
}

// scanReviewAnchor reads one bounded logical-anchor list projection.
func scanReviewAnchor(scanner interface{ Scan(...any) error }, contextID int64) (ReviewAnchor, error) {
	var item ReviewAnchor
	version, err := scanReviewAnchorVersionWithPrefix(scanner, &item)
	if err != nil {
		return item, err
	}
	item.Version = version
	if version.CreatedInContextID != contextID {
		value := version.CreatedInContextID
		item.InheritedFromContextID = &value
	}
	return item, nil
}

// scanReviewAnchorVersion reads one bounded immutable anchor-version projection.
func scanReviewAnchorVersion(scanner interface{ Scan(...any) error }) (ReviewAnchorVersion, error) {
	return scanReviewAnchorVersionWithPrefix(scanner, nil)
}

// scanReviewAnchorVersionWithPrefix shares decoding for logical-head and history projections.
func scanReviewAnchorVersionWithPrefix(scanner interface{ Scan(...any) error }, anchor *ReviewAnchor) (ReviewAnchorVersion, error) {
	var item ReviewAnchorVersion
	var parent, page sql.NullInt64
	var selectedText boundedText
	var rectanglesJSON, username, email sql.NullString
	targets := make([]any, 0, 18)
	if anchor != nil {
		targets = append(targets, &anchor.ID, &anchor.Label, &anchor.WorkID, &anchor.CreatedAt)
	}
	targets = append(targets, &item.ID, &item.AnchorID, &parent, &item.CreatedInContextID,
		&item.WorkRevisionID, &item.PDFContentHash, &item.State, &page, &selectedText.text,
		&selectedText.bytes, &rectanglesJSON, &item.CreatedAt, &username, &email)
	if err := scanner.Scan(targets...); err != nil {
		return item, err
	}
	item.ParentVersionID = nullInt64Pointer(parent)
	if page.Valid {
		value := int(page.Int64)
		item.Page = &value
	}
	item.SelectedText, item.SelectedTextTruncated = selectedText.optional()
	if rectanglesJSON.Valid {
		if err := json.Unmarshal([]byte(rectanglesJSON.String), &item.Rectangles); err != nil {
			return item, fmt.Errorf("decode anchor rectangles: %w", err)
		}
	}
	item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
	return item, nil
}

// getAnchorVersion reads one immutable geometry snapshot or tombstone.
func (r *ReviewRepository) getAnchorVersion(ctx context.Context, q queryRower, versionID int64) (*ReviewAnchorVersion, error) {
	var item ReviewAnchorVersion
	var parent sql.NullInt64
	var page sql.NullInt64
	var selectedText, rectanglesJSON, username, email sql.NullString
	err := q.QueryRowContext(ctx, `SELECT version.id, version.anchor_id, version.parent_version_id,
		version.created_in_context_id, version.work_revision_id, version.pdf_content_hash, version.state,
		version.page, version.selected_text, version.rectangles_json, version.created_at,
		reviewer.username, reviewer.email
		FROM review_anchor_versions version
		JOIN review_contexts context ON context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=context.pipeline_run_id
		WHERE version.id=?`, versionID).Scan(&item.ID, &item.AnchorID, &parent, &item.CreatedInContextID,
		&item.WorkRevisionID, &item.PDFContentHash, &item.State, &page, &selectedText, &rectanglesJSON,
		&item.CreatedAt, &username, &email)
	if err != nil {
		return nil, err
	}
	item.ParentVersionID = nullInt64Pointer(parent)
	if page.Valid {
		value := int(page.Int64)
		item.Page = &value
	}
	if selectedText.Valid {
		item.SelectedText = &selectedText.String
	}
	if rectanglesJSON.Valid {
		if err := json.Unmarshal([]byte(rectanglesJSON.String), &item.Rectangles); err != nil {
			return nil, fmt.Errorf("decode anchor rectangles: %w", err)
		}
	}
	item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
	return &item, nil
}

// validateAnchorVersion enforces safe identity, PDF binding, state, and normalized geometry.
func validateAnchorVersion(anchorID, contentHash, state string, page int, selectedText string, rectangles []AnchorRectangle) error {
	if !notes.ValidAnchorID(anchorID) {
		return reviewValidation("anchor ID has an invalid format")
	}
	decoded, err := hex.DecodeString(contentHash)
	if err != nil || len(decoded) != 32 {
		return reviewValidation("PDF content hash must be a lowercase SHA-256 value")
	}
	if contentHash != strings.ToLower(contentHash) {
		return reviewValidation("PDF content hash must be lowercase")
	}
	if state == "deleted" {
		if page != 0 || selectedText != "" || len(rectangles) != 0 {
			return reviewValidation("deleted anchor version must not contain replacement geometry")
		}
		return nil
	}
	if page < 1 {
		return reviewValidation("anchor page must be positive")
	}
	if strings.TrimSpace(selectedText) == "" {
		return reviewValidation("anchor selected text must not be blank")
	}
	if len(selectedText) > 16384 {
		return reviewValidation("anchor selected text exceeds 16384 bytes")
	}
	if len(rectangles) < 1 || len(rectangles) > 64 {
		return reviewValidation("anchor requires between 1 and 64 rectangles")
	}
	for _, rectangle := range rectangles {
		values := []float64{rectangle.X, rectangle.Y, rectangle.Width, rectangle.Height}
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
				return reviewValidation("anchor rectangle coordinates must be finite and normalized")
			}
		}
		if rectangle.Width <= 0 || rectangle.Height <= 0 || rectangle.X+rectangle.Width > 1 || rectangle.Y+rectangle.Height > 1 {
			return reviewValidation("anchor rectangle dimensions must be positive and remain within the page")
		}
	}
	return nil
}

// newAnchorID returns an opaque global identifier compatible with the note-language anchor grammar.
func newAnchorID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "a" + hex.EncodeToString(random[:]), nil
}

// anchorsEqual detects an identical save so the repository can avoid redundant history.
func anchorsEqual(current *ReviewAnchorVersion, state, contentHash string, page int, selectedText string, rectangles []AnchorRectangle) bool {
	if current.State != state || current.PDFContentHash != contentHash {
		return false
	}
	if state == "deleted" {
		return true
	}
	if current.Page == nil || *current.Page != page || current.SelectedText == nil || *current.SelectedText != selectedText || len(current.Rectangles) != len(rectangles) {
		return false
	}
	for index := range rectangles {
		if current.Rectangles[index] != rectangles[index] {
			return false
		}
	}
	return true
}

// sameNullableID compares optional immutable identifiers by value.
func sameNullableID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// IsReviewConflict reports whether an error is an optimistic head conflict.
func IsReviewConflict(err error) bool {
	var conflict *ReviewConflictError
	return errors.As(err, &conflict)
}
