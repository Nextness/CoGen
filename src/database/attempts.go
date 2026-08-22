// attempts.go provides the repository for pipeline-run source records,
// run steps, execution plans, and search revisions that track the
// per-attempt lifecycle of workspace iterations.
package database

import (
	"database/sql"
	"fmt"
	"time"

	"analysis/manifest"
)

// RunSource links a pipeline run to one of its declared sources.
type RunSource struct {
	ID                    int64  `json:"id"`
	PipelineRunID         int64  `json:"pipeline_run_id"`
	SourceName            string `json:"source_name"`
	SourceType            string `json:"source_type"`
	ExpectedFile          string `json:"expected_file"`
	Query                 string `json:"query,omitempty"`
	RequestedFields       string `json:"requested_fields,omitempty"`
	ExpectedResultCount   *int   `json:"expected_result_count,omitempty"`
	ObservedResultCount   *int   `json:"observed_result_count,omitempty"`
	ResultCountComparison string `json:"result_count_comparison,omitempty"`
	ExportDate            string `json:"export_date,omitempty"`
	CreatedAt             string `json:"created_at"`
}

// SourceRecord represents a single raw record loaded from a source.
type SourceRecord struct {
	ID           int64  `json:"id"`
	RunSourceID  int64  `json:"run_source_id"`
	RecordIndex  int    `json:"record_index"`
	RawPayload   string `json:"raw_payload"`
	ContentHash  string `json:"content_hash"`
	ParseStatus  string `json:"parse_status"`
	RejectReason string `json:"reject_reason,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// Artifact is a content-addressed immutable payload.
type Artifact struct {
	ID          int64  `json:"id"`
	ContentHash string `json:"content_hash"`
	ByteSize    int64  `json:"byte_size"`
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
}

// RunStep records a stage's execution within a pipeline run, including its
// input/output artifacts and optional reuse from a prior run.
type RunStep struct {
	ID                int64  `json:"id"`
	PipelineRunID     int64  `json:"pipeline_run_id"`
	StepName          string `json:"step_name"`
	StepStatus        string `json:"step_status"`
	InputArtifactID   *int64 `json:"input_artifact_id,omitempty"`
	OutputArtifactID  *int64 `json:"output_artifact_id,omitempty"`
	ReusedFromRunID   *int64 `json:"reused_from_run_id,omitempty"`
	InputFingerprint  string `json:"input_fingerprint,omitempty"`
	OutputFingerprint string `json:"output_fingerprint,omitempty"`
	StartedAt         string `json:"started_at,omitempty"`
	FinishedAt        string `json:"finished_at,omitempty"`
}

// RunSourceRepository provides CRUD for the run_sources table.
type RunSourceRepository struct {
	db *Database
}

// Create inserts a new run source link. Returns the source ID.
func (r *RunSourceRepository) Create(pipelineRunID int64, sourceName, sourceType, expectedFile, query, requestedFields string, expectedResultCount int, exportDate string) (int64, error) {
	res, err := r.db.DB.Exec(
		`INSERT INTO run_sources
			(pipeline_run_id, source_name, source_type, expected_file, query, requested_fields, expected_result_count, export_date)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		pipelineRunID, sourceName, sourceType, expectedFile, nullStr(query), nullStr(requestedFields), expectedResultCount, nullStr(exportDate),
	)
	if err != nil {
		lg.Debug("run source creation failed",
			"pipeline_run_id", pipelineRunID, "source", sourceName, "error", err)
		return 0, fmt.Errorf("create run source: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("run source ID read failed",
			"pipeline_run_id", pipelineRunID, "source", sourceName, "error", err)
		return 0, err
	}
	lg.Debug("run source creation successful",
		"pipeline_run_id", pipelineRunID, "source", sourceName, "id", id)
	return id, nil
}

