// corpus.go provides the handwritten run-scoped corpus relationship reads over
// the generated statements and the retained dynamic sort/search builders in
// corpus_dynamic.go. Coverage, totals, and stage summaries are static queries.
package work

import (
	"context"
	"fmt"
	"sort"
)

// CurrentCoverage returns work-revision and journal coverage for one run.
func (s *Store) CurrentCoverage(ctx context.Context, runID int64) (*Coverage, error) {
	row, err := s.queries.CurrentCoverage(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("current coverage: %w", err)
	}
	return &Coverage{WorkRevisions: row.Count, JournalCoverage: row.Column2}, nil
}

// RelationshipTotals counts canonical works, authorships, references, and
// resolved citations for one run.
func (s *Store) RelationshipTotals(ctx context.Context, runID int64) (*RelationshipTotals, error) {
	totals := &RelationshipTotals{}
	var err error
	if totals.WorkRevisions, err = s.queries.CountRunWorkRevisions(ctx, runID); err != nil {
		return nil, fmt.Errorf("count run work revisions: %w", err)
	}
	if totals.AnalysisReadyArticles, err = s.queries.CountRunAnalysisReadyArticles(ctx, runID); err != nil {
		return nil, fmt.Errorf("count run analysis-ready articles: %w", err)
	}
	if totals.Authorships, err = s.queries.CountRunAuthorships(ctx, runID); err != nil {
		return nil, fmt.Errorf("count run authorships: %w", err)
	}
	if totals.ReferenceMentions, err = s.queries.CountRunReferenceMentions(ctx, runID); err != nil {
		return nil, fmt.Errorf("count run reference mentions: %w", err)
	}
	if totals.InternalCitations, err = s.queries.CountRunInternalCitations(ctx, runID); err != nil {
		return nil, fmt.Errorf("count run internal citations: %w", err)
	}
	return totals, nil
}

// RunStageSummaries returns aggregate outcome counts by pipeline stage in the
// canonical stage order, with unknown stages sorted after the known ones.
func (s *Store) RunStageSummaries(ctx context.Context, runID int64) ([]*StageSummary, error) {
	rows, err := s.queries.RunStageSummaries(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("run stage summaries: %w", err)
	}
	byStage := make(map[string]*StageSummary)
	for _, row := range rows {
		summary := byStage[row.StageName]
		if summary == nil {
			summary = &StageSummary{
				StageName:       row.StageName,
				Outcomes:        map[string]int64{},
				FirstRecordedAt: row.FirstRecordedAt,
				LastRecordedAt:  row.LastRecordedAt,
			}
			byStage[row.StageName] = summary
		}
		summary.TotalRecords += row.Count
		summary.Outcomes[row.Outcome] = row.Count
		if row.FirstRecordedAt != "" && (summary.FirstRecordedAt == "" || row.FirstRecordedAt < summary.FirstRecordedAt) {
			summary.FirstRecordedAt = row.FirstRecordedAt
		}
		if row.LastRecordedAt > summary.LastRecordedAt {
			summary.LastRecordedAt = row.LastRecordedAt
		}
	}
	order := []string{
		StageNameParse,
		StageNameDeduplicate,
		StageNameEnrich,
		StageNameEnrichMetadata,
		StageNameEnrichIdentity,
		StageNameValidate,
		StageNameNormalize,
	}
	result := make([]*StageSummary, 0, len(byStage))
	for _, stage := range order {
		if summary := byStage[stage]; summary != nil {
			result = append(result, summary)
			delete(byStage, stage)
		}
	}
	remaining := make([]string, 0, len(byStage))
	for stage := range byStage {
		remaining = append(remaining, stage)
	}
	sort.Strings(remaining)
	for _, stage := range remaining {
		result = append(result, byStage[stage])
	}
	return result, nil
}

