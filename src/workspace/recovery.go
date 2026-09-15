package workspace

import (
	"analysis/database"
	"context"
)

// RecoverAbandonedRun marks one running attempt failed only while holding exclusive pipeline ownership.
func RecoverAbandonedRun(ctx context.Context, dbPath string, runID int64) error {
	lock, err := lockPipelineDatabase(dbPath, false)
	if err != nil {
		return err
	}
	defer lock.Close()
	db, err := database.OpenExisting(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.PipelineRuns.RecoverAbandoned(ctx, runID)
}
