// support.go provides the workspace pipeline orchestration:
// CSV/BibTeX loading, artifact persistence, content hashing, attempt
// lifecycle, enrichment processing, and DOI reconciliation with the
// companion PDF inventory.
package workspace

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"analysis/article"
	"analysis/bibtex"
	"analysis/database"
	"analysis/logging"
	"analysis/manifest"
)

var log = logging.Logger("workspace")

// finishPipelineRun finishes pipeline run and records its terminal state.
func finishPipelineRun(db *database.Database, runID int64, status, summary string) {
	if err := db.PipelineRuns.FinishRun(runID, status, summary); err != nil {
		log.Error("finish pipeline run", "status", status, "error", err)
		return
	}
	if status == "failed" {
		metadata, err := json.Marshal(map[string]string{"summary": summary})
		if err != nil {
			log.Error("marshal failed run audit", "error", err)
			return
		}
		if _, err := db.AuditEvents.Insert(&manifest.AuditEvent{OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "pipeline", PipelineRunID: runID, EntityType: "pipeline_run", EntityID: strconv.FormatInt(runID, 10), Action: manifest.AuditRunFailed, MetadataJSON: string(metadata), CorrelationID: "run-failed-" + strconv.FormatInt(runID, 10)}); err != nil {
			log.Error("record failed run audit", "error", err)
		}
	}
}

// StringListFlag accumulates repeated command-line flag values in declaration order.
type StringListFlag []string

// String returns the receiver's textual representation.
func (f *StringListFlag) String() string {
	return strings.Join(*f, ",")
}

