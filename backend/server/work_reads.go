// work_reads.go converts work family application types into the viewer's
// stable JSON row shapes. Nullable fields and empty collections are made
// explicit here so family types never leak into responses.
package server

import (
	"analysis/backend/database/work"
	"analysis/internal/jsonvalue"
)

// corpusReferenceRow maps one work family corpus reference into a viewer row.
func corpusReferenceRow(reference *work.CorpusReference) map[string]any {
	row := map[string]any{
		"id":               reference.ID,
		"work_revision_id": reference.WorkRevisionID,
		"mention_order":    reference.MentionOrder,
		"doi":              jsonvalue.Optional(reference.DOI),
		"title":            jsonvalue.Optional(reference.Title),
		"author":           jsonvalue.Optional(reference.Author),
		"year":             jsonvalue.Optional(reference.Year),
		"source":           jsonvalue.Optional(reference.Source),
		"citing_title":     jsonvalue.Optional(reference.CitingTitle),
		"created_at":       reference.CreatedAt,
	}
	if reference.ResolvedWorkID != nil {
		row["resolved_work_id"] = *reference.ResolvedWorkID
	} else {
		row["resolved_work_id"] = nil
	}
	return row
}

// corpusAuthorRow maps one work family corpus author into a viewer row.
func corpusAuthorRow(author *work.CorpusAuthor) map[string]any {
	row := map[string]any{
		"id":                author.ID,
		"citation_name":     author.CitationName,
		"first_name":        jsonvalue.Optional(author.FirstName),
		"last_name":         jsonvalue.Optional(author.LastName),
		"orcid":             jsonvalue.Optional(author.ORCID),
		"article_count":     author.ArticleCount,
		"affiliation_count": author.AffiliationCount,
		"created_at":        author.CreatedAt,
	}
	if author.PersonID != nil {
		row["person_id"] = *author.PersonID
	} else {
		row["person_id"] = nil
	}
	return row
}

// stageOutcomeRow maps one work family stage outcome into a viewer row.
func stageOutcomeRow(outcome *work.StageRow) map[string]any {
	return map[string]any{
		"id":         outcome.ID,
		"work_id":    outcome.WorkID,
		"stage_name": outcome.StageName,
		"outcome":    outcome.Outcome,
		"reason":     jsonvalue.Optional(outcome.Reason),
		"created_at": outcome.CreatedAt,
		"updated_at": outcome.UpdatedAt,
	}
}

// stageSummaryRow maps one work family stage summary into a viewer row.
func stageSummaryRow(summary *work.StageSummary) map[string]any {
	outcomes := make(map[string]int64, len(summary.Outcomes))
	for name, count := range summary.Outcomes {
		outcomes[name] = count
	}
	return map[string]any{
		"stage_name":        summary.StageName,
		"total_records":     summary.TotalRecords,
		"outcomes":          outcomes,
		"first_recorded_at": summary.FirstRecordedAt,
		"last_recorded_at":  summary.LastRecordedAt,
	}
}

// evaluationRow maps one work family evaluation row into a viewer row.
func evaluationRow(row *work.EvaluationRow) map[string]any {
	result := map[string]any{
		"id":                  row.ID,
		"work_id":             row.WorkID,
		"title":               jsonvalue.Optional(row.Title),
		"year":                jsonvalue.Optional(row.Year),
		"journal":             jsonvalue.Optional(row.Journal),
		"publisher":           jsonvalue.Optional(row.Publisher),
		"source":              jsonvalue.Optional(row.Source),
		"doi":                 jsonvalue.Optional(row.DOI),
		"validation_status":   jsonvalue.Optional(row.ValidationStatus),
		"citation_count":      jsonvalue.Optional(row.CitationCount),
		"reference_count":     jsonvalue.Optional(row.ReferenceCount),
		"producer_stage":      row.ProducerStage,
		"created_at":          row.CreatedAt,
		"abstract":            jsonvalue.Optional(row.Abstract),
		"keywords":            jsonvalue.Optional(row.Keywords),
		"keywords_plus":       jsonvalue.Optional(row.KeywordsPlus),
		"authors":             jsonvalue.Optional(row.Authors),
		"work_revision_id":    row.WorkRevisionID,
		"review_status":       row.ReviewStatus,
		"review_inherited":    row.ReviewInherited,
		"review_sub_statuses": row.ReviewSubStatuses,
	}
	if row.ReviewVersionID != nil {
		result["review_version_id"] = *row.ReviewVersionID
	} else {
		result["review_version_id"] = nil
	}
	if row.ReviewCreatedInContextID != nil {
		result["review_created_in_context_id"] = *row.ReviewCreatedInContextID
	} else {
		result["review_created_in_context_id"] = nil
	}
	return result
}

// evaluationSummaryPayload maps one work family evaluation summary into the viewer payload.
func evaluationSummaryPayload(summary *work.EvaluationSummary) map[string]any {
	return map[string]any{
		"total":             summary.Total,
		"reviewed":          summary.Reviewed,
		"unreviewed":        summary.Unreviewed,
		"pdf_available":     summary.PDFAvailable,
		"pdf_not_available": summary.PDFNotAvailable,
		"percent_reviewed":  summary.PercentReviewed,
		"facets": map[string]any{
			"review_status": facetRows(summary.Facets.ReviewStatus),
			"source":        facetRows(summary.Facets.Source),
			"review_source": facetRows(summary.Facets.ReviewSource),
			"qualifier":     facetRows(summary.Facets.Qualifier),
			"pdf_status":    facetRows(summary.Facets.PDFStatus),
		},
	}
}

