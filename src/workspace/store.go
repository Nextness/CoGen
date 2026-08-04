// store.go implements the immutable workspace pipeline
// orchestration: loading source payloads, normalising articles,
// enriching metadata, persisting revisions, recording audit events
// and metrics, and reconciling DOIs into the PDF inventory.
package workspace

import (
	"analysis/article"
	"analysis/database"
	"analysis/enrich"
	"analysis/manifest"
	"analysis/normalization"
	"analysis/pdfstore"
	"analysis/validation"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// databaseRegistryPath returns the path to the database registry config
// at <repo-root>/config/database.something. It walks up from CWD looking
// for go.mod (src/go.mod), then goes one level up to find the config dir.
func databaseRegistryPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "config/database.something"
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// go.mod is at <repo>/src/go.mod; config is at <repo>/config/
			return filepath.Join(filepath.Dir(dir), "config", "database.something")
		}
		if dir == filepath.Dir(dir) {
			// Fallback: resolve relative to CWD (works from repo root).
			return "config/database.something"
		}
	}
}

// RunPipeline is the immutable workspace pipeline. It does
// not use the deprecated mutable corpus repositories.
func RunPipeline(dbPath string, originalConfig []byte, run *Run, fresh bool) (runErr error) {
	db, err := database.Open(dbPath, databaseRegistryPath())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	runID, err := StartWorkspaceAttempt(db, originalConfig, run, fresh)
	if err != nil {
		return err
	}
	if runID == 0 {
		if _, _, err := syncNormalizedPDFInventory(context.Background(), db, dbPath, databaseRegistryPath()); err != nil {
			return fmt.Errorf("synchronize normalized PDF inventory: %w", err)
		}
		log.Info("reused completed execution plan", "workspace", Selector(run.Manifest.SearchID, run.Manifest.SearchRevision))
		return nil
	}
	log.Info("workspace pipeline started", "workspace", Selector(run.Manifest.SearchID, run.Manifest.SearchRevision), "run_id", runID, "enrichment", run.Manifest.EnrichmentEnabled)
	completed := false
	defer func() {
		if !completed && runErr != nil {
			if db != nil {
				finishPipelineRun(db, runID, "failed", runErr.Error())
			}
		}
	}()

	bySource := make(map[string][]*article.Article, len(run.Manifest.Sources))
	parsed := make([]*article.Article, 0)
	inputRecords := 0
	for _, source := range run.Manifest.Sources {
		fields, err := json.Marshal(source.RequestedFields)
		if err != nil {
			return fmt.Errorf("marshal requested fields for %s: %w", source.Name, err)
		}
		runSourceID, err := db.RunSources.Create(runID, source.Name, source.FileType, source.ExpectedFile, source.Query, string(fields), source.ExpectedResultCount, source.Date)
		if err != nil {
			return fmt.Errorf("create run source %s: %w", source.Name, err)
		}
		if err := db.Metrics.Set(runID, "expected_result_count", source.Name, source.ExpectedResultCount); err != nil {
			return err
		}
		// Persist source filter data
		if len(source.Filters) > 0 {
			filterJSON, err := json.Marshal(source.Filters)
			if err != nil {
				return fmt.Errorf("marshal filter data for %s: %w", source.Name, err)
			}
			if err := db.SourceFilterCounts.SetFilterData(runID, source.Name, string(filterJSON)); err != nil {
				return fmt.Errorf("persist filter data for %s: %w", source.Name, err)
			}
		}
		entries, err := loadWorkspaceEntries(source.FileType, source.ExpectedFile, source.Name)
		if err != nil {
			if observedCount, known := observedCountFromLoadError(err); known {
				comparison := resultCountComparison(source.ExpectedResultCount, observedCount)
				if updateErr := db.RunSources.SetObservedResultCount(runSourceID, observedCount, comparison); updateErr != nil {
					return fmt.Errorf("record empty source result count for %s: %w", source.Name, updateErr)
				}
				if metricErr := db.Metrics.Set(runID, "observed_result_count", source.Name, observedCount); metricErr != nil {
					return metricErr
				}
			}
			return err
		}
		comparison := resultCountComparison(source.ExpectedResultCount, len(entries))
		if err := db.RunSources.SetObservedResultCount(runSourceID, len(entries), comparison); err != nil {
			return fmt.Errorf("record source result count for %s: %w", source.Name, err)
		}
		if err := db.Metrics.Set(runID, "observed_result_count", source.Name, len(entries)); err != nil {
			return err
		}
		if err := db.Metrics.Set(runID, "input_records", source.Name, len(entries)); err != nil {
			return err
		}
		inputRecords += len(entries)
		accepted := make([]*article.Article, 0, len(entries))
		for index, entry := range entries {
			raw, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("marshal source record %s[%d]: %w", source.Name, index, err)
			}
			recordID, err := db.SourceRecords.Create(runSourceID, index, string(raw), contentHash(raw))
			if err != nil {
				return fmt.Errorf("create source record %s[%d]: %w", source.Name, index, err)
			}
			canonical := cloneStringMap(entry)
			article.RenameFields(canonical, source.PatchFields)
			article.KeepFields(canonical, source.KeepFields)
			a, err := article.NewFromMap(canonical, source.Name)
			if err != nil {
				if updateErr := db.SourceRecords.UpdateParseStatus(recordID, "rejected", err.Error()); updateErr != nil {
					return fmt.Errorf("record rejected source record %s[%d]: %w", source.Name, index, updateErr)
				}
				continue
			}
			if err := db.SourceRecords.UpdateParseStatus(recordID, "parsed", ""); err != nil {
				return fmt.Errorf("mark parsed source record %s[%d]: %w", source.Name, index, err)
			}
			accepted = append(accepted, a)
			parsed = append(parsed, a)
		}
		bySource[source.Name] = accepted
		if err := db.Metrics.Set(runID, "parsed_articles", source.Name, len(accepted)); err != nil {
			return err
		}
	}
	log.Info("sources loaded", "run_id", runID, "records", inputRecords, "parsed", len(parsed), "sources", len(run.Manifest.Sources))

	if err := recordWorkspaceStage(db, run, runID, "parse", nil, parsed); err != nil {
		return err
	}
	if _, _, err := persistWorkspaceStage(db, runID, parsed, database.ProducerStageParse, database.StageNameParse, database.OutcomeParsed, nil); err != nil {
		return fmt.Errorf("persist parsed works: %w", err)
	}
	unique, duplicates := article.MergeBySource(bySource)
	if err := recordWorkspaceStage(db, run, runID, "deduplicate", parsed, unique); err != nil {
		return err
	}
	if _, _, err := persistWorkspaceStage(db, runID, unique, database.ProducerStageDeduplicate, database.StageNameDeduplicate, database.OutcomeDeduplicated, nil); err != nil {
		return fmt.Errorf("persist deduplicated works: %w", err)
	}
	if err := setRunMetrics(db, runID,
		"input_records", inputRecords,
		"parsed_articles", len(parsed),
		"deduplicated_articles", len(unique),
		"duplicate_articles", len(duplicates)); err != nil {
		return err
	}
	log.Info("deduplication completed", "run_id", runID, "unique", len(unique), "duplicates", len(duplicates))

	if run.Manifest.EnrichmentEnabled {
		metadataUpdated, metadataChanges, err := enrichWorkspaceMetadata(db, runID, run, unique)
		if err != nil {
			return err
		}
		if err := recordWorkspaceStage(db, run, runID, "enrich_metadata", unique, unique); err != nil {
			return err
		}
		_, metadataRevisionIDs, err := persistWorkspaceStage(db, runID, unique, database.ProducerStageEnrichMetadata, database.StageNameEnrichMetadata, database.OutcomeEnriched, nil)
		if err != nil {
			return fmt.Errorf("persist metadata-enriched works: %w", err)
		}
		if err := emitFieldEnrichedAuditEvents(db, runID, metadataRevisionIDs, metadataChanges); err != nil {
			return err
		}

		updated := metadataUpdated
		allChanges := metadataChanges
		if _, hasORCID := run.Enrichment.Sources["orcid"]; hasORCID {
			identityUpdated, identityChanges, uncertainORCIDEvidence, err := enrichWorkspaceIdentity(db, runID, run, unique)
			if err != nil {
				if persistErr := persistUncertainORCIDEvidence(db, runID, metadataRevisionIDs, uncertainORCIDEvidence); persistErr != nil {
					return fmt.Errorf("identity enrichment failed: %v; persist partial ORCID evidence: %w", err, persistErr)
				}
				if stageErr := setWorkspaceStageOutcome(db, runID, articlesWithFailedIdentityEvidence(unique, uncertainORCIDEvidence), database.StageNameEnrichIdentity, database.OutcomeFailed, err.Error()); stageErr != nil {
					return fmt.Errorf("identity enrichment failed: %v; record failed identity stage: %w", err, stageErr)
				}
				return err
			}
			if err := recordWorkspaceStage(db, run, runID, "enrich_identity", unique, unique); err != nil {
				return err
			}
			_, identityRevisionIDs, err := persistWorkspaceStage(db, runID, unique, database.ProducerStageEnrichIdentity, database.StageNameEnrichIdentity, database.OutcomeEnriched, nil)
			if err != nil {
				return fmt.Errorf("persist identity-enriched works: %w", err)
			}
			if err := emitFieldEnrichedAuditEvents(db, runID, identityRevisionIDs, identityChanges); err != nil {
				return err
			}
			if err := persistUncertainORCIDEvidence(db, runID, metadataRevisionIDs, uncertainORCIDEvidence); err != nil {
				return err
			}
			updated += identityUpdated
			allChanges = append(allChanges, identityChanges...)
		} else if err := setWorkspaceStageOutcome(db, runID, unique, database.StageNameEnrichIdentity, database.OutcomeSkipped, "ORCID provider is not configured"); err != nil {
			return err
		}
		if err := recordFieldEnrichmentMetrics(db, runID, allChanges); err != nil {
			return err
		}
		if err := setRunMetrics(db, runID,
			"enrichment_skipped", 0,
			"enrichment_candidates", len(unique),
			"enriched_article_updates", updated); err != nil {
			return err
		}
		log.Info("enrichment completed", "run_id", runID, "candidates", len(unique), "updated", updated, "total_changes", len(allChanges))
	} else {
		for _, stage := range []string{database.StageNameEnrichMetadata, database.StageNameEnrichIdentity} {
			if err := setWorkspaceStageOutcome(db, runID, unique, stage, database.OutcomeSkipped, "enrichment disabled by workspace configuration"); err != nil {
				return err
			}
		}
		if err := setRunMetrics(db, runID,
			"enrichment_skipped", 1,
			"enrichment_candidates", len(unique),
			"enriched_article_updates", 0); err != nil {
			return err
		}
		log.Info("enrichment skipped", "run_id", runID, "reason", "disabled by workspace configuration")
	}

	valid, discarded, reasons := validateWorkspaceArticles(unique)
	if err := recordWorkspaceStage(db, run, runID, "validate", unique, unique); err != nil {
		return err
	}
	if _, _, err := persistWorkspaceStage(db, runID, unique, database.ProducerStageValidate, database.StageNameValidate, database.OutcomeValid, reasons); err != nil {
		return fmt.Errorf("persist validated works: %w", err)
	}
	if err := recordValidationAudit(db, runID, unique, reasons); err != nil {
		return err
	}
	if err := setRunMetrics(db, runID, "valid_articles", len(valid), "discarded_articles", len(discarded)); err != nil {
		return err
	}
	log.Info("validation completed", "run_id", runID, "valid", len(valid), "discarded", len(discarded))

	normalizationResults := normalizeWorkspaceArticles(valid)
	if err := recordNormalizationMetrics(db, runID, len(valid), normalizationResults); err != nil {
		return err
	}
	if err := recordWorkspaceStage(db, run, runID, "normalize", valid, valid); err != nil {
		return err
	}
	if _, _, err := persistWorkspaceStage(db, runID, valid, database.ProducerStageNormalize, database.StageNameNormalize, database.OutcomeNormalized, nil); err != nil {
		return fmt.Errorf("persist normalized works: %w", err)
	}
	registered, auditEventsFlushed, err := syncNormalizedPDFInventory(context.Background(), db, dbPath, databaseRegistryPath())
	if err != nil {
		return fmt.Errorf("synchronize normalized PDF inventory: %w", err)
	}
	log.Info("normalization completed", "run_id", runID, "articles", len(valid), "fields_processed", len(normalizationResults))
	log.Info("PDF inventory synchronized", "run_id", runID, "registered", registered, "audit_events_flushed", auditEventsFlushed)
	if err := completePipelineRun(db, runID); err != nil {
		return err
	}
	completed = true
	log.Info("workspace pipeline completed", "workspace", Selector(run.Manifest.SearchID, run.Manifest.SearchRevision), "run_id", runID)
	return nil
}

