import AxeBuilder from '@axe-core/playwright';
import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';

import { visit } from './support/visit.ts';

/** URL-owned state used to open one viewer presentation. */
type ViewState = Record<string, string>;

/** One named viewer state used by a parameterized browser check. */
type NamedViewState = readonly [string, ViewState];

const context: Record<string, string> = {
  search_id: '1',
  search_revision_id: '1',
  plan_id: '1',
  run_id: '1',
};

/** Navigates to a UI-quality fixture state through sessionStorage seeding. */
async function visitQuality(page: Page, overrides: Record<string, string> = {}): Promise<void> {
  const state = { ...context, ...overrides };
  await visit(page, state);
}

/** Asynchronously implements expect no page overflow for the viewer. */
async function expectNoPageOverflow(page: Page): Promise<void> {
  const dimensions = await page.evaluate(() => ({
    viewport: window.innerWidth,
    document: document.documentElement.scrollWidth,
    offenders: Array.from(document.querySelectorAll('body *')).map((element) => {
      const rect = element.getBoundingClientRect();
      return { element: element.tagName.toLowerCase() + (element.className ? `.${String(element.className).trim().replaceAll(' ', '.')}` : ''), left: Math.round(rect.left), right: Math.round(rect.right), width: Math.round(rect.width) };
    }).filter((entry) => entry.left < -1 || entry.right > window.innerWidth + 1 || entry.width > window.innerWidth + 1).slice(0, 8),
  }));
  const detail = dimensions.offenders.length ? `; offenders ${JSON.stringify(dimensions.offenders)}` : '';
  expect(dimensions.document, `document width ${dimensions.document} exceeds viewport ${dimensions.viewport}${detail}`).toBeLessThanOrEqual(dimensions.viewport);
}

test.describe('Research-context and responsive behavior', () => {
  test('detail breadcrumbs remain concise and identify the parent collection', async ({ page }) => {
    await visitQuality(page, { view: 'article', article_id: '1' });
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
    await visitQuality(page, { view: 'article', article_id: '1' });
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
    await visitQuality(page, { view: 'overview' });
    await expectNoPageOverflow(page);
    // At 375px, grid items wrap to a single column
    var itemWidth = await page.locator('.rw-context-grid > .ui.field').first().evaluate(function(el) {
      return el.getBoundingClientRect().width;
    });
    expect(itemWidth).toBeLessThan(400);

    await visitQuality(page, { view: 'corpus', section: 'articles' });
    await expectNoPageOverflow(page);
    await expect(page.locator('.table-wrap').first()).toHaveCSS('overflow-x', 'auto');

    await visitQuality(page, { view: 'provenance', section: 'audit' });
    await page.getByText('Recorded data').first().click();
    await expectNoPageOverflow(page);

    await visitQuality(page, { view: 'article', article_id: '1' });
    await expectNoPageOverflow(page);
    await expect(page.locator('.rw-pdf-page')).toHaveCount(1);
    await page.getByRole('button', { name: 'Start review' }).click();
    await expectNoPageOverflow(page);
    await page.getByRole('button', { name: 'Close review setup' }).click();

    await page.setViewportSize({ width: 768, height: 1024 });
    await visitQuality(page, { view: 'overview' });
    await expectNoPageOverflow(page);
    // At 768px, grid items should be narrower than the full viewport (multiple columns)
    var itemWidth768 = await page.locator('.rw-context-grid > .ui.field').first().evaluate(function(el) {
      return el.getBoundingClientRect().width;
    });
    expect(itemWidth768).toBeLessThan(400);

    await visitQuality(page, { view: 'article', article_id: '1' });
    await expectNoPageOverflow(page);
    await expect(page.locator('.rw-pdf-page')).toHaveCount(1);
    await expect(page.locator('.rw-pdf-pages')).toHaveCSS('overflow', 'auto');
  });

  test('320px and short-landscape layouts keep controls and evidence reachable', async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 568 });
    const responsiveStates: ViewState[] = [
      { view: 'home' },
      { view: 'evaluation' },
      { view: 'provenance', section: 'audit' },
      { view: 'article', article_id: '1' },
    ];
    for (const overrides of responsiveStates) {
      await visitQuality(page, overrides);
      await expectNoPageOverflow(page);
    }
    await page.locator('#search-select-trigger').click();
    await expect(page.getByLabel('Search available searches')).toBeVisible();
    await expectNoPageOverflow(page);

    await page.setViewportSize({ width: 667, height: 320 });
    await visitQuality(page, { view: 'relationships' });
    await expectNoPageOverflow(page);
    await expect(page.getByRole('button', { name: /Expand graph|Restore graph/ })).toBeVisible();
  });

  test('200 percent reflow, text spacing, and focused-input viewport changes preserve actions', async ({ page }) => {
    await page.setViewportSize({ width: 640, height: 800 });
    await visitQuality(page, { view: 'evaluation' });
    await page.evaluate(function() { document.documentElement.style.zoom = '2'; });
    await expectNoPageOverflow(page);
    await expect(page.locator('[data-evaluation-filters]').getByRole('button', { name: 'Apply filters' })).toBeVisible();

    await page.evaluate(function() {
      document.documentElement.style.zoom = '';
      const style = document.createElement('style');
      style.dataset.testTextSpacing = '';
      style.textContent = 'body { line-height: 1.5 !important; letter-spacing: .12em !important; word-spacing: .16em !important; } p { margin-bottom: 2em !important; }';
      document.head.append(style);
    });
    await visitQuality(page, { view: 'article', article_id: '1' });
    await expectNoPageOverflow(page);

    await page.setViewportSize({ width: 320, height: 420 });
    await visitQuality(page, { view: 'provenance', section: 'artifacts' });
    const search = page.getByLabel('Search artifacts');
    await search.focus();
    await search.fill('manifest');
    await expect(search).toBeFocused();
    await expect(page.locator('[data-artifact-filters]').getByRole('button', { name: 'Apply filters' })).toBeVisible();
    await expectNoPageOverflow(page);
  });

  test('skip link, errors, and reduced motion are announced', async ({ page }) => {
    // Home boots without programmatic title focus, so the first Tab stop is the skip link.
    await visitQuality(page, { view: 'home' });
    await page.keyboard.press('Tab');
    await expect(page.locator('.skip-link')).toBeFocused();

    // Error state — injected failure shows alert
    await page.route('**/api/overview?*', (route) => route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ error: { message: 'Injected overview failure' } }),
    }));
    await visitQuality(page, { view: 'overview' });
    await expect(page.locator('#notice')).toHaveAttribute('role', 'alert');
    await expect(page.locator('#notice')).toContainText('Injected overview failure');

    // Reduced motion — graph layout respects prefers-reduced-motion
    await page.unroute('**/api/overview?*');
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await visitQuality(page, { view: 'relationships' });
    await expect(page.locator('#graph-layout-status')).toHaveText('Physics layout placed with reduced motion');
  });
});

