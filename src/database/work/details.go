// details.go provides the handwritten article-detail and reference-detail
// relationship reads over the generated statements. The projections are stable
// application types; the server maps them to transport payloads.
package work

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/work/internal/sql"
)

// ErrArticleRevisionNotFound reports that a revision is not visible in the selected run.
var ErrArticleRevisionNotFound = errors.New("article revision not found")

// GetArticleRevision returns the full immutable revision projection for one
// article detail, or nil when the revision is not visible in the selected run.
func (s *Store) GetArticleRevision(ctx context.Context, revisionID, runID int64) (*ArticleRevision, error) {
	row, err := s.queries.GetArticleRevision(ctx, generated.GetArticleRevisionParams{
		ID:            revisionID,
		PipelineRunID: runID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get article revision: %w", err)
	}
	return articleRevisionFromGenerated(row), nil
}

// GetArticleDetailWorkID validates one visible article revision and returns its
// owning work. It returns ErrArticleRevisionNotFound when the revision is not
// visible in the selected run.
func (s *Store) GetArticleDetailWorkID(ctx context.Context, revisionID, runID int64) (int64, error) {
	workID, err := s.queries.GetArticleDetailWorkID(ctx, generated.GetArticleDetailWorkIDParams{
		ID:            revisionID,
		PipelineRunID: runID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrArticleRevisionNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get article detail work id: %w", err)
	}
	return workID, nil
}

// ListArticleReferences returns one bounded page of cited references for an
// immutable revision, with the exact total and a keyset continuation cursor.
func (s *Store) ListArticleReferences(ctx context.Context, input ArticleReferencePageInput) (*ArticleReferencePage, error) {
	if input.Limit < 1 {
		return nil, fmt.Errorf("list article references: limit must be positive")
	}
	total, err := s.queries.CountArticleReferences(ctx, input.RevisionID)
	if err != nil {
		return nil, fmt.Errorf("count article references: %w", err)
	}
	rows, err := s.queries.ListArticleReferences(ctx, generated.ListArticleReferencesParams{
		WorkRevisionID: input.RevisionID,
		CursorID:       input.CursorID,
		RowLimit:       int64(input.Limit + 1),
	})
	if err != nil {
		return nil, fmt.Errorf("list article references: %w", err)
	}
	hasMore := len(rows) > input.Limit
	if hasMore {
		rows = rows[:input.Limit]
	}
	items := make([]*ArticleReference, 0, len(rows))
	for _, row := range rows {
		items = append(items, articleReferenceFromGenerated(row))
	}
	var nextCursorID int64
	if hasMore && len(items) > 0 {
		nextCursorID = items[len(items)-1].ID
	}
	return &ArticleReferencePage{Items: items, Total: total, HasMore: hasMore, NextCursorID: nextCursorID}, nil
}

// ListArticleStages returns one bounded page of stage outcomes for one work in
// one run, with the exact total and a keyset continuation cursor.
func (s *Store) ListArticleStages(ctx context.Context, input ArticleStagePageInput) (*ArticleStagePage, error) {
	if input.Limit < 1 {
		return nil, fmt.Errorf("list article stages: limit must be positive")
	}
	total, err := s.queries.CountArticleStages(ctx, generated.CountArticleStagesParams{
		PipelineRunID: input.RunID,
		WorkID:        input.WorkID,
	})
	if err != nil {
		return nil, fmt.Errorf("count article stages: %w", err)
	}
	rows, err := s.queries.ListArticleStages(ctx, generated.ListArticleStagesParams{
		PipelineRunID: input.RunID,
		WorkID:        input.WorkID,
		CursorID:      input.CursorID,
		RowLimit:      int64(input.Limit + 1),
	})
	if err != nil {
		return nil, fmt.Errorf("list article stages: %w", err)
	}
	hasMore := len(rows) > input.Limit
	if hasMore {
		rows = rows[:input.Limit]
	}
	items := make([]*StageRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, &StageRow{
			ID:            row.ID,
			PipelineRunID: row.PipelineRunID,
			WorkID:        row.WorkID,
			StageName:     row.StageName,
			Outcome:       row.Outcome,
			Reason:        textPointer(row.Reason),
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		})
	}
	var nextCursorID int64
	if hasMore && len(items) > 0 {
		nextCursorID = items[len(items)-1].ID
	}
	return &ArticleStagePage{Items: items, Total: total, HasMore: hasMore, NextCursorID: nextCursorID}, nil
}

// ListAuthorArticles returns one bounded page of run-scoped article revisions
// attached to an author occurrence through an authorship, ordered by ascending
// authorship ID.
func (s *Store) ListAuthorArticles(ctx context.Context, filter AuthorArticleFilter) (*AuthorArticlePage, error) {
	if filter.Limit < 1 {
		return nil, fmt.Errorf("list author articles: limit must be positive")
	}
	total, err := s.queries.CountAuthorArticles(ctx, generated.CountAuthorArticlesParams{
		AuthorOccurrenceID: filter.AuthorOccurrenceID,
		PipelineRunID:      filter.RunID,
	})
	if err != nil {
		return nil, fmt.Errorf("count author articles: %w", err)
	}
	rows, err := s.queries.ListAuthorArticles(ctx, generated.ListAuthorArticlesParams{
		AuthorOccurrenceID: filter.AuthorOccurrenceID,
		PipelineRunID:      filter.RunID,
		CursorID:           filter.CursorID,
		RowLimit:           int64(filter.Limit + 1),
	})
	if err != nil {
		return nil, fmt.Errorf("list author articles: %w", err)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	items := make([]*AuthorArticle, 0, len(rows))
	for _, row := range rows {
		item := &AuthorArticle{
			RelationID:     row.RelationID,
			AuthorOrder:    row.AuthorOrder,
			WorkRevisionID: row.WorkRevisionID,
			WorkID:         row.WorkID,
			PipelineRunID:  row.PipelineRunID,
		}
		if row.Affiliation.Valid {
			affiliation := row.Affiliation.String
			item.Affiliation = &affiliation
		}
		if row.Title.Valid {
			title := row.Title.String
			item.Title = &title
		}
		if row.Year.Valid {
			year := row.Year.Int64
			item.Year = &year
		}
		if row.Doi.Valid {
			doi := row.Doi.String
			item.DOI = &doi
		}
		items = append(items, item)
	}
	var nextCursorID int64
	if hasMore && len(items) > 0 {
		nextCursorID = items[len(items)-1].RelationID
	}
	return &AuthorArticlePage{Items: items, Total: total, HasMore: hasMore, NextCursorID: nextCursorID}, nil
}

// GetReferenceDetail returns one reference mention with its citing and
// resolved-work context, or nil when it is not visible in the selected run.
func (s *Store) GetReferenceDetail(ctx context.Context, referenceID, runID int64) (*ReferenceDetail, error) {
	row, err := s.queries.GetReferenceDetail(ctx, generated.GetReferenceDetailParams{
		ID:            referenceID,
		PipelineRunID: runID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reference detail: %w", err)
	}
	return referenceDetailFromGenerated(row), nil
}

// articleRevisionFromGenerated maps one generated article revision row into an
// application revision and makes every nullable field explicit.
func articleRevisionFromGenerated(row generated.GetArticleRevisionRow) *ArticleRevision {
	return &ArticleRevision{
		ID:                 row.ID,
		WorkID:             row.WorkID,
		PipelineRunID:      row.PipelineRunID,
		FieldSchemaVersion: row.FieldSchemaVersion,
		PayloadHash:        row.PayloadHash,
		ProducerStage:      row.ProducerStage,
		CreatedAt:          row.CreatedAt,
		Title:              textPointer(row.Title),
		Abstract:           textPointer(row.Abstract),
		Year:               numberPointer(row.Year),
		Journal:            textPointer(row.Journal),
		Publisher:          textPointer(row.Publisher),
		Source:             textPointer(row.Source),
		Keywords:           textPointer(row.Keywords),
		KeywordsPlus:       textPointer(row.KeywordsPlus),
		CitationCount:      numberPointer(row.CitationCount),
		ReferenceCount:     numberPointer(row.ReferenceCount),
		ExtensionData:      textPointer(row.ExtensionData),
		DOI:                textPointer(row.Doi),
	}
}

// articleReferenceFromGenerated maps one generated reference row into an
// application reference and makes every nullable field explicit.
func articleReferenceFromGenerated(row generated.ListArticleReferencesRow) *ArticleReference {
	return &ArticleReference{
		ID:                 row.ID,
		WorkRevisionID:     row.WorkRevisionID,
		MentionOrder:       int(row.MentionOrder),
		CreatedAt:          row.CreatedAt,
		ResolvedWorkID:     numberPointer(row.ResolvedWorkID),
		DOI:                textPointer(row.Doi),
		Title:              textPointer(row.Title),
		Author:             textPointer(row.Author),
		Year:               numberPointer(row.Year),
		Source:             textPointer(row.Source),
		ResolvedRevisionID: numberPointer(row.ResolvedRevisionID),
		ResolvedTitle:      textPointer(row.ResolvedTitle),
	}
}

// referenceDetailFromGenerated maps one generated reference-detail row into an
// application reference detail and makes every nullable field explicit.
func referenceDetailFromGenerated(row generated.GetReferenceDetailRow) *ReferenceDetail {
	return &ReferenceDetail{
		ID:                 row.ID,
		WorkRevisionID:     row.WorkRevisionID,
		MentionOrder:       int(row.MentionOrder),
		CreatedAt:          row.CreatedAt,
		WorkID:             row.WorkID,
		PipelineRunID:      row.PipelineRunID,
		ResolvedWorkID:     numberPointer(row.ResolvedWorkID),
		RawReference:       textPointer(row.RawReference),
		DOI:                textPointer(row.Doi),
		Title:              textPointer(row.Title),
		Author:             textPointer(row.Author),
		Year:               numberPointer(row.Year),
		Source:             textPointer(row.Source),
		CitingTitle:        textPointer(row.CitingTitle),
		ResolvedRevisionID: numberPointer(row.ResolvedRevisionID),
		ResolvedTitle:      textPointer(row.ResolvedTitle),
	}
}