// ListByRun returns all sources for a given pipeline run, ordered by ID.
func (r *RunSourceRepository) ListByRun(pipelineRunID int64) ([]*RunSource, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, pipeline_run_id, source_name, source_type, expected_file,
		        query, requested_fields, expected_result_count, observed_result_count,
		        result_count_comparison, export_date, created_at
		 FROM run_sources WHERE pipeline_run_id = ? ORDER BY id`,
		pipelineRunID,
	)
	if err != nil {
		lg.Debug("run source list query failed", "pipeline_run_id", pipelineRunID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*RunSource
	for rows.Next() {
		var rs RunSource
		var query, requestedFields, comparison, exportDate sql.NullString
		var expectedCount, observedCount sql.NullInt64
		if err := rows.Scan(&rs.ID, &rs.PipelineRunID, &rs.SourceName, &rs.SourceType,
			&rs.ExpectedFile, &query, &requestedFields, &expectedCount, &observedCount, &comparison, &exportDate, &rs.CreatedAt); err != nil {
			lg.Debug("run source scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		if query.Valid {
			rs.Query = query.String
		}
		if requestedFields.Valid {
			rs.RequestedFields = requestedFields.String
		}
		if expectedCount.Valid {
			value := int(expectedCount.Int64)
			rs.ExpectedResultCount = &value
		}
		if observedCount.Valid {
			value := int(observedCount.Int64)
			rs.ObservedResultCount = &value
		}
		if comparison.Valid {
			rs.ResultCountComparison = comparison.String
		}
		if exportDate.Valid {
			rs.ExportDate = exportDate.String
		}
		result = append(result, &rs)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("run source iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("run source list query successful", "pipeline_run_id", pipelineRunID, "sources", len(result))
	return result, nil
}

// SetObservedResultCount records the raw export count observed for a source
// and its informational comparison with the count declared in the config.
func (r *RunSourceRepository) SetObservedResultCount(runSourceID int64, observedResultCount int, comparison string) error {
	_, err := r.db.DB.Exec(
		`UPDATE run_sources
		 SET observed_result_count = ?, result_count_comparison = ?
		 WHERE id = ?`,
		observedResultCount, comparison, runSourceID,
	)
	if err != nil {
		return fmt.Errorf("set observed source result count: %w", err)
	}
	return nil
}

// SourceRecordRepository provides CRUD for the source_records table.
type SourceRecordRepository struct {
	db *Database
}

// Create inserts a new source record. Returns the record ID.
func (r *SourceRecordRepository) Create(runSourceID int64, recordIndex int, rawPayload, contentHash string) (int64, error) {
	res, err := r.db.DB.Exec(
		`INSERT INTO source_records
			(run_source_id, record_index, raw_payload, content_hash)
		 VALUES (?, ?, ?, ?)`,
		runSourceID, recordIndex, rawPayload, contentHash,
	)
	if err != nil {
		lg.Debug("source record creation failed",
			"run_source_id", runSourceID, "index", recordIndex, "error", err)
		return 0, fmt.Errorf("create source record: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("source record ID read failed",
			"run_source_id", runSourceID, "index", recordIndex, "error", err)
		return 0, err
	}
	lg.Debug("source record creation successful",
		"run_source_id", runSourceID, "index", recordIndex, "id", id)
	return id, nil
}

// UpdateParseStatus updates the parse status and optional reject reason for a source record.
func (r *SourceRecordRepository) UpdateParseStatus(recordID int64, status, rejectReason string) error {
	_, err := r.db.DB.Exec(
		"UPDATE source_records SET parse_status = ?, reject_reason = ? WHERE id = ?",
		status, nullStr(rejectReason), recordID,
	)
	if err != nil {
		lg.Debug("source record parse status update failed",
			"record_id", recordID, "status", status, "error", err)
		return err
	}
	lg.Debug("source record parse status update successful",
		"record_id", recordID, "status", status)
	return nil
}

// ListBySource returns all records for a given run source, ordered by record index.
func (r *SourceRecordRepository) ListBySource(runSourceID int64) ([]*SourceRecord, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, run_source_id, record_index, raw_payload, content_hash,
		        parse_status, reject_reason, created_at
		 FROM source_records WHERE run_source_id = ? ORDER BY record_index`,
		runSourceID,
	)
	if err != nil {
		lg.Debug("source records list query failed", "run_source_id", runSourceID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*SourceRecord
	for rows.Next() {
		var sr SourceRecord
		var rejectReason sql.NullString
		if err := rows.Scan(&sr.ID, &sr.RunSourceID, &sr.RecordIndex, &sr.RawPayload,
			&sr.ContentHash, &sr.ParseStatus, &rejectReason, &sr.CreatedAt); err != nil {
			lg.Debug("source record scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		if rejectReason.Valid {
			sr.RejectReason = rejectReason.String
		}
		result = append(result, &sr)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("source record iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("source records list query successful", "run_source_id", runSourceID, "records", len(result))
	return result, nil
}

// CountBySource returns the number of records for a given run source.
func (r *SourceRecordRepository) CountBySource(runSourceID int64) (int, error) {
	var count int
	err := r.db.DB.QueryRow(
		"SELECT COUNT(*) FROM source_records WHERE run_source_id = ?", runSourceID,
	).Scan(&count)
	if err != nil {
		lg.Debug("source record count query failed", "run_source_id", runSourceID, "error", err)
		return 0, err
	}
	lg.Debug("source record count query successful", "run_source_id", runSourceID, "count", count)
	return count, nil
}

// ArtifactRepository provides CRUD for the artifacts table.
type ArtifactRepository struct {
	db *Database
}

// Create inserts a new artifact. Returns the artifact ID.
// If the content_hash already exists, returns the existing artifact ID.
func (r *ArtifactRepository) Create(contentHash, contentType string, byteSize int64) (int64, error) {
	res, err := r.db.DB.Exec(
		`INSERT OR IGNORE INTO artifacts (content_hash, byte_size, content_type)
		 VALUES (?, ?, ?)`,
		contentHash, byteSize, contentType,
	)
	if err != nil {
		lg.Debug("artifact creation failed", "hash", contentHash, "error", err)
		return 0, fmt.Errorf("create artifact: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("artifact creation result read failed", "hash", contentHash, "error", err)
		return 0, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("artifact inserted ID read failed", "hash", contentHash, "error", err)
			return 0, err
		}
		lg.Debug("artifact creation successful", "hash", contentHash, "id", id, "result", "inserted")
		return id, nil
	}

	// Already exists - return existing ID
	existing, err := r.GetByHash(contentHash)
	if err != nil {
		lg.Debug("artifact existing lookup failed", "hash", contentHash, "error", err)
		return 0, err
	}
	if existing == nil {
		lg.Debug("artifact creation failed", "hash", contentHash, "reason", "insert_skipped_but_not_found")
		return 0, fmt.Errorf("create artifact: insert skipped but existing row not found")
	}
	lg.Debug("artifact creation successful", "hash", contentHash, "id", existing.ID, "result", "already_existing")
	return existing.ID, nil
}

// GetByHash returns an artifact by its content hash, or nil if not found.
func (r *ArtifactRepository) GetByHash(contentHash string) (*Artifact, error) {
	var a Artifact
	err := r.db.DB.QueryRow(
		`SELECT id, content_hash, byte_size, content_type, created_at
		 FROM artifacts WHERE content_hash = ?`, contentHash,
	).Scan(&a.ID, &a.ContentHash, &a.ByteSize, &a.ContentType, &a.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("artifact query successful", "hash", contentHash, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("artifact query failed", "hash", contentHash, "error", err)
		return nil, err
	}
	lg.Debug("artifact query successful", "hash", contentHash, "id", a.ID, "result", "found")
	return &a, nil
}

// GetByID returns an artifact by its primary key, or nil if not found.
func (r *ArtifactRepository) GetByID(id int64) (*Artifact, error) {
	var a Artifact
	err := r.db.DB.QueryRow(
		`SELECT id, content_hash, byte_size, content_type, created_at
		 FROM artifacts WHERE id = ?`, id,
	).Scan(&a.ID, &a.ContentHash, &a.ByteSize, &a.ContentType, &a.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("artifact query successful", "id", id, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("artifact query failed", "id", id, "error", err)
		return nil, err
	}
	lg.Debug("artifact query successful", "id", id, "hash", a.ContentHash, "result", "found")
	return &a, nil
}

// ArtifactBlob stores the raw bytes for an artifact inline in the database.
type ArtifactBlob struct {
	ID            int64  `json:"id"`
	ArtifactID    int64  `json:"artifact_id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	Data          []byte `json:"-"`
	CreatedAt     string `json:"created_at"`
}

// ArtifactBlobRepository provides CRUD for the artifact_blobs table.
type ArtifactBlobRepository struct {
	db *Database
}

// Create inserts a new artifact blob. Returns the blob ID.
// If the artifact_id already exists, returns the existing blob ID (deduplicated).
func (r *ArtifactBlobRepository) Create(artifactID, pipelineRunID int64, data []byte) (int64, error) {
	res, err := r.db.DB.Exec(
		`INSERT OR IGNORE INTO artifact_blobs (artifact_id, pipeline_run_id, data)
		 VALUES (?, ?, ?)`,
		artifactID, pipelineRunID, data,
	)
	if err != nil {
		lg.Debug("artifact blob creation failed",
			"artifact_id", artifactID, "pipeline_run_id", pipelineRunID, "error", err)
		return 0, fmt.Errorf("create artifact blob: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("artifact blob creation result read failed",
			"artifact_id", artifactID, "error", err)
		return 0, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("artifact blob inserted ID read failed",
				"artifact_id", artifactID, "error", err)
			return 0, err
		}
		lg.Debug("artifact blob creation successful",
			"artifact_id", artifactID, "id", id, "result", "inserted")
		return id, nil
	}

	// Already exists - return existing ID
	existing, err := r.GetByArtifactID(artifactID)
	if err != nil {
		lg.Debug("artifact blob existing lookup failed",
			"artifact_id", artifactID, "error", err)
		return 0, err
	}
	if existing == nil {
		lg.Debug("artifact blob creation failed",
			"artifact_id", artifactID, "reason", "insert_skipped_but_not_found")
		return 0, fmt.Errorf("create artifact blob: insert skipped but existing row not found")
	}
	lg.Debug("artifact blob creation successful",
		"artifact_id", artifactID, "id", existing.ID, "result", "already_existing")
	return existing.ID, nil
}

// GetByArtifactID returns the blob for a given artifact, or nil if not found.
func (r *ArtifactBlobRepository) GetByArtifactID(artifactID int64) (*ArtifactBlob, error) {
	var b ArtifactBlob
	err := r.db.DB.QueryRow(
		`SELECT id, artifact_id, pipeline_run_id, data, created_at
		 FROM artifact_blobs WHERE artifact_id = ?`, artifactID,
	).Scan(&b.ID, &b.ArtifactID, &b.PipelineRunID, &b.Data, &b.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("artifact blob query successful", "artifact_id", artifactID, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("artifact blob query failed", "artifact_id", artifactID, "error", err)
		return nil, err
	}
	lg.Debug("artifact blob query successful", "artifact_id", artifactID, "id", b.ID, "result", "found")
	return &b, nil
}

// ListByRun returns all blobs written during a given pipeline run, ordered by ID.
func (r *ArtifactBlobRepository) ListByRun(pipelineRunID int64) ([]*ArtifactBlob, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, artifact_id, pipeline_run_id, data, created_at
		 FROM artifact_blobs WHERE pipeline_run_id = ? ORDER BY id`,
		pipelineRunID,
	)
	if err != nil {
		lg.Debug("artifact blob list by run failed", "pipeline_run_id", pipelineRunID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*ArtifactBlob
	for rows.Next() {
		var b ArtifactBlob
		if err := rows.Scan(&b.ID, &b.ArtifactID, &b.PipelineRunID, &b.Data, &b.CreatedAt); err != nil {
			lg.Debug("artifact blob scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, &b)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("artifact blob iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("artifact blob list by run successful", "pipeline_run_id", pipelineRunID, "blobs", len(result))
	return result, nil
}

// ExistsByArtifactID checks whether a blob already exists for the given artifact.
func (r *ArtifactBlobRepository) ExistsByArtifactID(artifactID int64) (bool, error) {
	var exists bool
	err := r.db.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM artifact_blobs WHERE artifact_id = ?)", artifactID,
	).Scan(&exists)
	if err != nil {
		lg.Debug("artifact blob exists check failed", "artifact_id", artifactID, "error", err)
		return false, err
	}
	lg.Debug("artifact blob exists check successful", "artifact_id", artifactID, "exists", exists)
	return exists, nil
}

// RunStepRepository provides CRUD for the run_steps table.
type RunStepRepository struct {
	db *Database
}

// runStepTimestamp returns a microsecond-precision UTC timestamp for persisted stage timing.
func runStepTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
}

// Create inserts a new run step record. Returns the step ID.
func (r *RunStepRepository) Create(pipelineRunID int64, stepName string) (int64, error) {
	res, err := r.db.DB.Exec(
		`INSERT INTO run_steps (pipeline_run_id, step_name, started_at)
		 VALUES (?, ?, ?)`,
		pipelineRunID, stepName, runStepTimestamp(),
	)
	if err != nil {
		lg.Debug("run step creation failed",
			"pipeline_run_id", pipelineRunID, "step", stepName, "error", err)
		return 0, fmt.Errorf("create run step: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("run step ID read failed",
			"pipeline_run_id", pipelineRunID, "step", stepName, "error", err)
		return 0, err
	}
	lg.Debug("run step creation successful",
		"pipeline_run_id", pipelineRunID, "step", stepName, "id", id)
	return id, nil
}

// UpdateStatus updates the status and optional finish time of a run step.
// The status must be a valid manifest.StageOutcome value. finished_at is only
// set for terminal statuses (completed, skipped, reused, failed).
func (r *RunStepRepository) UpdateStatus(stepID int64, status string) error {
	if err := manifest.ValidateStageOutcome(status); err != nil {
		lg.Debug("run step status update rejected", "step_id", stepID, "status", status, "error", err)
		return err
	}

	// Only set finished_at for terminal statuses.
	now := runStepTimestamp()
	finishedAt := &now
	if status == string(manifest.StagePending) || status == string(manifest.StageRunning) {
		finishedAt = nil
	}

	_, err := r.db.DB.Exec(
		"UPDATE run_steps SET step_status = ?, finished_at = ? WHERE id = ?",
		status, nullStrPtr(finishedAt), stepID,
	)
	if err != nil {
		lg.Debug("run step status update failed", "step_id", stepID, "status", status, "error", err)
		return err
	}
	lg.Debug("run step status update successful", "step_id", stepID, "status", status)
	return nil
}

// LinkReuse records that a step reused output from a prior run.
// It sets both the status to "reused" and the finished_at timestamp since
// reuse is a terminal stage outcome.
func (r *RunStepRepository) LinkReuse(stepID int64, reusedFromRunID int64) error {
	_, err := r.db.DB.Exec(
		"UPDATE run_steps SET reused_from_run_id = ?, step_status = 'reused', finished_at = ? WHERE id = ?",
		reusedFromRunID, runStepTimestamp(), stepID,
	)
	if err != nil {
		lg.Debug("run step reuse link failed", "step_id", stepID, "reused_from_run_id", reusedFromRunID, "error", err)
		return err
	}
	lg.Debug("run step reuse link successful", "step_id", stepID, "reused_from_run_id", reusedFromRunID)
	return nil
}

// LinkInputArtifact records that a step consumed a specific artifact as input.
func (r *RunStepRepository) LinkInputArtifact(stepID, artifactID int64) error {
	_, err := r.db.DB.Exec(
		"UPDATE run_steps SET input_artifact_id = ? WHERE id = ?",
		artifactID, stepID,
	)
	if err != nil {
		lg.Debug("run step input artifact link failed", "step_id", stepID, "artifact_id", artifactID, "error", err)
		return err
	}
	lg.Debug("run step input artifact link successful", "step_id", stepID, "artifact_id", artifactID)
	return nil
}

// LinkOutputArtifact records that a step produced a specific artifact as output.
func (r *RunStepRepository) LinkOutputArtifact(stepID, artifactID int64) error {
	_, err := r.db.DB.Exec(
		"UPDATE run_steps SET output_artifact_id = ? WHERE id = ?",
		artifactID, stepID,
	)
	if err != nil {
		lg.Debug("run step output artifact link failed", "step_id", stepID, "artifact_id", artifactID, "error", err)
		return err
	}
	lg.Debug("run step output artifact link successful", "step_id", stepID, "artifact_id", artifactID)
	return nil
}

// SetFingerprints records the immutable stage input and output fingerprints
// used to decide whether this stage may be reused by a later attempt.
func (r *RunStepRepository) SetFingerprints(stepID int64, inputFingerprint, outputFingerprint string) error {
	_, err := r.db.DB.Exec(
		"UPDATE run_steps SET input_fingerprint = ?, output_fingerprint = ? WHERE id = ?",
		inputFingerprint, outputFingerprint, stepID,
	)
	if err != nil {
		return fmt.Errorf("set run step fingerprints: %w", err)
	}
	return nil
}

// ListByRun returns all steps for a given pipeline run, ordered by ID.
func (r *RunStepRepository) ListByRun(pipelineRunID int64) ([]*RunStep, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, pipeline_run_id, step_name, step_status,
		        input_artifact_id, output_artifact_id, reused_from_run_id, input_fingerprint, output_fingerprint,
		        started_at, finished_at
		 FROM run_steps WHERE pipeline_run_id = ? ORDER BY id`,
		pipelineRunID,
	)
	if err != nil {
		lg.Debug("run step list query failed", "pipeline_run_id", pipelineRunID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*RunStep
	for rows.Next() {
		var rs RunStep
		var inputArtifactID, outputArtifactID, reusedFromRunID sql.NullInt64
		var startedAt, finishedAt sql.NullString
		if err := rows.Scan(&rs.ID, &rs.PipelineRunID, &rs.StepName, &rs.StepStatus,
			&inputArtifactID, &outputArtifactID, &reusedFromRunID, &rs.InputFingerprint, &rs.OutputFingerprint,
			&startedAt, &finishedAt); err != nil {
			lg.Debug("run step scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		if inputArtifactID.Valid {
			rs.InputArtifactID = &inputArtifactID.Int64
		}
		if outputArtifactID.Valid {
			rs.OutputArtifactID = &outputArtifactID.Int64
		}
		if reusedFromRunID.Valid {
			rs.ReusedFromRunID = &reusedFromRunID.Int64
		}
		if startedAt.Valid {
			rs.StartedAt = startedAt.String
		}
		if finishedAt.Valid {
			rs.FinishedAt = finishedAt.String
		}
		result = append(result, &rs)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("run step iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("run step list query successful", "pipeline_run_id", pipelineRunID, "steps", len(result))
	return result, nil
}

// SourceFilterCount holds per-source filter stage article counts for a run.
type SourceFilterCount struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	SourceName    string `json:"source_name"`
	FilterData    string `json:"filter_data"` // JSON array of {filters: string[], count: int}
}

// SourceFilterCountRepository provides CRUD for the source_filter_counts table.
type SourceFilterCountRepository struct {
	db *Database
}

// SetFilterData upserts the filter data for a source in a run.
func (r *SourceFilterCountRepository) SetFilterData(pipelineRunID int64, sourceName, filterData string) error {
	_, err := r.db.DB.Exec(
		`INSERT INTO source_filter_counts (pipeline_run_id, source_name, filter_data)
		 VALUES (?, ?, ?)
		 ON CONFLICT(pipeline_run_id, source_name) DO UPDATE SET filter_data = excluded.filter_data`,
		pipelineRunID, sourceName, filterData,
	)
	if err != nil {
		lg.Debug("source filter count upsert failed",
			"pipeline_run_id", pipelineRunID, "source", sourceName, "error", err)
		return fmt.Errorf("set source filter count: %w", err)
	}
	lg.Debug("source filter count upsert successful",
		"pipeline_run_id", pipelineRunID, "source", sourceName)
	return nil
}

// ListByRun returns filter data for all sources in a run, ordered by source name.
func (r *SourceFilterCountRepository) ListByRun(pipelineRunID int64) ([]*SourceFilterCount, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, pipeline_run_id, source_name, filter_data
		 FROM source_filter_counts WHERE pipeline_run_id = ? ORDER BY source_name`,
		pipelineRunID,
	)
	if err != nil {
		lg.Debug("source filter count list query failed", "pipeline_run_id", pipelineRunID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*SourceFilterCount
	for rows.Next() {
		var sfc SourceFilterCount
		if err := rows.Scan(&sfc.ID, &sfc.PipelineRunID, &sfc.SourceName, &sfc.FilterData); err != nil {
			lg.Debug("source filter count scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, &sfc)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("source filter count iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("source filter count list query successful",
		"pipeline_run_id", pipelineRunID, "sources", len(result))
	return result, nil
}

// GetByRunAndSource returns filter data for a specific source in a run.
func (r *SourceFilterCountRepository) GetByRunAndSource(pipelineRunID int64, sourceName string) (*SourceFilterCount, error) {
	var sfc SourceFilterCount
	err := r.db.DB.QueryRow(
		`SELECT id, pipeline_run_id, source_name, filter_data
		 FROM source_filter_counts WHERE pipeline_run_id = ? AND source_name = ?`,
		pipelineRunID, sourceName,
	).Scan(&sfc.ID, &sfc.PipelineRunID, &sfc.SourceName, &sfc.FilterData)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		lg.Debug("source filter count get failed",
			"pipeline_run_id", pipelineRunID, "source", sourceName, "error", err)
		return nil, err
	}
	return &sfc, nil
}
