// normalized_revision.go holds the viewer-local analysis-ready revision
// predicate. The work family and the review schema inputs keep their own
// copies; TestNormalizedRevisionPredicateConformance compares every copy
// against accepted, discarded, superseded, and missing-normalization cases.
package server

import "fmt"

// normalizedRevisionPredicate returns SQL selecting the latest valid normalize
// revision for each run and work.
func normalizedRevisionPredicate(alias string) string {
	return fmt.Sprintf(`%[1]s.producer_stage='normalize'
		AND %[1]s.id=(SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
			WHERE normalized_candidate.pipeline_run_id=%[1]s.pipeline_run_id
			AND normalized_candidate.work_id=%[1]s.work_id AND normalized_candidate.producer_stage='normalize')
		AND EXISTS (SELECT 1 FROM run_work_stages current_validation
			WHERE current_validation.pipeline_run_id=%[1]s.pipeline_run_id
			AND current_validation.work_id=%[1]s.work_id
			AND current_validation.stage_name='validate' AND current_validation.outcome='valid')`, alias)
}
