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
    const anchorID = `methods-${browserName}`;
    const anchors = await request.get('/api/runs/1/articles/1/anchors?limit=100');
    expect(anchors.ok()).toBeTruthy();
    if (!(await anchors.json()).anchors.some((anchor) => anchor.id === anchorID)) {
      const anchor = await request.post('/api/runs/1/articles/1/anchors', { data: { anchor_id: anchorID, page: 1, selected_text: 'Selectable fixture methods', rectangles: [{ x: 0.1, y: 0.1, width: 0.4, height: 0.08 }] } });
      expect(anchor.status()).toBe(201);
    }

    await page.goto('/?view=article&search_id=1&search_revision_id=1&plan_id=1&run_id=1&article_id=1');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('[data-review-host]')).toContainText('Approved');
    const reviewPanelGap = await page.locator('[data-review-host]').evaluate((host) => host.nextElementSibling.getBoundingClientRect().top - host.getBoundingClientRect().bottom);
    expect(reviewPanelGap).toBeGreaterThanOrEqual(15);
    await page.getByRole('button', { name: 'Notes' }).click();
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
    await expect(browserNote.locator('[aria-label="Unresolved link"]')).toBeVisible();
    await page.getByRole('button', { name: 'PDF anchors' }).click();
    await expect(page.locator('[data-anchor-list]')).toContainText(anchorID);
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
    await expect(page.locator('[data-review-host]')).toContainText(reason);
    await expect(page.locator('[data-anchor-list]')).toContainText(anchorID);
  });
});
