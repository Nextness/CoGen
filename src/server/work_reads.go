// work_reads.go converts work family application types into the viewer's
// stable JSON row shapes. Nullable fields and empty collections are made
// explicit here so family types never leak into responses.
package server

import (
	"analysis/database/work"
)

// corpusReferenceRow maps one work family corpus reference into a viewer row.
func corpusReferenceRow(reference *work.CorpusReference) map[string]any {
	row := map[string]any{
		"id":               reference.ID,
		"work_revision_id": reference.WorkRevisionID,
		"mention_order":    reference.MentionOrder,
		"doi":              reference.DOI,
		"title":            reference.Title,
		"author":           reference.Author,
		"year":             reference.Year,
		"source":           reference.Source,
		"citing_title":     reference.CitingTitle,
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
		"first_name":        author.FirstName,
		"last_name":         author.LastName,
		"orcid":             author.ORCID,
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
func stageOutcomeRow(outcome *work.StageOutcome) map[string]any {
	return map[string]any{
		"id":         outcome.ID,
		"work_id":    outcome.WorkID,
		"stage_name": outcome.StageName,
		"outcome":    outcome.Outcome,
		"reason":     outcome.Reason,
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
		"title":               row.Title,
		"year":                row.Year,
		"journal":             row.Journal,
		"publisher":           row.Publisher,
		"source":              row.Source,
		"doi":                 row.DOI,
		"validation_status":   row.ValidationStatus,
		"citation_count":      row.CitationCount,
		"reference_count":     row.ReferenceCount,
		"producer_stage":      row.ProducerStage,
		"created_at":          row.CreatedAt,
		"abstract":            row.Abstract,
		"keywords":            row.Keywords,
		"keywords_plus":       row.KeywordsPlus,
		"authors":             row.Authors,
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
		"title":   article.Title,
		"year":    article.Year,
		"source":  article.Source,
		"doi":     article.DOI,
	}
}

// graphAuthorshipItem maps one work family graph authorship into a viewer row.
func graphAuthorshipItem(authorship *work.GraphAuthorship) map[string]any {
	return map[string]any{
		"work_revision_id": authorship.WorkRevisionID,
		"author_id":        authorship.AuthorID,
		"citation_name":    authorship.CitationName,
		"orcid":            authorship.ORCID,
		"author_order":     int64(authorship.AuthorOrder),
		"affiliation":      authorship.Affiliation,
	}
}

// graphCitationItem maps one work family graph citation into a viewer row.
func graphCitationItem(citation *work.GraphCitation) map[string]any {
	return map[string]any{
		"work_revision_id": citation.WorkRevisionID,
		"resolved_work_id": citation.ResolvedWorkID,
	}
}

// graphReferenceItem maps one work family graph reference into a viewer row.
func graphReferenceItem(reference *work.GraphReference) map[string]any {
	return map[string]any{
		"reference_id":     reference.ID,
		"work_revision_id": reference.WorkRevisionID,
		"doi":              reference.DOI,
		"title":            reference.Title,
		"author":           reference.Author,
		"year":             reference.Year,
		"source":           reference.Source,
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
		"title":                revision.Title,
		"abstract":             revision.Abstract,
		"year":                 revision.Year,
		"journal":              revision.Journal,
		"publisher":            revision.Publisher,
		"source":               revision.Source,
		"keywords":             revision.Keywords,
		"keywords_plus":        revision.KeywordsPlus,
		"citation_count":       revision.CitationCount,
		"reference_count":      revision.ReferenceCount,
		"extension_data":       revision.ExtensionData,
		"producer_stage":       revision.ProducerStage,
		"created_at":           revision.CreatedAt,
		"doi":                  revision.DOI,
	}
}

// articleAuthorRow maps one work family article author into a viewer row.
func articleAuthorRow(author *work.ArticleAuthor) map[string]any {
	row := map[string]any{
		"id":            author.ID,
		"citation_name": author.CitationName,
		"first_name":    author.FirstName,
		"last_name":     author.LastName,
		"orcid":         author.ORCID,
		"author_order":  author.AuthorOrder,
		"affiliation":   author.Affiliation,
	}
	if author.PersonID != nil {
		row["person_id"] = *author.PersonID
	} else {
		row["person_id"] = nil
	}
	return row
}

// articleStageRow maps one work family article stage outcome into a viewer row.
func articleStageRow(outcome *work.StageOutcome) map[string]any {
	return map[string]any{
		"id":         outcome.ID,
		"stage_name": outcome.StageName,
		"outcome":    outcome.Outcome,
		"reason":     outcome.Reason,
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
		"doi":              reference.DOI,
		"title":            reference.Title,
		"author":           reference.Author,
		"year":             reference.Year,
		"source":           reference.Source,
		"created_at":       reference.CreatedAt,
	}
	if reference.ResolvedWorkID != nil {
		row["resolved_work_id"] = *reference.ResolvedWorkID
	} else {
		row["resolved_work_id"] = nil
	}
	if reference.ResolvedRevisionID != nil {
		row["resolved_revision_id"] = *reference.ResolvedRevisionID
		row["resolved_title"] = reference.ResolvedTitle
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
		"raw_reference":    detail.RawReference,
		"doi":              detail.DOI,
		"title":            detail.Title,
		"author":           detail.Author,
		"year":             detail.Year,
		"source":           detail.Source,
		"created_at":       detail.CreatedAt,
		"work_id":          detail.WorkID,
		"citing_title":     detail.CitingTitle,
		"pipeline_run_id":  detail.PipelineRunID,
	}
	if detail.ResolvedWorkID != 0 {
		row["resolved_work_id"] = detail.ResolvedWorkID
	} else {
		row["resolved_work_id"] = nil
	}
	if detail.ResolvedRevisionID != nil {
		row["resolved_revision_id"] = *detail.ResolvedRevisionID
		row["resolved_title"] = detail.ResolvedTitle
	} else {
		row["resolved_revision_id"] = nil
		row["resolved_title"] = nil
	}
	return row
}
