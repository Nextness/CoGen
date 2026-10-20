// reference.go provides the handwritten cited-reference data access layer over
// the generated statements. DOI normalization and best-effort resolution to a
// known work stay in Go around the generated insert.
package work

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/work/internal/sql"
)

// CreateReference stores one ordered reference mention. A DOI is normalized
// and, when it identifies an existing work, linked through ResolvedWorkID
// automatically.
func (s *Store) CreateReference(ctx context.Context, reference *Reference) (int64, error) {
	if reference == nil {
		return 0, fmt.Errorf("create reference mention: value is required")
	}
	if reference.WorkRevisionID == 0 {
		return 0, fmt.Errorf("create reference mention: work_revision_id is required")
	}
	if reference.MentionOrder < 1 {
		return 0, fmt.Errorf("create reference mention: mention_order must be positive")
	}

	reference.DOI = NormalizeDOI(reference.DOI)
	resolvedWorkID := reference.ResolvedWorkID
	if resolvedWorkID == 0 && reference.DOI != "" {
		resolved, err := s.GetWorkByDOI(ctx, reference.DOI)
		if err != nil {
			return 0, fmt.Errorf("create reference mention: resolve DOI: %w", err)
		}
		if resolved != nil {
			resolvedWorkID = resolved.ID
			reference.ResolvedWorkID = resolved.ID
		}
	}

	result, err := s.queries.InsertReferenceMention(ctx, generated.InsertReferenceMentionParams{
		WorkRevisionID: reference.WorkRevisionID,
		ResolvedWorkID: nullableInt64(resolvedWorkID),
		MentionOrder:   int64(reference.MentionOrder),
		RawReference:   nullableString(reference.RawReference),
		Doi:            nullableString(reference.DOI),
		Title:          nullableString(reference.Title),
		Author:         nullableString(reference.Author),
		Year:           nullableInt64(int64(reference.Year)),
		Source:         nullableString(reference.Source),
	})
	if err != nil {
		return 0, fmt.Errorf("create reference mention: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetReferenceByID returns a mention by primary key, or nil if it does not exist.
func (s *Store) GetReferenceByID(ctx context.Context, id int64) (*Reference, error) {
	row, err := s.queries.GetReferenceMentionByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return referenceFromGenerated(row), nil
}

// ListReferencesByRevisionID returns a revision's references in their source order.
func (s *Store) ListReferencesByRevisionID(ctx context.Context, revisionID int64) ([]*Reference, error) {
	rows, err := s.queries.ListReferenceMentionsByRevisionID(ctx, revisionID)
	if err != nil {
		return nil, err
	}
	references := make([]*Reference, 0, len(rows))
	for _, row := range rows {
		references = append(references, referenceFromGenerated(row))
	}
	return references, nil
}

// ListReferencesByResolvedWorkID returns workspace citations that resolve to
// one work.
func (s *Store) ListReferencesByResolvedWorkID(ctx context.Context, workID int64) ([]*Reference, error) {
	rows, err := s.queries.ListReferenceMentionsByResolvedWorkID(ctx, requiredInt64(workID))
	if err != nil {
		return nil, err
	}
	references := make([]*Reference, 0, len(rows))
	for _, row := range rows {
		references = append(references, referenceFromGenerated(row))
	}
	return references, nil
}

// referenceFromGenerated maps one generated reference row into an application
// reference and makes every nullable field explicit.
func referenceFromGenerated(row generated.ReferenceMention) *Reference {
	reference := &Reference{
		ID:             row.ID,
		WorkRevisionID: row.WorkRevisionID,
		MentionOrder:   int(row.MentionOrder),
		CreatedAt:      row.CreatedAt,
	}
	if row.ResolvedWorkID.Valid {
		reference.ResolvedWorkID = row.ResolvedWorkID.Int64
	}
	if row.RawReference.Valid {
		reference.RawReference = row.RawReference.String
	}
	if row.Doi.Valid {
		reference.DOI = row.Doi.String
	}
	if row.Title.Valid {
		reference.Title = row.Title.String
	}
	if row.Author.Valid {
		reference.Author = row.Author.String
	}
	if row.Year.Valid {
		reference.Year = int(row.Year.Int64)
	}
	if row.Source.Valid {
		reference.Source = row.Source.String
	}
	return reference
}
