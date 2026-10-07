// types.go defines the work family application types and vocabulary. Generated
// sqlc row types stay inside the private internal/sql package and never cross
// this family boundary.
package work

// Work is a stable corpus work identity. A work with a globally unique DOI is
// shared across searches; title-only records each get their own row because
// title identity is uncertain.
type Work struct {
	ID        int64  `json:"id"`
	DOI       string `json:"doi"`
	CreatedAt string `json:"created_at"`
}

// Identifier is an alternative identifier for a work, scoped by namespace.
// Examples: ("scopus", "2-s2.0-84912345678"), ("openalex", "W1234567890").
type Identifier struct {
	ID         int64  `json:"id"`
	WorkID     int64  `json:"work_id"`
	Namespace  string `json:"namespace"`
	Identifier string `json:"identifier"`
	CreatedAt  string `json:"created_at"`
}

// Revision is an immutable snapshot of a work's typed core metadata and
// extension data at the point it was produced by a pipeline run stage.
// ProducerStage records which pipeline stage created the revision.
type Revision struct {
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

// StageOutcome records what happened to one work at one pipeline stage within
// a single pipeline run. The (pipeline_run_id, work_id, stage_name) triplet is
// unique so the same stage cannot report two different outcomes for the same
// work in the same run. CreatedAt is the first time the outcome was set;
// UpdatedAt is the most recent time it changed.
type StageOutcome struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	WorkID        int64  `json:"work_id"`
	StageName     string `json:"stage_name"`
	Outcome       string `json:"outcome"`
	Reason        string `json:"reason"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Reference is one cited reference observed on an immutable work revision.
// ResolvedWorkID is optional because most citations are external to the
// current workspace.
type Reference struct {
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

// RunTerm is one stored search term and the source that declared it.
type RunTerm struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	SourceName    string `json:"source_name"`
	Term          string `json:"term"`
	CreatedAt     string `json:"created_at"`
}

// Valid producer stages (which pipeline stage created a work revision).
// New revisions must use one of these; LegacyUnknown is a reserved sentinel
// that is rejected by CreateRevision (no longer produced by any pipeline
// stage).
const (
	ProducerStageParse          = "parse"
	ProducerStageDeduplicate    = "deduplicate"
	ProducerStageValidate       = "validate"
	ProducerStageEnrich         = "enrich"
	ProducerStageEnrichMetadata = "enrich_metadata"
	ProducerStageEnrichIdentity = "enrich_identity"
	ProducerStageNormalize      = "normalize"
	// ProducerStageLegacyUnknown is a reserved sentinel value. It is not
	// produced by any current pipeline stage and is rejected by CreateRevision.
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

// InsertIdentifierInput identifies one alternate identifier to attach to a work.
type InsertIdentifierInput struct {
	WorkID     int64
	Namespace  string
	Identifier string
}

// StageOutcomeInput identifies one stage outcome to insert or update.
type StageOutcomeInput struct {
	RunID     int64
	WorkID    int64
	StageName string
	Outcome   string
	Reason    string
}

// ReplaceTermDataInput replaces one run's term inventory and revision matches.
type ReplaceTermDataInput struct {
	RunID         int64
	TermsBySource map[string][]string
	Matches       map[int64]map[string][]string
}

// ArticleAuthor is one ordered authorship attached to an immutable revision.
type ArticleAuthor struct {
	RelationID   int64
	ID           int64
	PersonID     *int64
	CitationName string
	FirstName    string
	LastName     string
	ORCID        string
	AuthorOrder  int
	Affiliation  string
}

// ArticleAuthorPageInput selects one bounded page of article authors.
type ArticleAuthorPageInput struct {
	RevisionID int64
	CursorID   int64
	Limit      int
}

// ArticleAuthorPage is one bounded page of article authors with its exact total.
type ArticleAuthorPage struct {
	Items        []*ArticleAuthor
	Total        int64
	HasMore      bool
	NextCursorID int64
}

// ArticleRevision is the full immutable revision projection for one article detail.
type ArticleRevision struct {
	ID                 int64
	WorkID             int64
	PipelineRunID      int64
	FieldSchemaVersion string
	PayloadHash        string
	Title              string
	Abstract           string
	Year               int
	Journal            string
	Publisher          string
	Source             string
	Keywords           string
	KeywordsPlus       string
	CitationCount      int
	ReferenceCount     int
	ExtensionData      string
	ProducerStage      string
	CreatedAt          string
	DOI                string
}

// ArticleReference is one cited reference with its resolved-work context.
type ArticleReference struct {
	ID                 int64
	WorkRevisionID     int64
	ResolvedWorkID     *int64
	MentionOrder       int
	DOI                string
	Title              string
	Author             string
	Year               int
	Source             string
	CreatedAt          string
	ResolvedRevisionID *int64
	ResolvedTitle      string
}

// ArticleReferencePageInput selects one bounded page of article references.
type ArticleReferencePageInput struct {
	RevisionID int64
	CursorID   int64
	Limit      int
}

// ArticleReferencePage is one bounded page of article references with its exact total.
type ArticleReferencePage struct {
	Items        []*ArticleReference
	Total        int64
	HasMore      bool
	NextCursorID int64
}

// ArticleStagePageInput selects one bounded page of article stage outcomes.
type ArticleStagePageInput struct {
	RunID    int64
	WorkID   int64
	CursorID int64
	Limit    int
}

// ArticleStagePage is one bounded page of article stage outcomes with its exact total.
type ArticleStagePage struct {
	Items        []*StageOutcome
	Total        int64
	HasMore      bool
	NextCursorID int64
}

// ReferenceDetail is one reference mention with its citing and resolved-work context.
type ReferenceDetail struct {
	Reference
	WorkID             int64
	CitingTitle        string
	PipelineRunID      int64
	ResolvedRevisionID *int64
	ResolvedTitle      string
}

// EvaluationFilter selects one bounded evaluation page and its review facets.
type EvaluationFilter struct {
	RunID             int64
	ContextID         int64
	CurrentRevisionID int64
	Query             string
	Source            string
	ReviewStatus      string
	Qualifier         string
	ReviewSource      string
	Reviewed          string
	PDFStatus         string
	AvailableDOIsJSON string
	SortField         string
	Order             string
	Page              int
	PerPage           int
}

// EvaluationRow is one analysis-ready article with its review and validation state.
type EvaluationRow struct {
	ID                       int64
	WorkID                   int64
	Title                    string
	Year                     int
	Journal                  string
	Publisher                string
	Source                   string
	DOI                      string
	ValidationStatus         string
	CitationCount            int
	ReferenceCount           int
	ProducerStage            string
	CreatedAt                string
	Abstract                 string
	Keywords                 string
	KeywordsPlus             string
	Authors                  string
	WorkRevisionID           int64
	ReviewStatus             string
	ReviewInherited          bool
	ReviewVersionID          *int64
	ReviewCreatedInContextID *int64
	ReviewSubStatuses        []string
}

// EvaluationPage is one bounded evaluation page with its exact total.
type EvaluationPage struct {
	Items []*EvaluationRow
	Total int64
}

// FacetCount is one facet value and its count.
type FacetCount struct {
	Value string
	Count int64
}

// EvaluationFacets groups the invariant evaluation facet counts.
type EvaluationFacets struct {
	ReviewStatus []*FacetCount
	Source       []*FacetCount
	ReviewSource []*FacetCount
	Qualifier    []*FacetCount
	PDFStatus    []*FacetCount
}

// EvaluationSummary is the invariant queue progress for one evaluation context.
type EvaluationSummary struct {
	Total           int64
	Reviewed        int64
	Unreviewed      int64
	PDFAvailable    int64
	PDFNotAvailable int64
	PercentReviewed *float64
	Facets          EvaluationFacets
}

// QueueNavigation is the adjacent unreviewed revision pair for one evaluation queue.
type QueueNavigation struct {
	PreviousWorkRevisionID *int64
	NextWorkRevisionID     *int64
}

// GraphFilter selects the bounded graph article nodes for one run.
type GraphFilter struct {
	RunID        int64
	Query        string
	Source       string
	YearMin      *int64
	YearMax      *int64
	CitationMin  *int64
	CitationMax  *int64
	ReferenceMin *int64
	ReferenceMax *int64
	Author       string
	ORCID        string
	Reference    string
	Limit        int
}

// GraphArticle is one selected article node.
type GraphArticle struct {
	ID     int64
	WorkID int64
	Title  string
	Year   int
	Source string
	DOI    string
}

// GraphAuthorship is one article-author relationship row.
type GraphAuthorship struct {
	WorkRevisionID int64
	AuthorID       int64
	CitationName   string
	ORCID          string
	AuthorOrder    int
	Affiliation    string
}

// GraphCitation is one resolved citation relationship row.
type GraphCitation struct {
	WorkRevisionID int64
	ResolvedWorkID int64
}

// GraphReference is one article-reference relationship row.
type GraphReference struct {
	ID             int64
	WorkRevisionID int64
	DOI            string
	Title          string
	Author         string
	Year           int
	Source         string
}

// Coverage is the work-revision and journal coverage for one run.
type Coverage struct {
	WorkRevisions   int64
	JournalCoverage int64
}

// RelationshipTotals counts canonical works, authorships, references, and resolved citations for a run.
type RelationshipTotals struct {
	WorkRevisions         int64
	AnalysisReadyArticles int64
	Authorships           int64
	ReferenceMentions     int64
	InternalCitations     int64
}

// CorpusReference is one run-scoped reference mention with its citing title.
type CorpusReference struct {
	ID             int64
	WorkRevisionID int64
	MentionOrder   int
	DOI            string
	Title          string
	Author         string
	Year           int
	Source         string
	ResolvedWorkID *int64
	CitingTitle    string
	CreatedAt      string
}

// CorpusReferenceFilter selects one bounded page of run-scoped references.
type CorpusReferenceFilter struct {
	RunID   int64
	Query   string
	Sort    string
	Order   string
	Page    int
	PerPage int
}

// CorpusReferencePage is one bounded page of run-scoped references with its exact total.
type CorpusReferencePage struct {
	Items []*CorpusReference
	Total int64
}

// CorpusAuthor is one run-scoped author occurrence with its article and
// affiliation counts.
type CorpusAuthor struct {
	ID               int64
	CitationName     string
	FirstName        string
	LastName         string
	ORCID            string
	PersonID         *int64
	ArticleCount     int64
	AffiliationCount int64
	CreatedAt        string
}

// CorpusAuthorFilter selects one bounded page of run-scoped authors.
type CorpusAuthorFilter struct {
	RunID   int64
	Query   string
	Sort    string
	Order   string
	Page    int
	PerPage int
}

// CorpusAuthorPage is one bounded page of run-scoped authors with its exact total.
type CorpusAuthorPage struct {
	Items []*CorpusAuthor
	Total int64
}

// RunStageFilter selects one bounded page of run stage outcomes.
type RunStageFilter struct {
	RunID   int64
	Query   string
	Sort    string
	Order   string
	Page    int
	PerPage int
}

// RunStagePage is one bounded page of run stage outcomes with its exact total.
type RunStagePage struct {
	Items []*StageOutcome
	Total int64
}

// StageSummary aggregates one run stage's outcome counts and time bounds.
type StageSummary struct {
	StageName       string
	TotalRecords    int64
	Outcomes        map[string]int64
	FirstRecordedAt string
	LastRecordedAt  string
}
