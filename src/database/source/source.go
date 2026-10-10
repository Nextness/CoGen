// source.go provides the handwritten run-source data access layer over the
// generated queries in this family's private internal/sql package. It owns
// declared source creation, ordered listing, and observed result counts;
// records and filter counts live in their specifically named files.
package source

import (
	"context"
	"database/sql"
	"fmt"

	generated "analysis/database/source/internal/sql"
)

// Store binds the generated source queries to one already configured connection.
type Store struct {
	db      *sql.DB
	queries *generated.Queries
}

// New returns a source family store over an already configured connection. It
// only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations.
func New(db *sql.DB) *Store {
	return &Store{db: db, queries: generated.New(db)}
}

// CreateSource inserts a new run source link. Returns the source ID.
func (s *Store) CreateSource(ctx context.Context, input CreateSourceInput) (int64, error) {
	result, err := s.queries.InsertRunSource(ctx, generated.InsertRunSourceParams{
		PipelineRunID:       input.RunID,
		SourceName:          input.Name,
		SourceType:          input.Type,
		ExpectedFile:        input.ExpectedFile,
		Query:               nullableString(input.Query),
		RequestedFields:     nullableString(input.RequestedFields),
		ExpectedResultCount: nullableInt64(int64(input.ExpectedCount)),
		ExportDate:          nullableString(input.ExportDate),
	})
	if err != nil {
		return 0, fmt.Errorf("create run source: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListSourcesByRun returns all sources for a given pipeline run, ordered by ID.
func (s *Store) ListSourcesByRun(ctx context.Context, pipelineRunID int64) ([]*Source, error) {
	rows, err := s.queries.ListRunSourcesByRun(ctx, pipelineRunID)
	if err != nil {
		return nil, err
	}
	var sources []*Source
	for _, row := range rows {
		sources = append(sources, sourceFromGenerated(row))
	}
	return sources, nil
}

// SetObservedResultCount records the raw export count observed for a source
// and its informational comparison with the count declared in the config.
func (s *Store) SetObservedResultCount(ctx context.Context, input ObservedResultCountInput) error {
	if err := s.queries.SetRunSourceObservedResultCount(ctx, generated.SetRunSourceObservedResultCountParams{
		ObservedResultCount:   nullableInt64(int64(input.ObservedResultCount)),
		ResultCountComparison: requiredString(input.Comparison),
		ID:                    input.RunSourceID,
	}); err != nil {
		return fmt.Errorf("set observed source result count: %w", err)
	}
	return nil
}

// sourceFromGenerated maps one generated run-source row into an application
// source and makes every nullable export field explicit.
func sourceFromGenerated(row generated.RunSource) *Source {
	source := &Source{
		ID:            row.ID,
		PipelineRunID: row.PipelineRunID,
		SourceName:    row.SourceName,
		SourceType:    row.SourceType,
		ExpectedFile:  row.ExpectedFile,
		CreatedAt:     row.CreatedAt,
	}
	if row.Query.Valid {
		source.Query = row.Query.String
	}
	if row.RequestedFields.Valid {
		source.RequestedFields = row.RequestedFields.String
	}
	if row.ExpectedResultCount.Valid {
		value := int(row.ExpectedResultCount.Int64)
		source.ExpectedResultCount = &value
	}
	if row.ObservedResultCount.Valid {
		value := int(row.ObservedResultCount.Int64)
		source.ObservedResultCount = &value
	}
	if row.ResultCountComparison.Valid {
		source.ResultCountComparison = row.ResultCountComparison.String
	}
	if row.ExportDate.Valid {
		source.ExportDate = row.ExportDate.String
	}
	return source
}

// nullableString maps empty optional text into a SQL NULL.
func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

// requiredString binds a required text value even when it is empty so a
// column's CHECK constraint reports the invalid value exactly as before.
func requiredString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}

// nullableInt64 maps a required integer into its generated null wrapper.
func nullableInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: true}
}
