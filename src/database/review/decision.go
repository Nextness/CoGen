// decision.go provides the immutable work-review decision versions, head
// compare-and-swap, ancestry reads, and the vocabulary validation that guards
// complete review states.
package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	generated "analysis/database/review/internal/sql"
	"analysis/manifest"
)

// GetWorkReview returns the complete current state for one context work revision.
func (s *Store) GetWorkReview(ctx context.Context, contextID, workRevisionID int64) (*WorkReviewState, error) {
	row, err := s.queries.GetWorkReviewHead(ctx, generated.GetWorkReviewHeadParams{
		ReviewContextID: contextID,
		WorkRevisionID:  workRevisionID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, reviewNotFound("work revision does not belong to review context")
	}
	if err != nil {
		return nil, err
	}
	state := WorkReviewState{ContextID: row.ReviewContextID, WorkID: row.WorkID, WorkRevisionID: row.WorkRevisionID}
	if row.ReviewVersionID.Valid {
		version, err := s.getWorkReviewVersion(ctx, s.queries, row.ReviewVersionID.Int64)
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
	err = s.withTx(ctx, func(queries *generated.Queries) error {
		runID, workID, current, err := s.mutableWorkHead(ctx, queries, contextID, workRevisionID)
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
			previous, err := s.getWorkReviewVersion(ctx, queries, *current)
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
		inserted, err := queries.InsertWorkReviewVersion(ctx, generated.InsertWorkReviewVersionParams{
			WorkID:             workID,
			WorkRevisionID:     workRevisionID,
			CreatedInContextID: contextID,
			ParentVersionID:    optionalInt64(current),
			Status:             status,
			Reason:             optionalString(normalizedReason),
			CreatedAt:          timestamp(),
		})
		if err != nil {
			return fmt.Errorf("insert work review version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		for _, substatus := range canonical {
			if err := queries.InsertWorkReviewSubstatus(ctx, generated.InsertWorkReviewSubstatusParams{
				ReviewVersionID: versionID,
				SubStatus:       substatus,
			}); err != nil {
				return fmt.Errorf("insert work review sub-status: %w", err)
			}
		}
		var updated sql.Result
		if current == nil {
			updated, err = queries.UpdateWorkReviewHeadCASNull(ctx, generated.UpdateWorkReviewHeadCASNullParams{
				ReviewVersionID: sql.NullInt64{Int64: versionID, Valid: true},
				ReviewContextID: contextID,
				WorkRevisionID:  workRevisionID,
			})
		} else {
			updated, err = queries.UpdateWorkReviewHeadCASValue(ctx, generated.UpdateWorkReviewHeadCASValueParams{
				ReviewVersionID:   sql.NullInt64{Int64: versionID, Valid: true},
				ReviewContextID:   contextID,
				WorkRevisionID:    workRevisionID,
				ExpectedVersionID: sql.NullInt64{Int64: *current, Valid: true},
			})
		}
		if err != nil {
			return err
		}
		if count, _ := updated.RowsAffected(); count != 1 {
			return &ConflictError{Expected: expectedVersionID, Current: current}
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "parent_version_id": current, "new_version_id": versionID}
		afterState := workReviewAuditState{Status: status, Reason: normalizedReason, Substatuses: canonical}
		if err := insertReviewChangeAudit(ctx, queries, runID, "work_review_version", strconv.FormatInt(versionID, 10), manifest.AuditWorkReviewVersionCreated, beforeState, afterState, metadata); err != nil {
			return err
		}
		version, err := s.getWorkReviewVersion(ctx, queries, versionID)
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
func (s *Store) mutableWorkHead(ctx context.Context, queries *generated.Queries, contextID, workRevisionID int64) (int64, int64, *int64, error) {
	row, err := queries.GetMutableWorkReviewHead(ctx, generated.GetMutableWorkReviewHeadParams{
		ReviewContextID: contextID,
		WorkRevisionID:  workRevisionID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil, reviewNotFound("work revision does not belong to review context")
	}
	if err != nil {
		return 0, 0, nil, err
	}
	if row.Status != string(manifest.AttemptCompleted) || row.VisibilityState == string(manifest.RunTrashed) {
		return 0, 0, nil, reviewLifecycle("review context run is read-only")
	}
	return row.PipelineRunID, row.WorkID, nullInt64Pointer(row.ReviewVersionID), nil
}

// ListWorkReviewVersions follows only the selected head's ancestor chain.
func (s *Store) ListWorkReviewVersions(ctx context.Context, contextID, workRevisionID, cursor int64, limit int) ([]WorkReviewVersion, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("version fetch limit must be between 1 and 101")
	}
	rows, err := s.queries.ListWorkReviewVersionAncestry(ctx, generated.ListWorkReviewVersionAncestryParams{
		PreviewBytes:    reviewTextPreviewBytes,
		Cursor:          cursor,
		PageLimit:       int64(limit),
		ReviewContextID: contextID,
		WorkRevisionID:  workRevisionID,
	})
	if err != nil {
		return nil, err
	}
	versions := make([]WorkReviewVersion, 0, len(rows))
	for _, row := range rows {
		item := WorkReviewVersion{
			ID:                 row.ID,
			WorkID:             row.WorkID,
			WorkRevisionID:     row.WorkRevisionID,
			CreatedInContextID: row.CreatedInContextID,
			ParentVersionID:    nullInt64Pointer(row.ParentVersionID),
			Status:             row.Status,
			CreatedAt:          row.CreatedAt,
			ReviewerDisplay:    reviewerDisplay(row.ReviewerUsername, row.ReviewerEmail),
		}
		item.Reason, item.ReasonTruncated = boundedPreview(row.ReasonPreview, row.ReasonBytes, row.HasReason != 0).optional()
		if err := json.Unmarshal([]byte(row.SubstatusesJson), &item.Substatuses); err != nil {
			return nil, err
		}
		versions = append(versions, item)
	}
	return versions, nil
}

// GetWorkReviewVersion returns one full version only when it belongs to the selected head ancestry.
func (s *Store) GetWorkReviewVersion(ctx context.Context, contextID, workRevisionID, versionID int64) (*WorkReviewVersion, error) {
	exists, err := s.queries.ReviewWorkReviewVersionInAncestry(ctx, generated.ReviewWorkReviewVersionInAncestryParams{
		VersionID:       versionID,
		ReviewContextID: contextID,
		WorkRevisionID:  workRevisionID,
	})
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return s.getWorkReviewVersion(ctx, s.queries, versionID)
}

// getWorkReviewVersion reads one immutable version with canonical sub-statuses and reviewer attribution.
func (s *Store) getWorkReviewVersion(ctx context.Context, queries *generated.Queries, id int64) (*WorkReviewVersion, error) {
	row, err := queries.GetWorkReviewVersionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item := WorkReviewVersion{
		ID:                 row.ID,
		WorkID:             row.WorkID,
		WorkRevisionID:     row.WorkRevisionID,
		CreatedInContextID: row.CreatedInContextID,
		ParentVersionID:    nullInt64Pointer(row.ParentVersionID),
		Status:             row.Status,
		CreatedAt:          row.CreatedAt,
		ReviewerDisplay:    reviewerDisplay(row.ReviewerUsername, row.ReviewerEmail),
	}
	if row.Reason.Valid {
		item.Reason = &row.Reason.String
	}
	substatuses, err := queries.ListWorkReviewSubstatuses(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Substatuses = make([]string, 0, len(substatuses))
	item.Substatuses = append(item.Substatuses, substatuses...)
	return &item, nil
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
