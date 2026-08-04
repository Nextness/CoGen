// authors_new.go provides the repositories for people, author occurrences,
// and authorship records that link immutable work revisions to their
// authors with ordering and provenance.
package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// Person represents an optional strong global identity for an author.
// ORCID is the canonical strong identity signal.
type Person struct {
	ID        int64  `json:"id"`
	ORCID     string `json:"orcid"`
	CreatedAt string `json:"created_at"`
}

// AuthorOccurrence represents observed author data at a point in time.
// An occurrence may optionally link to a global Person record when the ORCID
// is a known strong identity. ORCID-less occurrences with the same name are
// never merged globally.
type AuthorOccurrence struct {
	ID           int64  `json:"id"`
	PersonID     int64  `json:"person_id"`
	CitationName string `json:"citation_name"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ORCID        string `json:"orcid"`
	CreatedAt    string `json:"created_at"`
}

// Authorship links an immutable work_revision to an author_occurrence,
// preserving author order and optional affiliation. The authorships table
// is append-only (database-level triggers enforce this), so a historical
// revision's authorship set is immutable. Corrections create a new
// work_revision with a new authorship set.
type Authorship struct {
	ID                 int64  `json:"id"`
	WorkRevisionID     int64  `json:"work_revision_id"`
	AuthorOccurrenceID int64  `json:"author_occurrence_id"`
	AuthorOrder        int    `json:"author_order"`
	Affiliation        string `json:"affiliation"`
	CreatedAt          string `json:"created_at"`
}

// PersonRepository provides CRUD for the people table.
type PersonRepository struct {
	db *Database
}

// CreateByORCID inserts a new person by ORCID. If the ORCID already exists,
// returns the existing person ID (INSERT OR IGNORE semantics).
// The ORCID is normalized (lowercased, whitespace trimmed) before storage.
// A malformed ORCID or one that fails the ISO 7064 MOD 11-2 checksum is
// rejected — the people table is a strong identity registry, not a raw
// observation store.
func (r *PersonRepository) CreateByORCID(orcid string) (int64, error) {
	orcid = normalizeORCID(orcid)
	if orcid == "" {
		return 0, fmt.Errorf("create person: orcid is empty")
	}
	if !isValidORCID(orcid) {
		return 0, fmt.Errorf("create person: invalid orcid %q: must match format XXXX-XXXX-XXXX-XXXX with valid ISO 7064 MOD 11-2 checksum", orcid)
	}

	res, err := r.db.DB.Exec(
		"INSERT OR IGNORE INTO people (orcid) VALUES (?)",
		orcid,
	)
	if err != nil {
		lg.Debug("person creation by ORCID failed", "orcid", orcid, "error", err)
		return 0, fmt.Errorf("create person: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("person creation result read failed", "orcid", orcid, "error", err)
		return 0, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("person inserted ID read failed", "orcid", orcid, "error", err)
			return 0, err
		}
		lg.Debug("person creation successful",
			"orcid", orcid, "person_id", id, "result", "inserted")
		return id, nil
	}

	// Already exists — look up existing ID
	existing, err := r.GetByORCID(orcid)
	if err != nil {
		lg.Debug("person existing lookup failed", "orcid", orcid, "error", err)
		return 0, err
	}
	if existing == nil {
		lg.Debug("person creation failed",
			"orcid", orcid, "reason", "insert_skipped_but_not_found")
		return 0, fmt.Errorf("create person: insert skipped but existing row not found")
	}
	lg.Debug("person creation successful",
		"orcid", orcid, "person_id", existing.ID, "result", "already_existing")
	return existing.ID, nil
}

// GetByID returns a person by their primary key, or nil if not found.
func (r *PersonRepository) GetByID(id int64) (*Person, error) {
	var p Person
	var orcid sql.NullString
	err := r.db.DB.QueryRow(
		"SELECT id, orcid, created_at FROM people WHERE id = ?", id,
	).Scan(&p.ID, &orcid, &p.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("person query successful", "person_id", id, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("person query failed", "person_id", id, "error", err)
		return nil, err
	}
	if orcid.Valid {
		p.ORCID = orcid.String
	}
	lg.Debug("person query successful", "person_id", id, "orcid", p.ORCID, "result", "found")
	return &p, nil
}

// GetByORCID returns a person by their ORCID, or nil if not found.
// The ORCID is normalized the same way as CreateByORCID.
func (r *PersonRepository) GetByORCID(orcid string) (*Person, error) {
	orcid = normalizeORCID(orcid)
	if orcid == "" {
		lg.Debug("person query by ORCID skipped", "reason", "empty_after_normalization")
		return nil, nil
	}

	var p Person
	var orcidNull sql.NullString
	err := r.db.DB.QueryRow(
		"SELECT id, orcid, created_at FROM people WHERE orcid = ?", orcid,
	).Scan(&p.ID, &orcidNull, &p.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("person query successful", "orcid", orcid, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("person query failed", "orcid", orcid, "error", err)
		return nil, err
	}
	if orcidNull.Valid {
		p.ORCID = orcidNull.String
	}
	lg.Debug("person query successful", "person_id", p.ID, "orcid", p.ORCID, "result", "found")
	return &p, nil
}

// AuthorOccurrenceRepository provides CRUD for the author_occurrences table.
type AuthorOccurrenceRepository struct {
	db *Database
}

// Create inserts a new author occurrence. If the ORCID is non-empty and
// passes format-and-checksum validation, the method looks up or creates a
// Person record and links the occurrence to it. Invalid or malformed ORCIDs
// are stored as raw observed values on the occurrence but do not create or
// link to a person record — the design requires a strong identity signal
// before global merging, and an unvalidated string is not a strong signal.
func (r *AuthorOccurrenceRepository) Create(ao *AuthorOccurrence) (int64, error) {
	if ao.CitationName == "" {
		return 0, fmt.Errorf("create author occurrence: citation_name is required")
	}

	var personID any
	if ao.ORCID != "" {
		orcid := normalizeORCID(ao.ORCID)
		if isValidORCID(orcid) {
			pid, err := r.db.People.CreateByORCID(orcid)
			if err != nil {
				lg.Debug("author occurrence person lookup failed",
					"orcid", orcid, "error", err)
				return 0, fmt.Errorf("create author occurrence: %w", err)
			}
			personID = pid
			ao.PersonID = pid
		} else {
			lg.Debug("author occurrence skipped person linking",
				"orcid", orcid, "reason", "invalid_orcid_format_or_checksum")
		}
	}

	res, err := r.db.DB.Exec(`
		INSERT INTO author_occurrences
			(person_id, citation_name, first_name, last_name, orcid)
		VALUES (?, ?, ?, ?, ?)`,
		personID, ao.CitationName, nullStr(ao.FirstName), nullStr(ao.LastName),
		nullStr(ao.ORCID),
	)
	if err != nil {
		lg.Debug("author occurrence creation failed",
			"citation_name", ao.CitationName, "error", err)
		return 0, fmt.Errorf("create author occurrence: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("author occurrence inserted ID read failed",
			"citation_name", ao.CitationName, "error", err)
		return 0, err
	}
	lg.Debug("author occurrence creation successful",
		"citation_name", ao.CitationName, "occurrence_id", id,
		"person_id", personID)
	return id, nil
}

// GetByID returns an author occurrence by its primary key, or nil if not found.
func (r *AuthorOccurrenceRepository) GetByID(id int64) (*AuthorOccurrence, error) {
	return scanAuthorOccurrence(r.db.DB.QueryRow(`
		SELECT id, person_id, citation_name, first_name, last_name, orcid, created_at
		FROM author_occurrences WHERE id = ?`, id))
}

// GetByPersonID returns all author occurrences linked to a given person, in
// ID order.
func (r *AuthorOccurrenceRepository) GetByPersonID(personID int64) ([]*AuthorOccurrence, error) {
	rows, err := r.db.DB.Query(`
		SELECT id, person_id, citation_name, first_name, last_name, orcid, created_at
		FROM author_occurrences WHERE person_id = ? ORDER BY id`, personID)
	if err != nil {
		lg.Debug("author occurrence list by person query failed",
			"person_id", personID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*AuthorOccurrence
	for rows.Next() {
		ao, err := scanAuthorOccurrence(rows)
		if err != nil {
			lg.Debug("author occurrence list by person row scan failed",
				"person_id", personID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, ao)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("author occurrence list by person iteration failed",
			"person_id", personID, "error", err)
		return nil, err
	}
	lg.Debug("author occurrence list by person query successful",
		"person_id", personID, "occurrences", len(result))
	return result, nil
}

// AuthorshipRepository provides CRUD for the authorships table.
type AuthorshipRepository struct {
	db *Database
}

// Create inserts a new authorship linking a work revision to an author
// occurrence with the given order and optional affiliation.
func (r *AuthorshipRepository) Create(a *Authorship) (int64, error) {
	if a.WorkRevisionID == 0 {
		return 0, fmt.Errorf("create authorship: work_revision_id is required")
	}
	if a.AuthorOccurrenceID == 0 {
		return 0, fmt.Errorf("create authorship: author_occurrence_id is required")
	}
	if a.AuthorOrder < 1 {
		return 0, fmt.Errorf("create authorship: author_order must be >= 1")
	}

	res, err := r.db.DB.Exec(`
		INSERT INTO authorships
			(work_revision_id, author_occurrence_id, author_order, affiliation)
		VALUES (?, ?, ?, ?)`,
		a.WorkRevisionID, a.AuthorOccurrenceID, a.AuthorOrder, nullStr(a.Affiliation),
	)
	if err != nil {
		lg.Debug("authorship creation failed",
			"revision_id", a.WorkRevisionID, "occurrence_id", a.AuthorOccurrenceID,
			"order", a.AuthorOrder, "error", err)
		return 0, fmt.Errorf("create authorship: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("authorship inserted ID read failed",
			"revision_id", a.WorkRevisionID, "occurrence_id", a.AuthorOccurrenceID,
			"error", err)
		return 0, err
	}
	lg.Debug("authorship creation successful",
		"revision_id", a.WorkRevisionID, "occurrence_id", a.AuthorOccurrenceID,
		"order", a.AuthorOrder, "id", id)
	return id, nil
}

// GetByRevisionID returns all authorships for a given work revision, ordered
// by author_order.
func (r *AuthorshipRepository) GetByRevisionID(revisionID int64) ([]*Authorship, error) {
	rows, err := r.db.DB.Query(`
		SELECT id, work_revision_id, author_occurrence_id, author_order, affiliation, created_at
		FROM authorships WHERE work_revision_id = ? ORDER BY author_order`, revisionID)
	if err != nil {
		lg.Debug("authorship list by revision query failed",
			"revision_id", revisionID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*Authorship
	for rows.Next() {
		a, err := scanAuthorship(rows)
		if err != nil {
			lg.Debug("authorship list by revision row scan failed",
				"revision_id", revisionID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("authorship list by revision iteration failed",
			"revision_id", revisionID, "error", err)
		return nil, err
	}
	lg.Debug("authorship list by revision query successful",
		"revision_id", revisionID, "authorships", len(result))
	return result, nil
}

// GetByOccurrenceID returns all authorships for a given author occurrence,
// ordered by ID.
func (r *AuthorshipRepository) GetByOccurrenceID(occurrenceID int64) ([]*Authorship, error) {
	rows, err := r.db.DB.Query(`
		SELECT id, work_revision_id, author_occurrence_id, author_order, affiliation, created_at
		FROM authorships WHERE author_occurrence_id = ? ORDER BY id`, occurrenceID)
	if err != nil {
		lg.Debug("authorship list by occurrence query failed",
			"occurrence_id", occurrenceID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*Authorship
	for rows.Next() {
		a, err := scanAuthorship(rows)
		if err != nil {
			lg.Debug("authorship list by occurrence row scan failed",
				"occurrence_id", occurrenceID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("authorship list by occurrence iteration failed",
			"occurrence_id", occurrenceID, "error", err)
		return nil, err
	}
	lg.Debug("authorship list by occurrence query successful",
		"occurrence_id", occurrenceID, "authorships", len(result))
	return result, nil
}

// scanAuthorOccurrence decodes author occurrence from a database row.
func scanAuthorOccurrence(row scannable) (*AuthorOccurrence, error) {
	var ao AuthorOccurrence
	var personID sql.NullInt64
	var citationName, fn, ln, oc sql.NullString
	err := row.Scan(&ao.ID, &personID, &citationName, &fn, &ln, &oc, &ao.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		lg.Debug("author occurrence scan failed", "error", err)
		return nil, err
	}
	if citationName.Valid {
		ao.CitationName = citationName.String
	}
	if fn.Valid {
		ao.FirstName = fn.String
	}
	if ln.Valid {
		ao.LastName = ln.String
	}
	if oc.Valid {
		ao.ORCID = oc.String
	}
	if personID.Valid {
		ao.PersonID = personID.Int64
	}
	return &ao, nil
}

// scanAuthorship decodes authorship from a database row.
func scanAuthorship(row scannable) (*Authorship, error) {
	var a Authorship
	var affiliation sql.NullString
	err := row.Scan(&a.ID, &a.WorkRevisionID, &a.AuthorOccurrenceID,
		&a.AuthorOrder, &affiliation, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		lg.Debug("authorship scan failed", "error", err)
		return nil, err
	}
	a.Affiliation = nullStrPtrVal(affiliation)
	return &a, nil
}

// normalizeORCID lowercases and trims whitespace from an ORCID string.
// It does not validate the ORCID format; callers should use isValidORCID.
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
	// Accept both uppercase and lowercase for the check character
	return byte(orcid[18]) == expected || byte(orcid[18]) == expected+32
}
