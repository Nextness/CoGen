// metrics_audit.go provides the repositories for pipeline-run metrics
// and audit events, recording counters, field-enrichment provenance,
// and per-field audit trails for each pipeline run.
package database

import (
	"database/sql"
	"fmt"

	"analysis/manifest"
)

// PipelineRunMetric is a single counter snapshot for a pipeline run.
type PipelineRunMetric struct {
	PipelineRunID int64  `json:"pipeline_run_id"`
	Metric        string `json:"metric"`
	Source        string `json:"source"`
	Value         int    `json:"value"`
}

// AuditEventRecord is the persisted representation of an audit event.
type AuditEventRecord struct {
	ID            int64  `json:"id"`
	OccurredAt    string `json:"occurred_at"`
	Actor         string `json:"actor"`
	PipelineRunID *int64 `json:"pipeline_run_id,omitempty"`
	EntityType    string `json:"entity_type"`
	EntityID      string `json:"entity_id"`
	Action        string `json:"action"`
	BeforeJSON    string `json:"before_json,omitempty"`
	AfterJSON     string `json:"after_json,omitempty"`
	MetadataJSON  string `json:"metadata_json,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// MetricsRepository provides CRUD for the pipeline_run_metrics table.
type MetricsRepository struct {
	db *Database
}

// Set inserts or replaces a metric value for a given run, metric name, and source.
// If source is empty, it records a whole-run metric.
func (r *MetricsRepository) Set(runID int64, metric, source string, value int) error {
	_, err := r.db.DB.Exec(
		`INSERT OR REPLACE INTO pipeline_run_metrics
			(pipeline_run_id, metric, source, value)
		 VALUES (?, ?, ?, ?)`,
		runID, metric, source, value,
	)
	if err != nil {
		lg.Debug("metric set failed",
			"run_id", runID, "metric", metric, "source", source, "value", value, "error", err)
		return fmt.Errorf("set metric: %w", err)
	}
	lg.Debug("metric set successful",
		"run_id", runID, "metric", metric, "source", source, "value", value)
	return nil
}

// Get returns a single metric for a given run, metric name, and source.
// Returns nil, nil if the metric is not recorded for this run.
func (r *MetricsRepository) Get(runID int64, metric, source string) (*PipelineRunMetric, error) {
	var m PipelineRunMetric
	err := r.db.DB.QueryRow(
		`SELECT pipeline_run_id, metric, source, value
		 FROM pipeline_run_metrics WHERE pipeline_run_id = ? AND metric = ? AND source = ?`,
		runID, metric, source,
	).Scan(&m.PipelineRunID, &m.Metric, &m.Source, &m.Value)
	if err == sql.ErrNoRows {
		lg.Debug("metric get successful",
			"run_id", runID, "metric", metric, "source", source, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("metric get failed",
			"run_id", runID, "metric", metric, "source", source, "error", err)
		return nil, err
	}
	lg.Debug("metric get successful",
		"run_id", runID, "metric", metric, "source", source, "value", m.Value)
	return &m, nil
}

// ListByRun returns all metrics for a given pipeline run, ordered by metric name then source.
func (r *MetricsRepository) ListByRun(runID int64) ([]*PipelineRunMetric, error) {
	rows, err := r.db.DB.Query(
		`SELECT pipeline_run_id, metric, source, value
		 FROM pipeline_run_metrics WHERE pipeline_run_id = ? ORDER BY metric, source`,
		runID,
	)
	if err != nil {
		lg.Debug("metric list by run failed", "run_id", runID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*PipelineRunMetric
	for rows.Next() {
		var m PipelineRunMetric
		if err := rows.Scan(&m.PipelineRunID, &m.Metric, &m.Source, &m.Value); err != nil {
			lg.Debug("metric scan failed", "run_id", runID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, &m)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("metric iteration failed", "run_id", runID, "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("metric list by run successful", "run_id", runID, "metrics", len(result))
	return result, nil
}

// ListByRunAndSource returns all metrics for a given run and source.
func (r *MetricsRepository) ListByRunAndSource(runID int64, source string) ([]*PipelineRunMetric, error) {
	rows, err := r.db.DB.Query(
		`SELECT pipeline_run_id, metric, source, value
		 FROM pipeline_run_metrics WHERE pipeline_run_id = ? AND source = ? ORDER BY metric`,
		runID, source,
	)
	if err != nil {
		lg.Debug("metric list by run and source failed", "run_id", runID, "source", source, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*PipelineRunMetric
	for rows.Next() {
		var m PipelineRunMetric
		if err := rows.Scan(&m.PipelineRunID, &m.Metric, &m.Source, &m.Value); err != nil {
			lg.Debug("metric scan failed", "run_id", runID, "source", source, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, &m)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("metric iteration failed", "run_id", runID, "source", source, "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("metric list by run and source successful", "run_id", runID, "source", source, "metrics", len(result))
	return result, nil
}

// AuditEventRepository provides CRUD for the audit_events table.
type AuditEventRepository struct {
	db *Database
}

// Insert stores a new audit event. The event's action is validated against
// the manifest lifecycle vocabulary before insertion.
func (r *AuditEventRepository) Insert(event *manifest.AuditEvent) (int64, error) {
	if err := manifest.ValidateAuditAction(string(event.Action)); err != nil {
		lg.Debug("audit event insert rejected", "action", event.Action, "error", err)
		return 0, err
	}

	var runID any
	if event.PipelineRunID != 0 {
		runID = event.PipelineRunID
	}

	res, err := r.db.DB.Exec(
		`INSERT INTO audit_events
			(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action,
			 before_json, after_json, metadata_json, correlation_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.OccurredAt, event.Actor, runID,
		event.EntityType, event.EntityID, string(event.Action),
		nullStr(event.BeforeJSON), nullStr(event.AfterJSON),
		nullStr(event.MetadataJSON), nullStr(event.CorrelationID),
	)
	if err != nil {
		lg.Debug("audit event insert failed",
			"action", event.Action, "entity", event.EntityType, "id", event.EntityID, "error", err)
		return 0, fmt.Errorf("insert audit event: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("audit event ID read failed",
			"action", event.Action, "entity", event.EntityType, "error", err)
		return 0, err
	}
	lg.Debug("audit event insert successful",
		"id", id, "action", event.Action, "entity", event.EntityType, "entity_id", event.EntityID)
	return id, nil
}

