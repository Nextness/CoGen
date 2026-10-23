// store.go implements the immutable workspace pipeline
// orchestration: loading source payloads, normalising articles,
// enriching metadata, persisting revisions, recording audit events
// and metrics, and reconciling DOIs into the PDF inventory.
package workspace

import (
	"analysis/backend/workspace/enrichment"
	"analysis/backend/workspace/normalizer"
	"analysis/backend/workspace/pdfsync"
	"analysis/backend/workspace/providers"
	"analysis/backend/workspace/revisions"
	"analysis/backend/workspace/validator"

	"analysis/backend/article"
	"analysis/backend/database"
	dbauthor "analysis/backend/database/author"
	dbrun "analysis/backend/database/run"
	dbsource "analysis/backend/database/source"
	dbwork "analysis/backend/database/work"

	"analysis/backend/manifest"

	"analysis/backend/workspace/providercache"
	"analysis/backend/workspace/termindex"
	"analysis/internal/filehash"
	"context"
	"encoding/json"
	"fmt"

	"path/filepath"

	"slices"

	"strconv"

	"time"
)

// databaseRegistryPath returns the database registry relative to the supported repository-root working directory.
func databaseRegistryPath() string {
	return filepath.Join("config", "database.something")
}

// RunPipeline is the immutable workspace pipeline. It does
// not use the deprecated mutable corpus repositories.
func RunPipeline(dbPath string, originalConfig []byte, run *Run, fresh bool) error {
	return RunPipelineContext(context.Background(), dbPath, originalConfig, run, fresh)
}

