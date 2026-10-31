package workspace

import (
	"analysis/backend/database"
	"context"
	"fmt"
)

// RecoverAbandonedRun marks one running attempt failed only while holding exclusive pipeline ownership.
func RecoverAbandonedRun(ctx context.Context, dbPath string, runID int64) error {
	lock, err := lockPipelineDatabase(dbPath, false)
	if err != nil {
		return err
	}
	defer lock.Close()
	maintenance, err := database.AcquireMaintenanceLock(ctx, dbPath, database.MaintenanceShared)
	if err != nil {
		return fmt.Errorf("acquire shared maintenance ownership: %w", err)
	}
	defer maintenance.Close()
	db, err := database.OpenExisting(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Run.RecoverAbandoned(ctx, runID)
}
