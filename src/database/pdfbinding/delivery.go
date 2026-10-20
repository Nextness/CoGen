// delivery.go provides the metadata-side delivered-event audit recording used
// by the cross-database PDF outbox protocol.
package pdfbinding

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	generated "analysis/database/pdfbinding/internal/sql"
)

// RecordDeliveredAudit records one idempotent delivered-event audit row and
// link inside the caller's metadata transaction. An event whose pipeline run
// no longer exists in the metadata database is preserved with a NULL run link,
// because the companion PDF store is durable across metadata iterations.
func (s *Store) RecordDeliveredAudit(ctx context.Context, tx *sql.Tx, event DeliveredEvent) error {
	queries := s.queries.WithTx(tx)
	if _, err := queries.GetPDFAuditLink(ctx, event.EventKey); err == nil {
		return nil
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("read PDF metadata audit link: %w", err)
	}
	var pipelineRunID sql.NullInt64
	if event.PipelineRunID > 0 {
		if _, err := queries.PipelineRunExists(ctx, event.PipelineRunID); err == sql.ErrNoRows {
			pipelineRunID = sql.NullInt64{}
		} else if err != nil {
			return fmt.Errorf("read pipeline run for PDF metadata audit event: %w", err)
		} else {
			pipelineRunID = sql.NullInt64{Int64: event.PipelineRunID, Valid: true}
		}
	}
	result, err := queries.InsertPDFMetadataAuditEvent(ctx, generated.InsertPDFMetadataAuditEventParams{
		OccurredAt:    event.OccurredAt,
		Actor:         event.Actor,
		PipelineRunID: pipelineRunID,
		EntityType:    event.EntityType,
		EntityID:      event.EntityID,
		Action:        event.Action,
		MetadataJson:  sql.NullString{String: event.MetadataJSON, Valid: true},
		CorrelationID: sql.NullString{String: event.CorrelationID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert PDF metadata audit event: %w", err)
	}
	auditID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	if err := queries.InsertPDFAuditLink(ctx, generated.InsertPDFAuditLinkParams{
		EventKey:     event.EventKey,
		AuditEventID: auditID,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return fmt.Errorf("link PDF metadata audit event: %w", err)
	}
	return nil
}