// ListCorpusReferences returns one bounded page of run-scoped reference
// mentions with their citing titles.
func (s *Store) ListCorpusReferences(ctx context.Context, filter CorpusReferenceFilter) (*CorpusReferencePage, error) {
	where, args, err := corpusReferenceWhere(filter)
	if err != nil {
		return nil, err
	}
	total, err := s.countQuery(ctx, "SELECT COUNT(*) FROM reference_mentions rm JOIN work_revisions wr ON wr.id=rm.work_revision_id WHERE "+where, args)
	if err != nil {
		return nil, fmt.Errorf("count corpus references: %w", err)
	}
	query := "SELECT rm.id, rm.work_revision_id, rm.mention_order, rm.doi, rm.title, rm.author, rm.year, rm.source, rm.resolved_work_id, wr.title AS citing_title, rm.created_at " +
		"FROM reference_mentions rm JOIN work_revisions wr ON wr.id=rm.work_revision_id WHERE " + where +
		" ORDER BY " + corpusReferenceSortExpression(filter.Sort) + " " + sqlDirection(filter.Order) + ", rm.id " + sqlDirection(filter.Order) + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any(nil), args...), filter.PerPage, (clampPage(filter.Page, filter.PerPage, total)-1)*filter.PerPage)
	rows, err := s.queryRows(ctx, query, queryArgs)
	if err != nil {
		return nil, fmt.Errorf("list corpus references: %w", err)
	}
	items := make([]*CorpusReference, 0, len(rows))
	for _, row := range rows {
		items = append(items, corpusReferenceFromRow(row))
	}
	return &CorpusReferencePage{Items: items, Total: total}, nil
}

// ListCorpusAuthors returns one bounded page of run-scoped author occurrences
// with their article and affiliation counts.
func (s *Store) ListCorpusAuthors(ctx context.Context, filter CorpusAuthorFilter) (*CorpusAuthorPage, error) {
	where, args, err := corpusAuthorWhere(filter)
	if err != nil {
		return nil, err
	}
	total, err := s.countQuery(ctx, "SELECT COUNT(*) FROM (SELECT ao.id FROM author_occurrences ao JOIN authorships a ON a.author_occurrence_id=ao.id JOIN work_revisions wr ON wr.id=a.work_revision_id WHERE "+where+" GROUP BY ao.id)", args)
	if err != nil {
		return nil, fmt.Errorf("count corpus authors: %w", err)
	}
	query := "SELECT ao.id, ao.citation_name, ao.first_name, ao.last_name, ao.orcid, ao.person_id, COUNT(DISTINCT a.work_revision_id) AS article_count, COUNT(DISTINCT NULLIF(a.affiliation, '')) AS affiliation_count, ao.created_at " +
		"FROM author_occurrences ao JOIN authorships a ON a.author_occurrence_id=ao.id JOIN work_revisions wr ON wr.id=a.work_revision_id WHERE " + where +
		" GROUP BY ao.id ORDER BY " + corpusAuthorSortExpression(filter.Sort) + " " + sqlDirection(filter.Order) + ", ao.id " + sqlDirection(filter.Order) + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any(nil), args...), filter.PerPage, (clampPage(filter.Page, filter.PerPage, total)-1)*filter.PerPage)
	rows, err := s.queryRows(ctx, query, queryArgs)
	if err != nil {
		return nil, fmt.Errorf("list corpus authors: %w", err)
	}
	items := make([]*CorpusAuthor, 0, len(rows))
	for _, row := range rows {
		items = append(items, corpusAuthorFromRow(row))
	}
	return &CorpusAuthorPage{Items: items, Total: total}, nil
}

