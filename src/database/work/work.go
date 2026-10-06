// work.go provides the handwritten stable-work and alternate-identifier data
// access layer over the generated queries in this family's private internal/sql
// package. Revisions, stage outcomes, references, and term coverage live in
// their specifically named files.
package work

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	generated "analysis/database/work/internal/sql"
)

// Store binds the generated work queries to one already configured connection.
type Store struct {
	db      *sql.DB
	queries *generated.Queries
}

// New returns a work family store over an already configured connection. It
// only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations.
func New(db *sql.DB) *Store {
	return &Store{db: db, queries: generated.New(db)}
}

// NormalizeDOI applies the canonical DOI representation used by the works
// table and the companion PDF store.
func NormalizeDOI(doi string) string {
	doi = strings.TrimSpace(doi)
	doi = strings.ToLower(doi)
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	doi = strings.TrimPrefix(doi, "http://doi.org/")
	return doi
}

// CreateWorkByDOI inserts a new work by DOI. The DOI is normalized before
// storage. If the normalized DOI already exists, returns the existing work ID
// (INSERT OR IGNORE semantics).
func (s *Store) CreateWorkByDOI(ctx context.Context, doi string) (int64, error) {
	doi = NormalizeDOI(doi)
	if doi == "" {
		return 0, fmt.Errorf("create work: doi is empty")
	}

	result, err := s.queries.InsertWorkByDOI(ctx, sql.NullString{String: doi, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("create work: %w", err)
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

	// Already exists - return the existing work ID.
	existing, err := s.GetWorkByDOI(ctx, doi)
	if err != nil {
		return 0, err
	}
	if existing == nil {
		return 0, fmt.Errorf("create work: insert skipped but existing row not found")
	}
	return existing.ID, nil
}

// CreateWorkWithoutDOI inserts a new work without a DOI (for example a
// title-only record). Each call creates a distinct row; uncertain records are
// never globally merged.
func (s *Store) CreateWorkWithoutDOI(ctx context.Context) (int64, error) {
	result, err := s.queries.InsertWorkWithoutDOI(ctx)
	if err != nil {
		return 0, fmt.Errorf("create work without DOI: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetWorkByID returns a work by its primary key, or nil if not found.
func (s *Store) GetWorkByID(ctx context.Context, id int64) (*Work, error) {
	row, err := s.queries.GetWorkByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return workFromGenerated(row), nil
}

// GetWorkByDOI returns a work by its normalized DOI, or nil if not found.
// Empty or blank input short-circuits to nil without a query.
func (s *Store) GetWorkByDOI(ctx context.Context, doi string) (*Work, error) {
	doi = NormalizeDOI(doi)
	if doi == "" {
		return nil, nil
	}
	row, err := s.queries.GetWorkByDOI(ctx, sql.NullString{String: doi, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return workFromGenerated(row), nil
}

// ListWorksByIDs returns works matching the given IDs, in ID order. An empty
// list short-circuits to nil.
func (s *Store) ListWorksByIDs(ctx context.Context, ids []int64) ([]*Work, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.queries.ListWorksByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	var works []*Work
	for _, row := range rows {
		works = append(works, workFromGenerated(row))
	}
	return works, nil
}

// CountWorks returns the total number of works.
func (s *Store) CountWorks(ctx context.Context) (int, error) {
	count, err := s.queries.CountWorks(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// InsertIdentifier adds a new identifier for a work. If the
// (namespace, identifier) pair already exists for the same work, returns the
// existing ID. If it belongs to a different work, returns an error to prevent
// silent ownership conflicts.
func (s *Store) InsertIdentifier(ctx context.Context, input InsertIdentifierInput) (int64, error) {
	if input.Namespace == "" || input.Identifier == "" {
		return 0, fmt.Errorf("insert work identifier: namespace and identifier are required")
	}

	result, err := s.queries.InsertWorkIdentifier(ctx, generated.InsertWorkIdentifierParams{
		WorkID:     input.WorkID,
		Namespace:  input.Namespace,
		Identifier: input.Identifier,
	})
	if err != nil {
		return 0, fmt.Errorf("insert work identifier: %w", err)
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

	// Already exists - verify ownership and return the existing identifier ID.
	existing, err := s.queries.GetWorkIdentifierByNamespaceAndIdentifier(ctx, generated.GetWorkIdentifierByNamespaceAndIdentifierParams{
		Namespace:  input.Namespace,
		Identifier: input.Identifier,
	})
	if err != nil {
		return 0, err
	}
	if existing.WorkID != input.WorkID {
		return 0, fmt.Errorf("identifier %q in namespace %q already belongs to work %d, cannot reassign to work %d",
			input.Identifier, input.Namespace, existing.WorkID, input.WorkID)
	}
	return existing.ID, nil
}

// GetIdentifierByID returns a work identifier by its primary key, or nil if not found.
func (s *Store) GetIdentifierByID(ctx context.Context, id int64) (*Identifier, error) {
	row, err := s.queries.GetWorkIdentifierByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return identifierFromGenerated(row), nil
}

// ListIdentifiersByWorkID returns all identifiers for a given work, ordered by ID.
func (s *Store) ListIdentifiersByWorkID(ctx context.Context, workID int64) ([]*Identifier, error) {
	rows, err := s.queries.ListWorkIdentifiersByWorkID(ctx, workID)
	if err != nil {
		return nil, err
	}
	var identifiers []*Identifier
	for _, row := range rows {
		identifiers = append(identifiers, identifierFromGenerated(row))
	}
	return identifiers, nil
}

// GetIdentifierByNamespaceAndIdentifier returns the work identifier record for
// the given namespace and identifier pair, or nil if not found.
func (s *Store) GetIdentifierByNamespaceAndIdentifier(ctx context.Context, namespace, identifier string) (*Identifier, error) {
	row, err := s.queries.GetWorkIdentifierByNamespaceAndIdentifier(ctx, generated.GetWorkIdentifierByNamespaceAndIdentifierParams{
		Namespace:  namespace,
		Identifier: identifier,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return identifierFromGenerated(row), nil
}

// CountIdentifiersByWorkID returns the number of identifiers for a given work.
func (s *Store) CountIdentifiersByWorkID(ctx context.Context, workID int64) (int, error) {
	count, err := s.queries.CountWorkIdentifiersByWorkID(ctx, workID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// workFromGenerated maps one generated work row into an application work and
// makes the nullable DOI explicit.
func workFromGenerated(row generated.Work) *Work {
	work := &Work{
		ID:        row.ID,
		CreatedAt: row.CreatedAt,
	}
	if row.Doi.Valid {
		work.DOI = row.Doi.String
	}
	return work
}

// identifierFromGenerated maps one generated identifier row into an application identifier.
func identifierFromGenerated(row generated.WorkIdentifier) *Identifier {
	return &Identifier{
		ID:         row.ID,
		WorkID:     row.WorkID,
		Namespace:  row.Namespace,
		Identifier: row.Identifier,
		CreatedAt:  row.CreatedAt,
	}
}

// nullableString maps empty optional text into a SQL NULL.
func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

// nullableInt64 maps an optional integer into its generated null wrapper.
func nullableInt64(value int64) sql.NullInt64 {
	if value == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value, Valid: true}
}

// requiredInt64 binds a required integer even when it is zero so a column's
// constraint reports the invalid value exactly as before.
func requiredInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: true}
}

// withTx runs fn inside one transaction bound to the generated queries and
// commits only when every statement succeeds.
func (s *Store) withTx(ctx context.Context, fn func(*generated.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		// Rollback on panic; if fn returned an error the tx is already rolled back.
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(s.queries.WithTx(tx)); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
