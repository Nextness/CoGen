// graph.go provides the handwritten bounded graph relationship reads over the
// retained dynamic builders in graph_dynamic.go. The family returns ordered
// relationship rows; the server assembles the presentation graph.
package work

import (
	"context"
	"fmt"
)

// ListGraphArticles returns the bounded article nodes matching the graph
// filters plus the exact match count.
func (s *Store) ListGraphArticles(ctx context.Context, filter GraphFilter) ([]*GraphArticle, int, error) {
	where, args, err := graphArticleWhere(filter)
	if err != nil {
		return nil, 0, err
	}
	var matches int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM work_revisions wr JOIN works w ON w.id=wr.work_id WHERE "+where, args...).Scan(&matches); err != nil {
		return nil, 0, fmt.Errorf("count graph articles: %w", err)
	}
	queryArgs := append(append([]any(nil), args...), filter.Limit)
	rows, err := s.queryRows(ctx, "SELECT wr.id, wr.work_id, wr.title, wr.year, wr.source, w.doi FROM work_revisions wr JOIN works w ON w.id=wr.work_id WHERE "+where+" ORDER BY wr.id LIMIT ?", queryArgs)
	if err != nil {
		return nil, 0, fmt.Errorf("list graph articles: %w", err)
	}
	items := make([]*GraphArticle, 0, len(rows))
	for _, row := range rows {
		items = append(items, &GraphArticle{
			ID:     int64Value(row["id"]),
			WorkID: int64Value(row["work_id"]),
			Title:  stringValue(row["title"]),
			Year:   int(int64Value(row["year"])),
			Source: stringValue(row["source"]),
			DOI:    stringValue(row["doi"]),
		})
	}
	return items, matches, nil
}

// ListGraphAuthorships returns the bounded article-author relationship rows for
// the selected revisions.
func (s *Store) ListGraphAuthorships(ctx context.Context, revisionIDs []int64, authorLimit, rowLimit int) ([]*GraphAuthorship, error) {
	if len(revisionIDs) == 0 {
		return nil, nil
	}
	query, args, err := graphAuthorshipQuery(revisionIDs, authorLimit, rowLimit)
	if err != nil {
		return nil, err
	}
	rows, err := s.queryRows(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list graph authorships: %w", err)
	}
	items := make([]*GraphAuthorship, 0, len(rows))
	for _, row := range rows {
		items = append(items, &GraphAuthorship{
			WorkRevisionID: int64Value(row["work_revision_id"]),
			AuthorID:       int64Value(row["author_id"]),
			CitationName:   stringValue(row["citation_name"]),
			ORCID:          stringValue(row["orcid"]),
			AuthorOrder:    int(int64Value(row["author_order"])),
			Affiliation:    stringValue(row["affiliation"]),
		})
	}
	return items, nil
}

// ListGraphCitations returns the bounded resolved citation relationship rows
// for the selected revisions and works.
func (s *Store) ListGraphCitations(ctx context.Context, revisionIDs, workIDs []int64, rowLimit int) ([]*GraphCitation, error) {
	if len(revisionIDs) == 0 || len(workIDs) == 0 {
		return nil, nil
	}
	query, args, err := graphCitationQuery(revisionIDs, workIDs, rowLimit)
	if err != nil {
		return nil, err
	}
	rows, err := s.queryRows(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list graph citations: %w", err)
	}
	items := make([]*GraphCitation, 0, len(rows))
	for _, row := range rows {
		items = append(items, &GraphCitation{
			WorkRevisionID: int64Value(row["work_revision_id"]),
			ResolvedWorkID: int64Value(row["resolved_work_id"]),
		})
	}
	return items, nil
}

// ListGraphReferences returns the bounded article-reference relationship rows
// for the selected revisions.
func (s *Store) ListGraphReferences(ctx context.Context, revisionIDs []int64, rowLimit int) ([]*GraphReference, error) {
	if len(revisionIDs) == 0 {
		return nil, nil
	}
	query, args, err := graphReferenceQuery(revisionIDs, rowLimit)
	if err != nil {
		return nil, err
	}
	rows, err := s.queryRows(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list graph references: %w", err)
	}
	items := make([]*GraphReference, 0, len(rows))
	for _, row := range rows {
		items = append(items, &GraphReference{
			ID:             int64Value(row["reference_id"]),
			WorkRevisionID: int64Value(row["work_revision_id"]),
			DOI:            stringValue(row["doi"]),
			Title:          stringValue(row["title"]),
			Author:         stringValue(row["author"]),
			Year:           int(int64Value(row["year"])),
			Source:         stringValue(row["source"]),
		})
	}
	return items, nil
}