// RunPipelineContext executes one attempt under an exclusive process lock and finalizes cancellation as failure.
func RunPipelineContext(ctx context.Context, dbPath string, originalConfig []byte, run *Run, fresh bool) (runErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	lock, err := lockPipelineDatabase(dbPath, true)
	if err != nil {
		return err
	}
	defer lock.Close()
	db, err := database.Open(dbPath, databaseRegistryPath())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	runID, err := StartWorkspaceAttemptContext(ctx, db, originalConfig, run, fresh)
	if err != nil {
		return err
	}
	if runID == 0 {
		if _, _, err := pdfsync.Sync(ctx, db, dbPath, databaseRegistryPath()); err != nil {
			return fmt.Errorf("synchronize normalized PDF inventory: %w", err)
		}
		termindex.BestEffort(ctx, db)
		log.Info("reused completed execution plan", "workspace", Selector(run.Manifest.SearchID, run.Manifest.SearchRevision))
		return nil
	}
	log.Info("workspace pipeline started", "workspace", Selector(run.Manifest.SearchID, run.Manifest.SearchRevision), "run_id", runID, "enrichment", run.Manifest.EnrichmentEnabled)
	completed := false
	defer func() {
		if !completed && runErr != nil {
			if db != nil {
				finishPipelineRun(ctx, db, runID, "failed", runErr.Error())
			}
		}
	}()

	bySource := make(map[string][]*article.Article, len(run.Manifest.Sources))
	parsed := make([]*article.Article, 0)
	inputRecords := 0
	for _, source := range run.Manifest.Sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		fields, err := json.Marshal(source.RequestedFields)
		if err != nil {
			return fmt.Errorf("marshal requested fields for %s: %w", source.Name, err)
		}
		runSourceID, err := db.Source.CreateSource(ctx, dbsource.CreateSourceInput{
			RunID:           runID,
			Name:            source.Name,
			Type:            source.FileType,
			ExpectedFile:    source.ExpectedFile,
			Query:           source.Query,
			RequestedFields: string(fields),
			ExpectedCount:   source.ExpectedResultCount,
			ExportDate:      source.Date,
		})
		if err != nil {
			return fmt.Errorf("create run source %s: %w", source.Name, err)
		}
		if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "expected_result_count", Source: source.Name, Value: source.ExpectedResultCount}); err != nil {
			return err
		}
		// Persist source filter data
		if len(source.Filters) > 0 {
			filterJSON, err := json.Marshal(source.Filters)
			if err != nil {
				return fmt.Errorf("marshal filter data for %s: %w", source.Name, err)
			}
			if err := db.Source.SetFilterData(ctx, dbsource.FilterCountInput{RunID: runID, SourceName: source.Name, FilterData: string(filterJSON)}); err != nil {
				return fmt.Errorf("persist filter data for %s: %w", source.Name, err)
			}
		}
		entries, err := loadWorkspaceEntries(source.FileType, source.ExpectedFile, source.Name)
		if err != nil {
			if observedCount, known := observedCountFromLoadError(err); known {
				comparison := resultCountComparison(source.ExpectedResultCount, observedCount)
				if updateErr := db.Source.SetObservedResultCount(ctx, dbsource.ObservedResultCountInput{RunSourceID: runSourceID, ObservedResultCount: observedCount, Comparison: comparison}); updateErr != nil {
					return fmt.Errorf("record empty source result count for %s: %w", source.Name, updateErr)
				}
				if metricErr := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "observed_result_count", Source: source.Name, Value: observedCount}); metricErr != nil {
					return metricErr
				}
			}
			return err
		}
		if source.FileType == "csv" && len(entries) > 0 {
			headers := cloneStringMap(entries[0])
			article.RenameFields(headers, source.PatchFields)
			for _, required := range []string{"doi", "title", "year"} {
				if _, ok := headers[required]; !ok {
					return fmt.Errorf("CSV source %q is missing required mapped header %q", source.Name, required)
				}
			}
		}
		comparison := resultCountComparison(source.ExpectedResultCount, len(entries))
		if err := db.Source.SetObservedResultCount(ctx, dbsource.ObservedResultCountInput{RunSourceID: runSourceID, ObservedResultCount: len(entries), Comparison: comparison}); err != nil {
			return fmt.Errorf("record source result count for %s: %w", source.Name, err)
		}
		if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "observed_result_count", Source: source.Name, Value: len(entries)}); err != nil {
			return err
		}
		if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "input_records", Source: source.Name, Value: len(entries)}); err != nil {
			return err
		}
		inputRecords += len(entries)
		accepted := make([]*article.Article, 0, len(entries))
		for index, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			raw, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("marshal source record %s[%d]: %w", source.Name, index, err)
			}
			recordID, err := db.Source.CreateRecord(ctx, dbsource.CreateRecordInput{RunSourceID: runSourceID, RecordIndex: index, RawPayload: string(raw), ContentHash: filehash.Bytes(raw)})
			if err != nil {
				return fmt.Errorf("create source record %s[%d]: %w", source.Name, index, err)
			}
			canonical := cloneStringMap(entry)
			article.RenameFields(canonical, source.PatchFields)
			article.KeepFields(canonical, source.KeepFields)
			a, err := article.NewFromMap(canonical, source.Name)
			if err != nil {
				if updateErr := db.Source.UpdateRecordStatus(ctx, dbsource.RecordStatusInput{RecordID: recordID, Status: "rejected", RejectReason: err.Error()}); updateErr != nil {
					return fmt.Errorf("record rejected source record %s[%d]: %w", source.Name, index, updateErr)
				}
				continue
			}
			if err := db.Source.UpdateRecordStatus(ctx, dbsource.RecordStatusInput{RecordID: recordID, Status: "parsed"}); err != nil {
				return fmt.Errorf("mark parsed source record %s[%d]: %w", source.Name, index, err)
			}
			accepted = append(accepted, a)
			parsed = append(parsed, a)
		}
		bySource[source.Name] = accepted
		if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "parsed_articles", Source: source.Name, Value: len(accepted)}); err != nil {
			return err
		}
	}
	log.Info("sources loaded", "run_id", runID, "records", inputRecords, "parsed", len(parsed), "sources", len(run.Manifest.Sources))

	if err := recordWorkspaceStage(ctx, db, run, runID, "parse", nil, parsed); err != nil {
		return err
	}
	if _, _, err := revisions.Persist(ctx, db, runID, parsed, dbwork.ProducerStageParse, dbwork.StageNameParse, dbwork.OutcomeParsed, nil); err != nil {
		return fmt.Errorf("persist parsed works: %w", err)
	}
	unique, duplicates := article.MergeBySource(bySource)
	if err := recordWorkspaceStage(ctx, db, run, runID, "deduplicate", parsed, unique); err != nil {
		return err
	}
	if _, _, err := revisions.Persist(ctx, db, runID, unique, dbwork.ProducerStageDeduplicate, dbwork.StageNameDeduplicate, dbwork.OutcomeDeduplicated, nil); err != nil {
		return fmt.Errorf("persist deduplicated works: %w", err)
	}
	if err := setRunMetrics(ctx, db, runID,
		"input_records", inputRecords,
		"parsed_articles", len(parsed),
		"deduplicated_articles", len(unique),
		"duplicate_articles", len(duplicates)); err != nil {
		return err
	}
	log.Info("deduplication completed", "run_id", runID, "unique", len(unique), "duplicates", len(duplicates))

	if run.Manifest.EnrichmentEnabled {
		metadataInput := snapshotArticles(unique)
		metadataUpdated, metadataChanges, err := enrichWorkspaceMetadata(ctx, db, runID, run, unique)
		if err != nil {
			return err
		}
		if err := recordWorkspaceStage(ctx, db, run, runID, "enrich_metadata", metadataInput, unique); err != nil {
			return err
		}
		_, metadataRevisionIDs, err := revisions.Persist(ctx, db, runID, unique, dbwork.ProducerStageEnrichMetadata, dbwork.StageNameEnrichMetadata, dbwork.OutcomeEnriched, nil)
		if err != nil {
			return fmt.Errorf("persist metadata-enriched works: %w", err)
		}
		if err := enrichment.RecordAudit(ctx, db, runID, metadataRevisionIDs, metadataChanges); err != nil {
			return err
		}

		updated := metadataUpdated
		allChanges := metadataChanges
		if _, hasORCID := run.Enrichment.Sources["orcid"]; hasORCID {
			identityInput := snapshotArticles(unique)
			identityUpdated, identityChanges, uncertainORCIDEvidence, err := enrichWorkspaceIdentity(ctx, db, runID, run, unique)
			if err != nil {
				if persistErr := providers.PersistIdentityEvidence(ctx, db, runID, metadataRevisionIDs, uncertainORCIDEvidence); persistErr != nil {
					return fmt.Errorf("identity enrichment failed: %v; persist partial ORCID evidence: %w", err, persistErr)
				}
				if stageErr := setWorkspaceStageOutcome(ctx, db, runID, articlesWithFailedIdentityEvidence(unique, uncertainORCIDEvidence), dbwork.StageNameEnrichIdentity, dbwork.OutcomeFailed, err.Error()); stageErr != nil {
					return fmt.Errorf("identity enrichment failed: %v; record failed identity stage: %w", err, stageErr)
				}
				return err
			}
			if err := recordWorkspaceStage(ctx, db, run, runID, "enrich_identity", identityInput, unique); err != nil {
				return err
			}
			_, identityRevisionIDs, err := revisions.Persist(ctx, db, runID, unique, dbwork.ProducerStageEnrichIdentity, dbwork.StageNameEnrichIdentity, dbwork.OutcomeEnriched, nil)
			if err != nil {
				return fmt.Errorf("persist identity-enriched works: %w", err)
			}
			if err := enrichment.RecordAudit(ctx, db, runID, identityRevisionIDs, identityChanges); err != nil {
				return err
			}
			if err := providers.PersistIdentityEvidence(ctx, db, runID, metadataRevisionIDs, uncertainORCIDEvidence); err != nil {
				return err
			}
			updated += identityUpdated
			allChanges = append(allChanges, identityChanges...)
		} else if err := setWorkspaceStageOutcome(ctx, db, runID, unique, dbwork.StageNameEnrichIdentity, dbwork.OutcomeSkipped, "ORCID provider is not configured"); err != nil {
			return err
		}
		if err := enrichment.RecordMetrics(ctx, db, runID, allChanges); err != nil {
			return err
		}
		if err := setRunMetrics(ctx, db, runID,
			"enrichment_skipped", 0,
			"enrichment_candidates", len(unique),
			"enriched_article_updates", updated); err != nil {
			return err
		}
		log.Info("enrichment completed", "run_id", runID, "candidates", len(unique), "updated", updated, "total_changes", len(allChanges))
	} else {
		for _, stage := range []string{dbwork.StageNameEnrichMetadata, dbwork.StageNameEnrichIdentity} {
			if err := setWorkspaceStageOutcome(ctx, db, runID, unique, stage, dbwork.OutcomeSkipped, "enrichment disabled by workspace configuration"); err != nil {
				return err
			}
		}
		if err := setRunMetrics(ctx, db, runID,
			"enrichment_skipped", 1,
			"enrichment_candidates", len(unique),
			"enriched_article_updates", 0); err != nil {
			return err
		}
		log.Info("enrichment skipped", "run_id", runID, "reason", "disabled by workspace configuration")
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	valid, discarded, reasons := validator.Validate(unique)
	if err := recordWorkspaceStage(ctx, db, run, runID, "validate", unique, unique); err != nil {
		return err
	}
	if _, _, err := revisions.Persist(ctx, db, runID, unique, dbwork.ProducerStageValidate, dbwork.StageNameValidate, dbwork.OutcomeValid, reasons); err != nil {
		return fmt.Errorf("persist validated works: %w", err)
	}
	if err := validator.RecordAudit(ctx, db, runID, unique, reasons); err != nil {
		return err
	}
	if err := setRunMetrics(ctx, db, runID, "valid_articles", len(valid), "discarded_articles", len(discarded)); err != nil {
		return err
	}
	log.Info("validation completed", "run_id", runID, "valid", len(valid), "discarded", len(discarded))

	normalizeInput := snapshotArticles(valid)
	normalizationResults := normalizer.Normalize(valid)
	if err := normalizer.RecordMetrics(ctx, db, runID, len(valid), normalizationResults); err != nil {
		return err
	}
	if err := recordWorkspaceStage(ctx, db, run, runID, "normalize", normalizeInput, valid); err != nil {
		return err
	}
	_, normalizeRevisionIDs, err := revisions.Persist(ctx, db, runID, valid, dbwork.ProducerStageNormalize, dbwork.StageNameNormalize, dbwork.OutcomeNormalized, nil)
	if err != nil {
		return fmt.Errorf("persist normalized works: %w", err)
	}
	termsBySource, termMatches := termindex.Compute(run.Manifest.Sources, valid, normalizeRevisionIDs)
	if err := termindex.Persist(ctx, db, runID, termsBySource, termMatches); err != nil {
		return err
	}
	registered, auditEventsFlushed, err := pdfsync.Sync(ctx, db, dbPath, databaseRegistryPath())
	if err != nil {
		return fmt.Errorf("synchronize normalized PDF inventory: %w", err)
	}
	log.Info("normalization completed", "run_id", runID, "articles", len(valid), "fields_processed", len(normalizationResults))
	log.Info("PDF inventory synchronized", "run_id", runID, "registered", registered, "audit_events_flushed", auditEventsFlushed)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := completePipelineRun(ctx, db, runID); err != nil {
		return err
	}
	completed = true
	termindex.BestEffort(ctx, db)
	log.Info("workspace pipeline completed", "workspace", Selector(run.Manifest.SearchID, run.Manifest.SearchRevision), "run_id", runID)
	return nil
}

