// Package pdfsync delegates the workspace normalized PDF inventory
// reconciliation to the bundle consumer.
package pdfsync

import (
	"analysis/backend/bundle"
	"analysis/backend/database"
	"context"
)

// Sync reconciles the companion inventory from the authoritative normalized
// revisions after bundle preparation. It is safe to call after a new run or
// when a completed execution plan is reused.
func Sync(ctx context.Context, db *database.Database, dbPath, registryPath string) (int, int, error) {
	return bundle.SyncInventory(ctx, db, dbPath, registryPath)
}
