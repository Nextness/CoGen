// @ts-check
const { test, expect } = require('@playwright/test');

test.describe.configure({ mode: 'serial' });

test.describe('isolated review mutation lifecycle', () => {
  test('persists status, note, anchor, and custom PDF rendering across reload', async ({ page, request, browserName }) => {
    const context = await request.get('/api/runs/1/review-context');
    expect(context.ok()).toBeTruthy();
    if (!(await context.json()).context_initialized) {
      await page.goto('/?view=article&search_id=1&search_revision_id=1&plan_id=1&run_id=1&article_id=1');
      await page.waitForLoadState('networkidle');
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
    const current = await request.get('/api/runs/1/articles/1/review');
    expect(current.ok()).toBeTruthy();
    const currentReview = await current.json();
    const reason = `Browser ${browserName} review evidence`;
    const saved = await request.put('/api/runs/1/articles/1/review', { data: { expected_version_id: currentReview.review?.version?.id ?? null, status: 'approved', sub_statuses: [], reason } });
    expect(saved.ok()).toBeTruthy();
    const note = await request.post('/api/runs/1/articles/1/notes', { data: { body: `See [[pdf:page=2|${browserName} results page]] and [[note:999|unresolved note]].` } });
    expect(note.status()).toBe(201);
    const anchorLabel = `methods-${browserName}`;
    const anchors = await request.get('/api/runs/1/articles/1/anchors?limit=100');
    expect(anchors.ok()).toBeTruthy();
    if (!(await anchors.json()).anchors.some((anchor) => anchor.label === anchorLabel)) {
      const anchor = await request.post('/api/runs/1/articles/1/anchors', { data: { label: anchorLabel, page: 1, selected_text: 'Selectable fixture methods', rectangles: [{ x: 0.1, y: 0.1, width: 0.4, height: 0.08 }] } });
      expect(anchor.status()).toBe(201);
    }

    await page.goto('/?view=article&search_id=1&search_revision_id=1&plan_id=1&run_id=1&article_id=1');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('[data-review-host]')).toContainText('Approved');
    const reviewAuditEvents = page.locator('.rw-article-audit-panel .rw-audit-event--review');
    const reviewAuditCount = await reviewAuditEvents.count();
    await expect(page.locator('.rw-article-audit-panel')).toContainText('Work Review Version Created');
    const updatedReason = `${reason} saved from the review form`;
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
    const readingWorkspaceGap = await page.locator('.rw-reading-workspace').evaluate((workspace) => workspace.nextElementSibling.getBoundingClientRect().top - workspace.getBoundingClientRect().bottom);
    expect(readingWorkspaceGap).toBeGreaterThanOrEqual(15);
    await decisionTab.focus();
    await page.keyboard.press('ArrowRight');
    await expect(page.getByRole('tab', { name: 'Notes' })).toHaveAttribute('aria-selected', 'true');
    await page.getByText('Note syntax and link examples').click();
    await expect(page.locator('.rw-note-syntax')).toContainText('[[article:10.1000/example|Article title]]');
    const noteFormGaps = await page.locator('[data-note-form]').evaluate((form) => {
      const heading = form.querySelector('.rw-note-form__heading').getBoundingClientRect();
      const field = form.querySelector('.ui.field').getBoundingClientRect();
      const actions = form.querySelector('.rw-review-actions').getBoundingClientRect();
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
      const workspace = notes.querySelector('.rw-note-workspace').getBoundingClientRect();
      const saved = notes.querySelector('.rw-note-saved').getBoundingClientRect();
      return saved.top - workspace.bottom;
    });
    expect(noteSectionOrder).toBeGreaterThanOrEqual(15);
    await page.getByRole('tab', { name: 'PDF anchors' }).click();
    await expect(page.locator('[data-anchor-list]')).toContainText(anchorLabel);
    const anchorBacklinkGap = await page.locator('.rw-anchor-panel').evaluate((panel) => {
      const list = panel.querySelector('[data-anchor-list]').getBoundingClientRect();
      const backlinks = panel.querySelector('.rw-review-history').getBoundingClientRect();
      return backlinks.top - list.bottom;
    });
    expect(anchorBacklinkGap).toBeGreaterThanOrEqual(15);
    await expect(page.locator('.rw-pdf-page--current canvas')).toBeVisible();
    await expect(page.locator('.rw-pdf-page--current .textLayer')).toContainText('Selectable fixture methods');
    await expect(page.locator('.rw-pdf-page')).toHaveCount(1);
    await page.getByRole('button', { name: 'Next PDF page' }).click();
    await expect(page.locator('.rw-pdf-page')).toHaveCount(1);
    await expect(page.locator('.rw-pdf-page--current')).toHaveAttribute('data-pdf-page-number', '2');
    await expect(page.locator('.rw-pdf-page--current .textLayer')).toContainText('Selectable fixture conclusions');
    await expect(page.locator('[data-pdf-status]')).toHaveText('PDF page 2 of 2.');
    const containment = await page.locator('.rw-pdf-viewer').evaluate((viewer) => {
      const viewport = viewer.querySelector('.rw-pdf-pages');
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
});
