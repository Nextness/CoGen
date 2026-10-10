// work_adapter.go provides temporary forwarding adapters that keep the legacy
// flat work, work-identifier, work-revision, run-work-stage, reference-mention,
// and term-match repository API working over the work family store. The
// adapters contain no SQL or second implementation; remove them after every
// caller migrates to Database.Work.
package database

import (
	"context"

	"analysis/database/work"
)

// Valid producer stages (which pipeline stage created a work revision).
const (
	ProducerStageParse          = work.ProducerStageParse
	ProducerStageDeduplicate    = work.ProducerStageDeduplicate
	ProducerStageValidate       = work.ProducerStageValidate
	ProducerStageEnrich         = work.ProducerStageEnrich
	ProducerStageEnrichMetadata = work.ProducerStageEnrichMetadata
	ProducerStageEnrichIdentity = work.ProducerStageEnrichIdentity
	ProducerStageNormalize      = work.ProducerStageNormalize
	// ProducerStageLegacyUnknown is a reserved sentinel value. It is not
	// produced by any current pipeline stage and is rejected by CreateRevision.
	ProducerStageLegacyUnknown = work.ProducerStageLegacyUnknown
)

// Valid pipeline stage names (used as run_work_stages.stage_name).
const (
	StageNameParse          = work.StageNameParse
	StageNameDeduplicate    = work.StageNameDeduplicate
	StageNameValidate       = work.StageNameValidate
	StageNameEnrich         = work.StageNameEnrich
	StageNameEnrichMetadata = work.StageNameEnrichMetadata
	StageNameEnrichIdentity = work.StageNameEnrichIdentity
	StageNameNormalize      = work.StageNameNormalize
)

// Valid stage outcomes (used as run_work_stages.outcome).
const (
	OutcomeParsed       = work.OutcomeParsed
	OutcomeDuplicate    = work.OutcomeDuplicate
	OutcomeDeduplicated = work.OutcomeDeduplicated
	OutcomeValid        = work.OutcomeValid
	OutcomeDiscarded    = work.OutcomeDiscarded
	OutcomeEnriched     = work.OutcomeEnriched
	OutcomeFailed       = work.OutcomeFailed
	OutcomeNormalized   = work.OutcomeNormalized
	OutcomeSkipped      = work.OutcomeSkipped
	OutcomePending      = work.OutcomePending
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

// WorkRevision is an immutable snapshot of a work's typed core metadata and
// extension data at the point it was produced by a pipeline run stage.
// producer_stage records which pipeline stage created the revision.
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
	Keywords           string `json:"keywords"`      //  JSON array
	KeywordsPlus       string `json:"keywords_plus"` //  JSON array
	CitationCount      int    `json:"citation_count"`
	ReferenceCount     int    `json:"reference_count"`
	ExtensionData      string `json:"extension_data"` // JSON object
	CreatedAt          string `json:"created_at"`
}

// RunWorkStage records what happened to one work at one pipeline stage within
// a single pipeline run. The (pipeline_run_id, work_id, stage_name) triplet is
// unique so the same stage cannot report two different outcomes for the same
// work in the same run.
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

