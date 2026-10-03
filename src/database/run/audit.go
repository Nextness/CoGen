// audit.go provides the run-owned local audit statements. Run lifecycle
// workflows append their evidence through the same transaction-bound generated
// queries rather than calling an audit family DAL that would open a second
// transaction.
package run

import (
	"context"
	"fmt"

	generated "analysis/database/run/internal/sql"
	"analysis/manifest"
)

// insertRecoveryAudit appends the operator recovery evidence for one abandoned
// attempt using the caller's transaction-bound queries.
func insertRecoveryAudit(ctx context.Context, queries *generated.Queries, runID int64) error {
	return queries.InsertRunRecoveryAudit(ctx, generated.InsertRunRecoveryAuditParams{
		OccurredAt:    runTimestamp(),
		PipelineRunID: nullableInt64(runID),
		EntityID:      fmt.Sprint(runID),
		Action:        string(manifest.AuditRunFailed),
		CorrelationID: nullableString(fmt.Sprintf("run-recovered-%d", runID)),
	})
}
