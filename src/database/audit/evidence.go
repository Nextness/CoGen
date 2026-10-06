// evidence.go provides the handwritten audit evidence reads over the generated
// statements. Recorded payloads, filtered timelines, summaries, facets, and
// enrichment labels are audit/provenance evidence, so their queries belong to
// this family even when they join work revisions owned by another family.
package audit

import (
	"context"
	"database/sql"
	"errors"

	generated "analysis/database/audit/internal/sql"
)

// List returns a bounded page of audit events matching the filter, ordered by
// occurrence time and ID descending. It returns at most Limit+1 events so the
// caller can detect a further page. A cursor that does not identify an event
// returns ErrCursorNotFound.
func (s *Store) List(ctx context.Context, filter Filter) ([]*Event, error) {
	clauses, args := auditFilterClauses(filter)
	if filter.Cursor > 0 {
		occurredAt, err := s.queries.GetAuditEventOccurredAt(ctx, filter.Cursor)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCursorNotFound
		}
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, "(COALESCE(julianday(occurred_at), 0)<COALESCE(julianday(?), 0) OR (COALESCE(julianday(occurred_at), 0)=COALESCE(julianday(?), 0) AND id<?))")
		args = append(args, occurredAt, occurredAt, filter.Cursor)
	}
	query := "SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, before_json, after_json, metadata_json, correlation_id FROM audit_events" +
		auditWhere(clauses) + " ORDER BY COALESCE(julianday(occurred_at), 0) DESC, id DESC LIMIT ?"
	args = append(args, filter.Limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// Summary returns the filtered audit event count and per-action counts.
func (s *Store) Summary(ctx context.Context, filter Filter) (*Summary, error) {
	clauses, args := auditFilterClauses(filter)
	where := auditWhere(clauses)
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events"+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT action, COUNT(*) AS count FROM audit_events"+where+" GROUP BY action ORDER BY count DESC, action", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	summary := &Summary{TotalEvents: total}
	for rows.Next() {
		var action string
		var count int64
		if err := rows.Scan(&action, &count); err != nil {
			return nil, err
		}
		summary.Actions = append(summary.Actions, ActionCount{Action: action, Count: count})
	}
	return summary, rows.Err()
}

// Facets returns distinct non-empty actor, action, and entity type values in
// the filter's run scope.
func (s *Store) Facets(ctx context.Context, filter Filter) (*Facets, error) {
	scopeClause, scopeArgs := "", []any(nil)
	if filter.RunID > 0 {
		scopeClause, scopeArgs = auditRunScopeClause(filter)
	}
	actors, err := s.auditFacet(ctx, "actor", scopeClause, scopeArgs)
	if err != nil {
		return nil, err
	}
	actions, err := s.auditFacet(ctx, "action", scopeClause, scopeArgs)
	if err != nil {
		return nil, err
	}
	entityTypes, err := s.auditFacet(ctx, "entity_type", scopeClause, scopeArgs)
	if err != nil {
		return nil, err
	}
	return &Facets{Actors: actors, Actions: actions, EntityTypes: entityTypes}, nil
}

// auditFacet returns distinct non-empty values for an allowlisted audit column
// and run scope, stopping at 100 values.
func (s *Store) auditFacet(ctx context.Context, column, scopeClause string, scopeArgs []any) ([]string, error) {
	query := "SELECT DISTINCT COALESCE(" + column + ", '') FROM audit_events"
	if scopeClause != "" {
		query += " WHERE " + scopeClause
	}
	query += " ORDER BY " + column + " LIMIT 101"
	rows, err := s.db.QueryContext(ctx, query, scopeArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value != "" {
			values = append(values, value)
		}
		if len(values) == 100 {
			break
		}
	}
	return values, rows.Err()
}

// scanEvents decodes audit events from a database row set.
func scanEvents(rows *sql.Rows) ([]*Event, error) {
	var events []*Event
	for rows.Next() {
		var event Event
		var runID sql.NullInt64
		var beforeJSON, afterJSON, metadataJSON, correlationID sql.NullString
		if err := rows.Scan(
			&event.ID, &event.OccurredAt, &event.Actor, &runID,
			&event.EntityType, &event.EntityID, &event.Action,
			&beforeJSON, &afterJSON, &metadataJSON, &correlationID,
		); err != nil {
			return nil, err
		}
		if runID.Valid {
			event.PipelineRunID = &runID.Int64
		}
		if beforeJSON.Valid {
			event.BeforeJSON = beforeJSON.String
		}
		if afterJSON.Valid {
			event.AfterJSON = afterJSON.String
		}
		if metadataJSON.Valid {
			event.MetadataJSON = metadataJSON.String
		}
		if correlationID.Valid {
			event.CorrelationID = correlationID.String
		}
		events = append(events, &event)
	}
	return events, rows.Err()
}

// RecordedData returns one event's recorded JSON payloads when the event is
// visible in the supplied run scope. It returns nil, nil when the event does
// not exist or is outside the scope.
func (s *Store) RecordedData(ctx context.Context, eventID, runID int64) (*RecordedData, error) {
	row, err := s.queries.GetAuditEventRecordedData(ctx, generated.GetAuditEventRecordedDataParams{
		ID:            eventID,
		PipelineRunID: nullableRunID(runID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	data := &RecordedData{ID: row.ID}
	if row.BeforeJson.Valid {
		data.BeforeJSON = row.BeforeJson.String
	}
	if row.AfterJson.Valid {
		data.AfterJSON = row.AfterJson.String
	}
	if row.MetadataJson.Valid {
		data.MetadataJSON = row.MetadataJson.String
	}
	return data, nil
}

// EnrichmentSummary returns the bounded provider and field labels recorded for
// one work revision in a run. It stops at 100 distinct labels and reports
// whether the underlying set was truncated.
func (s *Store) EnrichmentSummary(ctx context.Context, workID, runID int64) (*EnrichmentSummary, error) {
	rows, err := s.queries.ListEnrichmentSummary(ctx, generated.ListEnrichmentSummaryParams{
		WorkID:        workID,
		PipelineRunID: runID,
	})
	if err != nil {
		return nil, err
	}
	summary := &EnrichmentSummary{Providers: make([]string, 0), Fields: make([]string, 0)}
	providerSeen, fieldSeen := make(map[string]bool), make(map[string]bool)
	count := 0
	for _, row := range rows {
		count++
		if count > 100 {
			summary.Truncated = true
			break
		}
		if row.Provider != "" && !providerSeen[row.Provider] {
			providerSeen[row.Provider] = true
			summary.Providers = append(summary.Providers, row.Provider)
		}
		if row.Field != "" && !fieldSeen[row.Field] {
			fieldSeen[row.Field] = true
			summary.Fields = append(summary.Fields, row.Field)
		}
	}
	return summary, nil
}
