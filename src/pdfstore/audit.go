// audit.go provides the PDF store's transactional audit outbox,
// which buffers durability events and flushes them to the metadata
// database for cross-store traceability.
package pdfstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const auditOutboxBatchSize = 100

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
// append-only audit row. An event whose pipeline run no longer exists in the
// bound metadata database is preserved with a NULL run link, because the PDF
// store is durable across metadata database iterations.
func (s *Store) FlushAuditOutbox(ctx context.Context, metadata *sql.DB) (int, error) {
	delivered := 0
	var afterOccurredAt, afterEventKey string
	var firstErr error
	for {
		count, eventCount, occurredAt, eventKey, err := s.flushAuditOutboxBatch(ctx, metadata, auditOutboxBatchSize, afterOccurredAt, afterEventKey)
		delivered += count
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if eventCount == 0 || eventCount < auditOutboxBatchSize {
			return delivered, firstErr
		}
		afterOccurredAt, afterEventKey = occurredAt, eventKey
	}
}

// flushAuditOutboxBatch delivers one ordered bounded batch and leaves unmatched PDF updates retryable.
func (s *Store) flushAuditOutboxBatch(ctx context.Context, metadata *sql.DB, limit int, afterOccurredAt, afterEventKey string) (int, int, string, string, error) {
	query := `SELECT event_key, occurred_at, actor, pipeline_run_id, entity_type,
		entity_id, action, metadata_json, correlation_id FROM pdf_audit_outbox WHERE delivered_at IS NULL`
	args := make([]any, 0, 3)
	if afterOccurredAt != "" {
		query += " AND (occurred_at>? OR (occurred_at=? AND event_key>?))"
		args = append(args, afterOccurredAt, afterOccurredAt, afterEventKey)
	}
	query += " ORDER BY occurred_at, event_key LIMIT ?"
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("read PDF audit outbox: %w", err)
	}
	var events []OutboxEvent
	for rows.Next() {
		var event OutboxEvent
		var pipelineRunID sql.NullInt64
		if err := rows.Scan(&event.EventKey, &event.OccurredAt, &event.Actor, &pipelineRunID, &event.EntityType,
			&event.EntityID, &event.Action, &event.MetadataJSON, &event.CorrelationID); err != nil {
			rows.Close()
			return 0, 0, "", "", err
		}
		if pipelineRunID.Valid {
			event.PipelineRunID = pipelineRunID.Int64
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, "", "", err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, "", "", err
	}

	if len(events) == 0 {
		return 0, 0, "", "", nil
	}
	tx, err := metadata.BeginTx(ctx, nil)
	if err != nil {
		return 0, len(events), events[len(events)-1].OccurredAt, events[len(events)-1].EventKey, err
	}
	defer tx.Rollback()
	successful := make([]OutboxEvent, 0, len(events))
	var firstErr error
	for index, event := range events {
		savepoint := fmt.Sprintf("pdf_outbox_%d", index)
		if _, err := tx.ExecContext(ctx, "SAVEPOINT "+savepoint); err != nil {
			return 0, len(events), events[len(events)-1].OccurredAt, events[len(events)-1].EventKey, err
		}
		if err := insertMetadataAuditEvent(ctx, tx, event); err != nil {
			_, _ = tx.ExecContext(ctx, "ROLLBACK TO "+savepoint)
			_, _ = tx.ExecContext(ctx, "RELEASE "+savepoint)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, "RELEASE "+savepoint); err != nil {
			return 0, len(events), events[len(events)-1].OccurredAt, events[len(events)-1].EventKey, err
		}
		successful = append(successful, event)
	}
	if len(successful) == 0 {
		return 0, len(events), events[len(events)-1].OccurredAt, events[len(events)-1].EventKey, firstErr
	}
	if err := tx.Commit(); err != nil {
		return 0, len(events), events[len(events)-1].OccurredAt, events[len(events)-1].EventKey, fmt.Errorf("commit PDF metadata audit batch: %w", err)
	}
	keys := make([]string, len(successful))
	args = make([]any, 0, len(successful)+1)
	args = append(args, time.Now().UTC().Format(time.RFC3339Nano))
	for index, event := range successful {
		keys[index] = "?"
		args = append(args, event.EventKey)
	}
	query = `UPDATE pdf_audit_outbox SET delivered_at=?
		WHERE delivered_at IS NULL AND event_key IN (` + strings.Join(keys, ",") + `)`
	result, err := s.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, len(events), events[len(events)-1].OccurredAt, events[len(events)-1].EventKey, fmt.Errorf("mark PDF audit event batch delivered: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, len(events), events[len(events)-1].OccurredAt, events[len(events)-1].EventKey, fmt.Errorf("read delivered PDF audit event count: %w", err)
	}
	return int(count), len(events), events[len(events)-1].OccurredAt, events[len(events)-1].EventKey, firstErr
}

// insertMetadataAuditEvent persists one idempotent cross-store audit row in an existing transaction.
func insertMetadataAuditEvent(ctx context.Context, tx *sql.Tx, event OutboxEvent) error {
	var auditID int64
	err := tx.QueryRowContext(ctx, "SELECT audit_event_id FROM pdf_audit_links WHERE event_key=?", event.EventKey).Scan(&auditID)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("read PDF metadata audit link: %w", err)
	}
	var pipelineRunID any
	if event.PipelineRunID > 0 {
		var runExists int64
		runErr := tx.QueryRowContext(ctx, "SELECT 1 FROM pipeline_runs WHERE id=?", event.PipelineRunID).Scan(&runExists)
		if runErr == sql.ErrNoRows {
			pipelineRunID = nil
		} else if runErr != nil {
			return fmt.Errorf("read pipeline run for PDF metadata audit event: %w", runErr)
		} else {
			pipelineRunID = event.PipelineRunID
		}
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO audit_events
		(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json, correlation_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, event.OccurredAt, event.Actor, pipelineRunID,
		event.EntityType, event.EntityID, event.Action, event.MetadataJSON, event.CorrelationID)
	if err != nil {
		return fmt.Errorf("insert PDF metadata audit event: %w", err)
	}
	auditID, err = result.LastInsertId()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO pdf_audit_links
		(event_key, audit_event_id, created_at) VALUES (?, ?, ?)`, event.EventKey, auditID, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("link PDF metadata audit event: %w", err)
	}
	return nil
}