// articlesWithFailedIdentityEvidence selects articles whose identity provider search failed.
func articlesWithFailedIdentityEvidence(articles []*article.Article, evidence []providers.IdentitySearchEvidence) []*article.Article {
	failedDOIs := make(map[string]struct{})
	for _, search := range evidence {
		if search.Status == dbauthor.IdentityStatusProviderFailed {
			failedDOIs[search.DOI] = struct{}{}
		}
	}
	result := make([]*article.Article, 0, len(failedDOIs))
	for _, a := range articles {
		if _, failed := failedDOIs[a.DOI]; failed {
			result = append(result, a)
		}
	}
	return result
}

// resultCountComparison classifies an observed source count as below, above, or matching its expectation.
func resultCountComparison(expected, observed int) string {
	switch {
	case observed < expected:
		return "below"
	case observed > expected:
		return "above"
	default:
		return "match"
	}
}

// loadWorkspaceEntries loads workspace entries from the supplied source.
func loadWorkspaceEntries(fileType, path, source string) ([]map[string]string, error) {
	switch fileType {
	case "csv":
		return loadCSVEntries(path, source)
	case "bib":
		return loadBibEntries(path, source)
	default:
		return nil, fmt.Errorf("unsupported source file type %q for %s", fileType, source)
	}
}

