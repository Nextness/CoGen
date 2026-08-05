// @ts-check
const { test, expect } = require('@playwright/test');

test.describe.configure({ mode: 'serial' });

test.describe('isolated review mutation lifecycle', () => {
  test('persists status, note, anchor, and custom PDF rendering across reload', async ({ page, request, browserName }) => {
    const context = await request.get('/api/runs/1/review-context');
    expect(context.ok()).toBeTruthy();
    if (!(await context.json()).context_initialized) {
      const initialized = await request.post('/api/runs/1/review-context', { data: { parent_context_id: null } });
      expect(initialized.status()).toBe(201);
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
    const browserNote = page.locator('[data-note-list] p').filter({ hasText: `${browserName} results page` });
    await expect(browserNote).toContainText('unresolved note');
    await expect(browserNote.locator('[aria-label="Unresolved link"]')).toBeVisible();
    await expect(page.locator('[data-anchor-list]')).toContainText(anchorID);
    await expect(page.locator('.rw-pdf-page--current canvas')).toBeVisible();
    await expect(page.locator('.rw-pdf-page--current .textLayer')).toContainText('Selectable fixture methods');

    await page.reload();
    await page.waitForLoadState('networkidle');
    await expect(page.locator('[data-review-host]')).toContainText(reason);
    await expect(page.locator('[data-anchor-list]')).toContainText(anchorID);
  });
});