test.describe('Automated accessibility checks', () => {
  test('the overview page document has no axe violations', async ({ page }) => {
    await visitQuality(page, { view: 'overview' });

    await expect(page.locator('meta[name="rw-page"]')).toHaveAttribute('content', 'overview');
    await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible();
    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
  });

  const accessibilityStates: NamedViewState[] = [
    ['home', { view: 'home' }],
    ['overview', { view: 'overview' }],
    ['corpus', { view: 'corpus', section: 'articles' }],
    ['article detail', { view: 'article', article_id: '1' }],
    ['relationships', { view: 'relationships' }],
    ['provenance audit', { view: 'provenance', section: 'audit' }],
    ['provenance artifacts', { view: 'provenance', section: 'artifacts' }],
    ['provenance cache', { view: 'provenance', section: 'cache' }],
    ['provenance stages', { view: 'provenance', section: 'stages' }],
    ['provenance run', { view: 'provenance', section: 'run' }],
    ['evaluation', { view: 'evaluation' }],
    ['advanced', { view: 'advanced' }],
    ['author detail', { view: 'author', author_id: '1' }],
    ['reference detail', { view: 'reference', reference_id: '1' }],
  ];
  for (const [name, overrides] of accessibilityStates) {
    test(`${name} has no axe violations`, async ({ page }) => {
      await visitQuality(page, overrides);
      const results = await new AxeBuilder({ page }).analyze();
      expect(results.violations).toEqual([]);
    });
  }

  test('review setup dialog has no axe violations and can be dismissed', async ({ page }) => {
    await visitQuality(page, { view: 'article', article_id: '1' });
    await page.getByRole('button', { name: 'Start review' }).click();
    const dialog = page.getByRole('dialog', { name: 'Start article review' });
    await expect(dialog).toBeVisible();
    const results = await new AxeBuilder({ page }).include('[data-review-dialog]').analyze();
    expect(results.violations).toEqual([]);
    await dialog.getByRole('button', { name: 'Close review setup' }).click();
    await expect(dialog).toBeHidden();
  });

  test('open context selector and mobile navigation remain keyboard-accessible', async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 568 });
    await visitQuality(page, { view: 'overview' });
    await page.locator('#search-select-trigger').click();
    await expect(page.getByLabel('Search available searches')).toBeFocused();
    var results = await new AxeBuilder({ page }).include('.rw-context-panel').analyze();
    expect(results.violations).toEqual([]);
    await page.keyboard.press('Escape');
    await expect(page.locator('#search-select-trigger')).toBeFocused();
    await page.getByRole('button', { name: 'Menu' }).click();
    await expect(page.getByRole('navigation', { name: 'Deepdive navigation' })).toBeVisible();
    results = await new AxeBuilder({ page }).include('header').analyze();
    expect(results.violations).toEqual([]);
  });

  test('expanded graph remains keyboard-accessible and restores opener focus', async ({ page }) => {
    await visitQuality(page, { view: 'relationships' });
    await page.locator('#graph-viewport').evaluate((viewport) => {
      Object.defineProperty(viewport, 'requestFullscreen', { value: undefined, configurable: true });
    });
    const expand = page.getByRole('button', { name: 'Expand graph' });
    await expand.click();
    await expect(page.locator('#graph-viewport')).toHaveClass(/rw-graph__viewport--expanded/);
    await expect(page.getByRole('button', { name: 'Restore graph' })).toBeVisible();
    const results = await new AxeBuilder({ page }).include('#graph-viewport').analyze();
    expect(results.violations).toEqual([]);
    await page.keyboard.press('Escape');
    await expect(page.locator('#graph-viewport')).not.toHaveClass(/rw-graph__viewport--expanded/);
    await expect(expand).toBeFocused();
  });

  test('artifact truncation and error states remain explicit and accessible', async ({ page }) => {
    await page.route('**/api/artifacts/*/inspect?*', (route) => route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: {
        artifact_id: 1,
        content: '{"bounded":"prefix"',
        format: 'json',
        truncated: true,
        byte_size: 131072,
        stored_byte_size: 131072,
        preview_byte_size: 65536,
        content_type: 'application/json',
      } }),
    }));
    await visitQuality(page, { view: 'provenance', section: 'artifacts' });
    await page.getByRole('button', { name: 'Inspect preview' }).first().click();
    await expect(page.locator('#artifact-inspector')).toContainText('Preview truncated');
    await expect(page.locator('#artifact-inspector')).toContainText('Formatted JSON is unavailable');
    var results = await new AxeBuilder({ page }).include('#artifact-inspector').analyze();
    expect(results.violations).toEqual([]);

    await page.unroute('**/api/artifacts/*/inspect?*');
    await page.route('**/api/artifacts/*/inspect?*', (route) => route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ error: { code: 'injected_failure', message: 'Injected artifact preview failure' } }),
    }));
    await page.getByRole('button', { name: 'Inspect preview' }).nth(1).click();
    await expect(page.locator('#artifact-inspector')).toContainText('Preview unavailable');
    await expect(page.locator('#artifact-inspector')).toContainText('Injected artifact preview failure');
    results = await new AxeBuilder({ page }).include('#artifact-inspector').analyze();
    expect(results.violations).toEqual([]);
  });
});

