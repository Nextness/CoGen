// decision.go provides the immutable work-review decision versions, head
// compare-and-swap, ancestry reads, and the vocabulary validation that guards
// complete review states.
package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"analysis/manifest"
)

// GetWorkReview returns the complete current state for one context work revision.
func (s *Store) GetWorkReview(ctx context.Context, contextID, workRevisionID int64) (*WorkReviewState, error) {
	var state WorkReviewState
	var versionID sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT review_context_id, work_id, work_revision_id, review_version_id
		FROM review_context_work_heads WHERE review_context_id=? AND work_revision_id=?`, contextID, workRevisionID).
		Scan(&state.ContextID, &state.WorkID, &state.WorkRevisionID, &versionID)
	if err == sql.ErrNoRows {
		return nil, reviewNotFound("work revision does not belong to review context")
	}
	if err != nil {
		return nil, err
	}
	if versionID.Valid {
		version, err := s.getWorkReviewVersion(ctx, s.db, versionID.Int64)
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
func (s *Store) AppendWorkReview(ctx context.Context, contextID, workRevisionID int64, expectedVersionID *int64, status string, substatuses []string, reason *string) (*WorkReviewState, bool, error) {
	canonical, normalizedReason, err := validateReviewState(status, substatuses, reason)
	if err != nil {
		return nil, false, err
	}
	var result *WorkReviewState
	changed := false
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		runID, workID, current, err := s.mutableWorkHead(ctx, tx, contextID, workRevisionID)
		if err != nil {
			return err
		}
		if !sameNullableID(expectedVersionID, current) {
			return &ConflictError{Expected: expectedVersionID, Current: current}
		}
		if current == nil && status == "not_evaluated" && len(canonical) == 0 && normalizedReason == nil {
			result = &WorkReviewState{ContextID: contextID, WorkID: workID, WorkRevisionID: workRevisionID}
			return nil
		}
		beforeState := workReviewAuditState{Status: "not_evaluated", Substatuses: []string{}}
		if current != nil {
			previous, err := s.getWorkReviewVersion(ctx, tx, *current)
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
			VALUES (?, ?, ?, ?, ?, ?, ?)`, workID, workRevisionID, contextID, nullablePointer(current), status, nullablePointer(normalizedReason), timestamp())
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
			versionID, contextID, workRevisionID, nullablePointer(current), nullablePointer(current))
		if err != nil {
			return err
		}
		if count, _ := updated.RowsAffected(); count != 1 {
			return &ConflictError{Expected: expectedVersionID, Current: current}
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "parent_version_id": current, "new_version_id": versionID}
		afterState := workReviewAuditState{Status: status, Reason: normalizedReason, Substatuses: canonical}
		if err := insertReviewChangeAudit(ctx, tx, runID, "work_review_version", strconv.FormatInt(versionID, 10), manifest.AuditWorkReviewVersionCreated, beforeState, afterState, metadata); err != nil {
			return err
		}
		version, err := s.getWorkReviewVersion(ctx, tx, versionID)
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
func (s *Store) mutableWorkHead(ctx context.Context, tx *sql.Tx, contextID, workRevisionID int64) (int64, int64, *int64, error) {
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
func (s *Store) ListWorkReviewVersions(ctx context.Context, contextID, workRevisionID, cursor int64, limit int) ([]WorkReviewVersion, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("version fetch limit must be between 1 and 101")
	}
	rows, err := s.db.QueryContext(ctx, `WITH RECURSIVE ancestry(id) AS (
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
func (s *Store) GetWorkReviewVersion(ctx context.Context, contextID, workRevisionID, versionID int64) (*WorkReviewVersion, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `WITH RECURSIVE ancestry(id) AS (
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
	return s.getWorkReviewVersion(ctx, s.db, versionID)
}

// getWorkReviewVersion reads one immutable version with canonical sub-statuses and reviewer attribution.
func (s *Store) getWorkReviewVersion(ctx context.Context, q reviewQuerier, id int64) (*WorkReviewVersion, error) {
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
