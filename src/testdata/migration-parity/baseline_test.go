// baseline_test.go binds the frozen pre-sqlc API to the shared parity scenarios.
package main

import (
	"analysis/database"
	"analysis/pdfstore"
	"context"
)

// openBindings calls the original implementations from the baseline checkout.
func openBindings(path, config string) (*bindings, error) {
	db, err := database.Open(path, config)
	if err != nil {
		return nil, err
	}
	return &bindings{
		raw: db.DB, close: db.Close,
		start: func(idle bool) (int64, int, error) {
			if idle {
				return db.PipelineRuns.StartAttemptIfIdle(1, "parity", "")
			}
			return db.PipelineRuns.StartAttempt(1, "parity", "")
		},
		finish: func(id int64, status string) error { return db.PipelineRuns.FinishRun(id, status, "parity") },
		step:   db.RunSteps.Create, stepStatus: db.RunSteps.UpdateStatus,

		stepLinks: func(step, previous, artifact int64) error {
			if err := db.RunSteps.LinkReuse(step, previous); err != nil {
				return err
			}
			if err := db.RunSteps.LinkInputArtifact(step, artifact); err != nil {
				return err
			}
			if err := db.RunSteps.LinkOutputArtifact(step, artifact); err != nil {
				return err
			}
			return db.RunSteps.SetFingerprints(step, "input", "output")
		},
		readSteps: func(id int64) (any, error) { return db.RunSteps.ListByRun(id) },
		readRun:   func(id int64) (any, error) { return db.PipelineRuns.GetByID(id) },
		readRuns:  func() (any, error) { return db.PipelineRuns.ListByPlan(1) },
		purge:     func(id int64) (any, error) { return db.PipelineRuns.CheckPurgeEligibility(id) },
		reviewer: func(insert bool) (any, error) {
			if insert {
				if err := db.PipelineRunReviewers.Insert(1, "Parity reviewer", "parity@example.invalid"); err != nil {
					return nil, err
				}
			}
			return db.PipelineRunReviewers.Get(1)
		},
		readArtifact: func(id int64) (any, error) { return db.Artifacts.GetByID(id) },
		recover:      func(id int64) error { return db.PipelineRuns.RecoverAbandoned(context.Background(), id) },
		artifact: func(hash, media string, size int64, data []byte) (int64, error) {
			return db.Artifacts.CreateWithBlob(hash, media, size, 1, data)
		},
		link: func(id int64, role string) error { return db.RunArtifacts.Link(1, id, role) },
		appendCache: func(status int, payload *int64, expiry string) (int64, error) {
			return db.CacheEntries.Upsert(&database.CacheEntry{Provider: "provider", Namespace: "namespace", RequestFingerprint: "request", ExtractorVersion: "v1", FetchedAt: "2026-01-01T00:00:00Z", ResponseStatus: status, PayloadArtifactID: payload, ExpiresAt: expiry})
		},
		useCache: func(run, entry int64, layer, outcome string) error {
			_, err := db.RunCacheUses.Create(&database.RunCacheUse{PipelineRunID: run, CacheEntryID: entry, CacheLayer: layer, Outcome: outcome})
			return err
		},
		findCache: func(run int64, anyLayer bool) (any, error) {
			if anyLayer {
				return db.RunCacheUses.FindAnyEntry(run, "provider", "namespace", "request", "v1")
			}
			return db.RunCacheUses.FindEntry(run, "active_run", "provider", "namespace", "request", "v1")
		},
		latestCache: func(global bool) (any, error) {
			if global {
				return db.CacheEntries.GetGlobal("provider", "namespace", "request", "v1")
			}
			return db.CacheEntries.Get("provider", "namespace", "request", "v1")
		},
		flush: func(store *pdfstore.Store) (int, error) { return store.FlushAuditOutbox(context.Background(), db.DB) },
	}, nil
}
