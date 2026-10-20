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
		"doi":              nullableText(reference.DOI),
		"title":            nullableText(reference.Title),
		"author":           nullableText(reference.Author),
		"year":             nullableIDPointer(reference.Year),
		"source":           nullableText(reference.Source),
		"citing_title":     nullableText(reference.CitingTitle),
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
		"first_name":        nullableText(author.FirstName),
		"last_name":         nullableText(author.LastName),
		"orcid":             nullableText(author.ORCID),
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
		"reason":     nullableText(outcome.Reason),
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
		"title":               nullableText(row.Title),
		"year":                nullableIDPointer(row.Year),
		"journal":             nullableText(row.Journal),
		"publisher":           nullableText(row.Publisher),
		"source":              nullableText(row.Source),
		"doi":                 nullableText(row.DOI),
		"validation_status":   nullableText(row.ValidationStatus),
		"citation_count":      nullableIDPointer(row.CitationCount),
		"reference_count":     nullableIDPointer(row.ReferenceCount),
		"producer_stage":      row.ProducerStage,
		"created_at":          row.CreatedAt,
		"abstract":            nullableText(row.Abstract),
		"keywords":            nullableText(row.Keywords),
		"keywords_plus":       nullableText(row.KeywordsPlus),
		"authors":             nullableText(row.Authors),
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
		"title":   nullableText(article.Title),
		"year":    nullableIDPointer(article.Year),
		"source":  nullableText(article.Source),
		"doi":     nullableText(article.DOI),
	}
}

// graphAuthorshipItem maps one work family graph authorship into a viewer row.
func graphAuthorshipItem(authorship *work.GraphAuthorship) map[string]any {
	return map[string]any{
		"work_revision_id": authorship.WorkRevisionID,
		"author_id":        authorship.AuthorID,
		"citation_name":    authorship.CitationName,
		"orcid":            nullableText(authorship.ORCID),
		"author_order":     int64(authorship.AuthorOrder),
		"affiliation":      nullableText(authorship.Affiliation),
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
		"doi":              nullableText(reference.DOI),
		"title":            nullableText(reference.Title),
		"author":           nullableText(reference.Author),
		"year":             nullableIDPointer(reference.Year),
		"source":           nullableText(reference.Source),
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
		"title":                nullableText(revision.Title),
		"abstract":             nullableText(revision.Abstract),
		"year":                 nullableIDPointer(revision.Year),
		"journal":              nullableText(revision.Journal),
		"publisher":            nullableText(revision.Publisher),
		"source":               nullableText(revision.Source),
		"keywords":             nullableText(revision.Keywords),
		"keywords_plus":        nullableText(revision.KeywordsPlus),
		"citation_count":       nullableIDPointer(revision.CitationCount),
		"reference_count":      nullableIDPointer(revision.ReferenceCount),
		"extension_data":       nullableText(revision.ExtensionData),
		"producer_stage":       revision.ProducerStage,
		"created_at":           revision.CreatedAt,
		"doi":                  nullableText(revision.DOI),
	}
}

// articleAuthorRow maps one work family article author into a viewer row.
func articleAuthorRow(author *work.ArticleAuthor) map[string]any {
	row := map[string]any{
		"id":            author.ID,
		"citation_name": author.CitationName,
		"first_name":    nullableText(author.FirstName),
		"last_name":     nullableText(author.LastName),
		"orcid":         nullableText(author.ORCID),
		"author_order":  author.AuthorOrder,
		"affiliation":   nullableText(author.Affiliation),
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
		"reason":     nullableText(outcome.Reason),
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
		"doi":              nullableText(reference.DOI),
		"title":            nullableText(reference.Title),
		"author":           nullableText(reference.Author),
		"year":             nullableIDPointer(reference.Year),
		"source":           nullableText(reference.Source),
		"created_at":       reference.CreatedAt,
	}
	if reference.ResolvedWorkID != nil {
		row["resolved_work_id"] = *reference.ResolvedWorkID
	} else {
		row["resolved_work_id"] = nil
	}
	if reference.ResolvedRevisionID != nil {
		row["resolved_revision_id"] = *reference.ResolvedRevisionID
		row["resolved_title"] = nullableText(reference.ResolvedTitle)
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
		"raw_reference":    nullableText(detail.RawReference),
		"doi":              nullableText(detail.DOI),
		"title":            nullableText(detail.Title),
		"author":           nullableText(detail.Author),
		"year":             nullableIDPointer(detail.Year),
		"source":           nullableText(detail.Source),
		"created_at":       detail.CreatedAt,
		"work_id":          detail.WorkID,
		"citing_title":     nullableText(detail.CitingTitle),
		"pipeline_run_id":  detail.PipelineRunID,
	}
	row["resolved_work_id"] = nullableIDPointer(detail.ResolvedWorkID)
	if detail.ResolvedRevisionID != nil {
		row["resolved_revision_id"] = *detail.ResolvedRevisionID
		row["resolved_title"] = nullableText(detail.ResolvedTitle)
	} else {
		row["resolved_revision_id"] = nil
		row["resolved_title"] = nil
	}
	return row
}
