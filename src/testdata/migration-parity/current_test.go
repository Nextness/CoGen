// current_test.go binds family APIs to the identical baseline parity scenarios.
package main

import (
	"analysis/database"
	"analysis/database/artifact"
	"analysis/database/cache"
	"analysis/database/run"
	"analysis/pdfstore"
	"context"
)

// openBindings calls the current handwritten family implementations.
func openBindings(path, config string) (*bindings, error) {
	db, err := database.Open(path, config)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	key := cache.Key{Provider: "provider", Namespace: "namespace", RequestFingerprint: "request", ExtractorVersion: "v1"}
	return &bindings{
		raw: db.RawDBForTest(), close: db.Close,
		start: func(idle bool) (int64, int, error) {
			input := run.StartAttemptInput{ExecutionPlanID: 1, Step: "parity"}
			if idle {
				return db.Run.StartAttemptIfIdle(ctx, input)
			}
			return db.Run.StartAttempt(ctx, input)
		},
		finish:     func(id int64, status string) error { return db.Run.FinishRun(ctx, id, status, "parity") },
		step:       func(id int64, name string) (int64, error) { return db.Run.CreateStep(ctx, id, name) },
		stepStatus: func(id int64, status string) error { return db.Run.UpdateStepStatus(ctx, id, status) },

		stepLinks: func(step, previous, artifact int64) error {
			if err := db.Run.LinkStepReuse(ctx, run.StepReuseInput{StepID: step, ReusedFromRunID: previous}); err != nil {
				return err
			}
			if err := db.Run.LinkStepInputArtifact(ctx, run.StepArtifactInput{StepID: step, ArtifactID: artifact}); err != nil {
				return err
			}
			if err := db.Run.LinkStepOutputArtifact(ctx, run.StepArtifactInput{StepID: step, ArtifactID: artifact}); err != nil {
				return err
			}
			return db.Run.SetStepFingerprints(ctx, run.StepFingerprintInput{StepID: step, InputFingerprint: "input", OutputFingerprint: "output"})
		},
		readSteps: func(id int64) (any, error) { return db.Run.ListStepsByRun(ctx, id) },
		readRun:   func(id int64) (any, error) { return db.Run.GetByID(ctx, id) },
		readRuns:  func() (any, error) { return db.Run.ListByPlan(ctx, 1) },
		purge:     func(id int64) (any, error) { return db.Run.CheckPurgeEligibility(ctx, id) },
		reviewer: func(insert bool) (any, error) {
			if insert {
				if err := db.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: 1, Username: "Parity reviewer", Email: "parity@example.invalid"}); err != nil {
					return nil, err
				}
			}
			return db.Run.GetReviewer(ctx, 1)
		},
		readArtifact: func(id int64) (any, error) { return db.Artifact.GetByID(ctx, id) },
		recover:      func(id int64) error { return db.Run.RecoverAbandoned(ctx, id) },
		artifact: func(hash, media string, size int64, data []byte) (int64, error) {
			return db.Artifact.CreateWithBlob(ctx, artifact.CreateWithBlobInput{ContentHash: hash, ContentType: media, ByteSize: size, PipelineRunID: 1, Data: data})
		},
		link: func(id int64, role string) error {
			return db.Artifact.Link(ctx, artifact.LinkInput{PipelineRunID: 1, ArtifactID: id, Role: role})
		},
		appendCache: func(status int, payload *int64, expiry string) (int64, error) {
			return db.Cache.AppendEntry(ctx, &cache.Entry{Provider: "provider", Namespace: "namespace", RequestFingerprint: "request", ExtractorVersion: "v1", FetchedAt: "2026-01-01T00:00:00Z", ResponseStatus: status, PayloadArtifactID: payload, ExpiresAt: expiry})
		},
		useCache: func(run, entry int64, layer, outcome string) error {
			_, err := db.Cache.AppendUse(ctx, &cache.Use{PipelineRunID: run, CacheEntryID: entry, CacheLayer: layer, Outcome: outcome})
			return err
		},
		findCache: func(run int64, anyLayer bool) (any, error) {
			if anyLayer {
				return db.Cache.FindAnyEntry(ctx, run, key)
			}
			return db.Cache.FindEntry(ctx, run, "active_run", key)
		},
		latestCache: func(global bool) (any, error) {
			if global {
				return db.Cache.GlobalEntry(ctx, key)
			}
			return db.Cache.LatestEntry(ctx, key)
		},
		flush: func(store *pdfstore.Store) (int, error) { return store.FlushAuditOutbox(ctx, db.PDFBindings) },
	}, nil
}
