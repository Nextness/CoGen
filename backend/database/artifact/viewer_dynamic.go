// viewer_dynamic.go retains the bounded dynamic run artifact evidence query.
// The relationship CTE, projection, and filter fragments are fixed SQL text;
// the order keyword is normalized to a fixed ASC or DESC constant, every user
// value is bound, and the search, role, and page inputs are validated and
// capped before assembly.
package artifact

import (
	"fmt"
	"strings"
)

// maxRunArtifactQueryLength bounds one artifact search string.
const maxRunArtifactQueryLength = 200

// maxRunArtifactRoleLength bounds one artifact relationship role filter.
const maxRunArtifactRoleLength = 100

// runArtifactRelationshipCTE is the fixed relationship CTE. Each of its five
// placeholders is the run identifier.
const runArtifactRelationshipCTE = `WITH artifact_relationships AS (
			SELECT artifact_id, 'run_role' AS relationship_role, artifact_role AS relationship_detail
			FROM run_artifacts WHERE pipeline_run_id=?
			UNION ALL
			SELECT input_artifact_id, 'step_input', step_name FROM run_steps
			WHERE pipeline_run_id=? AND input_artifact_id IS NOT NULL
			UNION ALL
			SELECT output_artifact_id, 'step_output', step_name FROM run_steps
			WHERE pipeline_run_id=? AND output_artifact_id IS NOT NULL
			UNION ALL
			SELECT ce.payload_artifact_id, 'cache_payload', ce.provider || ':' || ce.namespace
			FROM run_cache_uses use_record
			JOIN cache_entries ce ON ce.id=use_record.cache_entry_id
			WHERE use_record.pipeline_run_id=? AND ce.payload_artifact_id IS NOT NULL
			UNION ALL
			SELECT candidate.payload_artifact_id, 'identity_candidate_payload', resolution.provider
			FROM author_identity_resolutions resolution
			JOIN author_identity_candidates candidate ON candidate.identity_resolution_id=resolution.id
			WHERE resolution.pipeline_run_id=? AND candidate.payload_artifact_id IS NOT NULL
		), selected_artifacts AS (
			SELECT DISTINCT artifact_id FROM artifact_relationships
		)`

// runArtifactRunArgs returns count repeated run identifier arguments.
func runArtifactRunArgs(runID int64, count int) []any {
	args := make([]any, count)
	for index := range args {
		args[index] = runID
	}
	return args
}

// runArtifactFilterClauses builds the parameterized search and role filters.
func runArtifactFilterClauses(filter RunArtifactFilter) (string, []any, error) {
	if len(filter.Query) > maxRunArtifactQueryLength {
		return "", nil, fmt.Errorf("list run artifact evidence: query must be at most %d characters", maxRunArtifactQueryLength)
	}
	if len(filter.Role) > maxRunArtifactRoleLength {
		return "", nil, fmt.Errorf("list run artifact evidence: role must be at most %d characters", maxRunArtifactRoleLength)
	}
	where := " WHERE 1=1"
	args := []any{}
	if filter.Query != "" {
		where += ` AND (LOWER(a.content_hash) LIKE ? OR LOWER(a.content_type) LIKE ?
			OR EXISTS (SELECT 1 FROM artifact_relationships searchable
				WHERE searchable.artifact_id=a.id AND (LOWER(searchable.relationship_role) LIKE ? OR LOWER(searchable.relationship_detail) LIKE ?)))`
		pattern := "%" + strings.ToLower(filter.Query) + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if filter.Role != "" {
		where += ` AND EXISTS (SELECT 1 FROM artifact_relationships filtered_role
			WHERE filtered_role.artifact_id=a.id AND filtered_role.relationship_role=?)`
		args = append(args, filter.Role)
	}
	return where, args, nil
}

// sqlDirection returns the validated ascending/descending SQL keyword.
func sqlDirection(order string) string {
	if strings.EqualFold(order, "DESC") {
		return "DESC"
	}
	return "ASC"
}
