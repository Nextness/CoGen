// @ts-check
const { test, expect } = require('@playwright/test');
const AxeBuilder = require('@axe-core/playwright').default;

const context = {
  search_id: '1',
  search_revision_id: '1',
  plan_id: '1',
  run_id: '1',
};

/** Builds a fixture viewer URL. */
function url(overrides = {}) {
  return `/?${new URLSearchParams({ ...context, ...overrides })}`;
}

/** Navigates to a UI-quality fixture state. */
async function visit(page, overrides = {}) {
  await page.goto(url(overrides));
  await page.waitForLoadState('networkidle');
}

/** Asynchronously implements expect no page overflow for the viewer. */
async function expectNoPageOverflow(page) {
  const dimensions = await page.evaluate(() => ({
    viewport: window.innerWidth,
    document: document.documentElement.scrollWidth,
    body: document.body.scrollWidth,
    offenders: Array.from(document.querySelectorAll('body *')).map((element) => {
      const rect = element.getBoundingClientRect();
      return { element: element.tagName.toLowerCase() + (element.className ? `.${String(element.className).trim().replaceAll(' ', '.')}` : ''), left: Math.round(rect.left), right: Math.round(rect.right), width: Math.round(rect.width) };
    }).filter((entry) => entry.left < -1 || entry.right > window.innerWidth + 1 || entry.width > window.innerWidth + 1).slice(0, 8),
  }));
  const detail = dimensions.offenders.length ? `; offenders ${JSON.stringify(dimensions.offenders)}` : '';
  expect(dimensions.document, `document width ${dimensions.document} exceeds viewport ${dimensions.viewport}${detail}`).toBeLessThanOrEqual(dimensions.viewport);
  expect(dimensions.body, `body width ${dimensions.body} exceeds viewport ${dimensions.viewport}${detail}`).toBeLessThanOrEqual(dimensions.viewport);
}

test.describe('Research-context and responsive behavior', () => {
  test('detail breadcrumbs remain concise and identify the parent collection', async ({ page }) => {
    await visit(page, { view: 'article', article_id: '1' });
    const breadcrumb = page.getByRole('navigation', { name: 'Breadcrumb' });
    await expect(breadcrumb).toContainText('Home');
    await expect(breadcrumb).toContainText('Deepdive');
    await expect(breadcrumb).toContainText('Corpus');
    await expect(breadcrumb).toContainText('Analysis-ready articles');
    await expect(breadcrumb).toContainText('10.1000/1');
    await expect(breadcrumb).not.toContainText('deep-learning-nlp');
    await expect(page.getByRole('link', { name: 'Back to Corpus' })).toHaveCount(0);
  });

  test('article reading and review share the desktop workspace and stack on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await visit(page, { view: 'article', article_id: '1' });
    const desktopBoxes = await page.locator('.rw-reading-workspace > div').evaluateAll(function(items) {
      return items.map(function(item) {
        const rect = item.getBoundingClientRect();
        return { x: rect.x, width: rect.width };
      });
    });
    expect(desktopBoxes).toHaveLength(2);
    expect(desktopBoxes[1].x).toBeGreaterThan(desktopBoxes[0].x + desktopBoxes[0].width);
    expect(Math.abs(desktopBoxes[0].width - desktopBoxes[1].width)).toBeLessThan(2);

    await page.setViewportSize({ width: 375, height: 812 });
    await expectNoPageOverflow(page);
    const mobileBoxes = await page.locator('.rw-reading-workspace > div').evaluateAll(function(items) {
      return items.map(function(item) { return item.getBoundingClientRect().x; });
    });
    expect(Math.abs(mobileBoxes[0] - mobileBoxes[1])).toBeLessThan(2);
  });

  test('mobile and medium layouts fit the viewport while tables retain their own scroller', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await visit(page, { view: 'overview' });
    await expectNoPageOverflow(page);
    // At 375px, grid items wrap to a single column
    var itemWidth = await page.locator('.selection-grid > .ui.field').first().evaluate(function(el) {
      return el.getBoundingClientRect().width;
    });
    expect(itemWidth).toBeLessThan(400);

    await visit(page, { view: 'corpus', section: 'articles' });
    await expectNoPageOverflow(page);
    await expect(page.locator('.table-wrap').first()).toHaveCSS('overflow-x', 'auto');

    await visit(page, { view: 'provenance', section: 'audit' });
    await page.getByText('Recorded data').first().click();
    await expectNoPageOverflow(page);

    await visit(page, { view: 'article', article_id: '1' });
    await expectNoPageOverflow(page);
    await expect(page.locator('.rw-pdf-page')).toHaveCount(1);
    await page.getByRole('button', { name: 'Start review' }).click();
    await expectNoPageOverflow(page);
    await page.getByRole('button', { name: 'Close review setup' }).click();

    await page.setViewportSize({ width: 768, height: 1024 });
    await visit(page, { view: 'overview' });
    await expectNoPageOverflow(page);
    // At 768px, grid items should be narrower than the full viewport (multiple columns)
    var itemWidth768 = await page.locator('.selection-grid > .ui.field').first().evaluate(function(el) {
      return el.getBoundingClientRect().width;
    });
    expect(itemWidth768).toBeLessThan(400);

    await visit(page, { view: 'article', article_id: '1' });
    await expectNoPageOverflow(page);
    await expect(page.locator('.rw-pdf-page')).toHaveCount(1);
    await expect(page.locator('.rw-pdf-pages')).toHaveCSS('overflow', 'auto');
  });

  test('skip link, errors, and reduced motion are announced', async ({ page }) => {
    await visit(page, { view: 'overview' });
    // Skip link is first focusable element
    await page.keyboard.press('Tab');
    await expect(page.locator('.skip-link')).toBeFocused();

    // Error state — injected failure shows alert
    await page.route('**/api/overview?*', (route) => route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ error: { message: 'Injected overview failure' } }),
    }));
    await visit(page, { view: 'overview' });
    await expect(page.locator('#notice')).toHaveAttribute('role', 'alert');
    await expect(page.locator('#notice')).toContainText('Injected overview failure');

    // Reduced motion — graph layout respects prefers-reduced-motion
    await page.unroute('**/api/overview?*');
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await visit(page, { view: 'relationships' });
    await expect(page.locator('#graph-layout-status')).toHaveText('Physics layout placed with reduced motion');
  });
});

