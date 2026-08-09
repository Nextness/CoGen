// @ts-check
// dom-capture.spec.cjs is the Phase 3 equivalence capture spec. It loads a
// representative set of viewer routes, waits for each route's rendered
// content plus the health status and a settle tick, then serializes
// document.body.innerHTML verbatim into a JSON file per run. It never writes
// to the database. A baseline run (ASSETS_DIR=frontend/dist) and a candidate
// run (ASSETS_DIR=frontend/dist-ts) produce two files that compare-dom.mjs
// diffs.
const { test, expect } = require('@playwright/test');
const { mkdir, writeFile } = require('node:fs/promises');
const path = require('node:path');

// Fixture identifiers mirror viewer.spec.cjs: search 1 / revision 1 / plan 1 /
// run 1 (completed deep-learning-nlp run) and article 1 (available PDF).
const CONTEXT = 'search_id=1&search_revision_id=1&plan_id=1&run_id=1';

// ---------------------------------------------------------------------------
// Capture helpers
// ---------------------------------------------------------------------------

/**
 * Resolves the repository root from this spec file's location.
 */
function repoRoot() {
  return path.resolve(__dirname, '..', '..');
}

/**
 * Picks the output JSON path from the served tree in ASSETS_DIR.
 */
function capturePath() {
  const tree = capturedTree();
  return path.join(repoRoot(), 'build', 'playwright', `dom-capture-${tree}.json`);
}

/**
 * Derives the served tree name from the ASSETS_DIR environment variable.
 */
function capturedTree() {
  const assets = process.env.ASSETS_DIR || '';
  return path.basename(assets) === 'dist-ts' ? 'dist-ts' : 'dist';
}

/**
 * Waits for network quiet and universal rendered state on a route.
 */
async function settle(page, url) {
  await page.goto(url, { waitUntil: 'networkidle' });
  await expect(page.locator('#health-status')).toHaveText('Database healthy');
  await page.waitForTimeout(150);
}

/**
 * Captures the current document body HTML verbatim.
 */
async function captureBody(page) {
  return page.evaluate(() => document.body.innerHTML);
}

// ---------------------------------------------------------------------------
// Route definitions (name, URL, and a route-specific steady-state assertion)
// ---------------------------------------------------------------------------

/**
 * Waits until the home KPIs render.
 */
async function readyHome(page) {
  await expect(page.locator('.rw-home-kpis')).toBeVisible();
}

/**
 * Waits until the overview identity strip renders for the completed run.
 */
async function readyOverview(page) {
  await expect(page.locator('.rw-run-identity-strip').first()).toBeVisible();
}

/**
 * Waits until at least one corpus article row renders.
 */
async function readyCorpus(page) {
  await expect(page.locator('.rw-corpus-table tbody tr').first()).toBeVisible();
}

/**
 * Waits until the article detail PDF finishes rendering and the review
 * host shows the context-start onboarding.
 */
async function readyArticleDetail(page) {
  await expect(page.locator('[data-pdf-status]')).toHaveText('PDF page 1 of 2.');
  await expect(page.locator('[data-review-host] [data-start-review]')).toBeVisible();
}

/**
 * Opens the review setup dialog and waits for its candidate search to finish.
 */
async function readyReviewDialog(page) {
  await expect(page.locator('[data-review-host] [data-start-review]')).toBeVisible();
  await page.locator('[data-review-host] [data-start-review]').click();
  const dialog = page.locator('[data-review-dialog]');
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('[data-review-candidates]')).toContainText('Same-search contexts ready');
  await expect(dialog.locator('[data-review-parent]')).toBeVisible();
}

/**
 * Waits until the audit stream renders at least one event.
 */
async function readyAudit(page) {
  await expect(page.locator('#audit-event-stream .rw-audit-event').first()).toBeVisible();
}

const routes = [
  { name: 'home', url: '/', ready: readyHome },
  { name: 'overview', url: `/?view=overview&${CONTEXT}`, ready: readyOverview },
  { name: 'corpus', url: `/?view=corpus&section=articles&${CONTEXT}`, ready: readyCorpus },
  { name: 'article-detail', url: `/?view=article&article_id=1&${CONTEXT}`, ready: readyArticleDetail },
  { name: 'review-setup-dialog', url: `/?view=article&article_id=1&${CONTEXT}`, ready: readyReviewDialog },
  { name: 'provenance-audit', url: `/?view=provenance&section=audit&${CONTEXT}`, ready: readyAudit },
];

// ---------------------------------------------------------------------------
// The capture: one serial test so ordering is identical in both runs.
// ---------------------------------------------------------------------------

test.describe.configure({ mode: 'serial' });

test('captures rendered DOM for the equivalence routes', async ({ page }) => {
  const captures = {};
  for (const route of routes) {
    await settle(page, route.url);
    await route.ready(page);
    captures[route.name] = await captureBody(page);
  }
  const output = capturePath();
  await mkdir(path.dirname(output), { recursive: true });
  await writeFile(output, JSON.stringify({ tree: capturedTree(), routes: captures }, null, 2));
  console.log(`wrote DOM capture to ${output}`);
});