// Erasable TypeScript contracts for the viewer's JSON API and shared data components.
import type { ClassName } from "../jsx/classes.ts";

/** One identifier accepted by viewer links and API-backed controls. */
export type Identifier = string | number;

/** One primitive query parameter accepted by the endpoint builder. */
export type APIQueryValue = string | number | boolean | null | undefined;

/** Query parameters accepted by the endpoint builder. */
export type APIQuery = Record<string, APIQueryValue>;

/** One JSON object whose endpoint-specific fields are not known statically. */
export interface WireRecord {
  [key: string]: unknown;
}

/** One standard page description returned by table-like endpoints. */
export interface ScopedPagination {
  page: number;
  per_page: number;
  total_rows: number;
  total_pages: number;
  has_next?: boolean;
  sort?: string;
  order?: string;
}

/** One SQLite column exposed by the safe table browser. */
export interface ColumnInfo {
  name: string;
  type: string;
  primary_key: boolean;
}

/** One safely browsable SQLite table and its projected columns. */
export interface TableInfo {
  name: string;
  columns: ColumnInfo[];
  omitted_columns?: Record<string, string>;
  redacted_fields?: string[];
}

/** The discovered table-list response. */
export interface TablesResponse {
  tables: TableInfo[];
}

/** A server response that can be rendered by the shared data table. */
export interface TabularResponse {
  columns?: Array<string | ColumnInfo>;
  schema?: Array<string | ColumnInfo>;
  rows?: WireRecord[];
  items?: WireRecord[];
  table?: TableInfo;
  pagination?: ScopedPagination;
}

/** One page returned by the safe Advanced table browser. */
export interface TableRowsResponse extends TabularResponse {
  table: TableInfo;
  rows: WireRecord[];
  truncated_fields: Record<string, string[]>;
  limits: {
    cell_bytes: number;
    response_value_bytes: number;
  };
  pagination: ScopedPagination;
}

/** Immutable viewer capabilities reported by the health endpoint. */
export interface HealthResponse {
  readable: boolean;
  metadata_readable: boolean;
  table_count: number;
  tables: string[];
  corpus_id: string;
  review_writable: boolean;
  pdf_store_bound: boolean;
  pdf_store_readable: boolean;
  review: {
    available: boolean;
    metadata_writable: boolean;
    pdf_store_bound: boolean;
    pdf_store_readable: boolean;
    pdf_store_read_only: boolean;
  };
}

/** One search summary in hierarchy discovery. */
export interface HierarchySearch {
  id: number;
  search_id: string;
  created_at: string;
  revision_count: number;
  plan_count: number;
  run_count: number;
  latest_run_id: number | null;
  latest_plan_id: number | null;
  latest_revision_id: number | null;
}

/** One immutable search revision in hierarchy discovery. */
export interface HierarchyRevision {
  id: number;
  label: string;
  revision_label?: string;
  search_id?: string;
  created_at: string;
  plan_count: number;
  run_count: number;
  latest_run_id: number | null;
  latest_plan_id: number | null;
}

/** One execution plan in hierarchy discovery. */
export interface HierarchyPlan {
  id: number;
  search_revision_id: number;
  execution_fingerprint: string;
  resolved_manifest_hash?: string;
  input_manifest_hash?: string;
  enrichment_enabled: boolean | number;
  created_at: string;
}

/** One pipeline attempt in hierarchy discovery. */
export interface HierarchyAttempt {
  id: number;
  execution_plan_id: number;
  attempt_number: number | null;
  started_at: string;
  finished_at: string | null;
  status: string;
  visibility_state: string;
}

/** One run with complete ancestry in hierarchy discovery. */
export interface HierarchyRun {
  id: number;
  attempt_number: number | null;
  started_at: string;
  finished_at: string | null;
  status: string;
  visibility_state: string;
  search_id: number | null;
  search_name: string;
  search_revision_id: number | null;
  revision_label: string;
  execution_plan_id: number | null;
  summary?: string | null;
}

/** Any item returned by one hierarchy collection. */
export type HierarchyItem = HierarchySearch | HierarchyRevision | HierarchyPlan | HierarchyAttempt | HierarchyRun;

