// viewer_dynamic.go provides the retained dynamic audit filter builder. It
// maps closed enum values to fixed SQL fragments, binds every user value, and
// validates and caps every list and filter input before assembly; no caller
// value is ever interpolated into SQL text.
package audit

import (
	"fmt"
	"strings"
)

// maxAuditFilterValues bounds one audit facet value list.
const maxAuditFilterValues = 100

// maxAuditFilterValueLength bounds one audit filter value.
const maxAuditFilterValueLength = 200

// maxAuditReviewReasonLength bounds the review reason filter.
const maxAuditReviewReasonLength = 1000

// auditCategoryPredicates maps each supported presentation category to its
// fixed action predicate. The map is the allowlist for category filters.
var auditCategoryPredicates = map[string]string{
	"pipeline":   "(action LIKE 'pipeline_%' OR action IN ('plan_created','duplicate_plan_skipped','run_started','step_reused','run_completed','run_failed','run_trashed','run_restored','run_purged','revision_config_changed'))",
	"enrichment": "action IN ('field_enriched','cache_hit','network_fetch')",
	"validation": "action LIKE 'validation_%'",
	"pdf":        "action LIKE 'pdf_%'",
	"review":     "(action LIKE 'review_%' OR action LIKE 'work_review_%')",
}

// auditFacetColumns maps each supported facet name to its allowlisted column.
// The map is the only source of facet column identifiers.
var auditFacetColumns = map[string]string{
	"actor":       "actor",
	"action":      "action",
	"entity_type": "entity_type",
}

// validateAuditFilter validates the bounded audit filter inputs before any SQL
// text is assembled. It rejects oversized value lists and filter strings.
func validateAuditFilter(filter Filter) error {
	for _, values := range []struct {
		name   string
		values []string
	}{
		{"entity_type", filter.EntityTypes},
		{"action", filter.Actions},
		{"actor", filter.Actors},
		{"category", filter.Categories},
	} {
		if len(values.values) > maxAuditFilterValues {
			return fmt.Errorf("audit filter: %s accepts at most %d values", values.name, maxAuditFilterValues)
		}
		for _, value := range values.values {
			if len(value) > maxAuditFilterValueLength {
				return fmt.Errorf("audit filter: %s value must be at most %d characters", values.name, maxAuditFilterValueLength)
			}
		}
	}
	for _, value := range []struct {
		name  string
		value string
		limit int
	}{
		{"entity_id", filter.EntityID, maxAuditFilterValueLength},
		{"stage", filter.Stage, maxAuditFilterValueLength},
		{"outcome", filter.Outcome, maxAuditFilterValueLength},
		{"review_status", filter.ReviewStatus, maxAuditFilterValueLength},
		{"review_reason", filter.ReviewReason, maxAuditReviewReasonLength},
		{"review_substatus", filter.ReviewSubstatus, maxAuditFilterValueLength},
		{"query", filter.Query, maxAuditFilterValueLength},
	} {
		if len(value.value) > value.limit {
			return fmt.Errorf("audit filter: %s must be at most %d characters", value.name, value.limit)
		}
	}
	return nil
}

// auditFilterClauses builds the parameterized WHERE clauses for the viewer's
// audit filters. Only allowlisted columns and operators are assembled.
func auditFilterClauses(filter Filter) ([]string, []any) {
	clauses := make([]string, 0, 10)
	args := make([]any, 0, 10)
	if filter.EntityID != "" {
		clauses = append(clauses, "entity_id=?")
		args = append(args, filter.EntityID)
	}
	for _, values := range []struct {
		column string
		values []string
	}{
		{"entity_type", filter.EntityTypes},
		{"action", filter.Actions},
		{"actor", filter.Actors},
	} {
		if len(values.values) > 0 {
			clause, valueArgs := auditInClause(values.column, values.values)
			clauses = append(clauses, clause)
			args = append(args, valueArgs...)
		}
	}
	if len(filter.Categories) > 0 {
		categoryClauses := make([]string, 0, len(filter.Categories))
		for _, category := range filter.Categories {
			if predicate, ok := auditCategoryPredicates[category]; ok {
				categoryClauses = append(categoryClauses, predicate)
			}
		}
		if len(categoryClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(categoryClauses, " OR ")+")")
		}
	}
	if filter.Stage != "" {
		clauses = append(clauses, "CASE WHEN json_valid(metadata_json) THEN COALESCE(json_extract(metadata_json, '$.stage'), json_extract(metadata_json, '$.stage_name'), '') ELSE '' END=?")
		args = append(args, filter.Stage)
	}
	if filter.Outcome != "" {
		clauses = append(clauses, "CASE WHEN json_valid(metadata_json) THEN COALESCE(json_extract(metadata_json, '$.outcome'), json_extract(metadata_json, '$.status'), '') ELSE '' END=?")
		args = append(args, filter.Outcome)
	}
	if filter.ReviewStatus != "" {
		clauses = append(clauses, "CASE WHEN json_valid(after_json) THEN COALESCE(json_extract(after_json, '$.status'), '') ELSE '' END=?")
		args = append(args, filter.ReviewStatus)
	}
	if filter.ReviewReason != "" {
		clauses = append(clauses, "CASE WHEN json_valid(after_json) THEN COALESCE(json_extract(after_json, '$.reason'), '') ELSE '' END=?")
		args = append(args, filter.ReviewReason)
	}
	if filter.ReviewSubstatus != "" {
		clauses = append(clauses, "json_valid(after_json) AND EXISTS (SELECT 1 FROM json_each(after_json, '$.sub_statuses') WHERE value=?)")
		args = append(args, filter.ReviewSubstatus)
	}
	if filter.Query != "" {
		clauses = append(clauses, "(LOWER(actor) LIKE ? OR LOWER(entity_type) LIKE ? OR LOWER(entity_id) LIKE ? OR LOWER(action) LIKE ?)")
		needle := "%" + strings.ToLower(filter.Query) + "%"
		args = append(args, needle, needle, needle, needle)
	}
	if filter.RunID > 0 {
		clause, valueArgs := auditRunScopeClause(filter)
		clauses = append(clauses, clause)
		args = append(args, valueArgs...)
	}
	return clauses, args
}

// auditRunScopeClause builds the run scope predicate, including the
// run-scoped PDF evidence variants.
func auditRunScopeClause(filter Filter) (string, []any) {
	if filter.PDFSelected && filter.PDFScope == "workspace" {
		return "(pipeline_run_id=? OR (pipeline_run_id IS NULL AND action LIKE 'pdf_%'))", []any{filter.RunID}
	}
	if filter.PDFSelected {
		return `(pipeline_run_id=? OR (
			pipeline_run_id IS NULL AND action LIKE 'pdf_%' AND entity_type='work'
			AND EXISTS (SELECT 1 FROM work_revisions scoped_revision
				WHERE scoped_revision.pipeline_run_id=?
				AND CAST(scoped_revision.work_id AS TEXT)=audit_events.entity_id)))`, []any{filter.RunID, filter.RunID}
	}
	return "pipeline_run_id=?", []any{filter.RunID}
}

// auditInClause builds a parameterized SQL IN clause for validated audit facet values.
func auditInClause(column string, values []string) (string, []any) {
	markers := make([]string, len(values))
	args := make([]any, len(values))
	for index, value := range values {
		markers[index] = "?"
		args[index] = value
	}
	return column + " IN (" + strings.Join(markers, ",") + ")", args
}

// auditWhere joins audit predicates into an optional SQL WHERE clause.
func auditWhere(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(clauses, " AND ")
}