// normalizedInventoryWork identifies a persisted normalized work for companion PDF registration.
type normalizedInventoryWork struct {
	DOI           string
	WorkID        int64
	PipelineRunID int64
}

// syncNormalizedPDFInventory reconciles the companion inventory from the
// authoritative normalized revisions. It is safe to call after a new run or
// when a completed execution plan is reused.
func syncNormalizedPDFInventory(ctx context.Context, db *database.Database, dbPath, registryPath string) (int, int, error) {
	rows, err := db.DB.QueryContext(ctx, `SELECT w.doi, w.id, MIN(wr.pipeline_run_id)
		FROM works w
		JOIN work_revisions wr ON wr.work_id=w.id
		WHERE wr.producer_stage='normalize' AND w.doi IS NOT NULL AND trim(w.doi)<>''
		GROUP BY w.id, w.doi ORDER BY w.id`)
	if err != nil {
		return 0, 0, fmt.Errorf("read normalized works: %w", err)
	}
	var works []normalizedInventoryWork
	for rows.Next() {
		var work normalizedInventoryWork
		if err := rows.Scan(&work.DOI, &work.WorkID, &work.PipelineRunID); err != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("scan normalized work: %w", err)
		}
		works = append(works, work)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, fmt.Errorf("read normalized works: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, 0, fmt.Errorf("close normalized work rows: %w", err)
	}
	if len(works) == 0 {
		return 0, 0, nil
	}

	storePath, err := pdfstore.BoundStorePath(ctx, db.DB, dbPath)
	if err != nil {
		return 0, 0, err
	}
	store, err := pdfstore.Open(storePath, registryPath)
	if err != nil {
		return 0, 0, err
	}
	defer store.Close()

	flushed, err := store.FlushAuditOutbox(ctx, db.DB)
	if err != nil {
		return 0, flushed, err
	}
	registered := 0
	for _, work := range works {
		added, err := store.Register(ctx, work.DOI, work.WorkID, work.PipelineRunID)
		if err != nil {
			return registered, flushed, err
		}
		if added {
			registered++
		}
	}
	newlyFlushed, err := store.FlushAuditOutbox(ctx, db.DB)
	flushed += newlyFlushed
	if err != nil {
		return registered, flushed, err
	}
	return registered, flushed, nil
}

