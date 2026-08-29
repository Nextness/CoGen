// api-shapes.spec.ts validates every consumed viewer endpoint against the
// typed payload contracts in src/api/types.ts. The interfaces are erasable
// TypeScript, so this spec is the runtime guard that catches server drift:
// wrong nullability, missing fields, and renamed keys. It runs in the serial
// mutation suite because it initializes the review context on its fixture copy.

import { test, expect } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';

test.describe.configure({ mode: 'serial' });

// ── Fixture identifiers (see viewer.spec.ts) ─────────────────────────
const RUN = '1';
const ARTICLE = '1';
const AUTHOR = '1';
const REFERENCE = '1';
const ARTIFACT = '1';
const RESOLUTION = '1';
const TABLE = 'work_revisions';

// ── Runtime shape assertions ──────────────────────────────────────────

/** Fails the current test with a precise shape-mismatch message. */
function mismatch(path: string, expected: string, actual: unknown): never {
  throw new Error(`shape mismatch at ${path}: expected ${expected}, got ${JSON.stringify(actual)}`);
}

/** Asserts one value is a non-array object. */
function expectObject(value: unknown, path: string): asserts value is Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) mismatch(path, 'object', value);
}

/** Asserts one value is an array. */
function expectArray(value: unknown, path: string): asserts value is unknown[] {
  if (!Array.isArray(value)) mismatch(path, 'array', value);
}

/** Asserts one value is a string. */
function expectString(value: unknown, path: string): asserts value is string {
  if (typeof value !== 'string') mismatch(path, 'string', value);
}

/** Asserts one value is a finite number. */
function expectNumber(value: unknown, path: string): asserts value is number {
  if (typeof value !== 'number' || !Number.isFinite(value)) mismatch(path, 'number', value);
}

/** Asserts one value is a boolean. */
function expectBoolean(value: unknown, path: string): asserts value is boolean {
  if (typeof value !== 'boolean') mismatch(path, 'boolean', value);
}

/** Asserts one value is a string or null. */
function expectNullableString(value: unknown, path: string): void {
  if (value !== null) expectString(value, path);
}

/** Asserts one value is a finite number or null. */
function expectNullableNumber(value: unknown, path: string): void {
  if (value !== null) expectNumber(value, path);
}

/** Asserts one value is an array of strings. */
function expectStringArray(value: unknown, path: string): asserts value is string[] {
  expectArray(value, path);
  value.forEach((item, index) => expectString(item, `${path}[${index}]`));
}

/** Asserts one value is an array of objects. */
function expectObjectArray(value: unknown, path: string): asserts value is Record<string, unknown>[] {
  expectArray(value, path);
  value.forEach((item, index) => expectObject(item, `${path}[${index}]`));
}

/** Asserts one value is an object whose entries all satisfy one check. */
function expectRecordOf(value: unknown, path: string, itemCheck: (item: unknown, itemPath: string) => void): void {
  expectObject(value, path);
  for (const [key, item] of Object.entries(value)) itemCheck(item, `${path}.${key}`);
}

/** Asserts one value is a string-keyed record of numbers. */
function expectNumberRecord(value: unknown, path: string): void {
  expectRecordOf(value, path, (item, itemPath) => expectNumber(item, itemPath));
}

/** Asserts one value is a string-keyed record of metric evidence. */
function expectMetricRecord(value: unknown, path: string): void {
  expectRecordOf(value, path, (item, itemPath) => expectMetricEvidence(item, itemPath));
}

/** Asserts one metric evidence value. */
function expectMetricEvidence(value: unknown, path: string): void {
  expectObject(value, path);
  expectBoolean(value.available, `${path}.available`);
  expectString(value.state, `${path}.state`);
  if (value.value !== undefined) expectNumber(value.value, `${path}.value`);
  if (value.denominator !== undefined) expectNumber(value.denominator, `${path}.denominator`);
  if (value.percentage !== undefined) expectNumber(value.percentage, `${path}.percentage`);
  if (value.source !== undefined) expectString(value.source, `${path}.source`);
  if (value.metric !== undefined) expectString(value.metric, `${path}.metric`);
}

/** Asserts one scoped pagination envelope. */
function expectPagination(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.page, `${path}.page`);
  expectNumber(value.per_page, `${path}.per_page`);
  expectNumber(value.total_rows, `${path}.total_rows`);
  expectNumber(value.total_pages, `${path}.total_pages`);
  if (value.sort !== undefined) expectString(value.sort, `${path}.sort`);
  if (value.order !== undefined) expectString(value.order, `${path}.order`);
}

/** Asserts one hierarchy run item. */
function expectHierarchyRun(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.id, `${path}.id`);
  expectNullableNumber(value.attempt_number, `${path}.attempt_number`);
  expectString(value.started_at, `${path}.started_at`);
  expectNullableString(value.finished_at, `${path}.finished_at`);
  expectString(value.status, `${path}.status`);
  expectString(value.visibility_state, `${path}.visibility_state`);
  expectNullableNumber(value.search_id, `${path}.search_id`);
  expectString(value.search_name, `${path}.search_name`);
  expectNullableNumber(value.search_revision_id, `${path}.search_revision_id`);
  expectString(value.revision_label, `${path}.revision_label`);
  expectNullableNumber(value.execution_plan_id, `${path}.execution_plan_id`);
}

/** Asserts one review context record. */
function expectReviewContext(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.id, `${path}.id`);
  expectNumber(value.pipeline_run_id, `${path}.pipeline_run_id`);
  if (value.parent_context_id !== undefined) expectNullableNumber(value.parent_context_id, `${path}.parent_context_id`);
  expectString(value.created_at, `${path}.created_at`);
}