// cloneStringMap clones string map into an independent value.
func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// setWorkspaceStageOutcome sets workspace stage outcome using the supplied values.
func setWorkspaceStageOutcome(ctx context.Context, db *database.Database, runID int64, articles []*article.Article, stage, outcome, reason string) error {
	for _, a := range articles {
		work, err := db.Work.GetWorkByDOI(ctx, a.DOI)
		if err != nil {
			return err
		}
		if work == nil {
			return fmt.Errorf("work missing for DOI %q", a.DOI)
		}
		if err := db.Work.SetStageOutcome(ctx, dbwork.StageOutcomeInput{
			RunID: runID, WorkID: work.ID, StageName: stage, Outcome: outcome, Reason: reason,
		}); err != nil {
			return err
		}
	}
	return nil
}

// setRunMetrics sets run metrics using the supplied values.
func setRunMetrics(ctx context.Context, db *database.Database, runID int64, metrics ...any) error {
	for i := 0; i < len(metrics); i += 2 {
		if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: metrics[i].(string), Value: metrics[i+1].(int)}); err != nil {
			return err
		}
	}
	return nil
}

// snapshotArticles preserves the complete stage input before in-place article and relationship mutations.
func snapshotArticles(articles []*article.Article) []*article.Article {
	result := make([]*article.Article, len(articles))
	for i, a := range articles {
		if a == nil {
			continue
		}
		copy := *a
		copy.Authors = slices.Clone(a.Authors)
		copy.CitedReferences = slices.Clone(a.CitedReferences)
		copy.Keywords = slices.Clone(a.Keywords)
		copy.KeywordsAdditional = slices.Clone(a.KeywordsAdditional)
		result[i] = &copy
	}
	return result
}