/** The hierarchy workspace totals. */
export interface HierarchySummaryResponse {
  version: "1";
  totals: {
    searches: number;
    revisions: number;
    plans: number;
    runs: number;
    completed_runs: number;
  };
  latest_run: HierarchyRun | null;
}

/** One endpoint-bound hierarchy collection page. */
export interface HierarchyPage<T extends HierarchyItem> {
  version: "1";
  items: T[];
  has_more: boolean;
  next_cursor: string;
  limit: number;
  selected_item?: T;
}

/** The exact ancestry and lifecycle context for a selected run. */
export interface RunContextResponse {
  search: {
    id: number;
    search_id: string;
    created_at: string;
  };
  revision: {
    id: number;
    search_id: number;
    label: string;
    config_artifact_hash: string;
    resolved_manifest_hash: string;
    created_at: string;
    updated_at?: string | null;
  };
  plan: {
    id: number;
    search_revision_id: number;
    execution_fingerprint: string;
    resolved_manifest_hash: string;
    input_manifest_hash: string;
    enrichment_enabled: boolean;
    created_at: string;
  };
  run: {
    id: number;
    execution_plan_id: number;
    step: string;
    started_at: string;
    finished_at: string | null;
    status: string;
    summary: string | null;
    attempt_number: number;
    visibility_state: string;
    trashed_at: string | null;
    trash_reason: string | null;
  };
  lifecycle: {
    status: string;
    visibility_state: string;
    review_writable: boolean;
  };
  review: {
    initialized: boolean;
    context_id: number | null;
    run_writable: boolean;
  };
}

/** The response returned after a run visibility change. */
export interface RunVisibilityResponse {
  run_id: number;
  visibility_state: string;
  changed: boolean;
}

/** One recorded or unavailable metric. */
export interface MetricEvidence {
  available: boolean;
  state: string;
  value?: number;
  source?: string;
  metric?: string;
  denominator?: number;
  percentage?: number | string;
  basis?: string;
  unit?: string;
}

/** A metric value accepted by presentation helpers and unit fixtures. */
export type MetricValue = number | Partial<MetricEvidence>;

/** One configured source result-count comparison. */
export interface SourceResultCount {
  id?: number;
  source_name: string;
  source_type?: string;
  expected_file?: string;
  query?: string;
  expected_result_count: number | null;
  observed_result_count: number | null;
  result_count_comparison: string | null;
  export_date: string | null;
}

/** One recorded source-filter stage count. */
export interface SourceFilterCount {
  source: string;
  filters: string[];
  count: number;
  state: string;
}

/** One invalid or incomplete source-filter diagnostic. */
export interface SourceFilterDiagnostic {
  source: string;
  state: string;
  code: string;
  message: string;
  stage_index?: number;
}

/** The complete overview evidence for one run. */
export interface OverviewResponse {
  run_id: number;
  captured_metrics: MetricEvidence[];
  retention_funnel: Record<string, MetricEvidence>;
  source_breakdown: Record<string, MetricEvidence>;
  source_result_counts: SourceResultCount[];
  source_filter_counts: SourceFilterCount[];
  source_filter_diagnostics: SourceFilterDiagnostic[];
  validation_breakdown: Record<string, MetricEvidence>;
  cache_breakdown: Record<string, MetricEvidence>;
  enrichment_breakdown: Record<string, MetricEvidence>;
  enrichment_field_breakdown: Record<string, MetricEvidence>;
  enrichment_provider_breakdown: Record<string, MetricEvidence>;
  normalization_breakdown: Record<string, MetricEvidence>;
  normalization_field_breakdown: Record<string, Record<string, MetricEvidence>>;
  current_coverage: Record<string, MetricEvidence>;
  relationship_totals: Record<string, MetricEvidence>;
}

/** One search-term match summary attached to an article. */
export interface TermMatchSummary extends WireRecord {
  title?: string[];
  abstract?: string[];
  keywords?: string[];
  keywords_plus?: string[];
  terms_with_sources?: Record<string, string[]>;
  term_total?: number;
  matched_total?: number;
}

