import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import path from 'node:path';

import { test, expect } from '@playwright/test';
import type { APIRequestContext, Download, Page } from '@playwright/test';

import { inspectPDF } from "../../support/node/pdf-inspector.ts";
import { visit } from "../../support/node/visit.ts";

/** Persistent identifier shape returned by generated fixture APIs. */
type Identifier = string | number;

/** Complete URL context for one generated pipeline run. */
type GeneratedContext = {
  search_id: Identifier;
  search_revision_id: Identifier;
  plan_id: Identifier;
  run_id: Identifier;
};

/** Search fields used to locate the deterministic generated fixture. */
type SearchRecord = {
  id: Identifier;
  search_id: string;
  revisions: Array<{ id: Identifier }>;
};

/** Plan fields used to locate the deterministic generated fixture. */
type PlanRecord = {
  id: Identifier;
};

/** Run fields used by generated-context and lineage checks. */
type RunRecord = {
  id: Identifier;
  status: string;
  attempt_number: number;
};

/** Evaluation fields used to resolve one generated work revision. */
type EvaluationRecord = {
  title: string;
  work_revision_id: Identifier;
};

/** Completed A1/A2 runs and their Offline Complete One revisions. */
type ReviewRuns = {
  a1: RunRecord;
  a2: RunRecord;
  revisions: Record<string, Identifier>;
};

/** Returns the SHA-256 hex digest of one file. */
function digest(data: Buffer): string {
  return createHash('sha256').update(data).digest('hex');
}

/** Returns one generated E2E upload fixture beside the generated metadata database. */
function generatedFixturePath(name: string): string {
  const metadataPath = process.env.FIXTURE_DB;
  if (!metadataPath) throw new Error('FIXTURE_DB is required for generated E2E fixtures');
  return path.join(path.dirname(metadataPath), name);
}

/** Reads one completed browser download into memory. */
async function downloadBytes(download: Download): Promise<Buffer> {
  const file = await download.path();
  if (!file) throw new Error('Browser download path is unavailable');
  return readFile(file);
}

/** Extracts stored entries from one standard ZIP response for assertions. */
function storedZipEntries(data: Buffer): Map<string, Buffer> {
  var eocd = -1;
  for (var index = data.length - 22; index >= 0; index -= 1) {
    if (data.readUInt32LE(index) === 0x06054b50) {
      eocd = index;
      break;
    }
  }
  if (eocd < 0) throw new Error('ZIP end-of-central-directory record is missing');
  const count = data.readUInt16LE(eocd + 10);
  var offset = data.readUInt32LE(eocd + 16);
  const entries = new Map<string, Buffer>();
  for (var entry = 0; entry < count; entry += 1) {
    if (data.readUInt32LE(offset) !== 0x02014b50) throw new Error('ZIP central-directory record is invalid');
    const method = data.readUInt16LE(offset + 10);
    const compressedSize = data.readUInt32LE(offset + 20);
    const nameLength = data.readUInt16LE(offset + 28);
    const extraLength = data.readUInt16LE(offset + 30);
    const commentLength = data.readUInt16LE(offset + 32);
    const localOffset = data.readUInt32LE(offset + 42);
    const name = data.subarray(offset + 46, offset + 46 + nameLength).toString('utf8');
    if (method !== 0) throw new Error(`ZIP entry ${name} is not stored`);
    const localNameLength = data.readUInt16LE(localOffset + 26);
    const localExtraLength = data.readUInt16LE(localOffset + 28);
    const dataStart = localOffset + 30 + localNameLength + localExtraLength;
    entries.set(name, data.subarray(dataStart, dataStart + compressedSize));
    offset += 46 + nameLength + extraLength + commentLength;
  }
  return entries;
}

