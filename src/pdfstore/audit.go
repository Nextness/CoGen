// audit.go provides the PDF store's transactional audit outbox,
// which buffers durability events and flushes them to the metadata
// database for cross-store traceability.
package pdfstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OutboxEvent carries metadata audit evidence awaiting cross-database delivery.
type OutboxEvent struct {
	EventKey      string
	OccurredAt    string
	Actor         string
	PipelineRunID int64
	EntityType    string
	EntityID      string
	Action        string
	MetadataJSON  string
	CorrelationID string
}

// insertOutbox inserts outbox.
func insertOutbox(ctx context.Context, tx *sql.Tx, event OutboxEvent, occurredAt string) error {
	if event.EventKey == "" {
		var err error
		event.EventKey, err = newCorrelationID()
		if err != nil {
			return err
		}
	}
	if event.OccurredAt == "" {
		event.OccurredAt = occurredAt
	}
	var pipelineRunID any
	if event.PipelineRunID > 0 {
		pipelineRunID = event.PipelineRunID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO pdf_audit_outbox
		(event_key, occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json, correlation_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.EventKey, event.OccurredAt, event.Actor,
		pipelineRunID, event.EntityType, event.EntityID, event.Action, event.MetadataJSON, event.CorrelationID)
	if err != nil {
		return fmt.Errorf("insert PDF audit outbox event: %w", err)
	}
	return nil
}

// FlushAuditOutbox mirrors undelivered PDF events into the metadata database.
// The metadata audit row and delivery link commit together. Marking the PDF
// event delivered is a separate idempotent step so crashes cannot duplicate an
// append-only audit row.
func (s *Store) FlushAuditOutbox(ctx context.Context, metadata *sql.DB) (int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT event_key, occurred_at, actor, pipeline_run_id, entity_type,
		entity_id, action, metadata_json, correlation_id
		FROM pdf_audit_outbox WHERE delivered_at IS NULL ORDER BY occurred_at, event_key`)
	if err != nil {
		return 0, fmt.Errorf("read PDF audit outbox: %w", err)
	}
	var events []OutboxEvent
	for rows.Next() {
		var event OutboxEvent
		var pipelineRunID sql.NullInt64
		if err := rows.Scan(&event.EventKey, &event.OccurredAt, &event.Actor, &pipelineRunID, &event.EntityType,
			&event.EntityID, &event.Action, &event.MetadataJSON, &event.CorrelationID); err != nil {
			rows.Close()
			return 0, err
		}
		if pipelineRunID.Valid {
			event.PipelineRunID = pipelineRunID.Int64
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	delivered := 0
	for _, event := range events {
		tx, err := metadata.BeginTx(ctx, nil)
		if err != nil {
			return delivered, err
		}
		var auditID int64
		err = tx.QueryRowContext(ctx, "SELECT audit_event_id FROM pdf_audit_links WHERE event_key=?", event.EventKey).Scan(&auditID)
		if err == sql.ErrNoRows {
			var pipelineRunID any
			if event.PipelineRunID > 0 {
				pipelineRunID = event.PipelineRunID
			}
			result, insertErr := tx.ExecContext(ctx, `INSERT INTO audit_events
				(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json, correlation_id)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, event.OccurredAt, event.Actor, pipelineRunID,
				event.EntityType, event.EntityID, event.Action, event.MetadataJSON, event.CorrelationID)
			if insertErr != nil {
				tx.Rollback()
				return delivered, fmt.Errorf("insert PDF metadata audit event: %w", insertErr)
			}
			auditID, insertErr = result.LastInsertId()
			if insertErr != nil {
				tx.Rollback()
				return delivered, insertErr
			}
			if _, insertErr = tx.ExecContext(ctx, `INSERT INTO pdf_audit_links
				(event_key, audit_event_id, created_at) VALUES (?, ?, ?)`, event.EventKey, auditID,
				time.Now().UTC().Format(time.RFC3339Nano)); insertErr != nil {
				tx.Rollback()
				return delivered, fmt.Errorf("link PDF metadata audit event: %w", insertErr)
			}
		} else if err != nil {
			tx.Rollback()
			return delivered, fmt.Errorf("read PDF metadata audit link: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return delivered, fmt.Errorf("commit PDF metadata audit event: %w", err)
		}
		if _, err := s.DB.ExecContext(ctx, `UPDATE pdf_audit_outbox SET delivered_at=?
			WHERE event_key=? AND delivered_at IS NULL`, time.Now().UTC().Format(time.RFC3339Nano), event.EventKey); err != nil {
			return delivered, fmt.Errorf("mark PDF audit event delivered: %w", err)
		}
		delivered++
	}
	return delivered, nil
}