test.describe('Visual regression', () => {
  test.skip(({ browserName }) => browserName !== 'chromium', 'Visual baselines are maintained for Chromium only.');
  test.use({ viewport: { width: 1280, height: 900 }, colorScheme: 'light' });

  const visualStates: NamedViewState[] = [
    ['home', { view: 'home' }],
    ['overview', { view: 'overview' }],
    ['corpus', { view: 'corpus', section: 'articles' }],
    ['article-detail', { view: 'article', article_id: '1' }],
    ['relationships', { view: 'relationships' }],
    ['provenance-audit', { view: 'provenance', section: 'audit' }],
    ['provenance-stages', { view: 'provenance', section: 'stages' }],
    ['evaluation', { view: 'evaluation' }],
  ];
  for (const [name, overrides] of visualStates) {
    test(`${name} light`, async ({ page }) => {
      await visitQuality(page, overrides);
      if (name === 'relationships') {
        await expect(page.locator('#graph-layout-status')).toContainText(/settled|placed/);
      }
      await expect(page).toHaveScreenshot(`${name}-light.png`, { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
    });
  }

  test('overview dark', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
    await visitQuality(page, { view: 'overview' });
    await expect(page).toHaveScreenshot('overview-dark.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });

  test('provenance audit dark', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
    await visitQuality(page, { view: 'provenance', section: 'audit' });
    await expect(page).toHaveScreenshot('provenance-audit-dark.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });

  test('article review setup light', async ({ page }) => {
    await visitQuality(page, { view: 'article', article_id: '1' });
    await page.getByRole('button', { name: 'Start review' }).click();
    await expect(page.getByRole('dialog', { name: 'Start article review' })).toBeVisible();
    await expect(page).toHaveScreenshot('article-review-setup-light.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });

  test('article review setup dark', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
    await visitQuality(page, { view: 'article', article_id: '1' });
    await page.getByRole('button', { name: 'Start review' }).click();
    await expect(page.getByRole('dialog', { name: 'Start article review' })).toBeVisible();
    await expect(page).toHaveScreenshot('article-review-setup-dark.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });

  test('artifact preview light', async ({ page }) => {
    await visitQuality(page, { view: 'provenance', section: 'artifacts' });
    await page.getByRole('button', { name: 'Inspect preview' }).first().click();
    await expect(page.locator('#artifact-inspector')).toContainText('Bytes shown');
    await expect(page).toHaveScreenshot('artifact-preview-light.png', { fullPage: true, animations: 'disabled', caret: 'hide', timeout: 10000 });
  });
});