/** Asserts one PDF status payload. */
function expectPDFStatus(value: unknown, path: string): void {
  expectObject(value, path);
  expectString(value.status, `${path}.status`);
  if (value.work_id !== undefined) expectNumber(value.work_id, `${path}.work_id`);
  if (value.doi !== undefined) expectString(value.doi, `${path}.doi`);
  if (value.eligible !== undefined) expectBoolean(value.eligible, `${path}.eligible`);
  if (value.store_bound !== undefined) expectBoolean(value.store_bound, `${path}.store_bound`);
  if (value.content_hash !== undefined) expectNullableString(value.content_hash, `${path}.content_hash`);
  if (value.inventoried_at !== undefined) expectNullableString(value.inventoried_at, `${path}.inventoried_at`);
  if (value.byte_size !== undefined) expectNullableNumber(value.byte_size, `${path}.byte_size`);
}

/** Asserts one detail collection page envelope. */
function expectDetailCollection(value: unknown, path: string): asserts value is { items: Record<string, unknown>[]; total: number; limit: number; has_more: boolean; next_cursor: string | null } {
  expectObject(value, path);
  expectObjectArray(value.items, `${path}.items`);
  expectNumber(value.total, `${path}.total`);
  expectNumber(value.limit, `${path}.limit`);
  expectBoolean(value.has_more, `${path}.has_more`);
  expectNullableString(value.next_cursor, `${path}.next_cursor`);
}

/** Asserts one review collection page envelope. */
function expectReviewCollection(value: unknown, path: string): asserts value is { items: Record<string, unknown>[]; limit: number; has_more: boolean; next_cursor: string | null; [key: string]: unknown } {
  expectObject(value, path);
  expectObjectArray(value.items, `${path}.items`);
  expectNumber(value.limit, `${path}.limit`);
  expectBoolean(value.has_more, `${path}.has_more`);
  expectNullableString(value.next_cursor, `${path}.next_cursor`);
}

/** Asserts one review note version. */
function expectNoteVersion(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.id, `${path}.id`);
  expectNumber(value.note_id, `${path}.note_id`);
  expectNumber(value.created_in_context_id, `${path}.created_in_context_id`);
  expectString(value.state, `${path}.state`);
  expectNullableString(value.body, `${path}.body`);
  expectNumber(value.body_bytes, `${path}.body_bytes`);
  expectBoolean(value.body_truncated, `${path}.body_truncated`);
  expectString(value.title, `${path}.title`);
  expectString(value.excerpt, `${path}.excerpt`);
  expectNumber(value.link_count, `${path}.link_count`);
  expectString(value.created_at, `${path}.created_at`);
  expectString(value.reviewer_display, `${path}.reviewer_display`);
  expectArray(value.links, `${path}.links`);
}

/** Asserts one review note head. */
function expectNote(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.id, `${path}.id`);
  expectNumber(value.work_id, `${path}.work_id`);
  expectNumber(value.work_revision_id, `${path}.work_revision_id`);
  expectString(value.created_at, `${path}.created_at`);
  expectNoteVersion(value.version, `${path}.version`);
}

/** Asserts one review anchor version. */
function expectAnchorVersion(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.id, `${path}.id`);
  expectString(value.anchor_id, `${path}.anchor_id`);
  expectNumber(value.created_in_context_id, `${path}.created_in_context_id`);
  expectNumber(value.work_revision_id, `${path}.work_revision_id`);
  expectString(value.pdf_content_hash, `${path}.pdf_content_hash`);
  expectString(value.state, `${path}.state`);
  expectBoolean(value.selected_text_truncated, `${path}.selected_text_truncated`);
  expectString(value.created_at, `${path}.created_at`);
  expectString(value.reviewer_display, `${path}.reviewer_display`);
}

/** Asserts one review anchor head. */
function expectAnchor(value: unknown, path: string): void {
  expectObject(value, path);
  expectString(value.id, `${path}.id`);
  expectString(value.label, `${path}.label`);
  expectNumber(value.work_id, `${path}.work_id`);
  expectString(value.created_at, `${path}.created_at`);
  expectAnchorVersion(value.version, `${path}.version`);
}

/** Asserts one artifact record. */
function expectArtifact(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.id, `${path}.id`);
  expectString(value.content_hash, `${path}.content_hash`);
  expectNumber(value.byte_size, `${path}.byte_size`);
  expectString(value.content_type, `${path}.content_type`);
  expectString(value.created_at, `${path}.created_at`);
  expectNumber(value.has_blob, `${path}.has_blob`);
  expectBoolean(value.preview_available, `${path}.preview_available`);
  expectNumber(value.preview_limit_bytes, `${path}.preview_limit_bytes`);
  expectString(value.artifact_roles, `${path}.artifact_roles`);
  expectString(value.relationship_roles, `${path}.relationship_roles`);
  expectString(value.produced_by_steps, `${path}.produced_by_steps`);
  expectString(value.consumed_by_steps, `${path}.consumed_by_steps`);
}

/** Asserts one audit event. */
function expectAuditEvent(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.id, `${path}.id`);
  expectString(value.occurred_at, `${path}.occurred_at`);
  expectString(value.actor, `${path}.actor`);
  expectNullableNumber(value.pipeline_run_id, `${path}.pipeline_run_id`);
  expectString(value.entity_type, `${path}.entity_type`);
  expectNullableString(value.entity_id, `${path}.entity_id`);
  expectString(value.action, `${path}.action`);
  expectNullableString(value.before_json, `${path}.before_json`);
  expectNullableString(value.after_json, `${path}.after_json`);
  expectNullableString(value.metadata_json, `${path}.metadata_json`);
  expectNullableString(value.correlation_id, `${path}.correlation_id`);
  expectBoolean(value.recorded_data_available, `${path}.recorded_data_available`);
  expectStringArray(value.recorded_data_truncated_fields, `${path}.recorded_data_truncated_fields`);
}

/** Asserts one stage outcome row. */
function expectStageOutcome(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.id, `${path}.id`);
  expectNumber(value.work_id, `${path}.work_id`);
  expectString(value.stage_name, `${path}.stage_name`);
  expectString(value.outcome, `${path}.outcome`);
  expectNullableString(value.reason, `${path}.reason`);
  expectString(value.created_at, `${path}.created_at`);
  expectString(value.updated_at, `${path}.updated_at`);
}