/** Resolves completed A1/A2 runs and the Offline Complete One revision in each. */
async function completedReviewRuns(request: APIRequestContext, context: GeneratedContext): Promise<ReviewRuns> {
  const runsResponse = await request.get('/api/runs', { params: { plan_id: context.plan_id } });
  const runRecords = (await runsResponse.json()).runs as RunRecord[];
  const completed = runRecords.filter((item) => item.status === 'completed').sort((left, right) => left.attempt_number - right.attempt_number);
  expect(completed).toHaveLength(2);
  const revisions: Record<string, Identifier> = {};
  for (const run of completed) {
    const response = await request.get(`/api/runs/${run.id}/evaluation`, { params: { per_page: 100, sort: 'title', order: 'asc' } });
    const rows = (await response.json()).rows as EvaluationRecord[];
    const row = rows.find((item) => item.title === 'Offline Complete One');
    expect(row).toBeTruthy();
    if (!row) throw new Error('Generated E2E work revision is unavailable');
    revisions[run.id] = row.work_revision_id;
  }
  return { a1: completed[0], a2: completed[1], revisions };
}

test.skip(process.env.E2E_SPEC !== '1', 'Run through make test-e2e with a generated pipeline database.');
test.describe.configure({ mode: 'serial' });

/** Resolves the generated database's persistent URL-state identifiers through public APIs. */
async function generatedContext(request: APIRequestContext): Promise<GeneratedContext> {
  const searchResponse = await request.get('/api/searches');
  expect(searchResponse.ok()).toBeTruthy();
  const searchBody = await searchResponse.json();
  const searches = searchBody.searches as SearchRecord[];
  const search = searches.find((item) => item.search_id === 'e2e-deterministic');
  expect(search).toBeTruthy();
  if (!search) throw new Error('Generated E2E search is unavailable');
  expect(search.revisions).toHaveLength(1);
  const revision = search.revisions[0];

  const planResponse = await request.get('/api/plans', { params: { search_revision_id: revision.id } });
  expect(planResponse.ok()).toBeTruthy();
  const planBody = await planResponse.json();
  expect(planBody.plans).toHaveLength(1);
  const plan = (planBody.plans as PlanRecord[])[0];

  const runResponse = await request.get('/api/runs', { params: { plan_id: plan.id } });
  expect(runResponse.ok()).toBeTruthy();
  const runBody = await runResponse.json();
  const runs = runBody.runs as RunRecord[];
  const run = runs.find((item) => item.status === 'completed' && item.attempt_number === 1);
  expect(run).toBeTruthy();
  if (!run) throw new Error('Generated E2E run is unavailable');

  return {
    search_id: search.id,
    search_revision_id: revision.id,
    plan_id: plan.id,
    run_id: run.id,
  };
}

/** Builds a context-preserving viewer state for one generated route. */
function generatedState(context: GeneratedContext, updates: Record<string, Identifier>): Record<string, string> {
  const state: Record<string, string> = {
    search_id: String(context.search_id),
    search_revision_id: String(context.search_revision_id),
    plan_id: String(context.plan_id),
    run_id: String(context.run_id),
  };
  for (const [key, value] of Object.entries(updates)) state[key] = String(value);
  return state;
}

/** Seeds viewer state through sessionStorage and navigates to the clean generated route. */
async function visitGenerated(page: Page, context: GeneratedContext, updates: Record<string, Identifier>): Promise<void> {
  const state = generatedState(context, updates);
  await visit(page, state);
}

