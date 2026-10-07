// occurrence.go provides the handwritten observed-author data access layer
// over the generated statements. ORCID validation and person linking stay in
// Go around the generated insert.
package author

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/author/internal/sql"
)

// CreateOccurrence inserts a new author occurrence. If the ORCID is non-empty
// and passes format-and-checksum validation, the method looks up or creates a
// Person record and links the occurrence to it. Invalid or malformed ORCIDs
// are stored as raw observed values on the occurrence but do not create or
// link to a person record: the design requires a strong identity signal before
// global merging, and an unvalidated string is not a strong signal.
func (s *Store) CreateOccurrence(ctx context.Context, occurrence *Occurrence) (int64, error) {
	if occurrence == nil {
		return 0, fmt.Errorf("create author occurrence: value is required")
	}
	if occurrence.CitationName == "" {
		return 0, fmt.Errorf("create author occurrence: citation_name is required")
	}

	var personID sql.NullInt64
	if occurrence.ORCID != "" {
		orcid := normalizeORCID(occurrence.ORCID)
		if isValidORCID(orcid) {
			pid, err := s.CreatePersonByORCID(ctx, orcid)
			if err != nil {
				return 0, fmt.Errorf("create author occurrence: %w", err)
			}
			personID = sql.NullInt64{Int64: pid, Valid: true}
			occurrence.PersonID = pid
		}
	}

	result, err := s.queries.InsertAuthorOccurrence(ctx, generated.InsertAuthorOccurrenceParams{
		PersonID:     personID,
		CitationName: occurrence.CitationName,
		FirstName:    nullableString(occurrence.FirstName),
		LastName:     nullableString(occurrence.LastName),
		Orcid:        nullableString(occurrence.ORCID),
	})
	if err != nil {
		return 0, fmt.Errorf("create author occurrence: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetOccurrenceByID returns an author occurrence by its primary key, or nil if
// not found.
func (s *Store) GetOccurrenceByID(ctx context.Context, id int64) (*Occurrence, error) {
	row, err := s.queries.GetAuthorOccurrenceByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return occurrenceFromGenerated(row), nil
}

// ListOccurrencesByPersonID returns all author occurrences linked to a given
// person, in ID order.
func (s *Store) ListOccurrencesByPersonID(ctx context.Context, personID int64) ([]*Occurrence, error) {
	rows, err := s.queries.ListAuthorOccurrencesByPersonID(ctx, sql.NullInt64{Int64: personID, Valid: true})
	if err != nil {
		return nil, err
	}
	var occurrences []*Occurrence
	for _, row := range rows {
		occurrences = append(occurrences, occurrenceFromGenerated(row))
	}
	return occurrences, nil
}

// occurrenceFromGenerated maps one generated occurrence row into an
// application occurrence and makes every nullable field explicit.
func occurrenceFromGenerated(row generated.AuthorOccurrence) *Occurrence {
	occurrence := &Occurrence{
		ID:           row.ID,
		CitationName: row.CitationName,
		CreatedAt:    row.CreatedAt,
	}
	if row.PersonID.Valid {
		occurrence.PersonID = row.PersonID.Int64
	}
	if row.FirstName.Valid {
		occurrence.FirstName = row.FirstName.String
	}
	if row.LastName.Valid {
		occurrence.LastName = row.LastName.String
	}
	if row.Orcid.Valid {
		occurrence.ORCID = row.Orcid.String
	}
	return occurrence
}

// nullableString maps empty optional text into a SQL NULL.
func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