/** Asserts one run step. */
function expectRunStep(value: unknown, path: string): void {
  expectObject(value, path);
  expectString(value.step_name, `${path}.step_name`);
  expectString(value.step_status, `${path}.step_status`);
  expectNullableNumber(value.input_artifact_id, `${path}.input_artifact_id`);
  expectNullableNumber(value.output_artifact_id, `${path}.output_artifact_id`);
  expectNullableString(value.started_at, `${path}.started_at`);
  expectNullableString(value.finished_at, `${path}.finished_at`);
  expectString(value.input_fingerprint, `${path}.input_fingerprint`);
  expectString(value.output_fingerprint, `${path}.output_fingerprint`);
  expectNullableNumber(value.duration_seconds, `${path}.duration_seconds`);
}

/** Asserts one identity resolution row. */
function expectIdentityResolution(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.resolution_id, `${path}.resolution_id`);
  expectString(value.status, `${path}.status`);
  expectString(value.provider, `${path}.provider`);
  expectString(value.queried_citation_name, `${path}.queried_citation_name`);
  expectNullableString(value.error_message, `${path}.error_message`);
  expectString(value.resolved_at, `${path}.resolved_at`);
  expectNumber(value.author_occurrence_id, `${path}.author_occurrence_id`);
  expectNullableString(value.observed_orcid, `${path}.observed_orcid`);
  expectNullableNumber(value.person_id, `${path}.person_id`);
  expectString(value.article_title, `${path}.article_title`);
  expectNullableString(value.doi, `${path}.doi`);
  expectNumber(value.candidate_count, `${path}.candidate_count`);
  expectObjectArray(value.candidates, `${path}.candidates`);
  expectNumber(value.candidate_preview_limit, `${path}.candidate_preview_limit`);
  expectBoolean(value.candidates_truncated, `${path}.candidates_truncated`);
}

/** Asserts one identity candidate. */
function expectIdentityCandidate(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.id, `${path}.id`);
  expectString(value.candidate_orcid, `${path}.candidate_orcid`);
  expectNullableString(value.provider_display_name, `${path}.provider_display_name`);
  expectString(value.query_url, `${path}.query_url`);
  expectNullableNumber(value.payload_artifact_id, `${path}.payload_artifact_id`);
  expectNullableNumber(value.provider_rank, `${path}.provider_rank`);
  expectString(value.created_at, `${path}.created_at`);
}

/** Asserts one evaluation row. */
function expectEvaluationRow(value: unknown, path: string): void {
  expectObject(value, path);
  expectNumber(value.work_id, `${path}.work_id`);
  expectNumber(value.work_revision_id, `${path}.work_revision_id`);
  expectNullableString(value.title, `${path}.title`);
  expectNullableString(value.doi, `${path}.doi`);
  expectNullableString(value.source, `${path}.source`);
  expectString(value.review_status, `${path}.review_status`);
  expectBoolean(value.review_inherited, `${path}.review_inherited`);
  expectNullableNumber(value.review_version_id, `${path}.review_version_id`);
  expectNullableNumber(value.review_created_in_context_id, `${path}.review_created_in_context_id`);
  expectStringArray(value.review_sub_statuses, `${path}.review_sub_statuses`);
  expectString(value.inventory_status, `${path}.inventory_status`);
  expectNullableString(value.inventoried_at, `${path}.inventoried_at`);
}

/** Asserts one evaluation facet. */
function expectFacet(value: unknown, path: string): void {
  expectObject(value, path);
  expectString(value.value, `${path}.value`);
  expectNumber(value.count, `${path}.count`);
}

// ── Endpoint validators ───────────────────────────────────────────────

/** Validates GET /api/health. */
async function validateHealth(request: APIRequestContext): Promise<void> {
  const response = await request.get('/api/health');
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'health');
  expectBoolean(body.readable, 'health.readable');
  expectBoolean(body.metadata_readable, 'health.metadata_readable');
  expectNumber(body.table_count, 'health.table_count');
  expectStringArray(body.tables, 'health.tables');
  expectString(body.corpus_id, 'health.corpus_id');
  expectBoolean(body.review_writable, 'health.review_writable');
  expectBoolean(body.pdf_store_bound, 'health.pdf_store_bound');
  expectBoolean(body.pdf_store_readable, 'health.pdf_store_readable');
  expectObject(body.review, 'health.review');
  expectBoolean(body.review.available, 'health.review.available');
  expectBoolean(body.review.metadata_writable, 'health.review.metadata_writable');
  expectBoolean(body.review.pdf_store_bound, 'health.review.pdf_store_bound');
  expectBoolean(body.review.pdf_store_readable, 'health.review.pdf_store_readable');
  expectBoolean(body.review.pdf_store_read_only, 'health.review.pdf_store_read_only');
}

/** Validates GET /api/hierarchy summary and run pages. */
async function validateHierarchy(request: APIRequestContext): Promise<void> {
  const summaryResponse = await request.get('/api/hierarchy');
  expect(summaryResponse.ok()).toBeTruthy();
  const summary = await summaryResponse.json();
  expectObject(summary, 'hierarchy.summary');
  expectString(summary.version, 'hierarchy.summary.version');
  expectObject(summary.totals, 'hierarchy.summary.totals');
  expectNumber(summary.totals.searches, 'hierarchy.summary.totals.searches');
  expectNumber(summary.totals.revisions, 'hierarchy.summary.totals.revisions');
  expectNumber(summary.totals.plans, 'hierarchy.summary.totals.plans');
  expectNumber(summary.totals.runs, 'hierarchy.summary.totals.runs');
  expectNumber(summary.totals.completed_runs, 'hierarchy.summary.totals.completed_runs');
  if (summary.latest_run !== null) expectHierarchyRun(summary.latest_run, 'hierarchy.summary.latest_run');

  const runsResponse = await request.get('/api/hierarchy', { params: { section: 'runs' } });
  expect(runsResponse.ok()).toBeTruthy();
  const runs = await runsResponse.json();
  expectObject(runs, 'hierarchy.runs');
  expectString(runs.version, 'hierarchy.runs.version');
  expectObjectArray(runs.items, 'hierarchy.runs.items');
  runs.items.forEach((item, index) => expectHierarchyRun(item, `hierarchy.runs.items[${index}]`));
  expectBoolean(runs.has_more, 'hierarchy.runs.has_more');
  expectString(runs.next_cursor, 'hierarchy.runs.next_cursor');
  expectNumber(runs.limit, 'hierarchy.runs.limit');
}