test.describe('Automated accessibility checks', () => {
  for (const [name, overrides] of [
    ['home', { view: 'home' }],
    ['overview', { view: 'overview' }],
    ['corpus', { view: 'corpus', section: 'articles' }],
    ['article detail', { view: 'article', article_id: '1' }],
    ['relationships', { view: 'relationships' }],
    ['provenance audit', { view: 'provenance', section: 'audit' }],
    ['provenance artifacts', { view: 'provenance', section: 'artifacts' }],
    ['provenance stages', { view: 'provenance', section: 'stages' }],
    ['evaluation', { view: 'evaluation' }],
  ]) {
    test(`${name} has no axe violations`, async ({ page }) => {
      await visit(page, overrides);
      const results = await new AxeBuilder({ page }).analyze();
      expect(results.violations).toEqual([]);
    });
  }

  test('review setup dialog has no axe violations and can be dismissed', async ({ page }) => {
    await visit(page, { view: 'article', article_id: '1' });
    await page.getByRole('button', { name: 'Start review' }).click();
    const dialog = page.getByRole('dialog', { name: 'Start article review' });
    await expect(dialog).toBeVisible();
    const results = await new AxeBuilder({ page }).include('[data-review-dialog]').analyze();
    expect(results.violations).toEqual([]);
    await dialog.getByRole('button', { name: 'Close review setup' }).click();
    await expect(dialog).toBeHidden();
  });
});

test.describe('Visual regression', () => {
  test.skip(({ browserName }) => browserName !== 'chromium', 'Visual baselines are maintained for Chromium only.');
  test.use({ viewport: { width: 1280, height: 900 }, colorScheme: 'light', reducedMotion: 'reduce' });

  for (const [name, overrides] of [
    ['home', { view: 'home' }],
    ['overview', { view: 'overview' }],
    ['corpus', { view: 'corpus', section: 'articles' }],
    ['article-detail', { view: 'article', article_id: '1' }],
    ['relationships', { view: 'relationships' }],
    ['provenance-audit', { view: 'provenance', section: 'audit' }],
    ['provenance-stages', { view: 'provenance', section: 'stages' }],
    ['evaluation', { view: 'evaluation' }],
  ]) {
    test(`${name} light`, async ({ page }) => {
      await visit(page, overrides);
      if (name === 'relationships') {
        await expect(page.locator('#graph-layout-status')).toContainText(/settled|placed/);
      }
      await expect(page).toHaveScreenshot(`${name}-light.png`, { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
    });
  }

  test('overview dark', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
    await visit(page, { view: 'overview' });
    await expect(page).toHaveScreenshot('overview-dark.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });

  test('provenance audit dark', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
    await visit(page, { view: 'provenance', section: 'audit' });
    await expect(page).toHaveScreenshot('provenance-audit-dark.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });

  test('article review setup light', async ({ page }) => {
    await visit(page, { view: 'article', article_id: '1' });
    await page.getByRole('button', { name: 'Start review' }).click();
    await expect(page.getByRole('dialog', { name: 'Start article review' })).toBeVisible();
    await expect(page).toHaveScreenshot('article-review-setup-light.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });

  test('article review setup dark', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
    await visit(page, { view: 'article', article_id: '1' });
    await page.getByRole('button', { name: 'Start review' }).click();
    await expect(page.getByRole('dialog', { name: 'Start article review' })).toBeVisible();
    await expect(page).toHaveScreenshot('article-review-setup-dark.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });

  test('artifact preview light', async ({ page }) => {
    await visit(page, { view: 'provenance', section: 'artifacts' });
    await page.getByRole('button', { name: 'Inspect preview' }).first().click();
    await expect(page.locator('#artifact-inspector')).toContainText('Bytes shown');
    await expect(page).toHaveScreenshot('artifact-preview-light.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });
});
