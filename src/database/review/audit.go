// audit.go appends review audit evidence inside the caller's transaction so
// head moves, version appends, and their evidence commit or roll back together.
package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	generated "analysis/database/review/internal/sql"
	"analysis/manifest"
)

// insertReviewAudit appends identifier-only review evidence within the caller's head-move transaction.
func insertReviewAudit(ctx context.Context, queries *generated.Queries, runID int64, entityType, entityID string, action manifest.AuditAction, metadata any) error {
	return insertReviewChangeAudit(ctx, queries, runID, entityType, entityID, action, nil, nil, metadata)
}

// insertReviewChangeAudit appends identifier metadata and optional bounded decision-state changes.
func insertReviewChangeAudit(ctx context.Context, queries *generated.Queries, runID int64, entityType, entityID string, action manifest.AuditAction, before, after, metadata any) error {
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
	if err := queries.InsertReviewAuditEvent(ctx, generated.InsertReviewAuditEventParams{
		OccurredAt:    timestamp(),
		PipelineRunID: sql.NullInt64{Int64: runID, Valid: true},
		EntityType:    entityType,
		EntityID:      entityID,
		Action:        string(action),
		BeforeJson:    encodedBefore,
		AfterJson:     encodedAfter,
		MetadataJson:  encodedMetadata,
	}); err != nil {
		return fmt.Errorf("insert review audit event: %w", err)
	}
	return nil
}

// marshalReviewAuditValue returns a nullable JSON payload for one review audit field.
func marshalReviewAuditValue(name string, value any) (sql.NullString, error) {
	if value == nil {
		return sql.NullString{}, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("marshal review audit %s: %w", name, err)
	}
	return sql.NullString{String: string(encoded), Valid: true}, nil
}