// Set appends one command-line value to the receiver.
func (f *StringListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// emptySourceError means parsing completed successfully but yielded no raw
// records. The pipeline records its known observed count before failing the
// attempt because an empty configured export remains an input error.
type emptySourceError struct {
	fileType string
	source   string
	path     string
	detail   string
}

// Error returns the receiver's diagnostic message.
func (e *emptySourceError) Error() string {
	return fmt.Sprintf("%s source %q at %q %s", e.fileType, e.source, e.path, e.detail)
}

// observedCountFromLoadError extracts the known zero record count from an empty-source failure.
func observedCountFromLoadError(err error) (int, bool) {
	var emptyErr *emptySourceError
	if errors.As(err, &emptyErr) {
		return 0, true
	}
	return 0, false
}

// StartWorkspaceAttempt snapshots configuration and inputs, reuses or creates a plan, and starts an eligible pipeline attempt.
func StartWorkspaceAttempt(db *database.Database, originalConfig []byte, run *Run, fresh bool) (int64, error) {
	schemaVersion, err := db.SchemaVersion()
	if err != nil {
		return 0, err
	}
	run.Manifest.SchemaVersion = schemaVersion
	inputManifest, inputErr := buildInputManifest(run.Manifest)
	fingerprint, err := manifest.ComputeFingerprint(run.Manifest, inputManifest)
	if err != nil {
		return 0, err
	}
	configHash := contentHash(originalConfig)
	manifestHash, err := run.Manifest.Hash()
	if err != nil {
		return 0, err
	}
	inputManifestBytes, err := json.Marshal(inputManifest)
	if err != nil {
		return 0, fmt.Errorf("marshal input manifest: %w", err)
	}
	searchID, err := db.Searches.Create(run.Manifest.SearchID)
	if err != nil {
		return 0, err
	}
	revisionID, hashesUpdated, err := db.Revisions.Create(searchID, run.Manifest.SearchRevision, configHash, manifestHash)
	if err != nil {
		return 0, err
	}
	if hashesUpdated {
		metadata, _ := json.Marshal(map[string]any{
			"search_revision": run.Manifest.SearchRevision,
			"config_hash":     configHash,
			"manifest_hash":   manifestHash,
		})
		if _, err := db.AuditEvents.Insert(&manifest.AuditEvent{
			OccurredAt:    time.Now().UTC().Format(time.RFC3339Nano),
			Actor:         "pipeline",
			EntityType:    "search_revision",
			EntityID:      strconv.FormatInt(revisionID, 10),
			Action:        manifest.AuditRevisionConfigChanged,
			MetadataJSON:  string(metadata),
			CorrelationID: "revision-config-changed-" + strconv.FormatInt(revisionID, 10),
		}); err != nil {
			log.Warn("record revision config change audit", "revision_id", revisionID, "error", err)
		}
	}
	inputManifestHash := contentHash(inputManifestBytes)
	existingPlan, err := db.Plans.GetByFingerprint(revisionID, string(fingerprint))
	if err != nil {
		return 0, err
	}
	forceFresh := fresh || run.Manifest.ReusePolicy == "fresh"
	var reusedPreflightFromRunID *int64
	if existingPlan != nil {
		runs, err := db.PipelineRuns.ListByPlan(existingPlan.ID)
		if err != nil {
			return 0, err
		}
		for _, previous := range runs {
			if previous.Status == string(manifest.AttemptRunning) {
				return 0, fmt.Errorf("matching plan %d already has running attempt %d", existingPlan.ID, previous.ID)
			}
		}
		if len(runs) > 0 && runs[len(runs)-1].Status == string(manifest.AttemptCompleted) && !forceFresh {
			metadata, _ := json.Marshal(map[string]any{"reason": "matching_completed_plan", "execution_fingerprint": fingerprint})
			_, err := db.AuditEvents.Insert(&manifest.AuditEvent{
				OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "pipeline",
				EntityType: "execution_plan", EntityID: strconv.FormatInt(existingPlan.ID, 10),
				Action: manifest.AuditDuplicatePlanSkipped, MetadataJSON: string(metadata),
				CorrelationID: "plan-reuse-" + strconv.FormatInt(existingPlan.ID, 10),
			})
			return 0, err
		}
		if len(runs) > 0 && runs[len(runs)-1].Status == string(manifest.AttemptCompleted) && forceFresh {
			reusedPreflightFromRunID = &runs[len(runs)-1].ID
		}
	}
	planID, err := db.Plans.CreateWithInputManifest(revisionID, string(fingerprint), manifestHash, inputManifestHash, run.Manifest.EnrichmentEnabled)
	if err != nil {
		return 0, err
	}
	runID, _, err := db.PipelineRuns.StartAttemptIfIdle(planID, "parse+enrich", "")
	if err != nil {
		return 0, err
	}
	if err := db.Metrics.Set(runID, "enrichment_enabled", "", BoolMetric(run.Manifest.EnrichmentEnabled)); err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, err
	}
	metadata, err := json.Marshal(map[string]any{
		"enrichment_enabled": run.Manifest.EnrichmentEnabled,
		"fresh":              forceFresh,
		"reason":             freshReason(fresh, run.Manifest.ReusePolicy),
	})
	if err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, fmt.Errorf("marshal run audit metadata: %w", err)
	}
	if _, err := db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "pipeline", PipelineRunID: runID,
		EntityType: "pipeline_run", EntityID: strconv.FormatInt(runID, 10), Action: manifest.AuditRunStarted,
		MetadataJSON: string(metadata), CorrelationID: "run-start-" + strconv.FormatInt(runID, 10),
	}); err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, err
	}
	configArtifactID, err := persistArtifact(db, runID, originalConfig, "application/x-something-config")
	if err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, err
	}
	if err := db.RunArtifacts.Link(runID, configArtifactID, database.RunArtifactWorkspaceConfig); err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, err
	}
	resolvedBytes, err := json.Marshal(run.Manifest)
	if err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, fmt.Errorf("marshal resolved manifest: %w", err)
	}
	resolvedManifestArtifactID, err := persistArtifact(db, runID, resolvedBytes, "application/json")
	if err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, err
	}
	if err := db.RunArtifacts.Link(runID, resolvedManifestArtifactID, database.RunArtifactResolvedManifest); err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, err
	}
	inputManifestArtifactID, err := persistArtifact(db, runID, inputManifestBytes, "application/json")
	if err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, err
	}
	if err := db.RunArtifacts.Link(runID, inputManifestArtifactID, database.RunArtifactInputManifest); err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, err
	}
	if inputErr != nil {
		if err := recordPreflightStep(db, runID, configArtifactID, inputManifestArtifactID, string(fingerprint), inputManifestHash, nil, inputErr.Error()); err != nil {
			finishPipelineRun(db, runID, "failed", err.Error())
			return 0, err
		}
		finishPipelineRun(db, runID, "failed", inputErr.Error())
		log.Warn("configured source input preflight failed", "run_id", runID, "error", inputErr)
		return 0, inputErr
	}
	if err := recordPreflightStep(db, runID, configArtifactID, inputManifestArtifactID, string(fingerprint), inputManifestHash, reusedPreflightFromRunID, ""); err != nil {
		finishPipelineRun(db, runID, "failed", err.Error())
		return 0, err
	}
	return runID, nil
}

// recordPreflightStep records preflight step.
func recordPreflightStep(db *database.Database, runID, configArtifactID, inputManifestArtifactID int64, inputFingerprint, outputFingerprint string, reusedFromRunID *int64, failureSummary string) error {
	stepID, err := db.RunSteps.Create(runID, "preflight")
	if err != nil {
		return fmt.Errorf("create preflight step: %w", err)
	}
	if err := db.RunSteps.LinkInputArtifact(stepID, configArtifactID); err != nil {
		return fmt.Errorf("link preflight input artifact: %w", err)
	}
	if err := db.RunSteps.LinkOutputArtifact(stepID, inputManifestArtifactID); err != nil {
		return fmt.Errorf("link preflight output artifact: %w", err)
	}
	if err := db.RunSteps.SetFingerprints(stepID, inputFingerprint, outputFingerprint); err != nil {
		return fmt.Errorf("set preflight fingerprints: %w", err)
	}
	if failureSummary != "" {
		if err := db.RunSteps.UpdateStatus(stepID, string(manifest.StageFailed)); err != nil {
			return fmt.Errorf("fail preflight step: %w", err)
		}
		return nil
	}
	if reusedFromRunID == nil {
		if err := db.RunSteps.UpdateStatus(stepID, string(manifest.StageCompleted)); err != nil {
			return fmt.Errorf("complete preflight step: %w", err)
		}
		return nil
	}
	if err := db.RunSteps.LinkReuse(stepID, *reusedFromRunID); err != nil {
		return fmt.Errorf("link preflight reuse: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"step":               "preflight",
		"reused_from_run_id": *reusedFromRunID,
		"input_fingerprint":  inputFingerprint,
		"output_fingerprint": outputFingerprint,
	})
	if err != nil {
		return fmt.Errorf("marshal preflight reuse audit metadata: %w", err)
	}
	_, err = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "pipeline", PipelineRunID: runID,
		EntityType: "run_step", EntityID: strconv.FormatInt(stepID, 10), Action: manifest.AuditStepReused,
		MetadataJSON: string(metadata), CorrelationID: "preflight-reuse-" + strconv.FormatInt(runID, 10),
	})
	if err != nil {
		return fmt.Errorf("record preflight reuse audit: %w", err)
	}
	return nil
}

