// Package author provides the handwritten author table-family data access
// layer over the generated queries in this family's private internal/sql
// package. People, occurrences, authorships, identity resolutions, and
// candidate evidence live in their specifically named files.
package author

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	generated "analysis/database/author/internal/sql"
)

// Store binds the generated author queries to one already configured connection.
type Store struct {
	db      *sql.DB
	queries *generated.Queries
}

// New returns an author family store over an already configured connection. It
// only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations.
func New(db *sql.DB) *Store {
	return &Store{db: db, queries: generated.New(db)}
}

// CreatePersonByORCID inserts a new person by ORCID. If the ORCID already
// exists, returns the existing person ID (INSERT OR IGNORE semantics). The
// ORCID is normalized before storage. A malformed ORCID or one that fails the
// ISO 7064 MOD 11-2 checksum is rejected: the people table is a strong
// identity registry, not a raw observation store.
func (s *Store) CreatePersonByORCID(ctx context.Context, orcid string) (int64, error) {
	orcid = normalizeORCID(orcid)
	if orcid == "" {
		return 0, fmt.Errorf("create person: orcid is empty")
	}
	if !isValidORCID(orcid) {
		return 0, fmt.Errorf("create person: invalid orcid %q: must match format XXXX-XXXX-XXXX-XXXX with valid ISO 7064 MOD 11-2 checksum", orcid)
	}

	result, err := s.queries.InsertPersonByORCID(ctx, sql.NullString{String: orcid, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("create person: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if inserted > 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		return id, nil
	}

	// Already exists - return the existing person ID.
	existing, err := s.GetPersonByORCID(ctx, orcid)
	if err != nil {
		return 0, err
	}
	if existing == nil {
		return 0, fmt.Errorf("create person: insert skipped but existing row not found")
	}
	return existing.ID, nil
}

// GetPersonByID returns a person by their primary key, or nil if not found.
func (s *Store) GetPersonByID(ctx context.Context, id int64) (*Person, error) {
	row, err := s.queries.GetPersonByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return personFromGenerated(row), nil
}

// GetPersonByORCID returns a person by their normalized ORCID, or nil if not
// found. Empty or blank input short-circuits to nil without a query.
func (s *Store) GetPersonByORCID(ctx context.Context, orcid string) (*Person, error) {
	orcid = normalizeORCID(orcid)
	if orcid == "" {
		return nil, nil
	}
	row, err := s.queries.GetPersonByORCID(ctx, sql.NullString{String: orcid, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return personFromGenerated(row), nil
}

// personFromGenerated maps one generated person row into an application person
// and makes the nullable ORCID explicit.
func personFromGenerated(row generated.Person) *Person {
	person := &Person{
		ID:        row.ID,
		CreatedAt: row.CreatedAt,
	}
	if row.Orcid.Valid {
		person.ORCID = row.Orcid.String
	}
	return person
}

// normalizeORCID lowercases and trims whitespace from an ORCID string. It does
// not validate the ORCID format; callers should use isValidORCID.
func normalizeORCID(orcid string) string {
	return strings.TrimSpace(strings.ToLower(orcid))
}

// orcidDigit converts a byte to its integer value for checksum computation.
// '0'-'9' map to 0-9; 'x' and 'X' map to 10.
func orcidDigit(b byte) int {
	if b >= '0' && b <= '9' {
		return int(b - '0')
	}
	if b == 'x' || b == 'X' {
		return 10
	}
	return -1
}

// isValidORCID checks whether the given normalized string is a valid ORCID
// identifier. It must match the pattern XXXX-XXXX-XXXX-XXXX, where the last
// group ends with a digit or X that matches the ISO 7064 MOD 11-2 checksum.
// Hyphens are required for the pattern match but are stripped for checksum
// computation.
func isValidORCID(orcid string) bool {
	// Format: \d{4}-\d{4}-\d{4}-\d{3}[\dX]
	if len(orcid) != 19 {
		return false
	}
	for i := 0; i < 19; i++ {
		b := orcid[i]
		switch {
		case i == 4 || i == 9 || i == 14:
			if b != '-' {
				return false
			}
		case i == 18:
			if !(b >= '0' && b <= '9') && b != 'x' && b != 'X' {
				return false
			}
		default:
			if b < '0' || b > '9' {
				return false
			}
		}
	}

	// ISO 7064 MOD 11-2 checksum over the 15 base digits (hyphen-free,
	// excluding the final check character).
	//   total = 0
	//   for each digit d from left to right:
	//       total = (total + d) * 2
	//   check = (12 - (total mod 11)) mod 11
	//   check == 10 -> 'X', otherwise check is the remainder digit.
	total := 0
	for i := 0; i < 18; i++ {
		if orcid[i] == '-' {
			continue
		}
		d := orcidDigit(orcid[i])
		total = (total + d) * 2
	}
	check := (12 - (total % 11)) % 11

	expected := byte('0') + byte(check)
	if check == 10 {
		expected = 'X'
	}
	// Accept both uppercase and lowercase for the check character.
	return byte(orcid[18]) == expected || byte(orcid[18]) == expected+32
}
