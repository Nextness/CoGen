import AxeBuilder from '@axe-core/playwright';
import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';

import { selectFixtureText } from './helpers/pdf-selection.ts';

test.describe.configure({ mode: 'serial' });

/** The article review fixture state used by every mutation test. */
const articleState: Record<string, string> = {
  view: 'article',
  search_id: '1',
  search_revision_id: '1',
  plan_id: '1',
  run_id: '1',
  article_id: '1',
};

/** Seeds viewer state through sessionStorage and navigates to the clean article path. */
async function visitArticle(page: Page): Promise<void> {
  const path = '/article';
  await page.evaluate(({ seed, seedPath }) => {
    window.name = JSON.stringify(seed);
    try {
      sessionStorage.removeItem("rw-viewer-state");
      if (location.pathname === seedPath) history.replaceState(null, "", location.pathname);
    } catch (_) {
      // The initial about:blank document may deny sessionStorage access.
    }
  }, { seed: articleState, seedPath: path });
  await page.addInitScript(() => {
    const seed = window.name ? JSON.parse(window.name) : null;
    if (seed && !sessionStorage.getItem("rw-viewer-state")) sessionStorage.setItem("rw-viewer-state", JSON.stringify(seed));
  });
  await page.goto(path);
  await page.waitForLoadState('networkidle');
}

/** Reads the current viewer state from sessionStorage. */
async function viewerState(page: Page): Promise<Record<string, string>> {
  return page.evaluate(() => JSON.parse(sessionStorage.getItem('rw-viewer-state') || '{}'));
}

/** Activates a continuation control until the collection reports its terminal page. */
async function exhaustContinuation(page: Page, selector: string): Promise<void> {
  for (let pageNumber = 0; pageNumber < 20; pageNumber += 1) {
    const button = page.locator(selector);
    if (await button.count() === 0) return;
    const priorButton = await button.elementHandle();
    if (!priorButton) throw new Error(`Continuation control ${selector} disappeared before activation`);
    await button.click();
    await expect.poll(async () => priorButton.evaluate((element) => element.isConnected).catch(() => false)).toBe(false);
  }
  throw new Error(`Continuation control ${selector} did not terminate.`);
}

