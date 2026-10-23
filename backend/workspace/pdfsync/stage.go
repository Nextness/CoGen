// Package pdfsync owns the workspace normalized PDF inventory reconciliation.
package pdfsync

import (
	"analysis/backend/database"

	dbwork "analysis/backend/database/work"

	"analysis/backend/pdfstore"

	"context"

	"fmt"
)

const normalizedInventoryBatchSize = 100

// Sync reconciles the companion inventory from the
// authoritative normalized revisions. It is safe to call after a new run or
// when a completed execution plan is reused.
func Sync(ctx context.Context, db *database.Database, dbPath, registryPath string) (int, int, error) {
	bindings := db.PDFBindings
	storePath, err := bindings.ResolveBinding(ctx, dbPath)
	if err != nil {
		return 0, 0, err
	}
	store, err := pdfstore.Open(storePath, registryPath)
	if err != nil {
		return 0, 0, err
	}
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