test('pipeline evidence is consistent across Corpus, Provenance, and Evaluation', async ({ page, request }) => {
  const context = await generatedContext(request);

  await visitGenerated(page, context, { view: 'corpus', section: 'articles' });
  await page.waitForLoadState('networkidle');
  const corpus = page.locator(`[data-table-owner="work_revisions"]`);
  await expect(corpus).toBeVisible();
  await expect(corpus).toContainText('Offline Complete One');
  await expect(corpus).toContainText('Offline Complete Two');
  await expect(corpus).not.toContainText('Provider Enrichment Candidate');
  await expect(corpus).not.toContainText('Validation Discarded Invalid DOI');

  await visitGenerated(page, context, { view: 'provenance', section: 'audit' });
  await page.waitForLoadState('networkidle');
  const audit = page.locator('#audit-event-stream');
  await expect(audit).toBeVisible();
  await expect(audit).toContainText('Run Started');
  await expect(audit).toContainText('Validation Changed');
  await expect(audit).toContainText("PDF Inventory Registered");
  await expect(audit).toContainText('Run Completed');

  await visitGenerated(page, context, { view: 'corpus', section: 'articles' });
  await page.waitForLoadState('networkidle');
  const evaluation = page.locator('.rw-corpus-table');
  await expect(evaluation).toBeVisible();
  await expect(evaluation).toContainText('Offline Complete One');
  await expect(evaluation).toContainText('Offline Complete Two');
  await expect(evaluation.locator('.ui.orange.label', { hasText: 'Not Available' })).toHaveCount(2);
});