// ReferenceMention is one cited reference observed on an immutable work
// revision. ResolvedWorkID is optional because most citations are external to
// the current workspace.
type ReferenceMention struct {
	ID             int64  `json:"id"`
	WorkRevisionID int64  `json:"work_revision_id"`
	ResolvedWorkID int64  `json:"resolved_work_id"`
	MentionOrder   int    `json:"mention_order"`
	RawReference   string `json:"raw_reference"`
	DOI            string `json:"doi"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	Year           int    `json:"year"`
	Source         string `json:"source"`
	CreatedAt      string `json:"created_at"`
}

// RunSearchTerm is one stored search term and the source that declared it.
type RunSearchTerm struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	SourceName    string `json:"source_name"`
	Term          string `json:"term"`
	CreatedAt     string `json:"created_at"`
}

// WorkRepository forwards the legacy works API to the work family store.
type WorkRepository struct{ db *Database }

// WorkIdentifierRepository forwards the legacy work-identifier API to the work family store.
type WorkIdentifierRepository struct{ db *Database }

// WorkRevisionRepository forwards the legacy work-revision API to the work family store.
type WorkRevisionRepository struct{ db *Database }

// RunWorkStageRepository forwards the legacy run-work-stage API to the work family store.
type RunWorkStageRepository struct{ db *Database }

// ReferenceMentionRepository forwards the legacy reference-mention API to the work family store.
type ReferenceMentionRepository struct{ db *Database }

// TermMatchesRepository forwards the legacy term-match API to the work family store.
type TermMatchesRepository struct{ db *Database }

// NormalizeDOI applies the canonical DOI representation used by the works
// table and the companion PDF store.
func NormalizeDOI(doi string) string {
	return work.NormalizeDOI(doi)
}

// CreateByDOI inserts a new work by DOI. The DOI is normalized (lowercased,
// URL prefix stripped) before storage. If the normalized DOI already exists,
// returns the existing work ID (INSERT OR IGNORE semantics).
func (r *WorkRepository) CreateByDOI(doi string) (int64, error) {
	return r.db.Work.CreateWorkByDOI(context.Background(), doi)
}

// CreateWithoutDOI inserts a new work without a DOI (e.g. a title-only record).
// Each call creates a distinct row; uncertain records are never globally merged.
func (r *WorkRepository) CreateWithoutDOI() (int64, error) {
	return r.db.Work.CreateWorkWithoutDOI(context.Background())
}

// GetByID returns a work by its primary key, or nil if not found.
func (r *WorkRepository) GetByID(id int64) (*Work, error) {
	found, err := r.db.Work.GetWorkByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return workFromFamily(found), nil
}

// GetByDOI returns a work by its DOI, or nil if not found.
// The DOI is normalized the same way as CreateByDOI so that
// "10.1000/x", "10.1000/X", and "https://doi.org/10.1000/x" all
// resolve correctly.
func (r *WorkRepository) GetByDOI(doi string) (*Work, error) {
	found, err := r.db.Work.GetWorkByDOI(context.Background(), doi)
	if err != nil {
		return nil, err
	}
	return workFromFamily(found), nil
}

// ListByIDs returns works matching the given IDs, in ID order.
func (r *WorkRepository) ListByIDs(ids []int64) ([]*Work, error) {
	found, err := r.db.Work.ListWorksByIDs(context.Background(), ids)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*Work, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, workFromFamily(item))
	}
	return legacy, nil
}

// Count returns the total number of works.
func (r *WorkRepository) Count() (int, error) {
	return r.db.Work.CountWorks(context.Background())
}

// Insert adds a new identifier for a work. If the (namespace, identifier) pair
// already exists for the same work, returns the existing ID. If it belongs to
// a different work, returns an error to prevent silent ownership conflicts.
func (r *WorkIdentifierRepository) Insert(workID int64, namespace, identifier string) (int64, error) {
	return r.db.Work.InsertIdentifier(context.Background(), work.InsertIdentifierInput{
		WorkID:     workID,
		Namespace:  namespace,
		Identifier: identifier,
	})
}

// GetByID returns a work identifier by its primary key, or nil if not found.
func (r *WorkIdentifierRepository) GetByID(id int64) (*WorkIdentifier, error) {
	found, err := r.db.Work.GetIdentifierByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return identifierFromFamily(found), nil
}

// GetByWorkID returns all identifiers for a given work, ordered by ID.
func (r *WorkIdentifierRepository) GetByWorkID(workID int64) ([]*WorkIdentifier, error) {
	found, err := r.db.Work.ListIdentifiersByWorkID(context.Background(), workID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*WorkIdentifier, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, identifierFromFamily(item))
	}
	return legacy, nil
}

// GetByNamespaceAndIdentifier returns the work identifier record for the given
// namespace and identifier pair, or nil if not found.
func (r *WorkIdentifierRepository) GetByNamespaceAndIdentifier(namespace, identifier string) (*WorkIdentifier, error) {
	found, err := r.db.Work.GetIdentifierByNamespaceAndIdentifier(context.Background(), namespace, identifier)
	if err != nil {
		return nil, err
	}
	return identifierFromFamily(found), nil
}

// CountByWorkID returns the number of identifiers for a given work.
func (r *WorkIdentifierRepository) CountByWorkID(workID int64) (int, error) {
	return r.db.Work.CountIdentifiersByWorkID(context.Background(), workID)
}

// Create inserts a new immutable work revision and returns its ID.
// The payload hash is computed from the supplied core fields and extension data.
// If FieldSchemaVersion is empty, it defaults to "1".
// ProducerStage must be a known pipeline stage; legacy_unknown is rejected.
func (r *WorkRevisionRepository) Create(rev *WorkRevision) (int64, error) {
	if rev == nil {
		return r.db.Work.CreateRevision(context.Background(), nil)
	}
	family := workRevisionToFamily(rev)
	id, err := r.db.Work.CreateRevision(context.Background(), family)
	if err != nil {
		return 0, err
	}
	rev.FieldSchemaVersion = family.FieldSchemaVersion
	rev.PayloadHash = family.PayloadHash
	return id, nil
}

// GetByID returns a work revision by its primary key, or nil if not found.
func (r *WorkRevisionRepository) GetByID(id int64) (*WorkRevision, error) {
	found, err := r.db.Work.GetRevisionByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return workRevisionFromFamily(found), nil
}

// GetByWorkID returns all revisions for a given work, ordered by ID ascending
// (chronological order).
func (r *WorkRevisionRepository) GetByWorkID(workID int64) ([]*WorkRevision, error) {
	found, err := r.db.Work.ListRevisionsByWorkID(context.Background(), workID)
	if err != nil {
		return nil, err
	}
	return workRevisionsFromFamily(found), nil
}

// GetByRunID returns all revisions created by a given pipeline run.
func (r *WorkRevisionRepository) GetByRunID(runID int64) ([]*WorkRevision, error) {
	found, err := r.db.Work.ListRevisionsByRunID(context.Background(), runID)
	if err != nil {
		return nil, err
	}
	return workRevisionsFromFamily(found), nil
}

// CountByWorkID returns the number of revisions for a given work.
func (r *WorkRevisionRepository) CountByWorkID(workID int64) (int, error) {
	return r.db.Work.CountRevisionsByWorkID(context.Background(), workID)
}

// SetOutcome inserts or updates a stage outcome for a given work in a run.
// Uses an upsert so the row identity (id, created_at) is preserved across
// progressive outcome updates (e.g. "pending" -> "parsed").
func (r *RunWorkStageRepository) SetOutcome(runID, workID int64, stageName, outcome, reason string) error {
	return r.db.Work.SetStageOutcome(context.Background(), work.StageOutcomeInput{
		RunID:     runID,
		WorkID:    workID,
		StageName: stageName,
		Outcome:   outcome,
		Reason:    reason,
	})
}

// GetByRunAndWork returns the stage outcome for a specific work, run, and
// stage name, or nil if not found.
func (r *RunWorkStageRepository) GetByRunAndWork(runID, workID int64, stageName string) (*RunWorkStage, error) {
	found, err := r.db.Work.GetStageOutcome(context.Background(), runID, workID, stageName)
	if err != nil {
		return nil, err
	}
	return stageOutcomeFromFamily(found), nil
}

// GetByRunID returns all stage outcomes for a given pipeline run, ordered by ID.
func (r *RunWorkStageRepository) GetByRunID(runID int64) ([]*RunWorkStage, error) {
	found, err := r.db.Work.ListStageOutcomesByRun(context.Background(), runID)
	if err != nil {
		return nil, err
	}
	return stageOutcomesFromFamily(found), nil
}

// GetByWorkID returns all stage outcomes across runs for a given work, ordered by ID.
func (r *RunWorkStageRepository) GetByWorkID(workID int64) ([]*RunWorkStage, error) {
	found, err := r.db.Work.ListStageOutcomesByWork(context.Background(), workID)
	if err != nil {
		return nil, err
	}
	return stageOutcomesFromFamily(found), nil
}

// CountByStageAndOutcome returns the number of works that reached a given
// stage outcome within a pipeline run. Useful for dashboard funnel counts.
func (r *RunWorkStageRepository) CountByStageAndOutcome(runID int64, stageName, outcome string) (int, error) {
	return r.db.Work.CountStageOutcomes(context.Background(), runID, stageName, outcome)
}

// Create stores one ordered reference mention. A DOI is normalized and, when
// it identifies an existing work, linked through ResolvedWorkID automatically.
func (r *ReferenceMentionRepository) Create(mention *ReferenceMention) (int64, error) {
	if mention == nil {
		return r.db.Work.CreateReference(context.Background(), nil)
	}
	family := referenceToFamily(mention)
	id, err := r.db.Work.CreateReference(context.Background(), family)
	if err != nil {
		return 0, err
	}
	mention.DOI = family.DOI
	mention.ResolvedWorkID = family.ResolvedWorkID
	return id, nil
}

// GetByID returns a mention by primary key, or nil if it does not exist.
func (r *ReferenceMentionRepository) GetByID(id int64) (*ReferenceMention, error) {
	found, err := r.db.Work.GetReferenceByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return referenceFromFamily(found), nil
}

// GetByRevisionID returns a revision's references in their source order.
func (r *ReferenceMentionRepository) GetByRevisionID(revisionID int64) ([]*ReferenceMention, error) {
	found, err := r.db.Work.ListReferencesByRevisionID(context.Background(), revisionID)
	if err != nil {
		return nil, err
	}
	legacy := make([]*ReferenceMention, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, referenceFromFamily(item))
	}
	return legacy, nil
}

// GetByResolvedWorkID returns workspace citations that resolve to one work.
func (r *ReferenceMentionRepository) GetByResolvedWorkID(workID int64) ([]*ReferenceMention, error) {
	found, err := r.db.Work.ListReferencesByResolvedWorkID(context.Background(), workID)
	if err != nil {
		return nil, err
	}
	legacy := make([]*ReferenceMention, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, referenceFromFamily(item))
	}
	return legacy, nil
}

// ReplaceRunTermData replaces both the term inventory and the revision matches
// for one run in a single transaction, preserving per-run atomicity.
func (r *TermMatchesRepository) ReplaceRunTermData(runID int64, termsBySource map[string][]string, matches map[int64]map[string][]string) error {
	return r.db.Work.ReplaceRunTermData(context.Background(), work.ReplaceTermDataInput{
		RunID:         runID,
		TermsBySource: termsBySource,
		Matches:       matches,
	})
}

// HasRunTermData reports whether term reconciliation completed, including valid empty results.
func (r *TermMatchesRepository) HasRunTermData(runID int64) (bool, error) {
	return r.db.Work.HasRunTermData(context.Background(), runID)
}

// GetRunTerms returns the stored term inventory for one run ordered by id.
func (r *TermMatchesRepository) GetRunTerms(runID int64) ([]RunSearchTerm, error) {
	found, err := r.db.Work.ListRunTerms(context.Background(), runID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]RunSearchTerm, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, runTermFromFamily(item))
	}
	return legacy, nil
}

// GetRevisionMatches returns the per-field matched terms for one revision.
func (r *TermMatchesRepository) GetRevisionMatches(runID, revisionID int64) (map[string][]string, error) {
	return r.db.Work.GetRevisionMatches(context.Background(), runID, revisionID)
}

// GetRevisionMatchesBulk returns per-field matched terms for a page of
// revisions. It short-circuits when the revision list is empty.
func (r *TermMatchesRepository) GetRevisionMatchesBulk(runID int64, revisionIDs []int64) (map[int64]map[string][]string, error) {
	return r.db.Work.GetRevisionMatchesBulk(context.Background(), runID, revisionIDs)
}

// workFromFamily maps a work family work into the legacy application type.
func workFromFamily(found *work.Work) *Work {
	if found == nil {
		return nil
	}
	return &Work{ID: found.ID, DOI: found.DOI, CreatedAt: found.CreatedAt}
}

// identifierFromFamily maps a work family identifier into the legacy application type.
func identifierFromFamily(found *work.Identifier) *WorkIdentifier {
	if found == nil {
		return nil
	}
	return &WorkIdentifier{
		ID:         found.ID,
		WorkID:     found.WorkID,
		Namespace:  found.Namespace,
		Identifier: found.Identifier,
		CreatedAt:  found.CreatedAt,
	}
}

// workRevisionToFamily maps a legacy revision into the work family application type.
func workRevisionToFamily(rev *WorkRevision) *work.Revision {
	return &work.Revision{
		ID:                 rev.ID,
		WorkID:             rev.WorkID,
		PipelineRunID:      rev.PipelineRunID,
		ProducerStage:      rev.ProducerStage,
		FieldSchemaVersion: rev.FieldSchemaVersion,
		PayloadHash:        rev.PayloadHash,
		Title:              rev.Title,
		Abstract:           rev.Abstract,
		Year:               rev.Year,
		Journal:            rev.Journal,
		Publisher:          rev.Publisher,
		Source:             rev.Source,
		Keywords:           rev.Keywords,
		KeywordsPlus:       rev.KeywordsPlus,
		CitationCount:      rev.CitationCount,
		ReferenceCount:     rev.ReferenceCount,
		ExtensionData:      rev.ExtensionData,
		CreatedAt:          rev.CreatedAt,
	}
}

// workRevisionFromFamily maps a work family revision into the legacy application type.
func workRevisionFromFamily(found *work.Revision) *WorkRevision {
	if found == nil {
		return nil
	}
	return &WorkRevision{
		ID:                 found.ID,
		WorkID:             found.WorkID,
		PipelineRunID:      found.PipelineRunID,
		ProducerStage:      found.ProducerStage,
		FieldSchemaVersion: found.FieldSchemaVersion,
		PayloadHash:        found.PayloadHash,
		Title:              found.Title,
		Abstract:           found.Abstract,
		Year:               found.Year,
		Journal:            found.Journal,
		Publisher:          found.Publisher,
		Source:             found.Source,
		Keywords:           found.Keywords,
		KeywordsPlus:       found.KeywordsPlus,
		CitationCount:      found.CitationCount,
		ReferenceCount:     found.ReferenceCount,
		ExtensionData:      found.ExtensionData,
		CreatedAt:          found.CreatedAt,
	}
}

// workRevisionsFromFamily maps work family revisions into legacy application types.
// A nil input preserves the legacy nil slice identity.
func workRevisionsFromFamily(found []*work.Revision) []*WorkRevision {
	if found == nil {
		return nil
	}
	legacy := make([]*WorkRevision, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, workRevisionFromFamily(item))
	}
	return legacy
}

// stageOutcomeFromFamily maps a work family stage outcome into the legacy application type.
func stageOutcomeFromFamily(found *work.StageOutcome) *RunWorkStage {
	if found == nil {
		return nil
	}
	return &RunWorkStage{
		ID:            found.ID,
		PipelineRunID: found.PipelineRunID,
		WorkID:        found.WorkID,
		StageName:     found.StageName,
		Outcome:       found.Outcome,
		Reason:        found.Reason,
		CreatedAt:     found.CreatedAt,
		UpdatedAt:     found.UpdatedAt,
	}
}

// stageOutcomesFromFamily maps work family stage outcomes into legacy application types.
// A nil input preserves the legacy nil slice identity.
func stageOutcomesFromFamily(found []*work.StageOutcome) []*RunWorkStage {
	if found == nil {
		return nil
	}
	legacy := make([]*RunWorkStage, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, stageOutcomeFromFamily(item))
	}
	return legacy
}

// referenceToFamily maps a legacy reference mention into the work family application type.
func referenceToFamily(mention *ReferenceMention) *work.Reference {
	return &work.Reference{
		ID:             mention.ID,
		WorkRevisionID: mention.WorkRevisionID,
		ResolvedWorkID: mention.ResolvedWorkID,
		MentionOrder:   mention.MentionOrder,
		RawReference:   mention.RawReference,
		DOI:            mention.DOI,
		Title:          mention.Title,
		Author:         mention.Author,
		Year:           mention.Year,
		Source:         mention.Source,
		CreatedAt:      mention.CreatedAt,
	}
}

// referenceFromFamily maps a work family reference into the legacy application type.
func referenceFromFamily(found *work.Reference) *ReferenceMention {
	if found == nil {
		return nil
	}
	return &ReferenceMention{
		ID:             found.ID,
		WorkRevisionID: found.WorkRevisionID,
		ResolvedWorkID: found.ResolvedWorkID,
		MentionOrder:   found.MentionOrder,
		RawReference:   found.RawReference,
		DOI:            found.DOI,
		Title:          found.Title,
		Author:         found.Author,
		Year:           found.Year,
		Source:         found.Source,
		CreatedAt:      found.CreatedAt,
	}
}

// runTermFromFamily maps a work family run term into the legacy application type.
func runTermFromFamily(found work.RunTerm) RunSearchTerm {
	return RunSearchTerm{
		ID:            found.ID,
		PipelineRunID: found.PipelineRunID,
		SourceName:    found.SourceName,
		Term:          found.Term,
		CreatedAt:     found.CreatedAt,
	}
}