// articlesWithFailedIdentityEvidence selects articles whose identity provider search failed.
func articlesWithFailedIdentityEvidence(articles []*article.Article, evidence []uncertainORCIDSearchEvidence) []*article.Article {
	failedDOIs := make(map[string]struct{})
	for _, search := range evidence {
		if search.Status == database.AuthorIdentityStatusProviderFailed {
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

// persistWorkspaceStage persists workspace stage through the owning repository.
func persistWorkspaceStage(db *database.Database, runID int64, articles []*article.Article, producerStage, stage, outcome string, reasons map[string][]string) (map[string]int64, map[string]int64, error) {
	workIDs := make(map[string]int64, len(articles))
	revisionIDs := make(map[string]int64, len(articles))
	for _, a := range articles {
		reason := ""
		stageOutcome := outcome
		if reasons != nil && len(reasons[a.DOI]) > 0 {
			reasonBytes, _ := json.Marshal(reasons[a.DOI])
			reason = string(reasonBytes)
			if stage == database.StageNameValidate {
				stageOutcome = database.OutcomeDiscarded
			}
		}
		workID, revisionID, err := persistWorkSnapshot(db, runID, a, producerStage, workspaceExtension(a, reasons[a.DOI]))
		if err != nil {
			return nil, nil, err
		}
		workIDs[a.DOI] = workID
		revisionIDs[a.DOI] = revisionID
		if err := db.RunWorkStages.SetOutcome(runID, workID, stage, stageOutcome, reason); err != nil {
			return nil, nil, err
		}
	}
	return workIDs, revisionIDs, nil
}

// setWorkspaceStageOutcome sets workspace stage outcome using the supplied values.
func setWorkspaceStageOutcome(db *database.Database, runID int64, articles []*article.Article, stage, outcome, reason string) error {
	for _, a := range articles {
		work, err := db.Works.GetByDOI(a.DOI)
		if err != nil {
			return err
		}
		if work == nil {
			return fmt.Errorf("work missing for DOI %q", a.DOI)
		}
		if err := db.RunWorkStages.SetOutcome(runID, work.ID, stage, outcome, reason); err != nil {
			return err
		}
	}
	return nil
}

// setRunMetrics sets run metrics using the supplied values.
func setRunMetrics(db *database.Database, runID int64, metrics ...any) error {
	for i := 0; i < len(metrics); i += 2 {
		if err := db.Metrics.Set(runID, metrics[i].(string), "", metrics[i+1].(int)); err != nil {
			return err
		}
	}
	return nil
}

// recordWorkspaceStage records workspace stage.
func recordWorkspaceStage(db *database.Database, run *Run, runID int64, name string, input any, output any) error {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal %s input: %w", name, err)
	}
	outputBytes, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshal %s output: %w", name, err)
	}
	inputArtifactID, err := persistArtifact(db, runID, inputBytes, "application/json")
	if err != nil {
		return err
	}
	outputArtifactID, err := persistArtifact(db, runID, outputBytes, "application/json")
	if err != nil {
		return err
	}
	stepID, err := db.RunSteps.Create(runID, name)
	if err != nil {
		return err
	}
	if err := db.RunSteps.LinkInputArtifact(stepID, inputArtifactID); err != nil {
		return err
	}
	if err := db.RunSteps.LinkOutputArtifact(stepID, outputArtifactID); err != nil {
		return err
	}
	if err := db.RunSteps.SetFingerprints(stepID, contentHash(inputBytes), contentHash(outputBytes)); err != nil {
		return err
	}
	return db.RunSteps.UpdateStatus(stepID, string(manifest.StageCompleted))
}

// persistWorkSnapshot persists work snapshot through the owning repository.
func persistWorkSnapshot(db *database.Database, runID int64, a *article.Article, producerStage, extensionData string) (int64, int64, error) {
	if a == nil {
		return 0, 0, fmt.Errorf("persist work snapshot: article is nil")
	}
	workID, err := db.Works.CreateByDOI(a.DOI)
	if err != nil {
		return 0, 0, err
	}
	keywords, err := json.Marshal(a.Keywords)
	if err != nil {
		return 0, 0, err
	}
	keywordsPlus, err := json.Marshal(a.KeywordsAdditional)
	if err != nil {
		return 0, 0, err
	}
	revisionID, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: producerStage,
		Title: a.Title, Abstract: a.Abstract, Year: a.Year, Journal: a.Journal,
		Publisher: a.Publisher, Source: a.Source, Keywords: string(keywords),
		KeywordsPlus: string(keywordsPlus), CitationCount: a.CitationCount,
		ReferenceCount: len(a.CitedReferences), ExtensionData: extensionData,
	})
	if err != nil {
		return 0, 0, err
	}
	for order, author := range a.Authors {
		occurrenceID, err := db.AuthorOccs.Create(&database.AuthorOccurrence{CitationName: author.CitationName, FirstName: author.FirstName, LastName: author.LastName, ORCID: author.Orcid})
		if err != nil {
			return 0, 0, err
		}
		if _, err := db.Authorships.Create(&database.Authorship{WorkRevisionID: revisionID, AuthorOccurrenceID: occurrenceID, AuthorOrder: order + 1, Affiliation: author.Affiliation}); err != nil {
			return 0, 0, err
		}
	}
	for order, reference := range a.CitedReferences {
		if _, err := db.ReferenceMentions.Create(&database.ReferenceMention{WorkRevisionID: revisionID, MentionOrder: order + 1, RawReference: reference.Raw, DOI: reference.DOI, Title: reference.Title, Author: reference.Author, Year: reference.Year, Source: reference.Source}); err != nil {
			return 0, 0, err
		}
	}
	return workID, revisionID, nil
}

