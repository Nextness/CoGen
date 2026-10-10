// authorship.go provides the handwritten work-focused authorship reads over
// the generated statements. Authorship rows belong to the work family when the
// returned projection is scoped to one immutable work revision.
package work

import (
	"context"
	"fmt"

	generated "analysis/database/work/internal/sql"
)

// ListArticleAuthors returns one bounded page of ordered authorships for an
// immutable revision, with the exact total and a keyset continuation cursor.
func (s *Store) ListArticleAuthors(ctx context.Context, input ArticleAuthorPageInput) (*ArticleAuthorPage, error) {
	if input.Limit < 1 {
		return nil, fmt.Errorf("list article authors: limit must be positive")
	}
	total, err := s.queries.CountArticleAuthors(ctx, input.RevisionID)
	if err != nil {
		return nil, fmt.Errorf("count article authors: %w", err)
	}
	rows, err := s.queries.ListArticleAuthors(ctx, generated.ListArticleAuthorsParams{
		WorkRevisionID: input.RevisionID,
		CursorID:       input.CursorID,
		RowLimit:       int64(input.Limit + 1),
	})
	if err != nil {
		return nil, fmt.Errorf("list article authors: %w", err)
	}
	hasMore := len(rows) > input.Limit
	if hasMore {
		rows = rows[:input.Limit]
	}
	items := make([]*ArticleAuthor, 0, len(rows))
	for _, row := range rows {
		items = append(items, articleAuthorFromGenerated(row))
	}
	var nextCursorID int64
	if hasMore && len(items) > 0 {
		nextCursorID = items[len(items)-1].RelationID
	}
	return &ArticleAuthorPage{Items: items, Total: total, HasMore: hasMore, NextCursorID: nextCursorID}, nil
}

// articleAuthorFromGenerated maps one generated authorship row into an
// application author and makes every nullable field explicit.
func articleAuthorFromGenerated(row generated.ListArticleAuthorsRow) *ArticleAuthor {
	author := &ArticleAuthor{
		RelationID:   row.RelationID,
		ID:           row.ID,
		CitationName: row.CitationName,
		AuthorOrder:  int(row.AuthorOrder),
	}
	if row.PersonID.Valid {
		value := row.PersonID.Int64
		author.PersonID = &value
	}
	if row.FirstName.Valid {
		author.FirstName = row.FirstName.String
	}
	if row.LastName.Valid {
		author.LastName = row.LastName.String
	}
	if row.Orcid.Valid {
		author.ORCID = row.Orcid.String
	}
	if row.Affiliation.Valid {
		author.Affiliation = row.Affiliation.String
	}
	return author
}