/** One corpus row with the fields shared by the rendered collections. */
export interface CorpusRow extends WireRecord {
  id?: number;
  work_id?: number;
  work_revision_id?: number;
  title?: string | null;
  doi?: string | null;
  year?: number | string | null;
  source?: string | null;
  citation_name?: string | null;
  author?: string | null;
  citing_title?: string | null;
  status?: string | null;
  error_message?: string | null;
  term_matches?: TermMatchSummary | null;
}

/** One server-backed corpus collection page. */
export interface CorpusResponse extends TabularResponse {
  run_id: number;
  collection: string;
  columns: string[];
  rows: CorpusRow[];
  pagination: ScopedPagination;
  stats?: WireRecord;
  source_result_counts?: SourceResultCount[];
}

/** One author identity-resolution row in the corpus evidence view. */
export interface IdentityEvidenceRow extends CorpusRow {
  resolution_id?: number;
  author_occurrence_id?: number;
  provider?: string;
  outcome?: string;
  candidate_count?: number;
  queried_citation_name?: string;
  article_title?: string | null;
  observed_orcid?: string | null;
  candidates?: IdentityCandidate[];
}

/** The identity-evidence collection for one run. */
export interface IdentityEvidenceResponse extends CorpusResponse {
  rows: IdentityEvidenceRow[];
  stats: {
    resolutions: number;
    unclear: number;
    no_candidate: number;
    provider_failed: number;
    candidates: number;
  };
}

/** One evaluation facet value and count. */
export interface EvaluationFacet {
  value: string;
  count: number;
}

/** One article in the evaluation queue. */
export interface EvaluationRow extends WireRecord {
  work_id: number;
  work_revision_id: number;
  title: string | null;
  doi: string | null;
  source: string | null;
  review_status: string;
  review_inherited: boolean;
  review_version_id: number | null;
  review_created_in_context_id: number | null;
  review_sub_statuses: string[];
  inventory_status: string;
  inventoried_at: string | null;
}

/** Aggregate progress and filter facets for an evaluation queue. */
export interface EvaluationSummary {
  total: number;
  reviewed: number;
  unreviewed: number;
  pdf_available: number;
  pdf_not_available: number;
  percent_reviewed: number | null;
  facets: {
    review_status: EvaluationFacet[];
    source: EvaluationFacet[];
    review_source: EvaluationFacet[];
    qualifier: EvaluationFacet[];
    pdf_status: EvaluationFacet[];
  };
}

/** Adjacent unreviewed records within the active evaluation queue. */
export interface EvaluationNavigation {
  previous_work_revision_id: number | null;
  next_work_revision_id: number | null;
}

/** The complete evaluation queue response for one run. */
export interface EvaluationResponse extends TabularResponse {
  run_id: number;
  review_context_initialized: boolean;
  review_context: ReviewContext | null;
  review_summary: EvaluationSummary;
  queue_navigation: EvaluationNavigation;
  proposed_parent: ProposedParent | null;
  run_writable: boolean;
  columns: string[];
  rows: EvaluationRow[];
  pagination: ScopedPagination;
}

/** One raw graph node plus optional client layout state. */
export interface GraphNode {
  id: string;
  type: string;
  label: string;
  doi?: string | null;
  orcid?: string | null;
  author?: string | null;
  year?: number | string | null;
  revision_id?: number;
  work_id?: number;
  author_id?: number;
  reference_id?: number;
  identity_status?: string;
  cluster?: number;
  degree?: number;
  radius?: number;
  index?: number;
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
  fx?: number | null;
  fy?: number | null;
}

/** One graph relationship plus optional resolved node references. */
export interface GraphEdge extends WireRecord {
  id: string;
  source: string | number | GraphNode;
  target: string | number | GraphNode;
  type: string;
  label?: string;
  author_order?: number;
  affiliation?: string | null;
  shared_reference_count?: number;
  derived?: boolean;
  count?: number;
  index?: number;
}

/** One connected-cluster summary computed by the graph component. */
export interface ClusterSummary {
  id: number;
  size: number;
}

