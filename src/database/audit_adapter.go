// audit_adapter.go provides temporary forwarding adapters that keep the legacy
// flat audit-event repository API working over the audit family store. The
// adapters contain no SQL or second implementation; remove them after every
// caller migrates to Database.Audit.
package database

import (
	"context"

	"analysis/database/audit"
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

// AuditEventRepository forwards the legacy audit-event API to the audit family store.
type AuditEventRepository struct{ db *Database }

// Insert stores a new audit event. The event's action is validated against
// the manifest lifecycle vocabulary before insertion.
func (r *AuditEventRepository) Insert(event *manifest.AuditEvent) (int64, error) {
	return r.db.Audit.Insert(context.Background(), event)
}

// ListByRun returns all audit events for a given pipeline run, ordered by ID.
func (r *AuditEventRepository) ListByRun(runID int64) ([]*AuditEventRecord, error) {
	found, err := r.db.Audit.ListByRun(context.Background(), runID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*AuditEventRecord, 0, len(found))
	for _, event := range found {
		legacy = append(legacy, auditEventFromFamily(event))
	}
	return legacy, nil
}

// ListByEntity returns all audit events for a given entity type and ID, ordered by ID.
func (r *AuditEventRepository) ListByEntity(entityType, entityID string) ([]*AuditEventRecord, error) {
	found, err := r.db.Audit.ListByEntity(context.Background(), entityType, entityID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*AuditEventRecord, 0, len(found))
	for _, event := range found {
		legacy = append(legacy, auditEventFromFamily(event))
	}
	return legacy, nil
}

// auditEventFromFamily maps an audit family event into the legacy application type.
func auditEventFromFamily(found *audit.Event) *AuditEventRecord {
	if found == nil {
		return nil
	}
	return &AuditEventRecord{
		ID:            found.ID,
		OccurredAt:    found.OccurredAt,
		Actor:         found.Actor,
		PipelineRunID: found.PipelineRunID,
		EntityType:    found.EntityType,
		EntityID:      found.EntityID,
		Action:        found.Action,
		BeforeJSON:    found.BeforeJSON,
		AfterJSON:     found.AfterJSON,
		MetadataJSON:  found.MetadataJSON,
		CorrelationID: found.CorrelationID,
	}
}
