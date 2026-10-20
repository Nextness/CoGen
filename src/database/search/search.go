// search.go provides the handwritten search data access layer over the
// generated queries in this family's private internal/sql package.
package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/search/internal/sql"
)

// Store binds the generated search queries to one already configured connection.
type Store struct {
	queries *generated.Queries
}

// New returns a search family store over an already configured connection. It
// only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations.
func New(db *sql.DB) *Store {
	return &Store{queries: generated.New(db)}
}

// FindOrCreateSearch inserts a new search by its stable string identifier and
// returns the search ID. An existing identifier returns the existing ID.
func (s *Store) FindOrCreateSearch(ctx context.Context, searchID string) (int64, error) {
	result, err := s.queries.InsertSearchIgnore(ctx, searchID)
	if err != nil {
		return 0, fmt.Errorf("create search: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("create search: read affected rows: %w", err)
	}
	if rowsAffected > 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("create search: read inserted id: %w", err)
		}
		return id, nil
	}
	id, err := s.queries.GetSearchIDBySearchID(ctx, searchID)
	if err != nil {
		return 0, fmt.Errorf("create search: look up existing id: %w", err)
	}
	return id, nil
}

// GetSearchByID returns a search by its primary key, or nil if not found.
func (s *Store) GetSearchByID(ctx context.Context, id int64) (*Search, error) {
	row, err := s.queries.GetSearchByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get search: %w", err)
	}
	return searchFromGenerated(row), nil
}

// GetSearchBySearchID returns a search by its string identifier, or nil if not found.
func (s *Store) GetSearchBySearchID(ctx context.Context, searchID string) (*Search, error) {
	row, err := s.queries.GetSearchBySearchID(ctx, searchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get search: %w", err)
	}
	return searchFromGenerated(row), nil
}

// ListSearches returns all searches ordered by ID. An absent table returns a
// nil slice to preserve the current API shape.
func (s *Store) ListSearches(ctx context.Context) ([]*Search, error) {
	rows, err := s.queries.ListSearches(ctx)
	if err != nil {
		return nil, fmt.Errorf("list searches: %w", err)
	}
	var searches []*Search
	for _, row := range rows {
		searches = append(searches, searchFromGenerated(row))
	}
	return searches, nil
}

// searchFromGenerated maps one generated search row into an application search.
func searchFromGenerated(row generated.Search) *Search {
	return &Search{
		ID:        row.ID,
		SearchID:  row.SearchID,
		CreatedAt: row.CreatedAt,
	}
}