/** The bounded relationship graph response. */
export interface GraphResponse {
  nodes: GraphNode[];
  edges: GraphEdge[];
  filters: Record<string, string | number>;
  truncated: boolean;
  limits: Record<string, number>;
  counts: {
    article_matches: number;
    article_rendered: number;
    nodes_rendered: number;
    edges_rendered: number;
    node_types: Record<string, number>;
    edge_types: Record<string, number>;
  };
  truncation_reason: string;
}

/** One append-only audit event returned by the server. */
export interface AuditEvent extends WireRecord {
  id: number;
  action: string;
  entity_type: string;
  entity_id: Identifier | null;
  occurred_at: string;
  created_at?: string;
  actor: string;
  metadata_json: WireRecord | string | null;
  before_json: WireRecord | string | null;
  after_json: WireRecord | string | null;
  correlation_id: string | null;
  pipeline_run_id: number | null;
  recorded_data_available: boolean;
  recorded_data_truncated_fields: string[];
}

/** An audit event accepted by reusable presentation components and partial fixtures. */
export type AuditEventRecord = Partial<AuditEvent> & WireRecord;

/** One audit facet and its recorded count. */
export interface AuditFacet {
  value: string;
  count: number;
}

/** Aggregate audit-event counts for the active filters. */
export interface AuditSummary {
  total_events: number;
  actions: AuditFacet[];
}

/** The bounded audit timeline response. */
export interface AuditResponse {
  events: AuditEvent[];
  has_more: boolean;
  next_cursor: number | null;
  summary: AuditSummary | null;
  facets: {
    actors: string[] | AuditFacet[];
    actions: string[] | AuditFacet[];
    entity_types: string[] | AuditFacet[];
  } | null;
  scope: {
    run_id: string | null;
    pdf_scope: string;
  };
}

/** Explicitly requested recorded JSON for one audit event. */
export interface AuditRecordedData {
  event_id: number;
  byte_limit: number;
  metadata?: WireRecord | null;
  before?: WireRecord | null;
  after?: WireRecord | null;
  truncated_fields?: string[];
}

/** One artifact's immutable identity and run relationships. */
export interface ArtifactRecord extends WireRecord {
  id: number;
  content_hash: string;
  byte_size: number;
  content_type: string;
  created_at: string;
  has_blob: number;
  preview_available: boolean;
  artifact_roles?: string;
  relationship_roles?: string;
  produced_by_steps?: string;
  consumed_by_steps?: string;
}

/** The run ancestry displayed above artifact evidence. */
export interface ArtifactContext {
  run_id: number;
  attempt_number: number | null;
  search_id: string | null;
  search_revision_id: number | null;
  search_revision_label: string | null;
  execution_plan_id: number | null;
  execution_fingerprint: string | null;
}

/** A bounded run artifact collection. */
export interface ArtifactsResponse {
  run_id: number;
  context: ArtifactContext;
  artifacts: ArtifactRecord[];
  has_more: boolean;
  next_cursor: string | null;
  limit: number;
  filters: {
    q: string;
    role: string;
    artifact_id: number | null;
  };
  pagination?: ScopedPagination;
}

/** A bounded text preview for one artifact. */
export interface ArtifactInspectionResponse {
  artifact_id: number;
  content_type: string;
  byte_size: number;
  stored_byte_size: number;
  preview_byte_size: number;
  preview_limit_bytes: number;
  truncated: boolean;
  format: "text" | "json";
  content: string;
}

/** One cache-use row returned for a run. */
export interface CacheUse extends WireRecord {
  id: number;
  cache_layer: string;
  outcome: string;
  used_at: string;
  cache_entry_id: number;
  provider: string;
  namespace: string;
  request_fingerprint: string;
  response_status: number | null;
  payload_artifact_id: number | null;
  fetched_at: string;
  expires_at: string | null;
  extractor_version: string;
}

/** One page of cache evidence for a run. */
export interface CacheUsesResponse extends TabularResponse {
  run_id: number;
  columns: string[];
  rows: CacheUse[];
  cache_uses: CacheUse[];
  pagination: ScopedPagination;
}

/** One per-work stage outcome. */
export interface StageOutcome extends WireRecord {
  id: number;
  work_id: number;
  stage_name: string;
  outcome: string;
  reason: string | null;
  created_at: string;
  updated_at: string;
}