// recordWorkspaceStage records workspace stage.
func recordWorkspaceStage(ctx context.Context, db *database.Database, run *Run, runID int64, name string, input any, output any) error {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal %s input: %w", name, err)
	}
	outputBytes, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshal %s output: %w", name, err)
	}
	inputArtifactID, err := db.Artifact.StorePayload(ctx, runID, inputBytes, "application/json")
	if err != nil {
		return err
	}
	outputArtifactID, err := db.Artifact.StorePayload(ctx, runID, outputBytes, "application/json")
	if err != nil {
		return err
	}
	stepID, err := db.Run.CreateStep(ctx, runID, name)
	if err != nil {
		return err
	}
	if err := db.Run.LinkStepInputArtifact(ctx, dbrun.StepArtifactInput{StepID: stepID, ArtifactID: inputArtifactID}); err != nil {
		return err
	}
	if err := db.Run.LinkStepOutputArtifact(ctx, dbrun.StepArtifactInput{StepID: stepID, ArtifactID: outputArtifactID}); err != nil {
		return err
	}
	if err := db.Run.SetStepFingerprints(ctx, dbrun.StepFingerprintInput{StepID: stepID, InputFingerprint: filehash.Bytes(inputBytes), OutputFingerprint: filehash.Bytes(outputBytes)}); err != nil {
		return err
	}
	return db.Run.UpdateStepStatus(ctx, stepID, string(manifest.StageCompleted))
}

