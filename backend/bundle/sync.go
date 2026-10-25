// sync.go reconciles the companion PDF inventory from authoritative normalized
// revisions and flushes pending cross-database audit delivery.
package bundle

import (
	"context"
	"fmt"

	"analysis/backend/database"
	dbwork "analysis/backend/database/work"
	"analysis/backend/pdfstore"
)

const normalizedInventoryBatchSize = 100

// SyncInventory reconciles the companion inventory from authoritative
// normalized revisions and flushes pending audit delivery. Callers must prepare
// both schemas first; this operation never migrates or initializes storage and
// never creates a research attempt.
func SyncInventory(ctx context.Context, db *database.Database, dbPath, registryPath string) (int, int, error) {
	bindings := db.PDFBindings
	storePath, found, err := bindings.ResolveBoundPath(ctx, dbPath)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, fmt.Errorf("metadata corpus has no PDF store binding; run analysis migrate to initialize it")
	}
	if _, err := VerifyReadiness(ctx, registryPath, Paths{Metadata: dbPath, PDF: storePath}); err != nil {
		return 0, 0, fmt.Errorf("verify bundle readiness: %w", err)
	}
	conn, err := database.OpenExistingConnection(storePath)
	if err != nil {
		return 0, 0, fmt.Errorf("open PDF store: %w", err)
	}
	store := pdfstore.New(conn)
	defer store.Close()

	flushed, err := store.FlushAuditOutbox(ctx, bindings)
	if err != nil {
		return 0, flushed, err
	}
	registered := 0
	lastWorkID := int64(0)
	for {
		works, err := normalizedInventoryBatch(ctx, db, lastWorkID)
		if err != nil {
			return registered, flushed, err
		}
		if len(works) == 0 {
			break
		}
		for _, work := range works {
			added, err := store.Register(ctx, work.DOI, work.WorkID, work.PipelineRunID)
			if err != nil {
				return registered, flushed, err
			}
			if added {
				registered++
			}
		}
		lastWorkID = works[len(works)-1].WorkID
	}
	newlyFlushed, err := store.FlushAuditOutbox(ctx, bindings)
	flushed += newlyFlushed
	if err != nil {
		return registered, flushed, err
	}
	return registered, flushed, nil
}

// normalizedInventoryBatch reads one bounded, deterministic inventory page and
// closes the metadata cursor before callers touch the companion database.
func normalizedInventoryBatch(ctx context.Context, db *database.Database, afterWorkID int64) ([]dbwork.NormalizedInventoryWork, error) {
	works, err := db.Work.ListNormalizedInventoryBatch(ctx, afterWorkID, normalizedInventoryBatchSize)
	if err != nil {
		return nil, fmt.Errorf("read normalized works: %w", err)
	}
	return works, nil
}