// workspaceRevisionExtension stores normalized values and validation evidence outside canonical revision columns.
type workspaceRevisionExtension struct {
	NormalizedJournal string            `json:"normalized_journal,omitempty"`
	ValidationReasons []string          `json:"validation_reasons,omitempty"`
	NormalizedAuthors map[string]string `json:"normalized_authors,omitempty"`
}

// workspaceExtension serializes normalized journal, author names, and validation reasons for a revision.
func workspaceExtension(a *article.Article, reasons []string) string {
	names := make(map[string]string)
	for _, author := range a.Authors {
		if author.NormalizedName != "" {
			names[author.CitationName] = author.NormalizedName
		}
	}
	data, _ := json.Marshal(workspaceRevisionExtension{NormalizedJournal: normalization.NormalizeJournal(a.Journal), ValidationReasons: reasons, NormalizedAuthors: names})
	return string(data)
}

// validateWorkspaceArticles partitions articles by validation outcome and records discard reasons by DOI.
func validateWorkspaceArticles(articles []*article.Article) (valid, discarded []*article.Article, reasons map[string][]string) {
	reasons = make(map[string][]string)
	for _, a := range articles {
		problems := validation.ValidateFields(validation.Fields{DOI: a.DOI, Title: a.Title, Year: a.Year, Publisher: a.Publisher, ReferenceCount: len(a.CitedReferences)}, len(a.Authors))
		if len(problems) == 0 {
			valid = append(valid, a)
			continue
		}
		discarded = append(discarded, a)
		reasons[a.DOI] = problems
	}
	return valid, discarded, reasons
}

// normalizationFieldResult records one field's normalization outcome for an article DOI.
type normalizationFieldResult struct {
	DOI     string
	Field   string
	Outcome string
}

const (
	normalizationOutcomeChanged          = "changed"
	normalizationOutcomeAlreadyCanonical = "already_canonical"
	normalizationOutcomeUnavailable      = "unavailable"
)

var normalizationFields = []string{"publisher", "journal", "author_name", "affiliation"}

var normalizationOutcomes = []string{
	normalizationOutcomeChanged,
	normalizationOutcomeAlreadyCanonical,
	normalizationOutcomeUnavailable,
}

// normalizeWorkspaceArticles applies each normalizer and records one outcome
// for every checked field. Journal normalization is retained in revision
// extension data, so its result is measured here but not assigned to Journal.
func normalizeWorkspaceArticles(articles []*article.Article) []normalizationFieldResult {
	results := make([]normalizationFieldResult, 0, len(articles)*2)
	for _, a := range articles {
		publisher := normalization.NormalizePublisher(a.Publisher)
		results = append(results, normalizationResult(a.DOI, "publisher", a.Publisher, publisher))
		a.Publisher = publisher

		journal := normalization.NormalizeJournal(a.Journal)
		results = append(results, normalizationResult(a.DOI, "journal", a.Journal, journal))

		for index := range a.Authors {
			author := &a.Authors[index]
			name := normalization.NormalizeAuthorName(author.CitationName)
			results = append(results, normalizationResult(a.DOI, "author_name", author.CitationName, name))
			author.NormalizedName = name

			affiliation := normalization.NormalizeAffiliation(author.Affiliation)
			results = append(results, normalizationResult(a.DOI, "affiliation", author.Affiliation, affiliation))
			author.Affiliation = affiliation
		}
	}
	return results
}

// normalizationResult classifies a field as changed, already canonical, or unavailable.
func normalizationResult(doi, field, input, output string) normalizationFieldResult {
	outcome := normalizationOutcomeChanged
	if strings.TrimSpace(input) == "" {
		outcome = normalizationOutcomeUnavailable
	} else if input == output {
		outcome = normalizationOutcomeAlreadyCanonical
	}
	return normalizationFieldResult{DOI: doi, Field: field, Outcome: outcome}
}

// recordNormalizationMetrics stores mutually exclusive outcomes for each
// checked field. No per-field audit events are emitted: the immutable
// normalized revision already records the output, and the prior revision is
// retained as the corresponding input evidence.
func recordNormalizationMetrics(db *database.Database, runID int64, processedArticles int, results []normalizationFieldResult) error {
	totals := make(map[string]int, len(normalizationOutcomes))
	byField := make(map[string]map[string]int, len(normalizationFields))
	for _, field := range normalizationFields {
		byField[field] = make(map[string]int, len(normalizationOutcomes))
	}
	for _, result := range results {
		totals[result.Outcome]++
		byField[result.Field][result.Outcome]++
	}
	if err := setRunMetrics(db, runID,
		"normalized_articles_processed", processedArticles,
		"normalization_fields_processed", len(results),
		"normalization_fields_changed", totals[normalizationOutcomeChanged],
		"normalization_fields_already_canonical", totals[normalizationOutcomeAlreadyCanonical],
		"normalization_fields_unavailable", totals[normalizationOutcomeUnavailable]); err != nil {
		return err
	}
	for _, field := range normalizationFields {
		if err := db.Metrics.Set(runID, "normalization_fields_processed", field, sumNormalizationOutcomes(byField[field])); err != nil {
			return err
		}
		for _, outcome := range normalizationOutcomes {
			if err := db.Metrics.Set(runID, "normalization_fields_"+outcome, field, byField[field][outcome]); err != nil {
				return err
			}
		}
	}
	return nil
}

