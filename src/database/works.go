// works.go provides the repositories for works, work revisions, and
// DOI-based deduplication. It manages the immutable revision chain
// for each globally unique work and the ordered authorship records.
package database

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// Work represents a single work with a globally unique DOI.
// Title-only records each get their own Work row; they are never merged
// by title alone (uncertain identity).
type Work struct {
	ID        int64  `json:"id"`
	DOI       string `json:"doi"`
	CreatedAt string `json:"created_at"`
}

// WorkIdentifier is an alternative identifier for a work, scoped by namespace.
// Examples: ("scopus", "2-s2.0-84912345678"), ("openalex", "W1234567890").
type WorkIdentifier struct {
	ID         int64  `json:"id"`
	WorkID     int64  `json:"work_id"`
	Namespace  string `json:"namespace"`
	Identifier string `json:"identifier"`
	CreatedAt  string `json:"created_at"`
}

// WorkRepository provides CRUD for the works table.
type WorkRepository struct {
	db *Database
}

// CreateByDOI inserts a new work by DOI. The DOI is normalized (lowercased,
// URL prefix stripped) before storage. If the normalized DOI already exists,
// returns the existing work ID (INSERT OR IGNORE semantics).
func (r *WorkRepository) CreateByDOI(doi string) (int64, error) {
	doi = NormalizeDOI(doi)
	if doi == "" {
		return 0, fmt.Errorf("create work: doi is empty")
	}

	res, err := r.db.DB.Exec(
		"INSERT OR IGNORE INTO works (doi) VALUES (?)",
		doi,
	)
	if err != nil {
		lg.Debug("work creation by DOI failed", "doi", doi, "error", err)
		return 0, fmt.Errorf("create work: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("work creation result read failed", "doi", doi, "error", err)
		return 0, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("work inserted ID read failed", "doi", doi, "error", err)
			return 0, err
		}
		lg.Debug("work creation successful",
			"doi", doi, "work_id", id, "result", "inserted")
		return id, nil
	}

	// Already exists - look up existing ID
	existing, err := r.GetByDOI(doi)
	if err != nil {
		lg.Debug("work existing lookup failed", "doi", doi, "error", err)
		return 0, err
	}
	if existing == nil {
		lg.Debug("work creation failed",
			"doi", doi, "reason", "insert_skipped_but_not_found")
		return 0, fmt.Errorf("create work: insert skipped but existing row not found")
	}
	lg.Debug("work creation successful",
		"doi", doi, "work_id", existing.ID, "result", "already_existing")
	return existing.ID, nil
}

