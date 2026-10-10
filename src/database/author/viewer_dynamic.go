// viewer_dynamic.go retains the bounded dynamic identity-evidence query. Sort
// identifiers come from a closed map, the search projection is fixed, and
// every user value is bound; no caller value is interpolated into SQL text.
package author

import (
	"strings"
)

// identityEvidenceSortFields is the closed allowlist of identity-evidence sort expressions.
var identityEvidenceSortFields = map[string]string{
	"id":              "r.id",
	"status":          "r.status",
	"citation_name":   "r.queried_citation_name",
	"article_title":   "article_title",
	"doi":             "doi",
	"candidate_count": "candidate_count",
	"resolved_at":     "r.resolved_at",
}

// identityEvidenceSearchFields is the fixed searchable projection for identity evidence.
var identityEvidenceSearchFields = []string{"r.queried_citation_name", "ao.citation_name", "wr.title", "w.doi"}

// identityEvidenceSelect is the fixed identity-evidence projection.
const identityEvidenceSelect = `SELECT r.id AS resolution_id, r.status, r.provider, r.queried_citation_name,
        r.error_message, r.resolved_at, ao.id AS author_occurrence_id, ao.orcid AS observed_orcid,
        ao.person_id, wr.title AS article_title, w.doi, wr.id AS work_revision_id,
        captured.id AS evidence_revision_id, captured.producer_stage AS evidence_stage,
        COUNT(DISTINCT c.id) AS candidate_count`

// identityEvidenceFrom returns the fixed identity-evidence join tree. Its
// first placeholder is the run identifier for the evidence membership
// subquery.
func identityEvidenceFrom() string {
	return `FROM author_identity_resolutions r
        JOIN author_occurrences ao ON ao.id=r.author_occurrence_id
        LEFT JOIN (
            SELECT a.author_occurrence_id, source.pipeline_run_id, source.work_id, MAX(source.id) AS revision_id
            FROM authorships a JOIN work_revisions source ON source.id=a.work_revision_id
            WHERE source.pipeline_run_id=?
            GROUP BY a.author_occurrence_id, source.pipeline_run_id, source.work_id
        ) evidence ON evidence.author_occurrence_id=ao.id AND evidence.pipeline_run_id=r.pipeline_run_id
        LEFT JOIN work_revisions captured ON captured.id=evidence.revision_id
        LEFT JOIN work_revisions wr ON wr.id=COALESCE((
            SELECT current.id FROM work_revisions current
            WHERE current.work_id=evidence.work_id AND current.pipeline_run_id=r.pipeline_run_id
            AND ` + normalizedRevisionPredicate("current") + `), captured.id)
        LEFT JOIN works w ON w.id=wr.work_id
        LEFT JOIN author_identity_candidates c ON c.identity_resolution_id=r.id`
}

// identityEvidenceQuery builds the parameterized identity-evidence predicate.
// The arguments are the evidence run identifier, the resolution run
// identifier, and the optional search patterns.
func identityEvidenceQuery(filter IdentityEvidenceFilter) (string, string, []any) {
	where, searchArgs := scopedSearch("r.pipeline_run_id=?", identityEvidenceSearchFields, filter.RunID, filter.Query)
	args := append([]any{filter.RunID}, searchArgs...)
	return identityEvidenceFrom(), where, args
}

// scopedSearch appends bound LIKE conditions for one optional search query.
func scopedSearch(base string, fields []string, runID int64, query string) (string, []any) {
	args := []any{runID}
	if query == "" {
		return base, args
	}
	conditions := make([]string, 0, len(fields))
	needle := "%" + strings.ToLower(query) + "%"
	for _, field := range fields {
		conditions = append(conditions, "LOWER(COALESCE("+field+", '')) LIKE ?")
		args = append(args, needle)
	}
	return base + " AND (" + strings.Join(conditions, " OR ") + ")", args
}

// sqlDirection returns the validated ascending/descending SQL keyword.
func sqlDirection(order string) string {
	if strings.EqualFold(order, "DESC") {
		return "DESC"
	}
	return "ASC"
}