/** Aggregate outcomes recorded for one pipeline stage. */
export interface StageSummary {
  stage_name: string;
  total_records: number;
  outcomes: Record<string, number>;
  first_recorded_at: string;
  last_recorded_at: string;
}

/** One run-level execution step and its measured duration. */
export interface RunStep {
  step_name: string;
  step_status: string;
  input_artifact_id: number | null;
  output_artifact_id: number | null;
  input_fingerprint: string;
  output_fingerprint: string;
  started_at: string | null;
  finished_at: string | null;
  duration_seconds: number | null;
}

/** Stage progression and per-work outcome evidence for one run. */
export interface StagesResponse extends TabularResponse {
  run_id: number;
  columns: string[];
  rows: StageOutcome[];
  pagination: ScopedPagination;
  stage_summaries: StageSummary[];
  run_steps: RunStep[];
}

/** One immutable review context for a completed run. */
export interface ReviewContext {
  id: number;
  pipeline_run_id: number;
  parent_context_id?: number | null;
  created_at: string;
}

/** One eligible parent review context. */
export interface ProposedParent {
  context_id: number;
  pipeline_run_id: number;
  search_id: string;
  search_revision: string;
  execution_plan_id: number;
  attempt_number: number;
  started_at: string;
  inherited_work_count: number;
}

/** Review-context state and deterministic parent proposal for one run. */
export interface ReviewContextResponse {
  run_id: number;
  context_initialized: boolean;
  context: ReviewContext | null;
  run_writable: boolean;
  proposed_parent?: ProposedParent | null;
}

/** One page of eligible review-context candidates. */
export interface ReviewContextCandidatesResponse {
  items: ProposedParent[];
  rows: ProposedParent[];
  limit: number;
  has_more: boolean;
  next_cursor: string | null;
}

/** The response returned when a review context is initialized. */
export interface ReviewContextMutationResponse {
  context_initialized: true;
  context: ReviewContext;
}

/** One immutable complete article-review version. */
export interface WorkReviewVersion {
  id: number;
  work_id: number;
  work_revision_id: number;
  created_in_context_id: number;
  parent_version_id?: number;
  status: string;
  sub_statuses: string[];
  reason: string | null;
  reason_truncated: boolean;
  created_at: string;
  reviewer_display: string;
}

/** The current review state inherited or created in one context. */
export interface WorkReviewState {
  context_id: number;
  work_id: number;
  work_revision_id: number;
  version?: WorkReviewVersion;
  inherited_from_context_id?: number;
}

/** The article review panel response. */
export interface ArticleReviewResponse {
  run_id: number;
  work_id: number;
  work_revision_id: number;
  context_initialized: boolean;
  context?: ReviewContext;
  editable: boolean;
  editability: {
    decision: boolean;
    notes: boolean;
    anchors: boolean;
  };
  review?: WorkReviewState;
  state: {
    status: string;
    sub_statuses: string[];
    reason: string | null;
    version: WorkReviewVersion | null;
    inherited_from_context_id?: number;
  };
  pdf_status: PDFStatus;
  summary_counts?: Record<string, number>;
}

/** The response returned after saving one work review. */
export interface WorkReviewMutationResponse {
  review: WorkReviewState;
  changed: boolean;
}

/** One immutable review decision history page. */
export interface WorkReviewVersionsResponse {
  items: WorkReviewVersion[];
  versions: WorkReviewVersion[];
  limit: number;
  has_more: boolean;
  next_cursor: string | null;
}

/** One immutable review decision version response. */
export interface WorkReviewVersionResponse {
  version: WorkReviewVersion;
}

/** One context-sensitive link stored with a note version. */
export interface ResolvedNoteLink {
  ordinal: number;
  target_type: string;
  raw_target: string;
  display_text?: string | null;
  utf16_position?: number;
  utf16_length?: number;
  resolved: boolean;
  work_revision_id?: number;
  note_id?: number;
  anchor_id?: string;
  page?: number;
  url?: string;
}

