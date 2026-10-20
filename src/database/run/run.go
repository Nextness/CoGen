// run.go provides the handwritten pipeline-run data access layer over the
// generated queries in this family's private internal/sql package. It owns run
// lookup and plan or visibility listing; lifecycle transitions,
// reviewer attribution, steps, metrics, ancestry, and recovery live in
// their specifically named files.
package run

import (
	"context"
	"database/sql"
	"errors"
	"time"

	generated "analysis/database/run/internal/sql"
)

// Store binds the generated run queries to one already configured connection.
type Store struct {
	db      *sql.DB
	queries *generated.Queries
}

// New returns a run family store over an already configured connection. It
// only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations.
func New(db *sql.DB) *Store {
	return &Store{db: db, queries: generated.New(db)}
}

// GetByID returns a run by its primary key, or nil if not found.
func (s *Store) GetByID(ctx context.Context, runID int64) (*Run, error) {
	row, err := s.queries.GetRunByID(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return runFromGenerated(row), nil
}

// ListByPlan returns all runs for a given execution plan, ordered by attempt number.
func (s *Store) ListByPlan(ctx context.Context, executionPlanID int64) ([]*Run, error) {
	rows, err := s.queries.ListRunsByPlan(ctx, nullableInt64(executionPlanID))
	if err != nil {
		return nil, err
	}
	var runs []*Run
	for _, row := range rows {
		runs = append(runs, runFromListByPlan(row))
	}
	return runs, nil
}

// ListByVisibility returns all runs with a given visibility state, ordered by ID.
func (s *Store) ListByVisibility(ctx context.Context, visibilityState string) ([]*Run, error) {
	rows, err := s.queries.ListRunsByVisibility(ctx, visibilityState)
	if err != nil {
		return nil, err
	}
	var runs []*Run
	for _, row := range rows {
		runs = append(runs, runFromListByVisibility(row))
	}
	return runs, nil
}

// ListCompletedRunIDs returns the IDs of every completed pipeline run in
// ascending order. It backs the workspace term-match reconciliation pass.
func (s *Store) ListCompletedRunIDs(ctx context.Context) ([]int64, error) {
	return s.queries.ListCompletedRunIDs(ctx)
}

// runFromGenerated maps one generated run row into an application run and
// makes every nullable lifecycle field explicit.
func runFromGenerated(row generated.GetRunByIDRow) *Run {
	run := &Run{
		ID:              row.ID,
		Step:            row.Step,
		StartedAt:       row.StartedAt,
		Status:          row.Status,
		VisibilityState: row.VisibilityState,
	}
	if row.FinishedAt.Valid {
		run.FinishedAt = &row.FinishedAt.String
	}
	if row.Summary.Valid {
		run.Summary = &row.Summary.String
	}
	if row.SearchQuery.Valid {
		run.SearchQuery = &row.SearchQuery.String
	}
	if row.ExecutionPlanID.Valid {
		run.ExecutionPlanID = &row.ExecutionPlanID.Int64
	}
	if row.AttemptNumber.Valid {
		attempt := int(row.AttemptNumber.Int64)
		run.AttemptNumber = &attempt
	}
	if row.TrashedAt.Valid {
		run.TrashedAt = &row.TrashedAt.String
	}
	if row.TrashReason.Valid {
		run.TrashReason = &row.TrashReason.String
	}
	return run
}

// runFromListByPlan maps one generated plan-ordered run row into an application run.
func runFromListByPlan(row generated.ListRunsByPlanRow) *Run {
	return runFromGenerated(generated.GetRunByIDRow(row))
}

// runFromListByVisibility maps one generated visibility-ordered run row into an application run.
func runFromListByVisibility(row generated.ListRunsByVisibilityRow) *Run {
	return runFromGenerated(generated.GetRunByIDRow(row))
}

// runTimestamp returns the current UTC time in the run family's persisted format.
func runTimestamp() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

// nullableString maps empty optional text into a SQL NULL.
func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

// nullableStringPointer converts a scanned nullable text into an optional value.
func nullableStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

// nullableIntPointer converts a scanned nullable integer into an optional value.
func nullableIntPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

// nullableInt64Pointer converts a scanned nullable integer into an optional value.
func nullableInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

// Exists reports whether a pipeline run with the given ID exists.
func (s *Store) Exists(ctx context.Context, runID int64) (bool, error) {
	if _, err := s.queries.RunExists(ctx, runID); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// HasExecutionPlan reports whether the run is linked to an execution plan.
func (s *Store) HasExecutionPlan(ctx context.Context, runID int64) (bool, error) {
	hasPlan, err := s.queries.RunHasExecutionPlan(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return hasPlan, nil
}

// nullableInt64 maps a required identifier into its generated null wrapper.
func nullableInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: true}
}