// enrichWorkspaceMetadata applies article-level providers. Its output is
// persisted before identity enrichment so an ORCID provider failure cannot
// erase the metadata and authorship evidence already gathered for the run.
func enrichWorkspaceMetadata(ctx context.Context, db *database.Database, runID int64, run *Run, articles []*article.Article) (int, []enrichment.Change, error) {
	if !run.Manifest.EnrichmentEnabled {
		return 0, nil, nil
	}
	if run.Enrichment == nil {
		return 0, nil, fmt.Errorf("enrichment is enabled but no provider configuration is available")
	}
	byDOI := make(map[string]*article.Article, len(articles))
	for _, a := range articles {
		byDOI[a.DOI] = a
	}
	cache := providercache.New(db, runID, run.Manifest.CachePolicy)
	updated := 0
	var allChanges []enrichment.Change
	if source, ok := run.Enrichment.Sources["crossref"]; ok {
		result, err := providers.Crossref(ctx, cache, source, articles)
		if err != nil {
			return 0, nil, err
		}
		n, changes := enrichment.ApplyArticles(byDOI, source, result)
		updated += n
		allChanges = append(allChanges, changes...)
	}
	if source, ok := run.Enrichment.Sources["openalex"]; ok {
		result, err := providers.OpenAlex(ctx, cache, source, articles)
		if err != nil {
			return 0, nil, err
		}
		n, changes := enrichment.ApplyArticles(byDOI, source, result)
		updated += n
		allChanges = append(allChanges, changes...)
	}
	return updated, allChanges, nil
}

// enrichWorkspaceIdentity applies exact observed-ORCID profiles and records
// name-search evidence. The caller persists this evidence against the prior
// enrich_metadata snapshot, including when this function returns an error.
func enrichWorkspaceIdentity(ctx context.Context, db *database.Database, runID int64, run *Run, articles []*article.Article) (int, []enrichment.Change, []providers.IdentitySearchEvidence, error) {
	if !run.Manifest.EnrichmentEnabled || run.Enrichment == nil {
		return 0, nil, nil, nil
	}
	source, ok := run.Enrichment.Sources["orcid"]
	if !ok {
		return 0, nil, nil, nil
	}
	cache := providercache.New(db, runID, run.Manifest.CachePolicy)
	openAlexSource, hasOpenAlex := run.Enrichment.Sources["openalex"]
	hasOpenAlex = hasOpenAlex && openAlexSource.AllowsField("authors")
	return providers.ORCID(ctx, cache, source, openAlexSource, hasOpenAlex, articles)
}

// completePipelineRun completes pipeline run and records its terminal state.
func completePipelineRun(ctx context.Context, db *database.Database, runID int64) error {
	if err := db.Run.FinishRun(ctx, runID, "completed", ""); err != nil {
		return err
	}
	_, err := db.Audit.Insert(ctx, &manifest.AuditEvent{OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "pipeline", PipelineRunID: runID, EntityType: "pipeline_run", EntityID: strconv.FormatInt(runID, 10), Action: manifest.AuditRunCompleted, CorrelationID: "run-complete-" + strconv.FormatInt(runID, 10)})
	return err
}
