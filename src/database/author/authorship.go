// authorship.go provides the handwritten ordered-authorship data access layer
// over the generated statements. Required identity and order validation stay
// in Go around the generated insert.
package author

import (
	"context"
	"fmt"

	generated "analysis/database/author/internal/sql"
)

// CreateAuthorship inserts a new authorship linking a work revision to an
// author occurrence with the given order and optional affiliation.
func (s *Store) CreateAuthorship(ctx context.Context, authorship *Authorship) (int64, error) {
	if authorship == nil {
		return 0, fmt.Errorf("create authorship: value is required")
	}
	if authorship.WorkRevisionID == 0 {
		return 0, fmt.Errorf("create authorship: work_revision_id is required")
	}
	if authorship.AuthorOccurrenceID == 0 {
		return 0, fmt.Errorf("create authorship: author_occurrence_id is required")
	}
	if authorship.AuthorOrder < 1 {
		return 0, fmt.Errorf("create authorship: author_order must be >= 1")
	}

	result, err := s.queries.InsertAuthorship(ctx, generated.InsertAuthorshipParams{
		WorkRevisionID:     authorship.WorkRevisionID,
		AuthorOccurrenceID: authorship.AuthorOccurrenceID,
		AuthorOrder:        int64(authorship.AuthorOrder),
		Affiliation:        nullableString(authorship.Affiliation),
	})
	if err != nil {
		return 0, fmt.Errorf("create authorship: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListAuthorshipsByRevisionID returns all authorships for a given work
// revision, ordered by author_order.
func (s *Store) ListAuthorshipsByRevisionID(ctx context.Context, revisionID int64) ([]*Authorship, error) {
	rows, err := s.queries.ListAuthorshipsByRevisionID(ctx, revisionID)
	if err != nil {
		return nil, err
	}
	var authorships []*Authorship
	for _, row := range rows {
		authorships = append(authorships, authorshipFromGenerated(row))
	}
	return authorships, nil
}

// ListAuthorshipsByOccurrenceID returns all authorships for a given author
// occurrence, ordered by ID.
func (s *Store) ListAuthorshipsByOccurrenceID(ctx context.Context, occurrenceID int64) ([]*Authorship, error) {
	rows, err := s.queries.ListAuthorshipsByOccurrenceID(ctx, occurrenceID)
	if err != nil {
		return nil, err
	}
	var authorships []*Authorship
	for _, row := range rows {
		authorships = append(authorships, authorshipFromGenerated(row))
	}
	return authorships, nil
}

// authorshipFromGenerated maps one generated authorship row into an
// application authorship and makes the nullable affiliation explicit.
func authorshipFromGenerated(row generated.Authorship) *Authorship {
	authorship := &Authorship{
		ID:                 row.ID,
		WorkRevisionID:     row.WorkRevisionID,
		AuthorOccurrenceID: row.AuthorOccurrenceID,
		AuthorOrder:        int(row.AuthorOrder),
		CreatedAt:          row.CreatedAt,
	}
	if row.Affiliation.Valid {
		authorship.Affiliation = row.Affiliation.String
	}
	return authorship
}
