// viewer_dynamic.go retains the article detail logical-work event predicate.
// The predicate is fixed SQL text assembled once and shared by the count and
// list statements so the two cannot diverge; every caller value is bound and
// no caller value is interpolated into SQL text.
package audit

import "strconv"

// articleDetailEventCondition returns the run-scoped logical-work event
// predicate and its bound arguments.
func articleDetailEventCondition(workID, runID int64) (string, []any) {
	entityID := strconv.FormatInt(workID, 10)
	condition := `(entity_type='work_revision' AND entity_id IN (
			SELECT CAST(id AS TEXT) FROM work_revisions WHERE work_id=? AND pipeline_run_id=?))
		OR (entity_type='work' AND entity_id=? AND (pipeline_run_id=? OR (pipeline_run_id IS NULL AND action LIKE 'pdf_%')))
		OR (entity_type='work_review_version' AND pipeline_run_id=? AND entity_id IN (
			SELECT CAST(review.id AS TEXT) FROM work_review_versions review
			JOIN work_revisions revision ON revision.id=review.work_revision_id
			WHERE review.work_id=? AND revision.pipeline_run_id=?))
		OR (entity_type='review_note_version' AND pipeline_run_id=? AND entity_id IN (
			SELECT CAST(version.id AS TEXT) FROM review_note_versions version
			JOIN review_notes note ON note.id=version.note_id
			JOIN review_contexts context ON context.id=version.created_in_context_id
			WHERE note.work_id=? AND context.pipeline_run_id=?))
		OR (entity_type='review_anchor_version' AND pipeline_run_id=? AND entity_id IN (
			SELECT CAST(version.id AS TEXT) FROM review_anchor_versions version
			JOIN review_anchors anchor ON anchor.id=version.anchor_id
			JOIN review_contexts context ON context.id=version.created_in_context_id
			WHERE anchor.work_id=? AND context.pipeline_run_id=?))
		OR (entity_type='review_context' AND pipeline_run_id=?)`
	args := []any{
		workID, runID, entityID, runID,
		runID, workID, runID,
		runID, workID, runID,
		runID, workID, runID,
		runID,
	}
	return condition, args
}
