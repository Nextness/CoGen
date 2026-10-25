import AxeBuilder from '@axe-core/playwright';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';

import { test, expect } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';

import { visit, viewerState } from "../../../support/node/visit.ts";

test.describe.configure({ mode: 'serial' });

/** The run context shared by PDF management browser checks. */
const context: Record<string, string> = {
  view: 'article',
  search_id: '1',
  search_revision_id: '1',
  plan_id: '1',
  run_id: '1',
};

/** Returns one valid PDF derived from the fixture by appending a unique comment. */
function derivedPDF(source: Buffer, marker: string): Buffer {
  return Buffer.concat([source, Buffer.from(`\n% ${marker}\n`)]);
}

/** Returns the SHA-256 hex digest of one upload body. */
function digest(data: Buffer): string {
  return createHash('sha256').update(data).digest('hex');
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

/** Seeds viewer state for one article and opens the clean article path. */
async function visitArticle(page: Page, articleID: string, origin?: string): Promise<void> {
  const state: Record<string, string> = { ...context, article_id: articleID };
  if (origin) state.origin = origin;
  await visit(page, state, '/article');
}

/** Initializes the selected run's review context when the fixture has none. */
async function ensureReviewContext(request: APIRequestContext): Promise<void> {
  const current = await request.get('/api/runs/1/review-context');
  expect(current.ok()).toBeTruthy();
  if ((await current.json()).context_initialized) return;
  const created = await request.post('/api/runs/1/review-context', { data: { parent_context_id: null } });
  expect(created.ok()).toBeTruthy();
}

/** Ensures the selected article has available PDF bytes for replacement checks. */
async function ensureAvailablePDF(request: APIRequestContext, articleID: string, bytes: Buffer): Promise<void> {
  const statusResponse = await request.get(`/api/works/${articleID}/pdf-status`);
  expect(statusResponse.ok()).toBeTruthy();
  const status = await statusResponse.json();
  if (status.status === 'available') return;
  const uploaded = await request.put(`/api/runs/1/articles/${articleID}/pdf`, {
    headers: {
      'Content-Type': 'application/pdf',
      'If-Match': status.mutation_etag,
      'X-PDF-Size': String(bytes.length),
      'X-PDF-SHA256': digest(bytes),
    },
    data: bytes,
  });
  expect(uploaded.ok()).toBeTruthy();
}

/** Returns the article-scoped audit panel that follows the reader workspace. */
function articleAudit(page: Page) {
  return page.getByRole('heading', { name: 'Audit events', exact: true }).locator('xpath=../../..');
}

/** Waits for the asynchronous PDF audit relay to drain pending events. */
async function waitForPendingAudit(request: APIRequestContext): Promise<void> {
  await expect.poll(async () => {
    const response = await request.get('/api/health');
    if (!response.ok()) return -1;
    return (await response.json()).pdf_audit_pending;
  }, { timeout: 20_000, intervals: [500, 1000, 2000] }).toBe(0);
}

test.describe('isolated PDF management lifecycle', () => {
  test('uploads, downloads, replaces, removes, reuploads, and reports failures through visible controls', async ({ page, request, browserName }) => {
    test.setTimeout(180_000);
    await ensureReviewContext(request);
    const fixtureResponse = await request.get('/api/pdf/1');
    expect(fixtureResponse.ok()).toBeTruthy();
    const fixtureBytes = Buffer.from(await fixtureResponse.body());
    const replacementBytes = derivedPDF(fixtureBytes, `replacement ${browserName}`);
    const replacementHash = digest(replacementBytes);
    const origin = 'view=corpus&section=articles&search_id=1&search_revision_id=1&plan_id=1&run_id=1&q=preserved&page=3&expanded=2';

    await visitArticle(page, '2', origin);
    const management = page.locator('[data-pdf-management]');
    await expect(management).toContainText('Not Available');
    await expect(page.locator('[data-pdf-file]')).toHaveAttribute('accept', 'application/pdf,.pdf');
    await expect(page.getByRole('button', { name: 'Upload PDF', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Download PDF' })).toHaveCount(0);

    const axe = await new AxeBuilder({ page }).include('[data-pdf-management]').analyze();
    expect(axe.violations).toEqual([]);

    await page.getByRole('heading', { name: 'Review decision' }).waitFor();
    await page.locator('[data-review-reason]').fill('Unsaved management decision');
    await page.getByRole('tab', { name: 'Notes' }).click();
    await page.locator('[data-note-body]').fill('Unsaved management note');
    await expect(page.locator('[data-draft-status]')).toContainText('Browser draft saved');
    await page.getByRole('tab', { name: 'Decision' }).click();

    await page.locator('[data-pdf-file]').setInputFiles({ name: 'uploaded.pdf', mimeType: 'application/pdf', buffer: fixtureBytes });
    await expect(page.locator('[data-pdf-selected]')).toContainText('uploaded.pdf');
    const [uploadResponse] = await Promise.all([
      page.waitForResponse((response) => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/pdf')),
      page.getByRole('button', { name: 'Upload PDF', exact: true }).click(),
    ]);
    expect(uploadResponse.status()).toBe(200);
    await expect(page.locator('[data-pdf-feedback]')).toContainText('PDF uploaded. Audit delivery is pending.');
    await expect(management).toContainText('Available');
    await expect(page.getByRole('button', { name: 'Download PDF' })).toBeVisible();
    await expect(page.locator('[data-pdf-include-comments]')).toBeDisabled();
    await expect(page.locator('.rw-pdf-page canvas')).toBeVisible();
    await expect(page.locator('[data-review-reason]')).toHaveValue('Unsaved management decision');
    await page.getByRole('tab', { name: 'Notes' }).click();
    await expect(page.locator('[data-note-body]')).toHaveValue('Unsaved management note');
    await page.getByRole('tab', { name: 'Decision' }).click();
    await waitForPendingAudit(request);
    await page.getByRole('button', { name: 'Refresh PDF state' }).click();
    await expect(page.locator('[data-pdf-feedback]')).toContainText('Current PDF state refreshed');
    await expect(articleAudit(page).locator('.rw-audit-event--pdf').filter({ hasText: 'A validated PDF was added' }).first()).toBeVisible();

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Download PDF' }).click(),
    ]);
    expect(download.suggestedFilename()).toBe('Deep Reinforcement Learning for Robotics.pdf');
    const downloadPath = await download.path();
    expect(downloadPath).not.toBeNull();
    const downloaded = await readFile(downloadPath!);
    expect(downloaded.equals(fixtureBytes)).toBe(true);

    await page.reload();
    await page.waitForLoadState('networkidle');
    await expect(management).toContainText('Available');
    await expect(page.getByRole('button', { name: 'Download PDF' })).toBeVisible();
    await page.getByRole('heading', { name: 'Review decision' }).waitFor();
    await page.locator('[data-review-reason]').fill('Draft before replacement');
    await page.getByRole('tab', { name: 'Notes' }).click();
    await page.locator('[data-note-body]').fill('Note before replacement');
    await expect(page.locator('[data-draft-status]')).toContainText('Browser draft saved');
    await page.getByRole('tab', { name: 'Decision' }).click();

    await page.locator('[data-pdf-file]').setInputFiles({ name: 'replacement.pdf', mimeType: 'application/pdf', buffer: replacementBytes });
    await page.getByRole('button', { name: 'Replace PDF', exact: true }).click();
    const replaceDialog = page.getByRole('dialog', { name: 'Replace stored PDF' });
    await expect(replaceDialog).toBeVisible();
    await expect(replaceDialog).toContainText('10.1000/2');
    await expect(replaceDialog).toContainText('work 2 in every run, including historical runs');
    await expect(replaceDialog).toContainText('permanently deleted and cannot be restored');
    await expect(replaceDialog.getByRole('button', { name: 'Cancel' })).toBeFocused();
    await replaceDialog.getByRole('button', { name: 'Cancel' }).click();
    await expect(replaceDialog).toBeHidden();
    await expect(page.locator('[data-pdf-selected]')).toContainText('replacement.pdf');
    await page.getByRole('button', { name: 'Replace PDF', exact: true }).click();
    await expect(replaceDialog).toBeVisible();
    await replaceDialog.getByRole('button', { name: 'Confirm', exact: true }).focus();
    const [replaceResponse] = await Promise.all([
      page.waitForResponse((response) => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/pdf')),
      page.keyboard.press('Enter'),
    ]);
    expect(replaceResponse.status()).toBe(200);
    await expect(page.locator('[data-pdf-feedback]')).toContainText('PDF replaced. Audit delivery is pending.');
    await expect(management).toContainText(replacementHash);
    await expect(page.locator('[data-review-reason]')).toHaveValue('Draft before replacement');
    await page.getByRole('tab', { name: 'Notes' }).click();
    await expect(page.locator('[data-note-body]')).toHaveValue('Note before replacement');
    await page.getByRole('tab', { name: 'Decision' }).click();
    await waitForPendingAudit(request);
    await page.getByRole('button', { name: 'Refresh PDF state' }).click();
    await expect(articleAudit(page).locator('.rw-audit-event--pdf').filter({ hasText: 'The selected PDF was replaced' }).first()).toBeVisible();

    await page.getByRole('button', { name: 'Remove PDF', exact: true }).click();
    const removeDialog = page.getByRole('dialog', { name: 'Remove stored PDF' });
    await expect(removeDialog).toBeVisible();
    await expect(removeDialog.getByRole('button', { name: 'Cancel' })).toBeFocused();
    await removeDialog.getByRole('button', { name: 'Confirm', exact: true }).focus();
    const [removeResponse] = await Promise.all([
      page.waitForResponse((response) => response.request().method() === 'DELETE' && new URL(response.url()).pathname.endsWith('/pdf')),
      page.keyboard.press('Enter'),
    ]);
    expect(removeResponse.status()).toBe(200);
    await expect(page.locator('[data-pdf-feedback]')).toContainText('PDF removed. Audit delivery is pending.');
    await expect(management).toContainText('Not Available');
    await expect(page.getByRole('button', { name: 'Download PDF' })).toHaveCount(0);
    await expect(page.locator('[data-review-reason]')).toHaveValue('Draft before replacement');
    await page.getByRole('tab', { name: 'Notes' }).click();
    await expect(page.locator('[data-note-body]')).toHaveValue('Note before replacement');
    await page.getByRole('tab', { name: 'Decision' }).click();
    await waitForPendingAudit(request);
    await page.getByRole('button', { name: 'Refresh PDF state' }).click();
    await expect(articleAudit(page).locator('.rw-audit-event--pdf').filter({ hasText: 'The selected PDF was removed' }).first()).toBeVisible();

    await page.reload();
    await page.waitForLoadState('networkidle');
    await expect(management).toContainText('Not Available');
    await expect(page.getByRole('button', { name: 'Download PDF' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Upload PDF', exact: true })).toBeVisible();

    await page.locator('[data-pdf-file]').setInputFiles({ name: 'reupload.pdf', mimeType: 'application/pdf', buffer: replacementBytes });
    const [reuploadResponse] = await Promise.all([
      page.waitForResponse((response) => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/pdf')),
      page.getByRole('button', { name: 'Upload PDF', exact: true }).click(),
    ]);
    expect(reuploadResponse.status()).toBe(200);
    await expect(management).toContainText('Available');
    await expect(management).toContainText(replacementHash);

    const invalidOrigin = 'view=corpus&section=articles&search_id=1&search_revision_id=1&plan_id=1&run_id=1&q=preserved&page=3&expanded=3';
    await visitArticle(page, '3', invalidOrigin);
    await expect(management).toContainText('Not Available');
    await page.locator('[data-pdf-file]').setInputFiles({ name: 'invalid.pdf', mimeType: 'application/pdf', buffer: Buffer.from('not a PDF document') });
    const [invalidResponse] = await Promise.all([
      page.waitForResponse((response) => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/pdf')),
      page.getByRole('button', { name: 'Upload PDF', exact: true }).click(),
    ]);
    expect(invalidResponse.status()).toBe(422);
    await expect(page.locator('[data-pdf-feedback]')).toContainText('not a valid PDF file envelope');
    await expect(page.locator('[data-pdf-selected]')).toContainText('invalid.pdf');
    await expect(management).toContainText('Not Available');

    await page.getByRole('link', { name: /Return to Corpus/ }).click();
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveURL(/\/corpus$/);
    const returned = await viewerState(page);
    expect(returned.q).toBe('preserved');
    expect(returned.page).toBe('3');
    expect(returned.expanded).toBe('3');
  });


  test('downloads selected current-page PDFs as a ZIP with a manifest and export audit', async ({ page, request }) => {
    test.setTimeout(120_000);
    await ensureReviewContext(request);
    const beforeResponse = await request.get('/api/works/1/pdf-status');
    expect(beforeResponse.ok()).toBeTruthy();
    const before = await beforeResponse.json();
    const sourceResponse = await request.get('/api/pdf/1');
    expect(sourceResponse.ok()).toBeTruthy();
    const source = Buffer.from(await sourceResponse.body());

    await visitArticle(page, '1');
    await page.getByRole('heading', { name: 'Review decision' }).waitFor();
    await page.getByRole('tab', { name: 'Notes' }).click();
    await page.locator('[data-note-body]').fill('Draft across ZIP export');
    await expect(page.locator('[data-draft-status]')).toContainText('Browser draft saved');
    await visit(page, {
      view: 'corpus', section: 'articles', search_id: '1', search_revision_id: '1', plan_id: '1', run_id: '1', per_page: '20',
    }, '/corpus');
    await expect(page.locator('[data-export-selected]')).toBeVisible();
    await expect(page.locator('[data-export-include-comments]')).toBeDisabled();
    const row = page.locator('[data-select-row="1"]');
    await expect(row).toBeEnabled();
    await row.check();
    await expect(page.locator('[data-export-count]')).toContainText('1 selected');

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Download selected PDFs' }).click(),
    ]);
    expect(download.suggestedFilename()).toBe('run-1-pdfs.zip');
    const path = await download.path();
    expect(path).not.toBeNull();
    const entries = storedZipEntries(await readFile(path!));
    const manifestBytes = entries.get('export-manifest.json');
    expect(manifestBytes).toBeTruthy();
    const manifest = JSON.parse(manifestBytes!.toString('utf8'));
    expect(manifest.included).toBe(1);
    expect(manifest.skipped).toBe(0);
    expect(manifest.files[0].work_revision_id).toBe(1);
    expect(manifest.files[0].content_hash).toBe(before.content_hash);
    const pdfEntry = entries.get(manifest.files[0].filename);
    expect(pdfEntry).toBeTruthy();
    expect(pdfEntry!.equals(source)).toBe(true);
    await expect(page.locator('[data-export-feedback]')).toContainText('1 included, 0 skipped');

    const afterResponse = await request.get('/api/works/1/pdf-status');
    expect(afterResponse.ok()).toBeTruthy();
    const after = await afterResponse.json();
    expect(after.content_hash).toBe(before.content_hash);
    expect(after.change_counter).toBe(before.change_counter);

    await visitArticle(page, '1');
    await page.getByRole('tab', { name: 'Notes' }).click();
    await expect(page.locator('[data-note-body]')).toHaveValue('Draft across ZIP export');
    await page.getByRole('button', { name: 'Refresh PDF state' }).click();
    await expect(articleAudit(page).locator('.rw-audit-event--pdf').filter({ hasText: 'prepared for an explicit download' }).first()).toBeVisible();
  });

  test('rejects a stale second-tab replacement while preserving drafts and the selected file', async ({ page, request, browserName }) => {
    test.setTimeout(120_000);
    await ensureReviewContext(request);
    const fixtureResponse = await request.get('/api/pdf/1');
    expect(fixtureResponse.ok()).toBeTruthy();
    const fixtureBytes = Buffer.from(await fixtureResponse.body());
    await ensureAvailablePDF(request, '2', fixtureBytes);
    const currentResponse = await request.get('/api/pdf/2');
    expect(currentResponse.ok()).toBeTruthy();
    const currentBytes = Buffer.from(await currentResponse.body());
    const newerBytes = derivedPDF(currentBytes, `newer ${browserName}`);
    const staleBytes = derivedPDF(currentBytes, `stale ${browserName}`);

    await visitArticle(page, '2');
    const second = await page.context().newPage();
    await visitArticle(second, '2');
    await second.getByRole('heading', { name: 'Review decision' }).waitFor();
    await second.locator('[data-review-reason]').fill('Second tab decision');
    await second.getByRole('tab', { name: 'Notes' }).click();
    await second.locator('[data-note-body]').fill('Second tab note');
    await expect(second.locator('[data-draft-status]')).toContainText('Browser draft saved');

    await page.locator('[data-pdf-file]').setInputFiles({ name: 'newer.pdf', mimeType: 'application/pdf', buffer: newerBytes });
    await page.getByRole('button', { name: 'Replace PDF', exact: true }).click();
    await page.getByRole('dialog', { name: 'Replace stored PDF' }).getByRole('button', { name: 'Confirm', exact: true }).click();
    await expect(page.locator('[data-pdf-feedback]')).toContainText('PDF replaced. Audit delivery is pending.');

    await second.bringToFront();
    await second.locator('[data-pdf-file]').setInputFiles({ name: 'stale.pdf', mimeType: 'application/pdf', buffer: staleBytes });
    await second.getByRole('button', { name: 'Replace PDF', exact: true }).click();
    const staleDialog = second.getByRole('dialog', { name: 'Replace stored PDF' });
    await expect(staleDialog).toBeVisible();
    const staleResponsePromise = second.waitForResponse((response) => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith('/pdf'));
    await staleDialog.getByRole('button', { name: 'Confirm', exact: true }).click();
    const staleResponse = await staleResponsePromise;
    expect(staleResponse.status()).toBe(412);
    await expect(second.locator('[data-pdf-feedback]')).toContainText('selection changed');
    await expect(second.locator('[data-pdf-selected]')).toContainText('stale.pdf');
    await second.getByRole('tab', { name: 'Decision' }).click();
    await expect(second.locator('[data-review-reason]')).toHaveValue('Second tab decision');
    await second.getByRole('tab', { name: 'Notes' }).click();
    await expect(second.locator('[data-note-body]')).toHaveValue('Second tab note');
    await second.close();
  });
});