test.describe('isolated review mutation lifecycle', () => {
  test('creates and persists status, note, PDF selection anchor, and custom PDF rendering through the UI', async ({ page, request, browserName }) => {
    test.setTimeout(60_000);
    const context = await request.get('/api/runs/1/review-context');
    expect(context.ok()).toBeTruthy();
    if (!(await context.json()).context_initialized) {
      await visitArticle(page);
      await page.getByRole('button', { name: 'Start review' }).click();
      const setupDialog = page.getByRole('dialog', { name: 'Start article review' });
      await expect(setupDialog).toBeVisible();
      await expect(setupDialog.getByText('Same-search contexts ready')).toBeVisible();
      await setupDialog.getByRole('button', { name: 'Include all earlier searches' }).click();
      await expect(setupDialog.getByText('All earlier searches checked')).toBeVisible();
      await setupDialog.getByRole('button', { name: 'Cancel' }).click();
      await expect(setupDialog).toBeHidden();
      await page.getByRole('button', { name: 'Start review' }).click();
      await setupDialog.locator('[data-review-parent]').selectOption('');
      await setupDialog.getByRole('button', { name: 'Initialize review' }).click();
      await expect(page.getByRole('heading', { name: 'Review decision' })).toBeVisible();
    }
    await visitArticle(page);
    const reason = `Browser ${browserName} review evidence`;
    await page.locator('[data-review-status]').selectOption('approved');
    await page.locator('[data-review-reason]').fill(reason);
    const [decisionSaveResponse] = await Promise.all([
      page.waitForResponse((response) => response.request().method() === 'PUT' && /\/api\/runs\/1\/articles\/\d+\/review$/.test(new URL(response.url()).pathname)),
      page.getByRole('button', { name: 'Save review decision' }).click(),
    ]);
    expect(decisionSaveResponse.ok()).toBeTruthy();
    await page.waitForLoadState('networkidle');
    await expect(page.locator('[data-review-host]')).toContainText(reason);

    await page.getByRole('tab', { name: 'Notes' }).click();
    await page.locator('[data-note-body]').fill(`# ${browserName} results page\n\nSee [[pdf:page=2|${browserName} results page]] and [[note:999|unresolved note]].`);
    await expect(page.locator('[data-draft-status]')).toContainText('Browser draft saved');
    const [noteSaveResponse] = await Promise.all([
      page.waitForResponse((response) => response.request().method() === 'POST' && /\/api\/runs\/1\/articles\/\d+\/notes$/.test(new URL(response.url()).pathname)),
      page.getByRole('button', { name: 'Save note' }).click(),
    ]);
    expect(noteSaveResponse.status()).toBe(201);
    await expect(page.locator('[data-note-list]')).toContainText(`${browserName} results page`);

    const anchorLabel = `methods-${browserName}`;
    await selectFixtureText(page);
    await expect(page.getByRole('button', { name: 'Review selection' })).toBeVisible();
    await page.getByRole('button', { name: 'Review selection' }).click();
    await expect(page.getByRole('tab', { name: 'PDF anchors' })).toHaveAttribute('aria-selected', 'true');
    await page.locator('[data-anchor-label]').fill(anchorLabel);
    await page.getByRole('button', { name: 'Save anchor' }).click();
    await expect(page.locator('[data-anchor-list]')).toContainText(anchorLabel);

    await expect(page.locator('[data-review-host]')).toContainText('Approved');
    const auditHeading = page.getByRole("heading", {
      name: "Audit events",
      exact: true,
    });
    const articleAuditPanel = auditHeading.locator("xpath=../../..");
    const reviewAuditEvents = articleAuditPanel.locator(".rw-audit-event--review");
    const reviewAuditCount = await reviewAuditEvents.count();
    await expect(articleAuditPanel).toContainText("Work Review Version Created");
    const updatedReason = `${reason} saved from the review form`;
    await page.getByRole('tab', { name: 'Decision' }).click();
    await page.locator('[data-review-status]').selectOption('not_approved');
    await page.locator('[data-review-substatuses] input[value="not_peer_reviewed"]').check();
    await page.locator('[data-review-substatuses] input[value="out_of_scope"]').check();
    await page.locator('[data-review-reason]').fill(updatedReason);
    await page.getByRole('button', { name: 'Save review decision' }).click();
    await expect(page.locator('[data-review-host]')).toContainText(updatedReason);
    await expect(reviewAuditEvents).toHaveCount(reviewAuditCount + 1);
    const latestReviewAudit = reviewAuditEvents.first();
    await expect(latestReviewAudit).toContainText('Review decision changed from Approved to Not Approved.');
    await expect(latestReviewAudit).toContainText('Previous decision');
    await expect(latestReviewAudit).toContainText('New decision');
    await expect(latestReviewAudit).toContainText(reason);
    await expect(latestReviewAudit).toContainText(updatedReason);
    await expect(latestReviewAudit).toContainText('Not Peer Reviewed');
    await expect(latestReviewAudit).toContainText('Out Of Scope');
    const decisionTab = page.getByRole('tab', { name: 'Decision' });
    await expect(page.getByRole('tab')).toHaveCount(3);
    await expect(decisionTab).toHaveAttribute('aria-selected', 'true');
    const readingWorkspaceGap = await page.locator('.rw-reading-workspace').evaluate((workspace) => {
      const sibling = workspace.nextElementSibling;
      if (!sibling) throw new Error('Reading workspace sibling is unavailable');
      return sibling.getBoundingClientRect().top - workspace.getBoundingClientRect().bottom;
    });
    expect(readingWorkspaceGap).toBeGreaterThanOrEqual(15);
    await decisionTab.focus();
    await page.keyboard.press('ArrowRight');
    await expect(page.getByRole('tab', { name: 'Notes' })).toHaveAttribute('aria-selected', 'true');
    await page.getByText('Note syntax and link examples').click();
    await expect(page.locator('.rw-note-syntax')).toContainText('[[article:10.1000/example|Article title]]');
    const noteFormGaps = await page.locator('[data-note-form]').evaluate((form) => {
      const headingElement = form.querySelector('.rw-note-form__heading');
      const fieldElement = form.querySelector('.ui.field');
      const actionsElement = form.querySelector('.rw-review-actions');
      if (!headingElement || !fieldElement || !actionsElement) throw new Error('Note form layout elements are unavailable');
      const heading = headingElement.getBoundingClientRect();
      const field = fieldElement.getBoundingClientRect();
      const actions = actionsElement.getBoundingClientRect();
      return { heading: field.top - heading.bottom, actions: actions.top - field.bottom };
    });
    expect(noteFormGaps.heading).toBeGreaterThanOrEqual(15);
    expect(noteFormGaps.actions).toBeGreaterThanOrEqual(15);
    const browserNote = page.locator('[data-note-list] p').filter({ hasText: `${browserName} results page` });
    await expect(browserNote).toContainText('unresolved note');
    const unresolvedLink = browserNote.locator('[aria-label^="Unresolved note target: 999"]');
    await expect(unresolvedLink).toBeVisible();
    await expect(unresolvedLink).toHaveAttribute('aria-label', /Unresolved note target: 999/);
    await expect(page.getByText('Insert evidence link')).toBeVisible();
    await expect(page.getByText('Note syntax and link examples')).toBeVisible();
    const noteSectionOrder = await page.locator('[data-note-host]').evaluate((notes) => {
      const workspaceElement = notes.querySelector('.rw-note-workspace');
      const savedElement = notes.querySelector('.rw-note-saved');
      if (!workspaceElement || !savedElement) throw new Error('Note workspace sections are unavailable');
      const workspace = workspaceElement.getBoundingClientRect();
      const saved = savedElement.getBoundingClientRect();
      return saved.top - workspace.bottom;
    });
    expect(noteSectionOrder).toBeGreaterThanOrEqual(15);
    await page.getByRole('tab', { name: 'PDF anchors' }).click();
    await expect(page.locator('[data-anchor-list]')).toContainText(anchorLabel);
    const anchorBacklinkGap = await page.locator('.rw-anchor-panel').evaluate((panel) => {
      const listElement = panel.querySelector('[data-anchor-list]');
      const backlinksElement = panel.querySelector('.rw-review-history');
      if (!listElement || !backlinksElement) throw new Error('Anchor evidence sections are unavailable');
      const list = listElement.getBoundingClientRect();
      const backlinks = backlinksElement.getBoundingClientRect();
      return backlinks.top - list.bottom;
    });
    expect(anchorBacklinkGap).toBeGreaterThanOrEqual(15);
    await expect(page.locator(".rw-pdf-page canvas")).toBeVisible();
    await expect(page.locator(".rw-pdf-page .textLayer")).toContainText("Selectable fixture methods");
    await expect(page.locator('.rw-pdf-page')).toHaveCount(1);
    await page.getByRole('button', { name: 'Next PDF page' }).click();
    await expect(page.locator('.rw-pdf-page')).toHaveCount(1);
    await expect(page.locator(".rw-pdf-page")).toHaveAttribute("data-pdf-page-number", "2");
    await expect(page.locator(".rw-pdf-page .textLayer")).toContainText("Selectable fixture conclusions");
    await expect(page.locator('[data-pdf-status]')).toHaveText('PDF page 2 of 2.');
    const containment = await page.locator('.rw-pdf-viewer').evaluate((viewer) => {
      const viewport = viewer.querySelector('.rw-pdf-pages');
      if (!viewport) throw new Error('PDF page viewport is unavailable');
      return { viewerOverflow: getComputedStyle(viewer).overflow, viewportOverflow: getComputedStyle(viewport).overflow, viewerBottom: viewer.getBoundingClientRect().bottom, viewportBottom: viewport.getBoundingClientRect().bottom };
    });
    expect(containment.viewerOverflow).toBe('hidden');
    expect(containment.viewportOverflow).toBe('auto');
    expect(containment.viewportBottom).toBeLessThanOrEqual(containment.viewerBottom + 1);

    await page.reload();
    await page.waitForLoadState('networkidle');
    await expect(page.locator('[data-review-host]')).toContainText(updatedReason);
    await expect(page.locator('[data-review-status]')).toHaveValue('not_approved');
    await page.getByRole('tab', { name: 'PDF anchors' }).click();
    await expect(page.locator('[data-anchor-list]')).toContainText(anchorLabel);
  });

  test('creates an anchor through the fullscreen reader drawer', async ({ page, browserName }) => {
    test.setTimeout(60_000);
    const fullscreenAnchorLabel = `fullscreen-${browserName}`;
    await visitArticle(page);

    await page.getByRole('button', { name: 'Fullscreen', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Exit fullscreen', exact: true })).toBeVisible();
    await expect(page.locator('[data-drawer-edge]')).toBeVisible();
    await expect(page.locator('[data-review-host]')).toBeVisible();

    await page.locator('[data-drawer-edge]').click();
    await expect(page.locator('[data-review-host]')).toBeHidden();

    await selectFixtureText(page);
    await expect(page.getByRole('button', { name: 'Review selection' })).toBeVisible();
    await page.getByRole('button', { name: 'Review selection' }).click();
    await expect(page.locator('[data-review-host]')).toBeVisible();
    await expect(page.getByRole('tab', { name: 'PDF anchors' })).toHaveAttribute('aria-selected', 'true');
    await page.locator('[data-anchor-label]').fill(fullscreenAnchorLabel);
    await page.getByRole('button', { name: 'Save anchor' }).click();
    await expect(page.locator('[data-anchor-list]')).toContainText(fullscreenAnchorLabel);

    await page.getByRole('button', { name: 'Exit fullscreen', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Fullscreen', exact: true })).toBeVisible();
    await expect(page.locator('[data-drawer-edge]')).toHaveCount(0);
    await expect(page.locator('[data-review-host]')).toBeVisible();
    await expect(page.locator('[data-anchor-list]')).toContainText(fullscreenAnchorLabel);
  });

  test('edits, links, conflicts, removes, restores, and audits review evidence through visible controls', async ({ page, request, browserName }) => {
    test.setTimeout(60_000);
    const noteTitle = `${browserName} results page`;
    const anchorLabel = `methods-${browserName}`;
    await visitArticle(page);

    await page.getByRole('tab', { name: 'Decision' }).click();
    await page.getByRole('button', { name: 'Show version history' }).click();
    await expect(page.locator('[data-review-history-list]')).toContainText(/Version \d+.*Not Approved/i);

    await page.getByRole('tab', { name: 'Notes' }).click();
    await page.locator('[data-note-body]').fill('[[unknown:evidence]]');
    await expect(page.locator('[data-note-diagnostics]')).toContainText('unknown custom link scheme');
    var axe = await new AxeBuilder({ page }).include('[data-note-form]').analyze();
    expect(axe.violations).toEqual([]);
    var targetRow = page.locator('[data-note-id]').filter({ hasText: noteTitle }).first();
    const targetNoteID = await targetRow.getAttribute('data-note-id');
    expect(targetNoteID).toMatch(/^\d+$/);
    if (!targetNoteID) throw new Error('Target note ID is unavailable');
    await targetRow.getByRole('button', { name: 'Edit' }).click();
    await expect(page.locator('[data-note-editor-title]')).toContainText('Editing');
    await expect(page.locator('[data-note-body]')).toHaveValue(/unresolved note/);
    const localConflictBody = `# ${noteTitle}\n\n${'Long review evidence '.repeat(240)}\n\nLocal conflict resolution.`;
    await page.locator('[data-note-body]').fill(localConflictBody);
    await expect(page.locator('[data-note-body]')).toHaveValue(localConflictBody);
    await expect(page.locator('[data-draft-status]')).toContainText('Browser draft saved');
    const current = await request.get(`/api/runs/1/notes/${targetNoteID}`);
    expect(current.ok()).toBeTruthy();
    const currentNote = await current.json();
    const external = await request.post(`/api/runs/1/notes/${targetNoteID}/versions`, { data: {
      expected_version_id: currentNote.note.version.id,
      state: 'active',
      body: `# ${noteTitle}\n\nExternal writer update.`,
    } });
    expect(external.status()).toBe(201);
    await page.getByRole('button', { name: 'Save note' }).click();
    await expect(page.locator('.rw-draft-status')).toContainText('changed elsewhere');
    axe = await new AxeBuilder({ page }).include('[data-note-form]').analyze();
    expect(axe.violations).toEqual([]);
    await page.getByRole('button', { name: 'Load latest while keeping my input' }).click();
    await expect(page.locator('[data-note-body]')).toHaveValue(localConflictBody);
    await page.getByRole('button', { name: 'Save note' }).click();
    await expect(page.locator('[data-note-list]')).toContainText('Long review evidence');

    targetRow = page.locator(`[data-note-id="${targetNoteID}"]`);
    await targetRow.getByRole('button', { name: 'History' }).click();
    const noteHistory = page.locator('[data-note-history]');
    await expect(noteHistory.locator('[data-note-version]')).toHaveCount(3);
    axe = await new AxeBuilder({ page }).include('[data-note-history]').analyze();
    expect(axe.violations).toEqual([]);
    await noteHistory.locator('[data-note-version]').first().click();
    await expect(noteHistory.locator('[data-note-version-content]').first()).toContainText('Local conflict resolution');

    await page.locator('[data-note-body]').fill(`# Link source ${browserName}\n\nPoints to the saved target: `);
    await page.getByText('Insert evidence link').click();
    await page.locator('[data-note-link-type]').selectOption('note');
    await page.locator('[data-note-link-target]').fill(targetNoteID);
    await page.locator('[data-note-link-display]').fill(noteTitle);
    await page.getByRole('button', { name: 'Insert link' }).click();
    await expect(page.locator('[data-note-body]')).toHaveValue(new RegExp(`\\[\\[note:${targetNoteID}\\|${noteTitle}`));
    await expect(page.locator('[data-draft-status]')).toContainText('Browser draft saved');
    await page.getByRole('button', { name: 'Save note' }).click();
    const sourceRow = page.locator('[data-note-id]').filter({ hasText: `Link source ${browserName}` }).first();
    await expect(sourceRow).toBeVisible();
    const sourceNoteID = await sourceRow.getAttribute('data-note-id');
    expect(sourceNoteID).toMatch(/^\d+$/);
    if (!sourceNoteID) throw new Error('Source note ID is unavailable');

    targetRow = page.locator(`[data-note-id="${targetNoteID}"]`);
    await targetRow.getByRole('button', { name: 'Backlinks' }).click();
    await expect(targetRow.locator('[data-note-backlink-list]')).toContainText(`Link source ${browserName}`);
    await targetRow.locator('[data-note-backlink-list] a').click();
    await expect.poll(async () => (await viewerState(page)).note_id).toBe(sourceNoteID);

    targetRow = page.locator(`[data-note-id="${targetNoteID}"]`);
    page.once('dialog', (dialog) => dialog.accept());
    await targetRow.getByRole('button', { name: 'Remove' }).click();
    await expect(page.locator('[data-note-history]')).toContainText(`Note ${targetNoteID} history`);
    await page.locator('[data-note-history]').getByRole('button', { name: 'Restore previous content' }).click();
    await expect(page.locator(`[data-note-id="${targetNoteID}"]`)).toContainText('Long review evidence');

    await page.getByRole('tab', { name: 'PDF anchors' }).click();
    var anchorRow = page.locator('[data-anchor-id]').filter({ hasText: anchorLabel });
    await anchorRow.getByRole('button', { name: 'History' }).click();
    await expect(page.locator('.rw-anchor-history')).toContainText(`${anchorLabel} history`);
    page.once('dialog', (dialog) => dialog.accept());
    await anchorRow.getByRole('button', { name: 'Remove' }).click();
    await expect(page.locator('.rw-anchor-history')).toContainText('deleted');
    const anchorHistoryItems = page.locator('.rw-anchor-history li');
    const newestAnchorBox = await anchorHistoryItems.nth(0).boundingBox();
    const previousAnchorBox = await anchorHistoryItems.nth(1).boundingBox();
    expect(newestAnchorBox).not.toBeNull();
    expect(previousAnchorBox).not.toBeNull();
    if (!newestAnchorBox || !previousAnchorBox) throw new Error('Anchor history bounds are unavailable');
    expect(previousAnchorBox.y - newestAnchorBox.y - newestAnchorBox.height).toBeGreaterThanOrEqual(8);
    await page.locator('.rw-anchor-history').getByRole('button', { name: 'Restore anchor' }).click();
    anchorRow = page.locator('[data-anchor-id]').filter({ hasText: anchorLabel });
    await expect(anchorRow).toBeVisible();

    axe = await new AxeBuilder({ page }).include('[data-review-host]').analyze();
    expect(axe.violations).toEqual([]);
    await page.reload();
    await page.waitForLoadState('networkidle');
    await page.getByRole('tab', { name: 'Notes' }).click();
    const reloadedTarget = page.locator(`[data-note-id="${targetNoteID}"]`);
    await expect(reloadedTarget).toContainText('Long review evidence');
    await reloadedTarget.getByRole('button', { name: 'History' }).click();
    await page.locator('[data-note-history] [data-note-version]').first().click();
    await expect(page.locator('[data-note-history] [data-note-version-content]').first()).toContainText('Local conflict resolution');
    const auditHeading = page.getByRole("heading", {
      name: "Audit events",
      exact: true,
    });
    const articleAuditPanel = auditHeading.locator("xpath=../../..");
    await expect(articleAuditPanel).toContainText(/Note Version Created|Anchor Version Created/i);
  });

  test('restores and trashes a run through Home with persisted audit evidence', async ({ page, request }) => {
    await page.addInitScript(() => {
      sessionStorage.setItem('rw-viewer-state', JSON.stringify({ view: 'home', home_visibility: 'trashed' }));
    });
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    const run3 = page.locator('[data-home-run="3"]');
    await run3.getByRole('button', { name: 'Restore' }).click();
    const restoreDialog = page.getByRole('dialog', { name: 'Restore run 3?' });
    await expect(restoreDialog.getByRole('button', { name: 'Restore run' })).toBeFocused();
    await restoreDialog.getByRole('button', { name: 'Restore run' }).click();
    await expect(page.locator('[data-home-lifecycle-status]')).toContainText('Restored run 3');
    var contextResponse = await request.get('/api/runs/3/context');
    expect((await contextResponse.json()).run.visibility_state).toBe('active');

    await page.locator('[data-home-filters] select[name="visibility"]').selectOption('active');
    await page.locator('[data-home-filters] input[name="q"]').fill('');
    await page.locator('[data-home-filters]').getByRole('button', { name: 'Apply filters' }).click();
    const activeRun3 = page.locator('[data-home-run="3"]');
    await expect(activeRun3).toBeVisible();
    await activeRun3.getByRole('button', { name: 'Move to trash' }).click();
    const trashDialog = page.getByRole('dialog', { name: 'Move run 3 to trash?' });
    await trashDialog.getByLabel('Reason').fill('Browser lifecycle verification');
    await trashDialog.getByRole('button', { name: 'Move to trash' }).click();
    await expect(page.locator('[data-home-lifecycle-status]')).toContainText('Moved run 3');
    contextResponse = await request.get('/api/runs/3/context');
    expect((await contextResponse.json()).run.visibility_state).toBe('trashed');
    const audit = await request.get('/api/audit?run_id=3&action=run_trashed&limit=25');
    expect(audit.ok()).toBeTruthy();
    expect((await audit.json()).events.some((event: { action: string }) => event.action === 'run_trashed')).toBeTruthy();
  });

  test('traverses 101-record review collection boundaries through UI continuations', async ({ page, request, browserName }) => {
    test.skip(browserName !== 'chromium', 'The API boundary is browser-independent; the complete UI traversal runs once in Chromium.');
    test.setTimeout(180000);

    var reviewResponse = await request.get('/api/runs/1/articles/1/review');
    var review = await reviewResponse.json();
    var expectedReviewVersion = review.review?.version?.id ?? null;
    for (let index = 0; index < 101; index += 1) {
      reviewResponse = await request.put('/api/runs/1/articles/1/review', { data: {
        expected_version_id: expectedReviewVersion,
        status: index % 2 === 0 ? 'approved' : 'in_progress',
        sub_statuses: [],
        reason: `Boundary decision ${index.toString().padStart(3, '0')}`,
      } });
      expect(reviewResponse.ok()).toBeTruthy();
      review = await reviewResponse.json();
      expectedReviewVersion = review.review.version.id;
    }

    var response = await request.post('/api/runs/1/articles/1/notes', { data: { body: '# Boundary target\n\nInitial body.' } });
    expect(response.status()).toBe(201);
    var targetNote = (await response.json()).note;
    for (let index = 0; index < 101; index += 1) {
      response = await request.post(`/api/runs/1/notes/${targetNote.id}/versions`, { data: {
        expected_version_id: targetNote.version.id,
        state: 'active',
        body: `# Boundary target\n\nVersion ${index.toString().padStart(3, '0')}.`,
      } });
      expect(response.status()).toBe(201);
      targetNote = (await response.json()).note;
    }
    for (let index = 0; index < 101; index += 1) {
      response = await request.post('/api/runs/1/articles/1/notes', { data: {
        body: `# Boundary backlink ${index.toString().padStart(3, '0')}\n\n[[note:${targetNote.id}|Boundary target]]`,
      } });
      expect(response.status()).toBe(201);
    }

    response = await request.post('/api/runs/1/articles/1/anchors', { data: {
      label: 'boundary-anchor-target', page: 1, selected_text: 'Boundary anchor initial', rectangles: [{ x: 0.1, y: 0.1, width: 0.2, height: 0.05 }],
    } });
    expect(response.status()).toBe(201);
    var targetAnchor = (await response.json()).anchor;
    for (let index = 0; index < 101; index += 1) {
      response = await request.post(`/api/runs/1/anchors/${targetAnchor.id}/versions`, { data: {
        expected_version_id: targetAnchor.version.id,
        state: 'active',
        page: 1,
        selected_text: `Boundary anchor version ${index.toString().padStart(3, '0')}`,
        rectangles: [{ x: 0.1, y: 0.1, width: 0.2, height: 0.05 }],
      } });
      expect(response.status()).toBe(201);
      targetAnchor = (await response.json()).anchor;
    }
    for (let index = 0; index < 100; index += 1) {
      response = await request.post('/api/runs/1/articles/1/anchors', { data: {
        label: `boundary-anchor-${index.toString().padStart(3, '0')}`,
        page: 1,
        selected_text: `Boundary list anchor ${index}`,
        rectangles: [{ x: 0.1, y: 0.1, width: 0.2, height: 0.05 }],
      } });
      expect(response.status()).toBe(201);
    }

    await visitArticle(page);
    await page.getByRole('button', { name: 'Show version history' }).click();
    await expect(page.locator('[data-review-history-list] li')).toHaveCount(25);
    await exhaustContinuation(page, '[data-review-history-more]');
    const decisionVersions = await page.locator('[data-review-history-list] li').allTextContents();
    expect(decisionVersions.length).toBeGreaterThanOrEqual(101);
    expect(new Set(decisionVersions).size).toBe(decisionVersions.length);

    await page.getByRole('tab', { name: 'Notes' }).click();
    await expect(page.locator('[data-note-list] [data-note-id]')).toHaveCount(25);
    await exhaustContinuation(page, '[data-note-load-more]');
    const noteIDs = await page.locator('[data-note-list] [data-note-id]').evaluateAll((items) => items.map((item) => item.dataset.noteId));
    expect(noteIDs.length).toBeGreaterThanOrEqual(102);
    expect(new Set(noteIDs).size).toBe(noteIDs.length);
    const boundaryNoteRow = page.locator(`[data-note-id="${targetNote.id}"]`);
    await boundaryNoteRow.getByRole('button', { name: 'History' }).click();
    await expect(page.locator('[data-note-history] [data-note-version]')).toHaveCount(25);
    await exhaustContinuation(page, '[data-note-history-more]');
    const noteVersions = await page.locator('[data-note-history] [data-note-version]').allTextContents();
    expect(noteVersions.length).toBe(102);
    expect(new Set(noteVersions).size).toBe(noteVersions.length);
    await boundaryNoteRow.getByRole('button', { name: 'Backlinks' }).click();
    await expect(boundaryNoteRow.locator('[data-note-backlink-list] li')).toHaveCount(25);
    await exhaustContinuation(page, '[data-note-backlink-list] [data-backlinks-more]');
    const backlinks = await boundaryNoteRow.locator('[data-note-backlink-list] li').allTextContents();
    expect(backlinks).toHaveLength(101);
    expect(new Set(backlinks).size).toBe(101);

    await page.getByRole('tab', { name: 'PDF anchors' }).click();
    await expect(page.locator('[data-anchor-list] [data-anchor-id]')).toHaveCount(25);
    await exhaustContinuation(page, '[data-anchor-load-more]');
    const anchorIDs = await page.locator('[data-anchor-list] [data-anchor-id]').evaluateAll((items) => items.map((item) => item.dataset.anchorId));
    expect(anchorIDs.length).toBeGreaterThanOrEqual(101);
    expect(new Set(anchorIDs).size).toBe(anchorIDs.length);
    const boundaryAnchorRow = page.locator(`[data-anchor-id="${targetAnchor.id}"]`);
    await boundaryAnchorRow.getByRole('button', { name: 'History' }).click();
    await expect(page.locator('.rw-anchor-history li')).toHaveCount(25);
    await exhaustContinuation(page, '[data-anchor-history-more]');
    const anchorVersions = await page.locator('.rw-anchor-history li').allTextContents();
    expect(anchorVersions.length).toBe(102);
    expect(new Set(anchorVersions).size).toBe(anchorVersions.length);
  });
});