/** Validates GET /api/runs/{id}/context. */
async function validateRunContext(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/runs/${RUN}/context`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'runContext');
  expectObject(body.search, 'runContext.search');
  expectNumber(body.search.id, 'runContext.search.id');
  expectString(body.search.search_id, 'runContext.search.search_id');
  expectString(body.search.created_at, 'runContext.search.created_at');
  expectObject(body.revision, 'runContext.revision');
  expectNumber(body.revision.id, 'runContext.revision.id');
  expectNumber(body.revision.search_id, 'runContext.revision.search_id');
  expectString(body.revision.label, 'runContext.revision.label');
  expectString(body.revision.config_artifact_hash, 'runContext.revision.config_artifact_hash');
  expectString(body.revision.resolved_manifest_hash, 'runContext.revision.resolved_manifest_hash');
  expectString(body.revision.created_at, 'runContext.revision.created_at');
  expectObject(body.plan, 'runContext.plan');
  expectNumber(body.plan.id, 'runContext.plan.id');
  expectNumber(body.plan.search_revision_id, 'runContext.plan.search_revision_id');
  expectString(body.plan.execution_fingerprint, 'runContext.plan.execution_fingerprint');
  expectString(body.plan.resolved_manifest_hash, 'runContext.plan.resolved_manifest_hash');
  expectString(body.plan.input_manifest_hash, 'runContext.plan.input_manifest_hash');
  expectBoolean(body.plan.enrichment_enabled, 'runContext.plan.enrichment_enabled');
  expectString(body.plan.created_at, 'runContext.plan.created_at');
  expectObject(body.run, 'runContext.run');
  expectNumber(body.run.id, 'runContext.run.id');
  expectNumber(body.run.execution_plan_id, 'runContext.run.execution_plan_id');
  expectString(body.run.step, 'runContext.run.step');
  expectString(body.run.started_at, 'runContext.run.started_at');
  expectNullableString(body.run.finished_at, 'runContext.run.finished_at');
  expectString(body.run.status, 'runContext.run.status');
  expectNullableString(body.run.summary, 'runContext.run.summary');
  expectNumber(body.run.attempt_number, 'runContext.run.attempt_number');
  expectString(body.run.visibility_state, 'runContext.run.visibility_state');
  expectNullableString(body.run.trashed_at, 'runContext.run.trashed_at');
  expectNullableString(body.run.trash_reason, 'runContext.run.trash_reason');
  expectObject(body.lifecycle, 'runContext.lifecycle');
  expectString(body.lifecycle.status, 'runContext.lifecycle.status');
  expectString(body.lifecycle.visibility_state, 'runContext.lifecycle.visibility_state');
  expectBoolean(body.lifecycle.review_writable, 'runContext.lifecycle.review_writable');
  expectObject(body.review, 'runContext.review');
  expectBoolean(body.review.initialized, 'runContext.review.initialized');
  expectNullableNumber(body.review.context_id, 'runContext.review.context_id');
  expectBoolean(body.review.run_writable, 'runContext.review.run_writable');
}

/** Validates GET /api/overview. */
async function validateOverview(request: APIRequestContext): Promise<void> {
  const response = await request.get('/api/overview', { params: { run_id: RUN } });
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'overview');
  expectNumber(body.run_id, 'overview.run_id');
  expectArray(body.captured_metrics, 'overview.captured_metrics');
  body.captured_metrics.forEach((item, index) => expectMetricEvidence(item, `overview.captured_metrics[${index}]`));
  expectMetricRecord(body.retention_funnel, 'overview.retention_funnel');
  expectMetricRecord(body.source_breakdown, 'overview.source_breakdown');
  expectObjectArray(body.source_result_counts, 'overview.source_result_counts');
  body.source_result_counts.forEach((item, index) => {
    expectNumber(item.id, `overview.source_result_counts[${index}].id`);
    expectString(item.source_name, `overview.source_result_counts[${index}].source_name`);
    expectNullableNumber(item.expected_result_count, `overview.source_result_counts[${index}].expected_result_count`);
    expectNullableNumber(item.observed_result_count, `overview.source_result_counts[${index}].observed_result_count`);
    expectNullableString(item.result_count_comparison, `overview.source_result_counts[${index}].result_count_comparison`);
    expectNullableString(item.export_date, `overview.source_result_counts[${index}].export_date`);
  });
  expectObjectArray(body.source_filter_counts, 'overview.source_filter_counts');
  body.source_filter_counts.forEach((item, index) => {
    expectString(item.source, `overview.source_filter_counts[${index}].source`);
    expectStringArray(item.filters, `overview.source_filter_counts[${index}].filters`);
    expectNumber(item.count, `overview.source_filter_counts[${index}].count`);
    expectString(item.state, `overview.source_filter_counts[${index}].state`);
  });
  expectObjectArray(body.source_filter_diagnostics, 'overview.source_filter_diagnostics');
  expectMetricRecord(body.validation_breakdown, 'overview.validation_breakdown');
  expectMetricRecord(body.cache_breakdown, 'overview.cache_breakdown');
  expectMetricRecord(body.enrichment_breakdown, 'overview.enrichment_breakdown');
  expectMetricRecord(body.enrichment_field_breakdown, 'overview.enrichment_field_breakdown');
  expectMetricRecord(body.enrichment_provider_breakdown, 'overview.enrichment_provider_breakdown');
  expectMetricRecord(body.normalization_breakdown, 'overview.normalization_breakdown');
  expectRecordOf(body.normalization_field_breakdown, 'overview.normalization_field_breakdown', (field, fieldPath) => {
    expectMetricRecord(field, fieldPath);
  });
  expectMetricRecord(body.current_coverage, 'overview.current_coverage');
  expectMetricRecord(body.relationship_totals, 'overview.relationship_totals');
}

/** Validates GET /api/audit. */
async function validateAudit(request: APIRequestContext): Promise<void> {
  const response = await request.get('/api/audit', { params: { run_id: RUN } });
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'audit');
  expectObjectArray(body.events, 'audit.events');
  body.events.forEach((item, index) => expectAuditEvent(item, `audit.events[${index}]`));
  expectBoolean(body.has_more, 'audit.has_more');
  expectNullableNumber(body.next_cursor, 'audit.next_cursor');
  expectObject(body.summary, 'audit.summary');
  expectNumber(body.summary.total_events, 'audit.summary.total_events');
  expectObjectArray(body.summary.actions, 'audit.summary.actions');
  expectObject(body.facets, 'audit.facets');
  expectStringArray(body.facets.actors, 'audit.facets.actors');
  expectStringArray(body.facets.actions, 'audit.facets.actions');
  expectStringArray(body.facets.entity_types, 'audit.facets.entity_types');
  expectObject(body.scope, 'audit.scope');
  expectNullableString(body.scope.run_id, 'audit.scope.run_id');
  expectString(body.scope.pdf_scope, 'audit.scope.pdf_scope');
}

/** Validates GET /api/runs/{id}/artifacts and GET /api/artifacts/{id}/inspect. */
async function validateArtifacts(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/runs/${RUN}/artifacts`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'artifacts');
  expectNumber(body.run_id, 'artifacts.run_id');
  expectObject(body.context, 'artifacts.context');
  expectString(body.context.search_id, 'artifacts.context.search_id');
  expectNumber(body.context.search_revision_id, 'artifacts.context.search_revision_id');
  expectString(body.context.search_revision_label, 'artifacts.context.search_revision_label');
  expectNumber(body.context.execution_plan_id, 'artifacts.context.execution_plan_id');
  expectString(body.context.execution_fingerprint, 'artifacts.context.execution_fingerprint');
  expectNumber(body.context.run_id, 'artifacts.context.run_id');
  expectNumber(body.context.attempt_number, 'artifacts.context.attempt_number');
  expectObjectArray(body.artifacts, 'artifacts.artifacts');
  body.artifacts.forEach((item, index) => expectArtifact(item, `artifacts.artifacts[${index}]`));
  expectBoolean(body.has_more, 'artifacts.has_more');
  expectNullableString(body.next_cursor, 'artifacts.next_cursor');
  expectNumber(body.limit, 'artifacts.limit');
  expectObject(body.filters, 'artifacts.filters');
  expectString(body.filters.q, 'artifacts.filters.q');
  expectString(body.filters.role, 'artifacts.filters.role');
  expectNullableNumber(body.filters.artifact_id, 'artifacts.filters.artifact_id');

  const inspectResponse = await request.get(`/api/artifacts/${ARTIFACT}/inspect`);
  expect(inspectResponse.ok()).toBeTruthy();
  const inspect = await inspectResponse.json();
  expectObject(inspect, 'inspect');
  expectNumber(inspect.artifact_id, 'inspect.artifact_id');
  expectString(inspect.content_type, 'inspect.content_type');
  expectNumber(inspect.byte_size, 'inspect.byte_size');
  expectNumber(inspect.stored_byte_size, 'inspect.stored_byte_size');
  expectNumber(inspect.preview_byte_size, 'inspect.preview_byte_size');
  expectNumber(inspect.preview_limit_bytes, 'inspect.preview_limit_bytes');
  expectBoolean(inspect.truncated, 'inspect.truncated');
  expectString(inspect.format, 'inspect.format');
  expectString(inspect.content, 'inspect.content');
}