/** One immutable active note snapshot or deletion tombstone. */
export interface ReviewNoteVersion {
  id: number;
  note_id: number;
  parent_version_id?: number;
  created_in_context_id: number;
  state: string;
  body: string | null;
  body_bytes: number;
  body_truncated: boolean;
  title: string;
  excerpt: string;
  link_count: number;
  created_at: string;
  reviewer_display: string;
  links: ResolvedNoteLink[];
}

/** One logical review note and its selected context head. */
export interface ReviewNote {
  id: number;
  work_id: number;
  work_revision_id: number;
  created_at: string;
  version: ReviewNoteVersion;
  inherited_from_context_id?: number;
}

/** One rectangle normalized to a PDF page. */
export interface AnchorRectangle {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** One immutable active PDF anchor snapshot or deletion tombstone. */
export interface ReviewAnchorVersion {
  id: number;
  anchor_id: string;
  parent_version_id?: number;
  created_in_context_id: number;
  work_revision_id: number;
  pdf_content_hash: string;
  state: string;
  page?: number;
  selected_text?: string;
  selected_text_truncated: boolean;
  rectangles?: AnchorRectangle[];
  created_at: string;
  reviewer_display: string;
}

/** One stable review anchor and its selected context head. */
export interface ReviewAnchor {
  id: string;
  label: string;
  work_id: number;
  created_at: string;
  version: ReviewAnchorVersion;
  inherited_from_context_id?: number;
}

/** One cursor page from a review collection. */
export type ReviewCollectionPage<T, K extends string> = {
  items: T[];
  limit: number;
  has_more: boolean;
  next_cursor: string | null;
} & { [P in K]: T[] };

/** One note detail response. */
export interface ReviewNoteResponse {
  run_id: number;
  note: ReviewNote;
}

/** One note version detail response. */
export interface ReviewNoteVersionResponse {
  run_id: number;
  version: ReviewNoteVersion;
}

/** One note collection page. */
export type ReviewNotesResponse = ReviewCollectionPage<ReviewNote, "notes">;

/** One note history page. */
export type ReviewNoteVersionsResponse = ReviewCollectionPage<ReviewNoteVersion, "versions"> & { run_id: number };

/** The response returned after creating a note. */
export interface ReviewNoteCreateResponse {
  note: ReviewNote;
}

/** The response returned after changing a note head. */
export interface ReviewNoteMutationResponse {
  note: ReviewNote;
  changed: boolean;
}

/** One anchor collection page. */
export type ReviewAnchorsResponse = ReviewCollectionPage<ReviewAnchor, "anchors">;

/** One anchor history page. */
export type ReviewAnchorVersionsResponse = ReviewCollectionPage<ReviewAnchorVersion, "versions"> & {
  run_id: number;
  anchor: ReviewAnchor;
};

/** One anchor version detail response. */
export interface ReviewAnchorVersionResponse {
  run_id: number;
  version: ReviewAnchorVersion;
}

/** The response returned after creating an anchor. */
export interface ReviewAnchorCreateResponse {
  anchor: ReviewAnchor;
}

/** The response returned after changing an anchor head. */
export interface ReviewAnchorMutationResponse {
  anchor: ReviewAnchor;
  changed: boolean;
}

/** One backlink collection page. */
export type ReviewBacklinksResponse = ReviewCollectionPage<ReviewNote, "backlinks">;

/** One PDF inventory state attached to an article. */
export interface PDFStatus {
  status: string;
  content_hash?: string | null;
  byte_size?: number | null;
  created_at?: string | null;
  inventoried_at?: string | null;
}

/** One detail collection with endpoint-specific cursor pagination. */
export interface DetailCollectionPage<T> {
  items: T[];
  total: number;
  limit: number;
  has_more: boolean;
  next_cursor: string | null;
}

/** One normalized article revision displayed by the detail view. */
export interface ArticleRecord extends WireRecord {
  id: number;
  work_id: number;
  pipeline_run_id: number;
  title: string | null;
  doi: string | null;
  year: number | string | null;
  journal: string | null;
  source: string | null;
  publisher: string | null;
  abstract: string | null;
  keywords: string | string[] | null;
  keywords_plus: string | string[] | null;
  citation_count: number | null;
  reference_count: number | null;
  validation_status: string | null;
  created_at: string;
  payload_hash: string | null;
  producer_stage: string | null;
  extension_data: WireRecord | string | null;
}

/** One author occurrence displayed by the detail view. */
export interface AuthorRecord extends WireRecord {
  id: number;
  citation_name: string;
  first_name: string | null;
  last_name: string | null;
  orcid: string | null;
  person_id: number | null;
  person_orcid: string | null;
  created_at: string;
}

/** One reference mention displayed by the detail view. */
export interface ReferenceRecord extends WireRecord {
  id: number;
  work_revision_id: number;
  citing_title: string | null;
  pipeline_run_id: number;
  mention_order: number;
  doi: string | null;
  title: string | null;
  author: string | null;
  year: number | string | null;
  source: string | null;
  resolved_revision_id: number | null;
  resolved_title: string | null;
}

/** One enrichment provider and field summary attached to article detail. */
export interface EnrichmentSummary {
  providers: string[];
  fields: string[];
  truncated: boolean;
  pair_limit: number;
}

/** Complete article detail and its bounded related collections. */
export interface ArticleDetailResponse {
  article: ArticleRecord;
  authors: DetailCollectionPage<AuthorRecord>;
  references: DetailCollectionPage<ReferenceRecord>;
  stage_outcomes: DetailCollectionPage<StageOutcome>;
  audit_events: DetailCollectionPage<AuditEvent>;
  enrichment_summary: EnrichmentSummary;
  pdf_status: PDFStatus;
  review_context: ReviewContext | null;
  review_context_initialized: boolean;
  term_matches: TermMatchSummary | null;
}

/** One identity-resolution record attached to an author occurrence. */
export interface IdentityResolution extends WireRecord {
  id?: number;
  resolution_id?: number;
  author_occurrence_id?: number;
  provider: string;
  status: string;
  candidate_count?: number;
  candidate_preview_limit?: number;
  candidates_truncated?: boolean;
  candidates?: IdentityCandidate[];
  queried_citation_name?: string;
  observed_orcid?: string | null;
  article_title?: string | null;
  doi?: string | null;
  error_message?: string | null;
  resolved_at?: string;
}

/** Complete author detail and its bounded related collections. */
export interface AuthorDetailResponse {
  author: AuthorRecord;
  articles: DetailCollectionPage<ArticleRecord>;
  audit_events: DetailCollectionPage<AuditEvent>;
  identity_evidence: DetailCollectionPage<IdentityResolution>;
}

/** Complete reference-mention detail. */
export interface ReferenceDetailResponse {
  reference: ReferenceRecord;
}

/** One identity candidate returned for explicit investigation. */
export interface IdentityCandidate extends WireRecord {
  id?: number;
  candidate_orcid?: string | null;
  provider_display_name?: string | null;
  provider_rank?: number | null;
  query_url?: string;
  payload_artifact_id?: number | null;
  created_at?: string;
}

/** One page of identity candidates for a resolution. */
export interface IdentityCandidatesResponse extends DetailCollectionPage<IdentityCandidate> {}

/** Shared pagination presentation options. */
export interface PaginationOptions {
  page?: number | string;
  perPage?: number | string;
  itemLabel?: string;
  pageAttribute?: `data-${string}`;
  pageClass?: ClassName;
  visibleCount?: number;
  secondary?: string;
}

/** Shared data-table rendering options. */
export interface DataTableContext {
  columnsWhitelist?: string[];
  pageKey?: string;
  perPageKey?: string;
  sortKey?: string;
  orderKey?: string;
  queryKey?: string;
  expandedKey?: string;
  query?: string;
  page?: number;
  perPage?: number;
  sortFields?: string[];
  expandableFields?: Array<{ f: string; w: number | string; label?: string; render?: (row: WireRecord) => JSX.Element }>;
  rowKey?: string;
  columnConfig?: Record<string, { label?: string; className?: ClassName; render?: (row: WireRecord, value: unknown) => JSX.Element }>;
  expandLongCells?: boolean;
  tableClasses?: readonly ClassName[];
  itemLabel?: string;
  perPageSelector?: string;
  querySelector?: string;
  searchButtonSelector?: string;
  clearButtonSelector?: string;
}