// ListRunStages returns one bounded page of run stage outcomes.
func (s *Store) ListRunStages(ctx context.Context, filter RunStageFilter) (*RunStagePage, error) {
	where, args, err := runStageWhere(filter)
	if err != nil {
		return nil, err
	}
	total, err := s.countQuery(ctx, "SELECT COUNT(*) FROM run_work_stages rws WHERE "+where, args)
	if err != nil {
		return nil, fmt.Errorf("count run stages: %w", err)
	}
	query := "SELECT rws.id, rws.pipeline_run_id, rws.work_id, rws.stage_name, rws.outcome, rws.reason, rws.created_at, rws.updated_at " +
		"FROM run_work_stages rws WHERE " + where +
		" ORDER BY " + runStageSortExpression(filter.Sort) + " " + sqlDirection(filter.Order) + ", rws.id " + sqlDirection(filter.Order) + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any(nil), args...), filter.PerPage, (clampPage(filter.Page, filter.PerPage, total)-1)*filter.PerPage)
	rows, err := s.queryRows(ctx, query, queryArgs)
	if err != nil {
		return nil, fmt.Errorf("list run stages: %w", err)
	}
	items := make([]*StageOutcome, 0, len(rows))
	for _, row := range rows {
		items = append(items, stageOutcomeFromRow(row))
	}
	return &RunStagePage{Items: items, Total: total}, nil
}

// countQuery runs one scalar count query.
func (s *Store) countQuery(ctx context.Context, query string, args []any) (int64, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// clampPage maps an offset request past the end to the final populated page.
func clampPage(page, perPage int, total int64) int {
	totalPages := (total + int64(perPage) - 1) / int64(perPage)
	if totalPages == 0 {
		return 1
	}
	if int64(page) > totalPages {
		return int(totalPages)
	}
	return page
}

// queryRows runs one read query and returns every row as a string-keyed map.
func (s *Store) queryRows(ctx context.Context, query string, args []any) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var result []map[string]any
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(names))
		for index, name := range names {
			item[name] = values[index]
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// corpusReferenceFromRow maps one dynamic corpus reference row into an application reference.
func corpusReferenceFromRow(row map[string]any) *CorpusReference {
	reference := &CorpusReference{
		ID:             int64Value(row["id"]),
		WorkRevisionID: int64Value(row["work_revision_id"]),
		MentionOrder:   int(int64Value(row["mention_order"])),
		DOI:            stringValue(row["doi"]),
		Title:          stringValue(row["title"]),
		Author:         stringValue(row["author"]),
		Year:           int(int64Value(row["year"])),
		Source:         stringValue(row["source"]),
		CitingTitle:    stringValue(row["citing_title"]),
		CreatedAt:      stringValue(row["created_at"]),
	}
	if value, ok := row["resolved_work_id"].(int64); ok {
		reference.ResolvedWorkID = &value
	}
	return reference
}

// corpusAuthorFromRow maps one dynamic corpus author row into an application author.
func corpusAuthorFromRow(row map[string]any) *CorpusAuthor {
	author := &CorpusAuthor{
		ID:               int64Value(row["id"]),
		CitationName:     stringValue(row["citation_name"]),
		FirstName:        stringValue(row["first_name"]),
		LastName:         stringValue(row["last_name"]),
		ORCID:            stringValue(row["orcid"]),
		ArticleCount:     int64Value(row["article_count"]),
		AffiliationCount: int64Value(row["affiliation_count"]),
		CreatedAt:        stringValue(row["created_at"]),
	}
	if value, ok := row["person_id"].(int64); ok {
		author.PersonID = &value
	}
	return author
}

// stageOutcomeFromRow maps one dynamic stage row into an application stage outcome.
func stageOutcomeFromRow(row map[string]any) *StageOutcome {
	return &StageOutcome{
		ID:            int64Value(row["id"]),
		PipelineRunID: int64Value(row["pipeline_run_id"]),
		WorkID:        int64Value(row["work_id"]),
		StageName:     stringValue(row["stage_name"]),
		Outcome:       stringValue(row["outcome"]),
		Reason:        stringValue(row["reason"]),
		CreatedAt:     stringValue(row["created_at"]),
		UpdatedAt:     stringValue(row["updated_at"]),
	}
}

// int64Value converts one scanned SQLite integer into int64.
func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}

// stringValue converts one scanned SQLite text value into a string.
func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return ""
	}
}