/** Validates GET /api/runs/{id}/cache-uses. */
async function validateCacheUses(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/runs/${RUN}/cache-uses`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'cacheUses');
  expectNumber(body.run_id, 'cacheUses.run_id');
  expectStringArray(body.columns, 'cacheUses.columns');
  expectObjectArray(body.rows, 'cacheUses.rows');
  expectObjectArray(body.cache_uses, 'cacheUses.cache_uses');
  expectPagination(body.pagination, 'cacheUses.pagination');
}

/** Validates GET /api/runs/{id}/corpus/{kind}. */
async function validateCorpus(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/runs/${RUN}/corpus/articles`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'corpus');
  expectNumber(body.run_id, 'corpus.run_id');
  expectString(body.collection, 'corpus.collection');
  expectStringArray(body.columns, 'corpus.columns');
  expectObjectArray(body.rows, 'corpus.rows');
  expectPagination(body.pagination, 'corpus.pagination');
}

/** Validates GET /api/runs/{id}/evaluation. */
async function validateEvaluation(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/runs/${RUN}/evaluation`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'evaluation');
  expectNumber(body.run_id, 'evaluation.run_id');
  expectBoolean(body.review_context_initialized, 'evaluation.review_context_initialized');
  if (body.review_context !== null) expectReviewContext(body.review_context, 'evaluation.review_context');
  expectObject(body.review_summary, 'evaluation.review_summary');
  expectNumber(body.review_summary.total, 'evaluation.review_summary.total');
  expectNumber(body.review_summary.reviewed, 'evaluation.review_summary.reviewed');
  expectNumber(body.review_summary.unreviewed, 'evaluation.review_summary.unreviewed');
  expectNumber(body.review_summary.pdf_available, 'evaluation.review_summary.pdf_available');
  expectNumber(body.review_summary.pdf_not_available, 'evaluation.review_summary.pdf_not_available');
  expectNullableNumber(body.review_summary.percent_reviewed, 'evaluation.review_summary.percent_reviewed');
  expectObject(body.review_summary.facets, 'evaluation.review_summary.facets');
  for (const facetKey of ['review_status', 'source', 'review_source', 'qualifier', 'pdf_status']) {
    expectArray(body.review_summary.facets[facetKey], `evaluation.review_summary.facets.${facetKey}`);
    body.review_summary.facets[facetKey].forEach((item: unknown, index: number) => expectFacet(item, `evaluation.review_summary.facets.${facetKey}[${index}]`));
  }
  expectObject(body.queue_navigation, 'evaluation.queue_navigation');
  expectNullableNumber(body.queue_navigation.previous_work_revision_id, 'evaluation.queue_navigation.previous_work_revision_id');
  expectNullableNumber(body.queue_navigation.next_work_revision_id, 'evaluation.queue_navigation.next_work_revision_id');
  if (body.proposed_parent !== null) expectObject(body.proposed_parent, 'evaluation.proposed_parent');
  expectBoolean(body.run_writable, 'evaluation.run_writable');
  expectStringArray(body.columns, 'evaluation.columns');
  expectObjectArray(body.rows, 'evaluation.rows');
  body.rows.forEach((item, index) => expectEvaluationRow(item, `evaluation.rows[${index}]`));
  expectPagination(body.pagination, 'evaluation.pagination');
}

/** Validates GET /api/runs/{id}/review-context. */
async function validateReviewContext(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/runs/${RUN}/review-context`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'reviewContext');
  expectNumber(body.run_id, 'reviewContext.run_id');
  expectBoolean(body.context_initialized, 'reviewContext.context_initialized');
  if (body.context !== null) expectReviewContext(body.context, 'reviewContext.context');
  expectBoolean(body.run_writable, 'reviewContext.run_writable');
  if (body.proposed_parent !== undefined && body.proposed_parent !== null) {
    expectObject(body.proposed_parent, 'reviewContext.proposed_parent');
    expectNumber(body.proposed_parent.context_id, 'reviewContext.proposed_parent.context_id');
    expectNumber(body.proposed_parent.pipeline_run_id, 'reviewContext.proposed_parent.pipeline_run_id');
    expectString(body.proposed_parent.search_id, 'reviewContext.proposed_parent.search_id');
    expectString(body.proposed_parent.search_revision, 'reviewContext.proposed_parent.search_revision');
    expectNumber(body.proposed_parent.execution_plan_id, 'reviewContext.proposed_parent.execution_plan_id');
    expectNumber(body.proposed_parent.attempt_number, 'reviewContext.proposed_parent.attempt_number');
    expectString(body.proposed_parent.started_at, 'reviewContext.proposed_parent.started_at');
    expectNumber(body.proposed_parent.inherited_work_count, 'reviewContext.proposed_parent.inherited_work_count');
  }
}