// facetRows maps work family facet counts into viewer rows.
func facetRows(facets []*work.FacetCount) []map[string]any {
	rows := make([]map[string]any, 0, len(facets))
	for _, facet := range facets {
		rows = append(rows, map[string]any{"value": facet.Value, "count": facet.Count})
	}
	return rows
}

// graphArticleRow maps one work family graph article into a viewer row.
func graphArticleRow(article *work.GraphArticle) map[string]any {
	return map[string]any{
		"id":      article.ID,
		"work_id": article.WorkID,
		"title":   jsonvalue.Optional(article.Title),
		"year":    jsonvalue.Optional(article.Year),
		"source":  jsonvalue.Optional(article.Source),
		"doi":     jsonvalue.Optional(article.DOI),
	}
}

// articleRevisionRow maps one work family article revision into a viewer row.
func articleRevisionRow(revision *work.ArticleRevision) map[string]any {
	return map[string]any{
		"id":                   revision.ID,
		"work_id":              revision.WorkID,
		"pipeline_run_id":      revision.PipelineRunID,
		"field_schema_version": revision.FieldSchemaVersion,
		"payload_hash":         revision.PayloadHash,
		"title":                jsonvalue.Optional(revision.Title),
		"abstract":             jsonvalue.Optional(revision.Abstract),
		"year":                 jsonvalue.Optional(revision.Year),
		"journal":              jsonvalue.Optional(revision.Journal),
		"publisher":            jsonvalue.Optional(revision.Publisher),
		"source":               jsonvalue.Optional(revision.Source),
		"keywords":             jsonvalue.Optional(revision.Keywords),
		"keywords_plus":        jsonvalue.Optional(revision.KeywordsPlus),
		"citation_count":       jsonvalue.Optional(revision.CitationCount),
		"reference_count":      jsonvalue.Optional(revision.ReferenceCount),
		"extension_data":       jsonvalue.Optional(revision.ExtensionData),
		"producer_stage":       revision.ProducerStage,
		"created_at":           revision.CreatedAt,
		"doi":                  jsonvalue.Optional(revision.DOI),
	}
}

// articleAuthorRow maps one work family article author into a viewer row.
func articleAuthorRow(author *work.ArticleAuthor) map[string]any {
	row := map[string]any{
		"id":            author.ID,
		"citation_name": author.CitationName,
		"first_name":    jsonvalue.Optional(author.FirstName),
		"last_name":     jsonvalue.Optional(author.LastName),
		"orcid":         jsonvalue.Optional(author.ORCID),
		"author_order":  author.AuthorOrder,
		"affiliation":   jsonvalue.Optional(author.Affiliation),
	}
	if author.PersonID != nil {
		row["person_id"] = *author.PersonID
	} else {
		row["person_id"] = nil
	}
	return row
}

// articleStageRow maps one work family article stage outcome into a viewer row.
func articleStageRow(outcome *work.StageRow) map[string]any {
	return map[string]any{
		"id":         outcome.ID,
		"stage_name": outcome.StageName,
		"outcome":    outcome.Outcome,
		"reason":     jsonvalue.Optional(outcome.Reason),
		"created_at": outcome.CreatedAt,
		"updated_at": outcome.UpdatedAt,
	}
}

// articleReferenceRow maps one work family article reference into a viewer row.
func articleReferenceRow(reference *work.ArticleReference) map[string]any {
	row := map[string]any{
		"id":               reference.ID,
		"work_revision_id": reference.WorkRevisionID,
		"mention_order":    reference.MentionOrder,
		"doi":              jsonvalue.Optional(reference.DOI),
		"title":            jsonvalue.Optional(reference.Title),
		"author":           jsonvalue.Optional(reference.Author),
		"year":             jsonvalue.Optional(reference.Year),
		"source":           jsonvalue.Optional(reference.Source),
		"created_at":       reference.CreatedAt,
	}
	if reference.ResolvedWorkID != nil {
		row["resolved_work_id"] = *reference.ResolvedWorkID
	} else {
		row["resolved_work_id"] = nil
	}
	if reference.ResolvedRevisionID != nil {
		row["resolved_revision_id"] = *reference.ResolvedRevisionID
		row["resolved_title"] = jsonvalue.Optional(reference.ResolvedTitle)
	} else {
		row["resolved_revision_id"] = nil
		row["resolved_title"] = nil
	}
	return row
}

// referenceDetailRow maps one work family reference detail into a viewer row.
func referenceDetailRow(detail *work.ReferenceDetail) map[string]any {
	row := map[string]any{
		"id":               detail.ID,
		"work_revision_id": detail.WorkRevisionID,
		"mention_order":    detail.MentionOrder,
		"raw_reference":    jsonvalue.Optional(detail.RawReference),
		"doi":              jsonvalue.Optional(detail.DOI),
		"title":            jsonvalue.Optional(detail.Title),
		"author":           jsonvalue.Optional(detail.Author),
		"year":             jsonvalue.Optional(detail.Year),
		"source":           jsonvalue.Optional(detail.Source),
		"created_at":       detail.CreatedAt,
		"work_id":          detail.WorkID,
		"citing_title":     jsonvalue.Optional(detail.CitingTitle),
		"pipeline_run_id":  detail.PipelineRunID,
	}
	row["resolved_work_id"] = jsonvalue.Optional(detail.ResolvedWorkID)
	if detail.ResolvedRevisionID != nil {
		row["resolved_revision_id"] = *detail.ResolvedRevisionID
		row["resolved_title"] = jsonvalue.Optional(detail.ResolvedTitle)
	} else {
		row["resolved_revision_id"] = nil
		row["resolved_title"] = nil
	}
	return row
}