// sumNormalizationOutcomes totals the recognized mutually exclusive normalization outcomes.
func sumNormalizationOutcomes(outcomes map[string]int) int {
	total := 0
	for _, outcome := range normalizationOutcomes {
		total += outcomes[outcome]
	}
	return total
}

// enrichWorkspaceMetadata applies article-level providers. Its output is
// persisted before identity enrichment so an ORCID provider failure cannot
// erase the metadata and authorship evidence already gathered for the run.
func enrichWorkspaceMetadata(db *database.Database, runID int64, run *Run, articles []*article.Article) (int, []fieldChange, error) {
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
	cache := &workspaceCache{db: db, runID: runID, policy: run.Manifest.CachePolicy}
	updated := 0
	var allChanges []fieldChange
	if source, ok := run.Enrichment.Sources["crossref"]; ok {
		result, err := gatherCachedCrossref(context.Background(), cache, source, articles)
		if err != nil {
			return 0, nil, err
		}
		n, changes := applyConfiguredArticleEnrichment(byDOI, source, result)
		updated += n
		allChanges = append(allChanges, changes...)
	}
	if source, ok := run.Enrichment.Sources["openalex"]; ok {
		result, err := gatherCachedOpenAlex(context.Background(), cache, source, articles)
		if err != nil {
			return 0, nil, err
		}
		n, changes := applyConfiguredArticleEnrichment(byDOI, source, result)
		updated += n
		allChanges = append(allChanges, changes...)
	}
	return updated, allChanges, nil
}

// enrichWorkspaceIdentity applies exact observed-ORCID profiles and records
// name-search evidence. The caller persists this evidence against the prior
// enrich_metadata snapshot, including when this function returns an error.
func enrichWorkspaceIdentity(db *database.Database, runID int64, run *Run, articles []*article.Article) (int, []fieldChange, []uncertainORCIDSearchEvidence, error) {
	if !run.Manifest.EnrichmentEnabled || run.Enrichment == nil {
		return 0, nil, nil, nil
	}
	source, ok := run.Enrichment.Sources["orcid"]
	if !ok {
		return 0, nil, nil, nil
	}
	cache := &workspaceCache{db: db, runID: runID, policy: run.Manifest.CachePolicy}
	openAlexSource, hasOpenAlex := run.Enrichment.Sources["openalex"]
	return enrichCachedORCID(context.Background(), cache, source, openAlexSource, hasOpenAlex, articles)
}