/** Validates GET /api/runs/{id}/articles/{revision}/review. */
async function validateArticleReview(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/runs/${RUN}/articles/${ARTICLE}/review`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'articleReview');
  expectNumber(body.run_id, 'articleReview.run_id');
  expectNumber(body.work_id, 'articleReview.work_id');
  expectNumber(body.work_revision_id, 'articleReview.work_revision_id');
  expectBoolean(body.context_initialized, 'articleReview.context_initialized');
  if (body.context !== undefined && body.context !== null) expectReviewContext(body.context, 'articleReview.context');
  expectBoolean(body.editable, 'articleReview.editable');
  expectObject(body.editability, 'articleReview.editability');
  expectBoolean(body.editability.decision, 'articleReview.editability.decision');
  expectBoolean(body.editability.notes, 'articleReview.editability.notes');
  expectBoolean(body.editability.anchors, 'articleReview.editability.anchors');
  expectObject(body.state, 'articleReview.state');
  expectString(body.state.status, 'articleReview.state.status');
  expectStringArray(body.state.sub_statuses, 'articleReview.state.sub_statuses');
  expectNullableString(body.state.reason, 'articleReview.state.reason');
  if (body.state.version !== null) expectObject(body.state.version, 'articleReview.state.version');
  expectPDFStatus(body.pdf_status, 'articleReview.pdf_status');
  if (body.summary_counts !== undefined) expectNumberRecord(body.summary_counts, 'articleReview.summary_counts');
}

/** Validates the review note and anchor collections. */
async function validateReviewCollections(request: APIRequestContext): Promise<void> {
  const notesResponse = await request.get(`/api/runs/${RUN}/articles/${ARTICLE}/notes`);
  expect(notesResponse.ok()).toBeTruthy();
  const notes = await notesResponse.json();
  expectReviewCollection(notes, 'notes');
  expectArray(notes.notes, 'notes.notes');
  notes.items.forEach((item, index) => expectNote(item, `notes.items[${index}]`));

  const runNotesResponse = await request.get(`/api/runs/${RUN}/notes`);
  expect(runNotesResponse.ok()).toBeTruthy();
  const runNotes = await runNotesResponse.json();
  expectReviewCollection(runNotes, 'runNotes');
  expectArray(runNotes.notes, 'runNotes.notes');

  const anchorsResponse = await request.get(`/api/runs/${RUN}/articles/${ARTICLE}/anchors`);
  expect(anchorsResponse.ok()).toBeTruthy();
  const anchors = await anchorsResponse.json();
  expectReviewCollection(anchors, 'anchors');
  expectArray(anchors.anchors, 'anchors.anchors');
  anchors.items.forEach((item, index) => expectAnchor(item, `anchors.items[${index}]`));

  const backlinksResponse = await request.get(`/api/runs/${RUN}/links/backlinks`, {
    params: { target_type: 'note', target_id: '1' },
  });
  expect(backlinksResponse.ok()).toBeTruthy();
  const backlinks = await backlinksResponse.json();
  expectReviewCollection(backlinks, 'backlinks');
  expectArray(backlinks.backlinks, 'backlinks.backlinks');
}

/** Validates GET /api/runs/{id}/identity-evidence and candidate pages. */
async function validateIdentityEvidence(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/runs/${RUN}/identity-evidence`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'identityEvidence');
  expectNumber(body.run_id, 'identityEvidence.run_id');
  expectStringArray(body.columns, 'identityEvidence.columns');
  expectObjectArray(body.rows, 'identityEvidence.rows');
  body.rows.forEach((item, index) => expectIdentityResolution(item, `identityEvidence.rows[${index}]`));
  expectPagination(body.pagination, 'identityEvidence.pagination');
  expectObject(body.stats, 'identityEvidence.stats');
  expectNumber(body.stats.resolutions, 'identityEvidence.stats.resolutions');
  expectNumber(body.stats.unclear, 'identityEvidence.stats.unclear');
  expectNumber(body.stats.no_candidate, 'identityEvidence.stats.no_candidate');
  expectNumber(body.stats.provider_failed, 'identityEvidence.stats.provider_failed');
  expectNumber(body.stats.candidates, 'identityEvidence.stats.candidates');

  const candidatesResponse = await request.get(`/api/identity-resolutions/${RESOLUTION}/candidates`, {
    params: { run_id: RUN },
  });
  expect(candidatesResponse.ok()).toBeTruthy();
  const candidates = await candidatesResponse.json();
  expectObject(candidates, 'candidates');
  expectNumber(candidates.resolution_id, 'candidates.resolution_id');
  expectObjectArray(candidates.items, 'candidates.items');
  candidates.items.forEach((item, index) => expectIdentityCandidate(item, `candidates.items[${index}]`));
  expectBoolean(candidates.has_more, 'candidates.has_more');
  expectNullableString(candidates.next_cursor, 'candidates.next_cursor');
  expectNumber(candidates.limit, 'candidates.limit');
}

