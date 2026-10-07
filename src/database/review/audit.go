// audit.go appends review audit evidence inside the caller's transaction so
// head moves, version appends, and their evidence commit or roll back together.
package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"analysis/manifest"
)

// insertReviewAudit appends identifier-only review evidence within the caller's head-move transaction.
func insertReviewAudit(ctx context.Context, tx *sql.Tx, runID int64, entityType, entityID string, action manifest.AuditAction, metadata any) error {
	return insertReviewChangeAudit(ctx, tx, runID, entityType, entityID, action, nil, nil, metadata)
}

// insertReviewChangeAudit appends identifier metadata and optional bounded decision-state changes.
func insertReviewChangeAudit(ctx context.Context, tx *sql.Tx, runID int64, entityType, entityID string, action manifest.AuditAction, before, after, metadata any) error {
	if err := manifest.ValidateAuditAction(string(action)); err != nil {
		return err
	}
	encodedMetadata, err := marshalReviewAuditValue("metadata", metadata)
	if err != nil {
		return err
	}
	encodedBefore, err := marshalReviewAuditValue("before state", before)
	if err != nil {
		return err
	}
	encodedAfter, err := marshalReviewAuditValue("after state", after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events
		(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, before_json, after_json, metadata_json)
		VALUES (?, 'reviewer', ?, ?, ?, ?, ?, ?, ?)`, timestamp(), runID, entityType, entityID, string(action), encodedBefore, encodedAfter, encodedMetadata)
	if err != nil {
		return fmt.Errorf("insert review audit event: %w", err)
	}
	return nil
}

// marshalReviewAuditValue returns a nullable JSON payload for one review audit field.
func marshalReviewAuditValue(name string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal review audit %s: %w", name, err)
	}
	return string(encoded), nil
}