// ListByRun returns all audit events for a given pipeline run, ordered by ID.
func (r *AuditEventRepository) ListByRun(runID int64) ([]*AuditEventRecord, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id,
		        action, before_json, after_json, metadata_json, correlation_id
		 FROM audit_events WHERE pipeline_run_id = ? ORDER BY id`,
		runID,
	)
	if err != nil {
		lg.Debug("audit event list by run failed", "run_id", runID, "error", err)
		return nil, err
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

// ListByEntity returns all audit events for a given entity type and ID, ordered by ID.
func (r *AuditEventRepository) ListByEntity(entityType, entityID string) ([]*AuditEventRecord, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id,
		        action, before_json, after_json, metadata_json, correlation_id
		 FROM audit_events WHERE entity_type = ? AND entity_id = ? ORDER BY id`,
		entityType, entityID,
	)
	if err != nil {
		lg.Debug("audit event list by entity failed", "entity_type", entityType, "entity_id", entityID, "error", err)
		return nil, err
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

// ListByAction returns all audit events for a given action, ordered by ID.
func (r *AuditEventRepository) ListByAction(action manifest.AuditAction) ([]*AuditEventRecord, error) {
	if err := manifest.ValidateAuditAction(string(action)); err != nil {
		lg.Debug("audit event list by action rejected", "action", action, "error", err)
		return nil, err
	}
	rows, err := r.db.DB.Query(
		`SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id,
		        action, before_json, after_json, metadata_json, correlation_id
		 FROM audit_events WHERE action = ? ORDER BY id`,
		string(action),
	)
	if err != nil {
		lg.Debug("audit event list by action failed", "action", action, "error", err)
		return nil, err
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

// ListAll returns all audit events ordered by ID, with an optional limit.
// A limit of 0 returns all events.
func (r *AuditEventRepository) ListAll(limit int) ([]*AuditEventRecord, error) {
	q := `SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id,
	             action, before_json, after_json, metadata_json, correlation_id
	      FROM audit_events ORDER BY id`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = r.db.DB.Query(q+" LIMIT ?", limit)
	} else {
		rows, err = r.db.DB.Query(q)
	}
	if err != nil {
		lg.Debug("audit event list all failed", "limit", limit, "error", err)
		return nil, err
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

// scanAuditEvents decodes audit events from a database row.
func scanAuditEvents(rows *sql.Rows) ([]*AuditEventRecord, error) {
	var result []*AuditEventRecord
	for rows.Next() {
		var e AuditEventRecord
		var runID sql.NullInt64
		var beforeJSON, afterJSON, metadataJSON, correlationID sql.NullString
		if err := rows.Scan(
			&e.ID, &e.OccurredAt, &e.Actor, &runID,
			&e.EntityType, &e.EntityID, &e.Action,
			&beforeJSON, &afterJSON, &metadataJSON, &correlationID,
		); err != nil {
			lg.Debug("audit event scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		if runID.Valid {
			e.PipelineRunID = &runID.Int64
		}
		if beforeJSON.Valid {
			e.BeforeJSON = beforeJSON.String
		}
		if afterJSON.Valid {
			e.AfterJSON = afterJSON.String
		}
		if metadataJSON.Valid {
			e.MetadataJSON = metadataJSON.String
		}
		if correlationID.Valid {
			e.CorrelationID = correlationID.String
		}
		result = append(result, &e)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("audit event iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("audit event scan successful", "events", len(result))
	return result, nil
}

// PurgeEligibility describes whether a pipeline run can be safely purged.
type PurgeEligibility struct {
	// Eligible is true when no other run references this run's artifacts or
	// reusable outputs.
	Eligible bool `json:"eligible"`
	// SharedArtifactCount is the number of artifacts from this run that are
	// referenced by other runs.
	SharedArtifactCount int `json:"shared_artifact_count"`
	// ReusedByCount is the number of other runs that reuse one or more stages
	// from this run.
	ReusedByCount int `json:"reused_by_count"`
}

// CheckPurgeEligibility verifies that no other run shares artifacts or reusable
// stage outputs from the given run. It is the safety check before purge.
// Returns an error if no pipeline run with the given ID exists.
func (r *PipelineRunRepository) CheckPurgeEligibility(runID int64) (*PurgeEligibility, error) {
	// Verify the run exists first.
	existing, err := r.GetByID(runID)
	if err != nil {
		lg.Debug("purge eligibility lookup failed", "run_id", runID, "error", err)
		return nil, fmt.Errorf("check purge eligibility: %w", err)
	}
	if existing == nil {
		lg.Debug("purge eligibility not found", "run_id", runID)
		return nil, fmt.Errorf("pipeline run %d not found", runID)
	}

	// Check whether any other run's steps reference this run's artifacts.
	// We must check both input_artifact_id and output_artifact_id because the
	// artifacts table is content-addressed with INSERT OR IGNORE: a different
	// run may "produce" the same artifact (same content hash) and thus reference
	// this run's artifact as either input or output.
	var sharedArtifactCount int
	err = r.db.DB.QueryRow(
		`SELECT COUNT(DISTINCT artifact_id)
		 FROM (
		     SELECT input_artifact_id AS artifact_id FROM run_steps WHERE pipeline_run_id != ?
		     UNION ALL
		     SELECT output_artifact_id AS artifact_id FROM run_steps WHERE pipeline_run_id != ?
		 ) other
		 WHERE artifact_id IN (
		     SELECT output_artifact_id FROM run_steps WHERE pipeline_run_id = ?
		 )`,
		runID, runID, runID,
	).Scan(&sharedArtifactCount)
	if err != nil {
		lg.Debug("purge eligibility artifact check failed", "run_id", runID, "error", err)
		return nil, fmt.Errorf("check shared artifacts: %w", err)
	}

	// Check whether any other run reuses stages from this run.
	var reusedByCount int
	err = r.db.DB.QueryRow(
		"SELECT COUNT(DISTINCT pipeline_run_id) FROM run_steps WHERE reused_from_run_id = ? AND pipeline_run_id != ?",
		runID, runID,
	).Scan(&reusedByCount)
	if err != nil {
		lg.Debug("purge eligibility reuse check failed", "run_id", runID, "error", err)
		return nil, fmt.Errorf("check reused by count: %w", err)
	}

	eligible := sharedArtifactCount == 0 && reusedByCount == 0

	pe := &PurgeEligibility{
		Eligible:            eligible,
		SharedArtifactCount: sharedArtifactCount,
		ReusedByCount:       reusedByCount,
	}
	lg.Debug("purge eligibility check successful",
		"run_id", runID, "eligible", eligible,
		"shared_artifacts", sharedArtifactCount, "reused_by", reusedByCount)
	return pe, nil
}
