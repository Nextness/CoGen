// metrics_audit.go provides the repository for pipeline-run audit events,
// recording field-enrichment provenance and per-field audit trails for each
// pipeline run.
package database

import (
	"database/sql"
	"fmt"

	"analysis/manifest"
)

// AuditEventRecord is the persisted representation of an audit event.
type AuditEventRecord struct {
	ID            int64  `json:"id"`
	OccurredAt    string `json:"occurred_at"`
	Actor         string `json:"actor"`
	PipelineRunID *int64 `json:"pipeline_run_id,omitempty"`
	EntityType    string `json:"entity_type"`
	EntityID      string `json:"entity_id"`
	Action        string `json:"action"`
	BeforeJSON    string `json:"before_json,omitempty"`
	AfterJSON     string `json:"after_json,omitempty"`
	MetadataJSON  string `json:"metadata_json,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// AuditEventRepository provides CRUD for the audit_events table.
type AuditEventRepository struct {
	db *Database
}

// Insert stores a new audit event. The event's action is validated against
// the manifest lifecycle vocabulary before insertion.
func (r *AuditEventRepository) Insert(event *manifest.AuditEvent) (int64, error) {
	if event == nil {
		return 0, fmt.Errorf("insert audit event: value is required")
	}
	if err := manifest.ValidateAuditAction(string(event.Action)); err != nil {
		lg.Debug("audit event insert rejected", "action", event.Action, "error", err)
		return 0, err
	}

	var runID any
	if event.PipelineRunID != 0 {
		runID = event.PipelineRunID
	}

	res, err := r.db.DB.Exec(
		`INSERT INTO audit_events
			(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action,
			 before_json, after_json, metadata_json, correlation_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.OccurredAt, event.Actor, runID,
		event.EntityType, event.EntityID, string(event.Action),
		nullStr(event.BeforeJSON), nullStr(event.AfterJSON),
		nullStr(event.MetadataJSON), nullStr(event.CorrelationID),
	)
	if err != nil {
		lg.Debug("audit event insert failed", "action", event.Action, "entity", event.EntityType, "id", event.EntityID, "error", err)
		return 0, fmt.Errorf("insert audit event: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("audit event ID read failed", "action", event.Action, "entity", event.EntityType, "error", err)
		return 0, err
	}
	lg.Debug("audit event insert successful", "id", id, "action", event.Action, "entity", event.EntityType, "entity_id", event.EntityID)
	return id, nil
}

// ListByRun returns all audit events for a given pipeline run, ordered by ID.
func (r *AuditEventRepository) ListByRun(runID int64) ([]*AuditEventRecord, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id,
			action, before_json, after_json, metadata_json, correlation_id
		FROM audit_events WHERE pipeline_run_id = ? ORDER BY id`,
		runID,
	)
	if err != nil {
		lg.Debug("audit event list by run failed", "run_id", runID, "error", err)
		return nil, err
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

// ListByEntity returns all audit events for a given entity type and ID, ordered by ID.
func (r *AuditEventRepository) ListByEntity(entityType, entityID string) ([]*AuditEventRecord, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id,
			action, before_json, after_json, metadata_json, correlation_id
		FROM audit_events WHERE entity_type = ? AND entity_id = ? ORDER BY id`,
		entityType, entityID,
	)
	if err != nil {
		lg.Debug("audit event list by entity failed", "entity_type", entityType, "entity_id", entityID, "error", err)
		return nil, err
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

// scanAuditEvents decodes audit events from a database row.
func scanAuditEvents(rows *sql.Rows) ([]*AuditEventRecord, error) {
	var result []*AuditEventRecord
	for rows.Next() {
		var e AuditEventRecord
		var runID sql.NullInt64
		var beforeJSON, afterJSON, metadataJSON, correlationID sql.NullString
		if err := rows.Scan(
			&e.ID, &e.OccurredAt, &e.Actor, &runID,
			&e.EntityType, &e.EntityID, &e.Action,
			&beforeJSON, &afterJSON, &metadataJSON, &correlationID,
		); err != nil {
			lg.Debug("audit event scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		if runID.Valid {
			e.PipelineRunID = &runID.Int64
		}
		if beforeJSON.Valid {
			e.BeforeJSON = beforeJSON.String
		}
		if afterJSON.Valid {
			e.AfterJSON = afterJSON.String
		}
		if metadataJSON.Valid {
			e.MetadataJSON = metadataJSON.String
		}
		if correlationID.Valid {
			e.CorrelationID = correlationID.String
		}
		result = append(result, &e)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("audit event iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("audit event scan successful", "events", len(result))
	return result, nil
}
