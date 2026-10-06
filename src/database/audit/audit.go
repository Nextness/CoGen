// audit.go provides the handwritten audit-event data access layer over the
// generated queries in this family's private internal/sql package. It owns
// append-only insertion and ordered run or entity reads; the viewer evidence
// reads and the dynamic filter builder live in their specifically named files.
package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/audit/internal/sql"
	"analysis/manifest"
)

// Store binds the generated audit queries to one already configured connection.
type Store struct {
	db      *sql.DB
	queries *generated.Queries
}

// New returns an audit family store over an already configured connection. It
// only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations.
func New(db *sql.DB) *Store {
	return &Store{db: db, queries: generated.New(db)}
}

// Insert stores a new audit event. The event's action is validated against the
// manifest lifecycle vocabulary before insertion.
func (s *Store) Insert(ctx context.Context, event *manifest.AuditEvent) (int64, error) {
	if event == nil {
		return 0, fmt.Errorf("insert audit event: value is required")
	}
	if err := manifest.ValidateAuditAction(string(event.Action)); err != nil {
		return 0, err
	}
	result, err := s.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{
		OccurredAt:    event.OccurredAt,
		Actor:         event.Actor,
		PipelineRunID: nullableRunID(event.PipelineRunID),
		EntityType:    event.EntityType,
		EntityID:      event.EntityID,
		Action:        string(event.Action),
		BeforeJson:    nullableString(event.BeforeJSON),
		AfterJson:     nullableString(event.AfterJSON),
		MetadataJson:  nullableString(event.MetadataJSON),
		CorrelationID: nullableString(event.CorrelationID),
	})
	if err != nil {
		return 0, fmt.Errorf("insert audit event: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListByRun returns all audit events for a given pipeline run, ordered by ID.
func (s *Store) ListByRun(ctx context.Context, runID int64) ([]*Event, error) {
	rows, err := s.queries.ListAuditEventsByRun(ctx, nullableRunID(runID))
	if err != nil {
		return nil, err
	}
	var events []*Event
	for _, row := range rows {
		events = append(events, eventFromGenerated(row))
	}
	return events, nil
}

// ListByEntity returns all audit events for a given entity type and ID, ordered by ID.
func (s *Store) ListByEntity(ctx context.Context, entityType, entityID string) ([]*Event, error) {
	rows, err := s.queries.ListAuditEventsByEntity(ctx, generated.ListAuditEventsByEntityParams{
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, err
	}
	var events []*Event
	for _, row := range rows {
		events = append(events, eventFromGenerated(row))
	}
	return events, nil
}

// eventFromGenerated maps one generated audit row into an application event and
// makes every nullable column explicit.
func eventFromGenerated(row generated.AuditEvent) *Event {
	event := &Event{
		ID:         row.ID,
		OccurredAt: row.OccurredAt,
		Actor:      row.Actor,
		EntityType: row.EntityType,
		EntityID:   row.EntityID,
		Action:     row.Action,
	}
	if row.PipelineRunID.Valid {
		event.PipelineRunID = &row.PipelineRunID.Int64
	}
	if row.BeforeJson.Valid {
		event.BeforeJSON = row.BeforeJson.String
	}
	if row.AfterJson.Valid {
		event.AfterJSON = row.AfterJson.String
	}
	if row.MetadataJson.Valid {
		event.MetadataJSON = row.MetadataJson.String
	}
	if row.CorrelationID.Valid {
		event.CorrelationID = row.CorrelationID.String
	}
	return event
}

// nullableRunID maps a zero run ID into SQL NULL.
func nullableRunID(runID int64) sql.NullInt64 {
	if runID == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: runID, Valid: true}
}

// nullableString maps empty optional text into SQL NULL.
func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

// ErrCursorNotFound reports that a cursor does not identify an audit event.
var ErrCursorNotFound = errors.New("cursor must identify an audit event")