test('PDF lifecycle uploads, replaces, removes, reuploads, and downloads through the application', async ({ page, request }) => {
  test.setTimeout(120_000);
  const context = await generatedContext(request);
  const { a1, revisions } = await completedReviewRuns(request, context);
  const revision = revisions[a1.id];
  const original = await readFile(generatedFixturePath('review.fixture.pdf'));
  const replacement = await readFile(generatedFixturePath('review.replacement.pdf'));
  const originalHash = digest(original);
  const replacementHash = digest(replacement);

  await visitGenerated(page, context, { view: 'article', article_id: revision });
  await page.waitForLoadState('networkidle');
  const management = page.locator('[data-pdf-management]');
  await expect(management).toContainText('Not Available');
  await page.locator('[data-pdf-file]').setInputFiles({ name: 'e2e-review.pdf', mimeType: 'application/pdf', buffer: original });
  const [uploadResponse] = await Promise.all([
    page.waitForResponse((response) => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/pdf')),
    page.getByRole('button', { name: 'Upload PDF', exact: true }).click(),
  ]);
  expect(uploadResponse.status()).toBe(200);
  await expect(management).toContainText('Available');
  await expect(management).toContainText(originalHash);

  const [singleDownload] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Download PDF' }).click(),
  ]);
  expect((await downloadBytes(singleDownload)).equals(original)).toBe(true);

  await visitGenerated(page, context, { view: 'corpus', section: 'articles' });
  await page.waitForLoadState('networkidle');
  await page.locator(`[data-select-row="${revision}"]`).check();
  const [zipDownload] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Download selected PDFs' }).click(),
  ]);
  const entries = storedZipEntries(await downloadBytes(zipDownload));
  const manifestBytes = entries.get('export-manifest.json');
  expect(manifestBytes).toBeTruthy();
  const manifest = JSON.parse(manifestBytes!.toString('utf8'));
  expect(manifest.files[0].content_hash).toBe(originalHash);
  const zipPDF = entries.get(manifest.files[0].filename);
  expect(zipPDF?.equals(original)).toBe(true);

  await visitGenerated(page, context, { view: 'article', article_id: revision });
  await page.waitForLoadState('networkidle');
  await page.locator('[data-pdf-file]').setInputFiles({ name: 'replacement.pdf', mimeType: 'application/pdf', buffer: replacement });
  await page.getByRole('button', { name: 'Replace PDF', exact: true }).click();
  const replaceDialog = page.getByRole('dialog', { name: 'Replace stored PDF' });
  await expect(replaceDialog).toBeVisible();
  const [replaceResponse] = await Promise.all([
    page.waitForResponse((response) => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/pdf')),
    replaceDialog.getByRole('button', { name: 'Confirm', exact: true }).click(),
  ]);
  expect(replaceResponse.status()).toBe(200);
  await expect(management).toContainText(replacementHash);

  await page.locator('[data-pdf-file]').setInputFiles({ name: 'invalid.pdf', mimeType: 'application/pdf', buffer: Buffer.from('not a PDF document') });
  await page.getByRole('button', { name: 'Replace PDF', exact: true }).click();
  const failedDialog = page.getByRole('dialog', { name: 'Replace stored PDF' });
  await expect(failedDialog).toBeVisible();
  const [failedResponse] = await Promise.all([
    page.waitForResponse((response) => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/pdf')),
    failedDialog.getByRole('button', { name: 'Confirm', exact: true }).click(),
  ]);
  expect(failedResponse.status()).toBe(422);
  await expect(page.locator('[data-pdf-feedback]')).toContainText('not a valid PDF file envelope');
  await expect(management).toContainText(replacementHash);

  await page.getByRole('button', { name: 'Remove PDF', exact: true }).click();
  const removeDialog = page.getByRole('dialog', { name: 'Remove stored PDF' });
  await expect(removeDialog).toBeVisible();
  const [removeResponse] = await Promise.all([
    page.waitForResponse((response) => response.request().method() === 'DELETE' && new URL(response.url()).pathname.endsWith('/pdf')),
    removeDialog.getByRole('button', { name: 'Confirm', exact: true }).click(),
  ]);
  expect(removeResponse.status()).toBe(200);
  await expect(management).toContainText('Not Available');

  await page.locator('[data-pdf-file]').setInputFiles({ name: 'reupload.pdf', mimeType: 'application/pdf', buffer: original });
  const [reuploadResponse] = await Promise.all([
    page.waitForResponse((response) => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/pdf')),
    page.getByRole('button', { name: 'Upload PDF', exact: true }).click(),
  ]);
  expect(reuploadResponse.status()).toBe(200);
  await expect(management).toContainText(originalHash);
});

test('A2 inherits immutable A1 review heads and diverges without changing A1', async ({ page, request }) => {
  const context = await generatedContext(request);
  const { a1: a1Run, a2: a2Run, revisions } = await completedReviewRuns(request, context);
  const a1Revision = revisions[a1Run.id];
  const a2Revision = revisions[a2Run.id];

  const a1ContextResponse = await request.post(`/api/runs/${a1Run.id}/review-context`, { data: { parent_context_id: null } });
  expect(a1ContextResponse.status()).toBe(201);
  const a1Context = (await a1ContextResponse.json()).context;
  const a1DetailResponse = await request.get(`/api/articles/${a1Revision}`, { params: { run_id: a1Run.id } });
  expect(a1DetailResponse.ok()).toBeTruthy();
  const a1PDFHash = (await a1DetailResponse.json()).pdf_status.content_hash;
  expect(typeof a1PDFHash).toBe('string');
  const a1ReviewResponse = await request.put(`/api/runs/${a1Run.id}/articles/${a1Revision}/review`, { data: { expected_version_id: null, status: 'approved', sub_statuses: [], reason: 'A1 approved evidence' } });
  expect(a1ReviewResponse.ok()).toBeTruthy();
  const a1Review = (await a1ReviewResponse.json()).review;
  const noteResponse = await request.post(`/api/runs/${a1Run.id}/articles/${a1Revision}/notes`, { data: { body: 'A1 future link [[note:2|target note]].' } });
  expect(noteResponse.status()).toBe(201);
  const a1Note = (await noteResponse.json()).note;
  const anchorResponse = await request.post(`/api/runs/${a1Run.id}/articles/${a1Revision}/anchors`, { data: { label: 'e2e-methods', page: 1, selected_text: 'Selectable E2E methods', rectangles: [{ x: 0.1, y: 0.1, width: 0.4, height: 0.08 }], pdf_content_hash: a1PDFHash } });
  expect(anchorResponse.status()).toBe(201);
  const a1Anchor = (await anchorResponse.json()).anchor;

  const a2ContextResponse = await request.post(`/api/runs/${a2Run.id}/review-context`, { data: { parent_context_id: a1Context.id } });
  expect(a2ContextResponse.status()).toBe(201);
  const inheritedReview = await (await request.get(`/api/runs/${a2Run.id}/articles/${a2Revision}/review`)).json();
  expect(inheritedReview.review.version.id).toBe(a1Review.version.id);
  expect(inheritedReview.review.inherited_from_context_id).toBe(a1Context.id);
  const inheritedNotes = await (await request.get(`/api/runs/${a2Run.id}/articles/${a2Revision}/notes`)).json();
  expect(inheritedNotes.notes[0].id).toBe(a1Note.id);
  const inheritedNote = await (await request.get(`/api/runs/${a2Run.id}/notes/${a1Note.id}`)).json();
  expect(inheritedNote.note.version.links[0].resolved).toBeFalsy();
  const targetResponse = await request.post(`/api/runs/${a2Run.id}/articles/${a2Revision}/notes`, { data: { body: 'A2 target note.' } });
  expect((await targetResponse.json()).note.id).toBe(2);
  const resolvedNote = await (await request.get(`/api/runs/${a2Run.id}/notes/${a1Note.id}`)).json();
  expect(resolvedNote.note.version.links[0].resolved).toBeTruthy();
  const editedNote = await request.post(`/api/runs/${a2Run.id}/notes/${a1Note.id}/versions`, { data: { expected_version_id: a1Note.version.id, state: 'active', body: 'A2 edited note.' } });
  expect(editedNote.status()).toBe(201);
  const changedReview = await request.put(`/api/runs/${a2Run.id}/articles/${a2Revision}/review`, { data: { expected_version_id: a1Review.version.id, status: 'not_approved', sub_statuses: ['out_of_scope'], reason: 'A2 changed evidence' } });
  expect(changedReview.ok()).toBeTruthy();

  const stableA1 = await (await request.get(`/api/runs/${a1Run.id}/articles/${a1Revision}/review`)).json();
  expect(stableA1.review.version.status).toBe('approved');
  expect(stableA1.review.version.id).toBe(a1Review.version.id);
  const stableA1Note = await (await request.get(`/api/runs/${a1Run.id}/notes/${a1Note.id}`)).json();
  expect(stableA1Note.note.version.id).toBe(a1Note.version.id);

  const commentNoteResponse = await request.post(`/api/runs/${a1Run.id}/articles/${a1Revision}/notes`, { data: { body: `E2E saved comment [[anchor:${a1Anchor.id}|methods]].` } });
  expect(commentNoteResponse.status()).toBe(201);
  await visitGenerated(page, context, { view: 'article', article_id: a1Revision });
  await page.waitForLoadState('networkidle');
  const comments = page.locator('[data-pdf-include-comments]');
  await expect(comments).not.toBeChecked();
  await comments.check();
  const [commentResponse, commentDownload] = await Promise.all([
    page.waitForResponse((response) => response.request().method() === 'POST' && response.url().endsWith('/pdf-export')),
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Download PDF' }).click(),
  ]);
  expect(commentResponse.status()).toBe(200);
  expect(commentResponse.headers()['x-pdf-export-comments-status']).toBe('included');
  const annotated = await downloadBytes(commentDownload);
  const inspection = await inspectPDF(new Uint8Array(annotated));
  const annotation = inspection.pages[0].annotations.find((item) => item.contents.includes('E2E saved comment'));
  expect(annotation).toBeTruthy();

  await visitGenerated(page, { ...context, run_id: a2Run.id }, { view: 'article', article_id: a2Revision });
  await page.waitForLoadState('networkidle');
  await expect(page.locator('[data-review-host]')).toContainText('A2 changed evidence');
  await page.getByRole('tab', { name: 'PDF anchors' }).click();
  await expect(page.locator('[data-anchor-list]')).toContainText('e2e-methods');
  await expect(page.locator(".rw-pdf-page .textLayer")).toContainText("Selectable E2E methods");
});