// buildInputManifest hashes configured source files and returns both captured evidence and aggregate read failure.
func buildInputManifest(resolved *manifest.ResolvedManifest) (*manifest.InputManifest, error) {
	files := make(map[string]manifest.SourceFileInfo, len(resolved.Sources))
	var readFailures []string
	for _, source := range resolved.Sources {
		data, err := os.ReadFile(source.ExpectedFile)
		if err != nil {
			files[source.Name] = manifest.SourceFileInfo{Path: source.ExpectedFile, ReadError: err.Error()}
			readFailures = append(readFailures, fmt.Sprintf("source %q at %q: %v", source.Name, source.ExpectedFile, err))
			continue
		}
		files[source.Name] = manifest.SourceFileInfo{Path: source.ExpectedFile, SHA256: contentHash(data), Size: int64(len(data))}
	}
	inputManifest, err := manifest.NewInputManifest(resolved, files)
	if err != nil {
		return nil, err
	}
	if len(readFailures) > 0 {
		return inputManifest, fmt.Errorf("read configured source files: %s", strings.Join(readFailures, "; "))
	}
	return inputManifest, nil
}

// persistArtifact stores content-addressed artifact metadata and bytes for a run.
func persistArtifact(db *database.Database, runID int64, data []byte, contentType string) (int64, error) {
	hash := contentHash(data)
	id, err := db.Artifacts.Create(hash, contentType, int64(len(data)))
	if err != nil {
		return 0, err
	}
	if _, err := db.ArtifactBlobs.Create(id, runID, data); err != nil {
		return 0, err
	}
	return id, nil
}

// contentHash returns the lowercase hexadecimal SHA-256 digest of data.
func contentHash(data []byte) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("%x", digest)
}

// BoolMetric converts a Boolean value to the metric convention of one or zero.
func BoolMetric(value bool) int {
	if value {
		return 1
	}
	return 0
}

// freshReason returns the persisted reason for starting a non-reused attempt.
func freshReason(explicitFresh bool, reusePolicy string) string {
	if explicitFresh {
		return "explicit_fresh"
	}
	if reusePolicy == "fresh" {
		return "declared_fresh"
	}
	return "new_or_retry"
}

// loadCSVEntries loads csv entries from the supplied source.
func loadCSVEntries(path, source string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CSV source %q at %q: %w", source, path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read CSV source %q at %q: %w", source, path, err)
	}
	if len(records) == 0 {
		return nil, &emptySourceError{fileType: "CSV", source: source, path: path, detail: "is empty"}
	}
	if len(records) == 1 {
		return nil, &emptySourceError{fileType: "CSV", source: source, path: path, detail: "has no data records"}
	}
	headers := make([]string, len(records[0]))
	for i, h := range records[0] {
		headers[i] = strings.TrimSpace(strings.ToLower(h))
	}
	entries := make([]map[string]string, 0, len(records)-1)
	for _, row := range records[1:] {
		entry := make(map[string]string, len(headers)+1)
		for i, h := range headers {
			if i < len(row) {
				entry[h] = article.SanitizeText(strings.TrimSpace(row[i]))
			}
		}
		entry["article_source"] = source
		entries = append(entries, entry)
	}
	return entries, nil
}

// loadBibEntries loads bib entries from the supplied source.
func loadBibEntries(path, source string) ([]map[string]string, error) {
	p := bibtex.NewParser(nil)
	data, err := p.LoadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load BibTeX source %q at %q: %w", source, path, err)
	}
	lib, err := p.Parse(data, source, true)
	if err != nil {
		return nil, fmt.Errorf("parse BibTeX source %q at %q: %w", source, path, err)
	}
	if len(lib) == 0 {
		return nil, &emptySourceError{fileType: "BibTeX", source: source, path: path, detail: "has no article entries"}
	}
	entries := make([]map[string]string, 0, len(lib))
	for _, entry := range lib {
		m := make(map[string]string, len(entry))
		for k, v := range entry {
			m[k] = article.SanitizeText(v)
		}
		entries = append(entries, m)
	}
	return entries, nil
}