/** Validates GET /api/runs/{id}/stages. */
async function validateStages(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/runs/${RUN}/stages`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'stages');
  expectNumber(body.run_id, 'stages.run_id');
  expectStringArray(body.columns, 'stages.columns');
  expectObjectArray(body.rows, 'stages.rows');
  body.rows.forEach((item, index) => expectStageOutcome(item, `stages.rows[${index}]`));
  expectPagination(body.pagination, 'stages.pagination');
  expectObjectArray(body.stage_summaries, 'stages.stage_summaries');
  body.stage_summaries.forEach((item, index) => {
    expectString(item.stage_name, `stages.stage_summaries[${index}].stage_name`);
    expectNumber(item.total_records, `stages.stage_summaries[${index}].total_records`);
    expectNumberRecord(item.outcomes, `stages.stage_summaries[${index}].outcomes`);
    expectString(item.first_recorded_at, `stages.stage_summaries[${index}].first_recorded_at`);
    expectString(item.last_recorded_at, `stages.stage_summaries[${index}].last_recorded_at`);
  });
  expectObjectArray(body.run_steps, 'stages.run_steps');
  body.run_steps.forEach((item, index) => expectRunStep(item, `stages.run_steps[${index}]`));
}

/** Validates GET /api/tables and GET /api/tables/{table}. */
async function validateTables(request: APIRequestContext): Promise<void> {
  const tablesResponse = await request.get('/api/tables');
  expect(tablesResponse.ok()).toBeTruthy();
  const tables = await tablesResponse.json();
  expectObject(tables, 'tables');
  expectObjectArray(tables.tables, 'tables.tables');
  tables.tables.forEach((item, index) => {
    expectString(item.name, `tables.tables[${index}].name`);
    expectObjectArray(item.columns, `tables.tables[${index}].columns`);
    item.columns.forEach((column, columnIndex) => {
      expectString(column.name, `tables.tables[${index}].columns[${columnIndex}].name`);
      expectString(column.type, `tables.tables[${index}].columns[${columnIndex}].type`);
      expectBoolean(column.primary_key, `tables.tables[${index}].columns[${columnIndex}].primary_key`);
    });
  });

  const rowsResponse = await request.get(`/api/tables/${TABLE}`);
  expect(rowsResponse.ok()).toBeTruthy();
  const rows = await rowsResponse.json();
  expectObject(rows, 'tableRows');
  expectObject(rows.table, 'tableRows.table');
  expectString(rows.table.name, 'tableRows.table.name');
  expectObjectArray(rows.table.columns, 'tableRows.table.columns');
  expectObjectArray(rows.rows, 'tableRows.rows');
  expectObject(rows.truncated_fields, 'tableRows.truncated_fields');
  expectObject(rows.limits, 'tableRows.limits');
  expectNumber(rows.limits.cell_bytes, 'tableRows.limits.cell_bytes');
  expectNumber(rows.limits.response_value_bytes, 'tableRows.limits.response_value_bytes');
  expectPagination(rows.pagination, 'tableRows.pagination');
}

/** Validates the article, author, and reference detail endpoints. */
async function validateDetails(request: APIRequestContext): Promise<void> {
  const articleResponse = await request.get(`/api/articles/${ARTICLE}`, { params: { run_id: RUN } });
  expect(articleResponse.ok()).toBeTruthy();
  const article = await articleResponse.json();
  expectObject(article, 'articleDetail');
  expectObject(article.article, 'articleDetail.article');
  expectNumber(article.article.id, 'articleDetail.article.id');
  expectNumber(article.article.work_id, 'articleDetail.article.work_id');
  expectNumber(article.article.pipeline_run_id, 'articleDetail.article.pipeline_run_id');
  expectNullableString(article.article.title, 'articleDetail.article.title');
  expectNullableString(article.article.doi, 'articleDetail.article.doi');
  expectString(article.article.created_at, 'articleDetail.article.created_at');
  expectDetailCollection(article.authors, 'articleDetail.authors');
  expectDetailCollection(article.references, 'articleDetail.references');
  expectDetailCollection(article.stage_outcomes, 'articleDetail.stage_outcomes');
  expectDetailCollection(article.audit_events, 'articleDetail.audit_events');
  expectObject(article.enrichment_summary, 'articleDetail.enrichment_summary');
  expectStringArray(article.enrichment_summary.providers, 'articleDetail.enrichment_summary.providers');
  expectStringArray(article.enrichment_summary.fields, 'articleDetail.enrichment_summary.fields');
  expectBoolean(article.enrichment_summary.truncated, 'articleDetail.enrichment_summary.truncated');
  expectNumber(article.enrichment_summary.pair_limit, 'articleDetail.enrichment_summary.pair_limit');
  expectPDFStatus(article.pdf_status, 'articleDetail.pdf_status');
  if (article.review_context !== null) expectReviewContext(article.review_context, 'articleDetail.review_context');
  expectBoolean(article.review_context_initialized, 'articleDetail.review_context_initialized');
  if (article.term_matches !== null) expectObject(article.term_matches, 'articleDetail.term_matches');

  const authorResponse = await request.get(`/api/authors/${AUTHOR}`, { params: { run_id: RUN } });
  expect(authorResponse.ok()).toBeTruthy();
  const author = await authorResponse.json();
  expectObject(author, 'authorDetail');
  expectObject(author.author, 'authorDetail.author');
  expectNumber(author.author.id, 'authorDetail.author.id');
  expectString(author.author.citation_name, 'authorDetail.author.citation_name');
  expectDetailCollection(author.articles, 'authorDetail.articles');
  expectDetailCollection(author.audit_events, 'authorDetail.audit_events');
  expectDetailCollection(author.identity_evidence, 'authorDetail.identity_evidence');

  const referenceResponse = await request.get(`/api/references/${REFERENCE}`, { params: { run_id: RUN } });
  expect(referenceResponse.ok()).toBeTruthy();
  const reference = await referenceResponse.json();
  expectObject(reference, 'referenceDetail');
  expectObject(reference.reference, 'referenceDetail.reference');
  expectNumber(reference.reference.id, 'referenceDetail.reference.id');
  expectNumber(reference.reference.work_revision_id, 'referenceDetail.reference.work_revision_id');
  expectNumber(reference.reference.pipeline_run_id, 'referenceDetail.reference.pipeline_run_id');
  expectNumber(reference.reference.mention_order, 'referenceDetail.reference.mention_order');
}

/** Validates GET /api/graph. */
async function validateGraph(request: APIRequestContext): Promise<void> {
  const response = await request.get('/api/graph', { params: { run_id: RUN } });
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectObject(body, 'graph');
  expectObjectArray(body.nodes, 'graph.nodes');
  body.nodes.forEach((item, index) => {
    expectString(item.id, `graph.nodes[${index}].id`);
    expectString(item.type, `graph.nodes[${index}].type`);
    expectString(item.label, `graph.nodes[${index}].label`);
  });
  expectObjectArray(body.edges, 'graph.edges');
  body.edges.forEach((item, index) => {
    expectString(item.id, `graph.edges[${index}].id`);
    expectString(item.type, `graph.edges[${index}].type`);
  });
  expectObject(body.filters, 'graph.filters');
  expectNumber(body.filters.run_id, 'graph.filters.run_id');
  expectString(body.filters.mode, 'graph.filters.mode');
  expectNumber(body.filters.article_limit, 'graph.filters.article_limit');
  expectBoolean(body.truncated, 'graph.truncated');
  expectObject(body.limits, 'graph.limits');
  expectNumber(body.limits.article_nodes, 'graph.limits.article_nodes');
  expectNumber(body.limits.related_nodes, 'graph.limits.related_nodes');
  expectNumber(body.limits.edges, 'graph.limits.edges');
  expectObject(body.counts, 'graph.counts');
  expectNumber(body.counts.article_matches, 'graph.counts.article_matches');
  expectNumber(body.counts.article_rendered, 'graph.counts.article_rendered');
  expectNumber(body.counts.nodes_rendered, 'graph.counts.nodes_rendered');
  expectNumber(body.counts.edges_rendered, 'graph.counts.edges_rendered');
  expectNumberRecord(body.counts.node_types, 'graph.counts.node_types');
  expectNumberRecord(body.counts.edge_types, 'graph.counts.edge_types');
  expectString(body.truncation_reason, 'graph.truncation_reason');
}

/** Validates GET /api/works/{work_id}/pdf-status. */
async function validatePDFStatus(request: APIRequestContext): Promise<void> {
  const response = await request.get(`/api/works/${ARTICLE}/pdf-status`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expectPDFStatus(body, 'pdfStatus');
}

// ── Spec body ─────────────────────────────────────────────────────────

test('initializes the review context on the fixture copy', async ({ request }) => {
  const contextResponse = await request.get(`/api/runs/${RUN}/review-context`);
  expect(contextResponse.ok()).toBeTruthy();
  const context = await contextResponse.json();
  if (context.context_initialized) return;
  const createResponse = await request.post(`/api/runs/${RUN}/review-context`, {
    data: { parent_context_id: null },
  });
  expect(createResponse.ok()).toBeTruthy();
  const created = await createResponse.json();
  expectObject(created, 'reviewContextMutation');
  expect(created.context_initialized).toBe(true);
  expectReviewContext(created.context, 'reviewContextMutation.context');
});

test('health and hierarchy endpoints match their contracts', async ({ request }) => {
  await validateHealth(request);
  await validateHierarchy(request);
});

test('run context and overview endpoints match their contracts', async ({ request }) => {
  await validateRunContext(request);
  await validateOverview(request);
});

test('audit, artifact, and cache endpoints match their contracts', async ({ request }) => {
  await validateAudit(request);
  await validateArtifacts(request);
  await validateCacheUses(request);
});

test('corpus, evaluation, and review-context endpoints match their contracts', async ({ request }) => {
  await validateCorpus(request);
  await validateEvaluation(request);
  await validateReviewContext(request);
});

test('article review and review collections match their contracts', async ({ request }) => {
  await validateArticleReview(request);
  await validateReviewCollections(request);
});

test('identity evidence, stages, and table endpoints match their contracts', async ({ request }) => {
  await validateIdentityEvidence(request);
  await validateStages(request);
  await validateTables(request);
});

test('detail, graph, and PDF-status endpoints match their contracts', async ({ request }) => {
  await validateDetails(request);
  await validateGraph(request);
  await validatePDFStatus(request);
});