// CreateWithoutDOI inserts a new work without a DOI (e.g. a title-only record).
// Each call creates a distinct row; uncertain records are never globally merged.
func (r *WorkRepository) CreateWithoutDOI() (int64, error) {
	res, err := r.db.DB.Exec("INSERT INTO works (doi) VALUES (NULL)")
	if err != nil {
		lg.Debug("work creation without DOI failed", "error", err)
		return 0, fmt.Errorf("create work without DOI: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("work inserted ID read failed", "error", err)
		return 0, err
	}
	lg.Debug("work creation successful",
		"work_id", id, "doi", "(none)", "result", "inserted")
	return id, nil
}

// GetByID returns a work by its primary key, or nil if not found.
func (r *WorkRepository) GetByID(id int64) (*Work, error) {
	var w Work
	var doi sql.NullString
	err := r.db.DB.QueryRow(
		"SELECT id, doi, created_at FROM works WHERE id = ?", id,
	).Scan(&w.ID, &doi, &w.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("work query successful", "work_id", id, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("work query failed", "work_id", id, "error", err)
		return nil, err
	}
	if doi.Valid {
		w.DOI = doi.String
	}
	lg.Debug("work query successful", "work_id", id, "doi", w.DOI, "result", "found")
	return &w, nil
}

// GetByDOI returns a work by its DOI, or nil if not found.
// The DOI is normalized the same way as CreateByDOI so that
// "10.1000/x", "10.1000/X", and "https://doi.org/10.1000/x" all
// resolve correctly.
func (r *WorkRepository) GetByDOI(doi string) (*Work, error) {
	doi = NormalizeDOI(doi)
	if doi == "" {
		lg.Debug("work query by DOI skipped", "reason", "empty_after_normalization")
		return nil, nil
	}

	var w Work
	var doiNull sql.NullString
	err := r.db.DB.QueryRow(
		"SELECT id, doi, created_at FROM works WHERE doi = ?", doi,
	).Scan(&w.ID, &doiNull, &w.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("work query successful", "doi", doi, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("work query failed", "doi", doi, "error", err)
		return nil, err
	}
	if doiNull.Valid {
		w.DOI = doiNull.String
	}
	lg.Debug("work query successful", "work_id", w.ID, "doi", w.DOI, "result", "found")
	return &w, nil
}

// ListByIDs returns works matching the given IDs, in ID order.
func (r *WorkRepository) ListByIDs(ids []int64) ([]*Work, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Build placeholder string
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT id, doi, created_at FROM works WHERE id IN (%s) ORDER BY id",
		strings.Join(placeholders, ", "),
	)

	rows, err := r.db.DB.Query(query, args...)
	if err != nil {
		lg.Debug("work list query failed", "ids", ids, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*Work
	for rows.Next() {
		var w Work
		var doi sql.NullString
		if err := rows.Scan(&w.ID, &doi, &w.CreatedAt); err != nil {
			lg.Debug("work list row scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		if doi.Valid {
			w.DOI = doi.String
		}
		result = append(result, &w)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("work list iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("work list query successful", "ids", len(ids), "found", len(result))
	return result, nil
}

// Count returns the total number of works.
func (r *WorkRepository) Count() (int, error) {
	var n int
	err := r.db.DB.QueryRow("SELECT COUNT(*) FROM works").Scan(&n)
	if err != nil {
		lg.Debug("work count query failed", "error", err)
		return 0, err
	}
	lg.Debug("work count query successful", "works", n)
	return n, nil
}

// WorkIdentifierRepository provides CRUD for the work_identifiers table.
type WorkIdentifierRepository struct {
	db *Database
}

// Insert adds a new identifier for a work. If the (namespace, identifier) pair
// already exists for the same work, returns the existing ID. If it belongs to
// a different work, returns an error to prevent silent ownership conflicts.
func (r *WorkIdentifierRepository) Insert(workID int64, namespace, identifier string) (int64, error) {
	if namespace == "" || identifier == "" {
		return 0, fmt.Errorf("insert work identifier: namespace and identifier are required")
	}

	res, err := r.db.DB.Exec(
		`INSERT OR IGNORE INTO work_identifiers
			(work_id, namespace, identifier)
		 VALUES (?, ?, ?)`,
		workID, namespace, identifier,
	)
	if err != nil {
		lg.Debug("work identifier insertion failed",
			"work_id", workID, "namespace", namespace, "identifier", identifier, "error", err)
		return 0, fmt.Errorf("insert work identifier: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("work identifier result read failed",
			"work_id", workID, "namespace", namespace, "identifier", identifier, "error", err)
		return 0, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("work identifier inserted ID read failed",
				"work_id", workID, "namespace", namespace, "identifier", identifier, "error", err)
			return 0, err
		}
		lg.Debug("work identifier insertion successful",
			"work_id", workID, "namespace", namespace, "identifier", identifier,
			"id", id, "result", "inserted")
		return id, nil
	}

	// Already exists - verify ownership
	var existingWorkID int64
	err = r.db.DB.QueryRow(
		"SELECT work_id FROM work_identifiers WHERE namespace = ? AND identifier = ?",
		namespace, identifier,
	).Scan(&existingWorkID)
	if err != nil {
		lg.Debug("work identifier existing lookup failed",
			"namespace", namespace, "identifier", identifier, "error", err)
		return 0, err
	}
	if existingWorkID != workID {
		lg.Debug("work identifier ownership conflict",
			"namespace", namespace, "identifier", identifier,
			"requested_work_id", workID, "existing_work_id", existingWorkID)
		return 0, fmt.Errorf(
			"identifier %q in namespace %q already belongs to work %d, cannot reassign to work %d",
			identifier, namespace, existingWorkID, workID)
	}

	// Same work — return the existing identifier ID
	var existingID int64
	err = r.db.DB.QueryRow(
		"SELECT id FROM work_identifiers WHERE namespace = ? AND identifier = ?",
		namespace, identifier,
	).Scan(&existingID)
	if err != nil {
		lg.Debug("work identifier existing ID lookup failed",
			"namespace", namespace, "identifier", identifier, "error", err)
		return 0, err
	}
	lg.Debug("work identifier insertion successful",
		"work_id", workID, "namespace", namespace, "identifier", identifier,
		"id", existingID, "result", "already_existing")
	return existingID, nil
}

// GetByID returns a work identifier by its primary key, or nil if not found.
func (r *WorkIdentifierRepository) GetByID(id int64) (*WorkIdentifier, error) {
	var wi WorkIdentifier
	err := r.db.DB.QueryRow(
		`SELECT id, work_id, namespace, identifier, created_at
		 FROM work_identifiers WHERE id = ?`, id,
	).Scan(&wi.ID, &wi.WorkID, &wi.Namespace, &wi.Identifier, &wi.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("work identifier query successful", "id", id, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("work identifier query failed", "id", id, "error", err)
		return nil, err
	}
	lg.Debug("work identifier query successful", "id", id, "work_id", wi.WorkID, "result", "found")
	return &wi, nil
}

// GetByWorkID returns all identifiers for a given work, ordered by ID.
func (r *WorkIdentifierRepository) GetByWorkID(workID int64) ([]*WorkIdentifier, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, work_id, namespace, identifier, created_at
		 FROM work_identifiers WHERE work_id = ? ORDER BY id`,
		workID,
	)
	if err != nil {
		lg.Debug("work identifier list query failed", "work_id", workID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*WorkIdentifier
	for rows.Next() {
		var wi WorkIdentifier
		if err := rows.Scan(&wi.ID, &wi.WorkID, &wi.Namespace, &wi.Identifier, &wi.CreatedAt); err != nil {
			lg.Debug("work identifier list row scan failed", "work_id", workID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, &wi)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("work identifier list iteration failed", "work_id", workID, "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("work identifier list query successful", "work_id", workID, "identifiers", len(result))
	return result, nil
}

// GetByNamespaceAndIdentifier returns the work identifier record for the given
// namespace and identifier pair, or nil if not found.
func (r *WorkIdentifierRepository) GetByNamespaceAndIdentifier(namespace, identifier string) (*WorkIdentifier, error) {
	var wi WorkIdentifier
	err := r.db.DB.QueryRow(
		`SELECT id, work_id, namespace, identifier, created_at
		 FROM work_identifiers WHERE namespace = ? AND identifier = ?`,
		namespace, identifier,
	).Scan(&wi.ID, &wi.WorkID, &wi.Namespace, &wi.Identifier, &wi.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("work identifier lookup successful",
			"namespace", namespace, "identifier", identifier, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("work identifier lookup failed",
			"namespace", namespace, "identifier", identifier, "error", err)
		return nil, err
	}
	lg.Debug("work identifier lookup successful",
		"namespace", namespace, "identifier", identifier,
		"work_id", wi.WorkID, "result", "found")
	return &wi, nil
}

// CountByWorkID returns the number of identifiers for a given work.
func (r *WorkIdentifierRepository) CountByWorkID(workID int64) (int, error) {
	var n int
	err := r.db.DB.QueryRow(
		"SELECT COUNT(*) FROM work_identifiers WHERE work_id = ?", workID,
	).Scan(&n)
	if err != nil {
		lg.Debug("work identifier count query failed", "work_id", workID, "error", err)
		return 0, err
	}
	lg.Debug("work identifier count query successful", "work_id", workID, "identifiers", n)
	return n, nil
}

// NormalizeDOI applies the canonical DOI representation used by the works
// table and the companion PDF store.
func NormalizeDOI(doi string) string {
	doi = strings.TrimSpace(doi)
	doi = strings.ToLower(doi)
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	doi = strings.TrimPrefix(doi, "http://doi.org/")
	return doi
}

// WorkRevision is an immutable snapshot of a work's typed core metadata and
// extension data at the point it was produced by a pipeline run stage.
// producer_stage records which pipeline stage created the revision
// (e.g. "parse", "enrich").
type WorkRevision struct {
	ID                 int64  `json:"id"`
	WorkID             int64  `json:"work_id"`
	PipelineRunID      int64  `json:"pipeline_run_id"`
	ProducerStage      string `json:"producer_stage"`
	FieldSchemaVersion string `json:"field_schema_version"`
	PayloadHash        string `json:"payload_hash"`
	Title              string `json:"title"`
	Abstract           string `json:"abstract"`
	Year               int    `json:"year"`
	Journal            string `json:"journal"`
	Publisher          string `json:"publisher"`
	Source             string `json:"source"`
	Keywords           string `json:"keywords"`      // JSON array
	KeywordsPlus       string `json:"keywords_plus"` // JSON array
	CitationCount      int    `json:"citation_count"`
	ReferenceCount     int    `json:"reference_count"`
	ExtensionData      string `json:"extension_data"` // JSON object
	CreatedAt          string `json:"created_at"`
}

// RunWorkStage records what happened to one work at one pipeline stage within
// a single pipeline run. The (pipeline_run_id, work_id, stage_name) triplet
// is unique so the same stage cannot report two different outcomes for the
// same work in the same run. created_at is the first time the outcome was set;
// updated_at is the most recent time it changed (via ON CONFLICT DO UPDATE).
type RunWorkStage struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	WorkID        int64  `json:"work_id"`
	StageName     string `json:"stage_name"`
	Outcome       string `json:"outcome"`
	Reason        string `json:"reason"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Valid producer stages (which pipeline stage created a work revision).
// New revisions must use one of these; legacy_unknown is a reserved sentinel
// that is rejected by Create (no longer produced by any pipeline stage).
const (
	ProducerStageParse       = "parse"
	ProducerStageDeduplicate = "deduplicate"
	ProducerStageValidate    = "validate"
	ProducerStageEnrich      = "enrich"
	// ProducerStageEnrichMetadata records article metadata after Crossref and
	// OpenAlex complete, before any identity-provider lookup can fail.
	ProducerStageEnrichMetadata = "enrich_metadata"
	// ProducerStageEnrichIdentity records the result of exact observed-ORCID
	// profile enrichment. Name-search evidence stays attached to the preceding
	// metadata snapshot because it is not an identity assertion.
	ProducerStageEnrichIdentity = "enrich_identity"
	ProducerStageNormalize      = "normalize"
	// LegacyUnknown is a reserved sentinel value. It is not produced by any
	// current pipeline stage and is rejected by Create.
	ProducerStageLegacyUnknown = "legacy_unknown"
)

// Valid pipeline stage names (used as run_work_stages.stage_name).
const (
	StageNameParse          = "parse"
	StageNameDeduplicate    = "deduplicate"
	StageNameValidate       = "validate"
	StageNameEnrich         = "enrich"
	StageNameEnrichMetadata = "enrich_metadata"
	StageNameEnrichIdentity = "enrich_identity"
	StageNameNormalize      = "normalize"
)

// Valid stage outcomes (used as run_work_stages.outcome).
const (
	OutcomeParsed       = "parsed"
	OutcomeDuplicate    = "duplicate"
	OutcomeDeduplicated = "deduplicated"
	OutcomeValid        = "valid"
	OutcomeDiscarded    = "discarded"
	OutcomeEnriched     = "enriched"
	OutcomeFailed       = "failed"
	OutcomeNormalized   = "normalized"
	OutcomeSkipped      = "skipped"
	OutcomePending      = "pending"
)

// WorkRevisionRepository provides CRUD for the work_revisions table.
type WorkRevisionRepository struct {
	db *Database
}

// Create inserts a new immutable work revision and returns its ID.
// The payload hash is computed from the supplied core fields and extension data.
// If FieldSchemaVersion is empty, it defaults to "1".
// ProducerStage must be a known pipeline stage; legacy_unknown is rejected.
func (r *WorkRevisionRepository) Create(rev *WorkRevision) (int64, error) {
	if rev == nil {
		return 0, fmt.Errorf("create work revision: value is required")
	}
	if rev.ProducerStage == "" {
		return 0, fmt.Errorf("create work revision: producer_stage is required")
	}
	if !validProducerStage(rev.ProducerStage) {
		return 0, fmt.Errorf("create work revision: invalid producer_stage %q", rev.ProducerStage)
	}
	if rev.ProducerStage == ProducerStageLegacyUnknown {
		return 0, fmt.Errorf("create work revision: producer_stage %q is a reserved sentinel and must not be used for new revisions", ProducerStageLegacyUnknown)
	}
	if rev.FieldSchemaVersion == "" {
		rev.FieldSchemaVersion = "1"
	}
	h := computeRevisionPayloadHash(rev)

	res, err := r.db.DB.Exec(`
		INSERT INTO work_revisions
			(work_id, pipeline_run_id, producer_stage,
			 field_schema_version, payload_hash,
			 title, abstract, year, journal, publisher, source,
			 keywords, keywords_plus, citation_count, reference_count,
			 extension_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rev.WorkID, rev.PipelineRunID, rev.ProducerStage,
		rev.FieldSchemaVersion, h,
		nullStr(rev.Title), nullStr(rev.Abstract), nullInt(int64(rev.Year)),
		nullStr(rev.Journal), nullStr(rev.Publisher), nullStr(rev.Source),
		nullStr(rev.Keywords), nullStr(rev.KeywordsPlus),
		nullInt(int64(rev.CitationCount)), nullInt(int64(rev.ReferenceCount)),
		nullStr(rev.ExtensionData),
	)
	if err != nil {
		lg.Debug("work revision creation failed",
			"work_id", rev.WorkID, "run_id", rev.PipelineRunID, "error", err)
		return 0, fmt.Errorf("create work revision: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("work revision inserted ID read failed",
			"work_id", rev.WorkID, "run_id", rev.PipelineRunID, "error", err)
		return 0, err
	}
	rev.PayloadHash = h
	lg.Debug("work revision creation successful",
		"work_id", rev.WorkID, "run_id", rev.PipelineRunID, "revision_id", id,
		"payload_hash", h)
	return id, nil
}

// GetByID returns a work revision by its primary key, or nil if not found.
func (r *WorkRevisionRepository) GetByID(id int64) (*WorkRevision, error) {
	return scanWorkRevision(r.db.DB.QueryRow(
		`SELECT id, work_id, pipeline_run_id, producer_stage,
		        field_schema_version, payload_hash,
		        title, abstract, year, journal, publisher, source,
		        keywords, keywords_plus, citation_count, reference_count,
		        extension_data, created_at
		 FROM work_revisions WHERE id = ?`, id))
}

// GetByWorkID returns all revisions for a given work, ordered by ID ascending
// (chronological order).
func (r *WorkRevisionRepository) GetByWorkID(workID int64) ([]*WorkRevision, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, work_id, pipeline_run_id, producer_stage,
		        field_schema_version, payload_hash,
		        title, abstract, year, journal, publisher, source,
		        keywords, keywords_plus, citation_count, reference_count,
		        extension_data, created_at
		 FROM work_revisions WHERE work_id = ? ORDER BY id`, workID)
	if err != nil {
		lg.Debug("work revision list query failed", "work_id", workID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*WorkRevision
	for rows.Next() {
		wr, err := scanWorkRevision(rows)
		if err != nil {
			lg.Debug("work revision list row scan failed", "work_id", workID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, wr)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("work revision list iteration failed", "work_id", workID, "error", err)
		return nil, err
	}
	lg.Debug("work revision list query successful", "work_id", workID, "revisions", len(result))
	return result, nil
}

// GetByRunID returns all revisions created by a given pipeline run.
func (r *WorkRevisionRepository) GetByRunID(runID int64) ([]*WorkRevision, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, work_id, pipeline_run_id, producer_stage,
		        field_schema_version, payload_hash,
		        title, abstract, year, journal, publisher, source,
		        keywords, keywords_plus, citation_count, reference_count,
		        extension_data, created_at
		 FROM work_revisions WHERE pipeline_run_id = ? ORDER BY id`, runID)
	if err != nil {
		lg.Debug("work revision list by run query failed", "run_id", runID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*WorkRevision
	for rows.Next() {
		wr, err := scanWorkRevision(rows)
		if err != nil {
			lg.Debug("work revision list by run row scan failed", "run_id", runID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, wr)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("work revision list by run iteration failed", "run_id", runID, "error", err)
		return nil, err
	}
	lg.Debug("work revision list by run query successful", "run_id", runID, "revisions", len(result))
	return result, nil
}

// CountByWorkID returns the number of revisions for a given work.
func (r *WorkRevisionRepository) CountByWorkID(workID int64) (int, error) {
	var n int
	err := r.db.DB.QueryRow(
		"SELECT COUNT(*) FROM work_revisions WHERE work_id = ?", workID,
	).Scan(&n)
	if err != nil {
		lg.Debug("work revision count query failed", "work_id", workID, "error", err)
		return 0, err
	}
	lg.Debug("work revision count query successful", "work_id", workID, "revisions", n)
	return n, nil
}

// RunWorkStageRepository provides CRUD for the run_work_stages table.
type RunWorkStageRepository struct {
	db *Database
}

// SetOutcome inserts or updates a stage outcome for a given work in a run.
// Uses INSERT ... ON CONFLICT DO UPDATE so the row identity (id, created_at)
// is preserved across progressive outcome updates (e.g. "pending" -> "parsed").
// Stage names and outcomes are validated against known stage/outcome pairs;
// impossible combinations (e.g. parse/valid) are rejected.
func (r *RunWorkStageRepository) SetOutcome(runID, workID int64, stageName, outcome, reason string) error {
	if stageName == "" {
		return fmt.Errorf("set run work stage outcome: stage_name is required")
	}
	if outcome == "" {
		return fmt.Errorf("set run work stage outcome: outcome is required")
	}
	if !validStageName(stageName) {
		return fmt.Errorf("set run work stage outcome: invalid stage_name %q", stageName)
	}
	if !validStageOutcomeForStage(stageName, outcome) {
		return fmt.Errorf("set run work stage outcome: outcome %q is not valid for stage %q", outcome, stageName)
	}

	_, err := r.db.DB.Exec(`
		INSERT INTO run_work_stages
			(pipeline_run_id, work_id, stage_name, outcome, reason, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT (pipeline_run_id, work_id, stage_name)
		DO UPDATE SET
			outcome    = excluded.outcome,
			reason     = excluded.reason,
			updated_at = datetime('now')`,
		runID, workID, stageName, outcome, nullStr(reason),
	)
	if err != nil {
		lg.Debug("run work stage set outcome failed",
			"run_id", runID, "work_id", workID, "stage", stageName,
			"outcome", outcome, "error", err)
		return fmt.Errorf("set run work stage outcome: %w", err)
	}
	lg.Debug("run work stage set outcome successful",
		"run_id", runID, "work_id", workID, "stage", stageName, "outcome", outcome)
	return nil
}

// Valid producer stages for new work revisions.
var validProducerStages = map[string]bool{
	ProducerStageParse:          true,
	ProducerStageDeduplicate:    true,
	ProducerStageValidate:       true,
	ProducerStageEnrich:         true,
	ProducerStageEnrichMetadata: true,
	ProducerStageEnrichIdentity: true,
	ProducerStageNormalize:      true,
}

// Valid stage names for run_work_stages.
var validStageNames = map[string]bool{
	StageNameParse:          true,
	StageNameDeduplicate:    true,
	StageNameValidate:       true,
	StageNameEnrich:         true,
	StageNameEnrichMetadata: true,
	StageNameEnrichIdentity: true,
	StageNameNormalize:      true,
}

// Allowed outcomes per stage. Each stage can only report outcomes that make
// sense for that pipeline step.
var allowedStageOutcomes = map[string]map[string]bool{
	StageNameParse: {
		OutcomeParsed:  true,
		OutcomeSkipped: true,
		OutcomePending: true,
	},
	StageNameDeduplicate: {
		OutcomeDuplicate:    true,
		OutcomeDeduplicated: true,
		OutcomeSkipped:      true,
		OutcomePending:      true,
	},
	StageNameValidate: {
		OutcomeValid:     true,
		OutcomeDiscarded: true,
		OutcomeSkipped:   true,
		OutcomePending:   true,
	},
	StageNameEnrich: {
		OutcomeEnriched: true,
		OutcomeSkipped:  true,
		OutcomePending:  true,
	},
	StageNameEnrichMetadata: {
		OutcomeEnriched: true,
		OutcomeSkipped:  true,
		OutcomePending:  true,
	},
	StageNameEnrichIdentity: {
		OutcomeEnriched: true,
		OutcomeFailed:   true,
		OutcomeSkipped:  true,
		OutcomePending:  true,
	},
	StageNameNormalize: {
		OutcomeNormalized: true,
		OutcomeSkipped:    true,
		OutcomePending:    true,
	},
}

// validProducerStage reports whether the supplied producer stage is supported.
func validProducerStage(s string) bool { return validProducerStages[s] }

// validStageName reports whether the supplied stage name is supported.
func validStageName(s string) bool { return validStageNames[s] }

// validStageOutcomeForStage returns true if outcome is allowed for the given stage.
func validStageOutcomeForStage(stageName, outcome string) bool {
	allowed, ok := allowedStageOutcomes[stageName]
	if !ok {
		return false
	}
	return allowed[outcome]
}

// GetByRunAndWork returns the stage outcome for a specific work, run, and
// stage name, or nil if not found.
func (r *RunWorkStageRepository) GetByRunAndWork(runID, workID int64, stageName string) (*RunWorkStage, error) {
	return scanRunWorkStage(r.db.DB.QueryRow(
		`SELECT id, pipeline_run_id, work_id, stage_name, outcome, reason, created_at, updated_at
		 FROM run_work_stages
		 WHERE pipeline_run_id = ? AND work_id = ? AND stage_name = ?`,
		runID, workID, stageName))
}

// GetByRunID returns all stage outcomes for a given pipeline run, ordered by ID.
func (r *RunWorkStageRepository) GetByRunID(runID int64) ([]*RunWorkStage, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, pipeline_run_id, work_id, stage_name, outcome, reason, created_at, updated_at
		 FROM run_work_stages WHERE pipeline_run_id = ? ORDER BY id`, runID)
	if err != nil {
		lg.Debug("run work stage list by run query failed", "run_id", runID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*RunWorkStage
	for rows.Next() {
		rws, err := scanRunWorkStage(rows)
		if err != nil {
			lg.Debug("run work stage list by run row scan failed", "run_id", runID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, rws)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("run work stage list by run iteration failed", "run_id", runID, "error", err)
		return nil, err
	}
	return result, nil
}

// GetByWorkID returns all stage outcomes across runs for a given work, ordered by ID.
func (r *RunWorkStageRepository) GetByWorkID(workID int64) ([]*RunWorkStage, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, pipeline_run_id, work_id, stage_name, outcome, reason, created_at, updated_at
		 FROM run_work_stages WHERE work_id = ? ORDER BY id`, workID)
	if err != nil {
		lg.Debug("run work stage list by work query failed", "work_id", workID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*RunWorkStage
	for rows.Next() {
		rws, err := scanRunWorkStage(rows)
		if err != nil {
			lg.Debug("run work stage list by work row scan failed", "work_id", workID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, rws)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("run work stage list by work iteration failed", "work_id", workID, "error", err)
		return nil, err
	}
	return result, nil
}

// CountByStageAndOutcome returns the number of works that reached a given
// stage outcome within a pipeline run. Useful for dashboard funnel counts.
func (r *RunWorkStageRepository) CountByStageAndOutcome(runID int64, stageName, outcome string) (int, error) {
	var n int
	err := r.db.DB.QueryRow(
		`SELECT COUNT(*) FROM run_work_stages
		 WHERE pipeline_run_id = ? AND stage_name = ? AND outcome = ?`,
		runID, stageName, outcome,
	).Scan(&n)
	if err != nil {
		lg.Debug("run work stage count query failed",
			"run_id", runID, "stage", stageName, "outcome", outcome, "error", err)
		return 0, err
	}
	lg.Debug("run work stage count query successful",
		"run_id", runID, "stage", stageName, "outcome", outcome, "count", n)
	return n, nil
}

// scannable defines the behavior required of scannable implementations.
type scannable interface {
	Scan(dest ...any) error
}

// scanWorkRevision decodes work revision from a database row.
func scanWorkRevision(row scannable) (*WorkRevision, error) {
	var wr WorkRevision
	var title, abstract, journal, publisher, source, keywords, keywordsPlus, extensionData sql.NullString
	var year, citationCount, referenceCount sql.NullInt64
	err := row.Scan(
		&wr.ID, &wr.WorkID, &wr.PipelineRunID, &wr.ProducerStage,
		&wr.FieldSchemaVersion, &wr.PayloadHash,
		&title, &abstract, &year, &journal, &publisher, &source,
		&keywords, &keywordsPlus, &citationCount, &referenceCount,
		&extensionData, &wr.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		lg.Debug("work revision scan failed", "error", err)
		return nil, err
	}
	wr.Title = nullStrPtrVal(title)
	wr.Abstract = nullStrPtrVal(abstract)
	if year.Valid {
		wr.Year = int(year.Int64)
	}
	wr.Journal = nullStrPtrVal(journal)
	wr.Publisher = nullStrPtrVal(publisher)
	wr.Source = nullStrPtrVal(source)
	wr.Keywords = nullStrPtrVal(keywords)
	wr.KeywordsPlus = nullStrPtrVal(keywordsPlus)
	if citationCount.Valid {
		wr.CitationCount = int(citationCount.Int64)
	}
	if referenceCount.Valid {
		wr.ReferenceCount = int(referenceCount.Int64)
	}
	wr.ExtensionData = nullStrPtrVal(extensionData)
	return &wr, nil
}

// scanRunWorkStage decodes run work stage from a database row.
func scanRunWorkStage(row scannable) (*RunWorkStage, error) {
	var rws RunWorkStage
	var reason sql.NullString
	err := row.Scan(&rws.ID, &rws.PipelineRunID, &rws.WorkID,
		&rws.StageName, &rws.Outcome, &reason, &rws.CreatedAt, &rws.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		lg.Debug("run work stage scan failed", "error", err)
		return nil, err
	}
	rws.Reason = nullStrPtrVal(reason)
	return &rws, nil
}

// nullStrPtrVal returns a nullable SQL string's value or an empty string.
func nullStrPtrVal(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// computeRevisionPayloadHash computes a deterministic SHA-256 hex hash from
// the typed core fields and extension data of a WorkRevision.
// producer_stage is excluded because it is provenance metadata, not content.
// field_schema_version is included because it affects how the fields should be
// interpreted.
func computeRevisionPayloadHash(rev *WorkRevision) string {
	// Collect key-value pairs and sort by key for determinism.
	m := map[string]string{
		"title":                rev.Title,
		"abstract":             rev.Abstract,
		"year":                 fmt.Sprintf("%d", rev.Year),
		"journal":              rev.Journal,
		"publisher":            rev.Publisher,
		"source":               rev.Source,
		"keywords":             rev.Keywords,
		"keywords_plus":        rev.KeywordsPlus,
		"citation_count":       fmt.Sprintf("%d", rev.CitationCount),
		"reference_count":      fmt.Sprintf("%d", rev.ReferenceCount),
		"extension_data":       rev.ExtensionData,
		"field_schema_version": rev.FieldSchemaVersion,
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(m[k]))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