// gatherCachedCrossref gathers cached crossref from the supplied inputs.
func gatherCachedCrossref(ctx context.Context, cache *workspaceCache, source enrich.SourceConfig, articles []*article.Article) (*enrich.GatherResult, error) {
	client := enrich.NewClient(source)
	defer client.Close()
	result := &enrich.GatherResult{Source: "crossref", Articles: make(map[string]*enrich.ArticleEnrichment)}
	for _, a := range articles {
		if a.DOI == "" {
			continue
		}
		response, err := cache.resolve(ctx, cacheRequest{Provider: "crossref", Namespace: "work_by_doi", Identity: strings.ToLower(a.DOI), URL: source.BaseURL + a.DOI}, func(ctx context.Context) *enrich.FetchResult {
			return client.Fetch(ctx, source.BaseURL+a.DOI)
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve crossref DOI %q: %w", a.DOI, err)
		}
		if response.Status == 404 {
			result.DOINotFound = append(result.DOINotFound, a.DOI)
			continue
		}
		if enrichment := enrich.DecodeCrossrefResponse(response.Body, a.DOI); enrichment != nil {
			result.Articles[a.DOI] = enrichment
		}
	}
	return result, nil
}

// gatherCachedOpenAlex gathers cached open alex from the supplied inputs.
func gatherCachedOpenAlex(ctx context.Context, cache *workspaceCache, source enrich.SourceConfig, articles []*article.Article) (*enrich.GatherResult, error) {
	client := enrich.NewClient(source)
	defer client.Close()
	result := &enrich.GatherResult{Source: "openalex", Articles: make(map[string]*enrich.ArticleEnrichment)}
	refIDsByDOI := make(map[string][]string)
	for _, a := range articles {
		if a.DOI == "" {
			continue
		}
		url := source.BaseURL + "doi:" + a.DOI
		response, err := cache.resolve(ctx, cacheRequest{Provider: "openalex", Namespace: "work_by_doi", Identity: strings.ToLower(a.DOI), URL: url}, func(ctx context.Context) *enrich.FetchResult {
			return client.Fetch(ctx, url)
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve openalex DOI %q: %w", a.DOI, err)
		}
		if response.Status == 404 {
			result.DOINotFound = append(result.DOINotFound, a.DOI)
			continue
		}
		if enrichment, referenceIDs := enrich.DecodeOpenAlexResponse(response.Body, a.DOI); enrichment != nil {
			result.Articles[a.DOI] = enrichment
			refIDsByDOI[a.DOI] = referenceIDs
		}
	}
	references, err := resolveCachedOpenAlexReferences(ctx, cache, client, source, refIDsByDOI)
	if err != nil {
		return nil, err
	}
	for doi, ids := range refIDsByDOI {
		enrichment := result.Articles[doi]
		if enrichment == nil {
			continue
		}
		for _, id := range ids {
			if reference, ok := references[id]; ok {
				enrichment.References = append(enrichment.References, reference)
			}
		}
	}
	return result, nil
}

// resolveCachedOpenAlexReferences resolves cached open alex references from the supplied context.
func resolveCachedOpenAlexReferences(ctx context.Context, cache *workspaceCache, client *enrich.Client, source enrich.SourceConfig, refIDsByDOI map[string][]string) (map[string]enrich.EnrichedReference, error) {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, articleIDs := range refIDsByDOI {
		for _, id := range articleIDs {
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	result := make(map[string]enrich.EnrichedReference)
	batchSize := source.BatchSize
	if batchSize < 1 {
		batchSize = 50
	}
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		identity := strings.Join(batch, "|")
		url := fmt.Sprintf("%s?filter=openalex_id:%s&select=id,doi,title,publication_year&per_page=%d", strings.TrimRight(source.BaseURL, "/"), identity, batchSize)
		response, err := cache.resolve(ctx, cacheRequest{Provider: "openalex", Namespace: "work_references", Identity: identity, URL: url}, func(ctx context.Context) *enrich.FetchResult {
			return client.Fetch(ctx, url)
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve openalex reference batch: %w", err)
		}
		if response.Status == 404 {
			continue
		}
		for id, reference := range enrich.DecodeOpenAlexReferenceResponse(response.Body) {
			result[id] = reference
		}
	}
	return result, nil
}

// uncertainORCIDCandidate stores one unconfirmed ORCID name-search result and its provenance.
type uncertainORCIDCandidate struct {
	ORCID             string
	ProviderDisplay   string
	QueryURL          string
	PayloadArtifactID int64
	ProviderRank      int
}

// uncertainORCIDSearchEvidence records candidate-search results or provider failure for one author occurrence.
type uncertainORCIDSearchEvidence struct {
	DOI          string
	AuthorIndex  int
	CitationName string
	Candidates   []uncertainORCIDCandidate
	Status       string
	ErrorMessage string
}

// enrichCachedORCID gathers confirmed ORCID records and uncertain name-search evidence through the workspace cache.
func enrichCachedORCID(ctx context.Context, cache *workspaceCache, orcidSource, openAlexSource enrich.SourceConfig, hasOpenAlex bool, articles []*article.Article) (int, []fieldChange, []uncertainORCIDSearchEvidence, error) {
	orcidClient := enrich.NewClient(orcidSource)
	defer orcidClient.Close()
	var openAlexClient *enrich.Client
	if hasOpenAlex {
		openAlexClient = enrich.NewClient(openAlexSource)
		defer openAlexClient.Close()
	}
	updated := 0
	var changes []fieldChange

	// Build a DOI lookup for authors so we can record per-article field changes.
	authorDOI := make(map[*article.Author]string)
	for _, a := range articles {
		for i := range a.Authors {
			authorDOI[&a.Authors[i]] = a.DOI
		}
	}

	byORCID := make(map[string][]*article.Author)
	for _, a := range articles {
		for index := range a.Authors {
			author := &a.Authors[index]
			if author.Orcid != "" {
				byORCID[author.Orcid] = append(byORCID[author.Orcid], author)
			}
		}
	}
	for orcid, authors := range byORCID {
		profile, err := resolveCachedORCIDProfile(ctx, cache, orcidSource, orcidClient, openAlexSource, openAlexClient, hasOpenAlex, orcid)
		if err != nil {
			return 0, nil, nil, err
		}
		if profile != nil {
			changed := applyAuthorProfile(authors, profile, "")
			if changed {
				updated++
				changes = append(changes, recordAuthorFieldChanges(authors, profile, authorDOI)...)
			}
		}
	}
	var evidence []uncertainORCIDSearchEvidence
	for _, a := range articles {
		for authorIndex := range a.Authors {
			author := &a.Authors[authorIndex]
			if author.Orcid != "" || author.CitationName == "" {
				continue
			}
			candidates, err := resolveCachedORCIDNameCandidates(ctx, cache, orcidSource, orcidClient, author.CitationName)
			searchEvidence := uncertainORCIDSearchEvidence{DOI: a.DOI, AuthorIndex: authorIndex, CitationName: author.CitationName, Candidates: candidates}
			if err != nil {
				searchEvidence.Status = database.AuthorIdentityStatusProviderFailed
				searchEvidence.ErrorMessage = err.Error()
				evidence = append(evidence, searchEvidence)
				return updated, changes, evidence, err
			}
			evidence = append(evidence, searchEvidence)
		}
	}
	return updated, changes, evidence, nil
}

// recordAuthorFieldChanges returns fieldChange entries for each author field
// that was filled by the given profile. It assumes applyAuthorProfile has
// already been called and the author fields are now populated.
func recordAuthorFieldChanges(authors []*article.Author, profile *enrich.EnrichedAuthor, authorDOI map[*article.Author]string) []fieldChange {
	var changes []fieldChange
	for authorIndex, author := range authors {
		doi := authorDOI[author]
		if doi == "" {
			continue
		}
		if author.Orcid != "" && profile.ORCID != "" {
			changes = append(changes, fieldChange{DOI: doi, Field: fmt.Sprintf("author_orcid_%d", authorIndex), Provider: "orcid"})
		}
		if author.FirstName != "" && profile.FirstName != "" {
			changes = append(changes, fieldChange{DOI: doi, Field: fmt.Sprintf("author_first_name_%d", authorIndex), Provider: "orcid"})
		}
		if author.LastName != "" && profile.LastName != "" {
			changes = append(changes, fieldChange{DOI: doi, Field: fmt.Sprintf("author_last_name_%d", authorIndex), Provider: "orcid"})
		}
		if author.CitationName != "" && profile.CitationName != "" {
			changes = append(changes, fieldChange{DOI: doi, Field: fmt.Sprintf("author_citation_name_%d", authorIndex), Provider: "orcid"})
		}
		if author.Affiliation != "" && profile.Institution != "" {
			changes = append(changes, fieldChange{DOI: doi, Field: fmt.Sprintf("author_affiliation_%d", authorIndex), Provider: "orcid"})
		}
	}
	return changes
}

// resolveCachedORCIDProfile resolves cached orcid profile from the supplied context.
func resolveCachedORCIDProfile(ctx context.Context, cache *workspaceCache, orcidSource enrich.SourceConfig, orcidClient *enrich.Client, openAlexSource enrich.SourceConfig, openAlexClient *enrich.Client, hasOpenAlex bool, orcid string) (*enrich.EnrichedAuthor, error) {
	if hasOpenAlex {
		baseURL := openAlexSource.ExtraURLs["author"]
		if baseURL == "" {
			baseURL = "https://api.openalex.org/authors/"
		}
		url := baseURL + "orcid:" + orcid
		response, err := cache.resolve(ctx, cacheRequest{Provider: "openalex", Namespace: "author_by_orcid", Identity: strings.ToLower(orcid), URL: url}, func(ctx context.Context) *enrich.FetchResult {
			return openAlexClient.Fetch(ctx, url)
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve openalex ORCID %q: %w", orcid, err)
		}
		if response.Status == 200 {
			return enrich.DecodeOpenAlexAuthorResponse(response.Body, orcid), nil
		}
	}
	url := orcidSource.BaseURL + orcid
	response, err := cache.resolve(ctx, cacheRequest{Provider: "orcid", Namespace: "author_by_orcid", Identity: strings.ToLower(orcid), URL: url}, func(ctx context.Context) *enrich.FetchResult {
		return orcidClient.Fetch(ctx, url)
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("resolve ORCID %q: %w", orcid, err)
	}
	if response.Status == 404 {
		return nil, nil
	}
	return enrich.DecodeORCIDRecordResponse(response.Body, orcid), nil
}

// resolveCachedORCIDNameCandidates resolves cached orcid name candidates from the supplied context.
func resolveCachedORCIDNameCandidates(ctx context.Context, cache *workspaceCache, source enrich.SourceConfig, client *enrich.Client, name string) ([]uncertainORCIDCandidate, error) {
	seen := make(map[string]struct{})
	candidates := make([]uncertainORCIDCandidate, 0)
	for _, url := range enrich.ORCIDNameSearchURLs(source, name) {
		response, err := cache.resolve(ctx, cacheRequest{Provider: "orcid", Namespace: "author_name_search", Identity: normalizeCacheName(name) + "|" + url, URL: url}, func(ctx context.Context) *enrich.FetchResult {
			return client.Fetch(ctx, url)
		}, func(body []byte) bool { return len(enrich.DecodeORCIDNameSearchCandidates(body)) == 0 })
		if err != nil {
			return candidates, fmt.Errorf("resolve ORCID name %q: %w", name, err)
		}
		if response.Status != 200 {
			continue
		}
		for _, decoded := range enrich.DecodeORCIDNameSearchCandidates(response.Body) {
			orcid := strings.TrimSpace(decoded.ORCID)
			if orcid == "" {
				continue
			}
			key := strings.ToLower(orcid)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, uncertainORCIDCandidate{
				ORCID: orcid, QueryURL: url, PayloadArtifactID: response.PayloadArtifactID,
				ProviderRank: len(candidates) + 1,
			})
		}
	}
	return candidates, nil
}

// persistUncertainORCIDEvidence persists uncertain orcid evidence through the owning repository.
func persistUncertainORCIDEvidence(db *database.Database, runID int64, revisionIDs map[string]int64, evidence []uncertainORCIDSearchEvidence) error {
	for _, search := range evidence {
		revisionID, ok := revisionIDs[search.DOI]
		if !ok {
			return fmt.Errorf("persist uncertain ORCID evidence: missing metadata-enriched revision for DOI %q", search.DOI)
		}
		authorships, err := db.Authorships.GetByRevisionID(revisionID)
		if err != nil {
			return err
		}
		if search.AuthorIndex < 0 || search.AuthorIndex >= len(authorships) {
			return fmt.Errorf("persist uncertain ORCID evidence: author index %d is missing from metadata-enriched revision %d", search.AuthorIndex, revisionID)
		}
		status := search.Status
		if status == "" && len(search.Candidates) > 0 {
			status = database.AuthorIdentityStatusORCIDUnclear
		}
		if status == "" {
			status = database.AuthorIdentityStatusNoORCIDCandidate
		}
		resolutionID, err := db.IdentityResolutions.Create(&database.AuthorIdentityResolution{
			PipelineRunID: runID, AuthorOccurrenceID: authorships[search.AuthorIndex].AuthorOccurrenceID,
			Status: status, Provider: "orcid", QueriedCitationName: search.CitationName,
			ErrorMessage: search.ErrorMessage,
			ResolvedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return err
		}
		for _, candidate := range search.Candidates {
			if _, err := db.IdentityCandidates.Create(&database.AuthorIdentityCandidate{
				IdentityResolutionID: resolutionID, CandidateORCID: candidate.ORCID,
				ProviderDisplayName: candidate.ProviderDisplay, QueryURL: candidate.QueryURL,
				PayloadArtifactID: candidate.PayloadArtifactID, ProviderRank: candidate.ProviderRank,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// normalizeCacheName normalizes cache name.
func normalizeCacheName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// applyAuthorProfile applies author profile to the supplied state.
func applyAuthorProfile(authors []*article.Author, profile *enrich.EnrichedAuthor, matchedORCID string) bool {
	changed := false
	for _, author := range authors {
		if author.Orcid == "" && matchedORCID != "" {
			author.Orcid = matchedORCID
			changed = true
		}
		if author.FirstName == "" && profile.FirstName != "" {
			author.FirstName = profile.FirstName
			changed = true
		}
		if author.LastName == "" && profile.LastName != "" {
			author.LastName = profile.LastName
			changed = true
		}
		if author.CitationName == "" && profile.CitationName != "" {
			author.CitationName = profile.CitationName
			changed = true
		}
		if author.Affiliation == "" && profile.Institution != "" {
			author.Affiliation = profile.Institution
			changed = true
		}
	}
	return changed
}

// fieldChange records one enriched field change for audit purposes.
type fieldChange struct {
	DOI      string
	Field    string
	Provider string
}

// applyArticleEnrichment applies article enrichment to the supplied state.
func applyArticleEnrichment(articles map[string]*article.Article, result *enrich.GatherResult) (int, []fieldChange) {
	if result == nil {
		return 0, nil
	}
	updated := 0
	var changes []fieldChange
	for doi, enrichment := range result.Articles {
		a := articles[doi]
		if a == nil || enrichment == nil {
			continue
		}
		changed := false
		allow := func(existing string) bool { return !result.FillMissingOnly || existing == "" }
		if enrichment.Title != "" && enrichment.Title != a.Title && allow(a.Title) {
			a.Title, changed = enrichment.Title, true
			changes = append(changes, fieldChange{DOI: doi, Field: "title", Provider: result.Source})
		}
		if enrichment.Abstract != "" && enrichment.Abstract != a.Abstract && allow(a.Abstract) {
			a.Abstract, changed = enrichment.Abstract, true
			changes = append(changes, fieldChange{DOI: doi, Field: "abstract", Provider: result.Source})
		}
		if enrichment.Publisher != "" && enrichment.Publisher != a.Publisher && allow(a.Publisher) {
			a.Publisher, changed = enrichment.Publisher, true
			changes = append(changes, fieldChange{DOI: doi, Field: "publisher", Provider: result.Source})
		}
		if enrichment.CitationCount > 0 && enrichment.CitationCount != a.CitationCount && (!result.FillMissingOnly || a.CitationCount == 0) {
			a.CitationCount, changed = enrichment.CitationCount, true
			changes = append(changes, fieldChange{DOI: doi, Field: "citation_count", Provider: result.Source})
		}
		if len(enrichment.References) > 0 && (!result.FillMissingOnly || len(a.CitedReferences) == 0) {
			a.CitedReferences = make([]article.Reference, len(enrichment.References))
			for i, ref := range enrichment.References {
				a.CitedReferences[i] = article.Reference{DOI: ref.DOI, Title: ref.Title, Author: ref.Author, Year: ref.Year, Source: ref.Source}
			}
			changed = true
			changes = append(changes, fieldChange{DOI: doi, Field: "references", Provider: result.Source})
		}
		if len(enrichment.Authors) > 0 && (!result.FillMissingOnly || len(a.Authors) == 0) {
			authors := usableEnrichedAuthors(enrichment.Authors)
			if len(authors) > 0 {
				a.Authors = authors
				changed = true
				changes = append(changes, fieldChange{DOI: doi, Field: "authors", Provider: result.Source})
			}
		}
		if changed {
			updated++
		}
	}
	return updated, changes
}

// usableEnrichedAuthors converts enriched authors that contain a non-empty citation name.
func usableEnrichedAuthors(enriched []enrich.EnrichedAuthor) []article.Author {
	authors := make([]article.Author, 0, len(enriched))
	for _, author := range enriched {
		citationName := strings.TrimSpace(author.CitationName)
		if citationName == "" {
			continue
		}
		authors = append(authors, article.Author{
			Orcid: author.ORCID, FirstName: author.FirstName, LastName: author.LastName,
			CitationName: citationName, Affiliation: author.Affiliation,
		})
	}
	return authors
}

// applyConfiguredArticleEnrichment applies configured article enrichment to the supplied state.
func applyConfiguredArticleEnrichment(articles map[string]*article.Article, source enrich.SourceConfig, result *enrich.GatherResult) (int, []fieldChange) {
	if result != nil {
		result.FillMissingOnly = source.FillMissingOnly
	}
	return applyArticleEnrichment(articles, result)
}

// emitFieldEnrichedAuditEvents records one field_enriched audit event per field
// change. revisionIDs maps DOI -> work_revision ID.
func emitFieldEnrichedAuditEvents(db *database.Database, runID int64, revisionIDs map[string]int64, changes []fieldChange) error {
	for _, c := range changes {
		revID, ok := revisionIDs[c.DOI]
		if !ok {
			continue
		}
		metadata, err := json.Marshal(map[string]string{
			"field":    c.Field,
			"provider": c.Provider,
		})
		if err != nil {
			return err
		}
		correlationID := fmt.Sprintf("enrich-%d-%d-%s", runID, revID, c.Field)
		if _, err := db.AuditEvents.Insert(&manifest.AuditEvent{
			OccurredAt:    time.Now().UTC().Format(time.RFC3339Nano),
			Actor:         c.Provider,
			PipelineRunID: runID,
			EntityType:    "work_revision",
			EntityID:      strconv.FormatInt(revID, 10),
			Action:        manifest.AuditFieldEnriched,
			MetadataJSON:  string(metadata),
			CorrelationID: correlationID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// recordFieldEnrichmentMetrics records per-field and per-provider enrichment
// metrics to pipeline_run_metrics. Per-author-index fields (e.g.
// author_orcid_0, author_first_name_1) are aggregated into a single count
// per field type (author_orcid, author_first_name, etc.).
func recordFieldEnrichmentMetrics(db *database.Database, runID int64, changes []fieldChange) error {
	fieldCounts := make(map[string]int)
	providerCounts := make(map[string]int)
	authorFieldSuffix := regexp.MustCompile(`_\d+$`)
	for _, c := range changes {
		// Aggregate per-author-index fields by stripping the trailing _N suffix.
		field := authorFieldSuffix.ReplaceAllString(c.Field, "")
		fieldCounts[field]++
		providerCounts[c.Provider]++
	}
	if err := db.Metrics.Set(runID, "enriched_fields_total", "", len(changes)); err != nil {
		return err
	}
	for field, count := range fieldCounts {
		if err := db.Metrics.Set(runID, "enriched_fields_"+field, "", count); err != nil {
			return err
		}
	}
	for provider, count := range providerCounts {
		if err := db.Metrics.Set(runID, "enriched_fields", provider, count); err != nil {
			return err
		}
	}
	return nil
}

// completePipelineRun completes pipeline run and records its terminal state.
func completePipelineRun(db *database.Database, runID int64) error {
	if err := db.PipelineRuns.FinishRun(runID, "completed", ""); err != nil {
		return err
	}
	_, err := db.AuditEvents.Insert(&manifest.AuditEvent{OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "pipeline", PipelineRunID: runID, EntityType: "pipeline_run", EntityID: strconv.FormatInt(runID, 10), Action: manifest.AuditRunCompleted, CorrelationID: "run-complete-" + strconv.FormatInt(runID, 10)})
	return err
}

// recordValidationAudit records validation audit.
func recordValidationAudit(db *database.Database, runID int64, articles []*article.Article, reasons map[string][]string) error {
	for _, a := range articles {
		work, err := db.Works.GetByDOI(a.DOI)
		if err != nil {
			return err
		}
		if work == nil {
			return fmt.Errorf("work missing for validation audit DOI %q", a.DOI)
		}
		status := "valid"
		metadata := "{}"
		if problems := reasons[a.DOI]; len(problems) > 0 {
			status = "discarded"
			data, err := json.Marshal(map[string]any{"reasons": problems})
			if err != nil {
				return err
			}
			metadata = string(data)
		}
		if _, err := db.AuditEvents.Insert(&manifest.AuditEvent{OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "pipeline", PipelineRunID: runID, EntityType: "work", EntityID: strconv.FormatInt(work.ID, 10), Action: manifest.AuditValidationChanged, AfterJSON: fmt.Sprintf(`{"status":%q}`, status), MetadataJSON: metadata, CorrelationID: "validation-" + strconv.FormatInt(runID, 10)}); err != nil {
			return err
		}
	}
	return nil
}
