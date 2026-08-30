import { readFile } from 'node:fs/promises';

import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';

// ── Fixture identifiers ───────────────────────────────────────────────
// URL parameters use row IDs (not search_id strings or execution fingerprints)
// as set by the SPA's select onChange handlers.

// Search row IDs: 1=deep-learning-nlp, 2=quantum-computing, 3=crispr-gene-editing
const SEARCH_DL = '1';
const SEARCH_QC = '2';
const SEARCH_CR = '3';

// Search revision row IDs: 1=dl-r1, 2=dl-r2, 3=dl-r3, 4=qc-r1, 5=cr-r1
const REV_DL_R1 = '1';
const REV_DL_R2 = '2';
const REV_DL_R3 = '3';
const REV_QC_R1 = '4';
const REV_CR_R1 = '5';

// Execution plan row IDs: 1=dl-r1, 2=dl-r2, 3=dl-r3, 4=qc-r1, 5=cr-r1
const PLAN_DL_R1 = '1';
const PLAN_DL_R2 = '2';
const PLAN_DL_R3 = '3';
const PLAN_QC_R1 = '4';
const PLAN_CR_R1 = '5';

// Pipeline run row IDs
const RUN_1_COMPLETED = '1';  // dl-r1, attempt 1, completed, visible
const RUN_2_FAILED = '2';     // dl-r1, attempt 2, failed, visible
const RUN_3_TRASHED = '3';    // dl-r2, attempt 1, trashed
const RUN_4_NO_ENRICH = '4';  // dl-r3, attempt 1, completed, visible, enrichment disabled
const RUN_5_QC = '5';         // qc-r1, attempt 1, completed, visible
const RUN_6_CR = '6';         // cr-r1, attempt 1, completed, visible

// Work revision row IDs
const ARTICLE_1_ID = '1'; // "Attention Mechanisms in Transformer Models"
const ARTICLE_2_ID = '2'; // "Deep Reinforcement Learning for Robotics"
const AUTHOR_1_ID = '1';  // Ada Lovelace
const REF_1_ID = '1';     // Points to 10.1000/2

// ── Helpers ───────────────────────────────────────────────────────────

/**
 * Navigate to a URL and wait for network idle.
 */
async function goto(page: Page, url: string): Promise<void> {
  await page.goto(url);
  await page.waitForLoadState('networkidle');
}

/**
 * Build a context URL with search, revision, plan, and run IDs.
 */
function contextURL(overrides: Record<string, string> = {}): string {
  const params = new URLSearchParams({
    view: 'overview',
    search_id: SEARCH_DL,
    search_revision_id: REV_DL_R1,
    plan_id: PLAN_DL_R1,
    run_id: RUN_1_COMPLETED,
    ...overrides,
  });
  return `/?${params.toString()}`;
}

/**
 * Navigate to a fully selected context URL.
 */
async function selectRun(page: Page, searchId: string, revisionId: string, planId: string, runId: string): Promise<void> {
  await goto(page, contextURL({ search_id: searchId, search_revision_id: revisionId, plan_id: planId, run_id: runId }));
}

// ── Setup ─────────────────────────────────────────────────────────────

test.beforeEach(async ({ page }) => {
  await page.goto('/');
  await page.waitForLoadState('networkidle');
});

// ── 1. Health check and basic page load ───────────────────────────────

test.describe('Health and page load', () => {
  test('health endpoint returns readable status', async ({ request }) => {
    const resp = await request.get('/api/health');
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    expect(body).toHaveProperty('readable', true);
    expect(body).toHaveProperty('table_count');
    expect(Array.isArray(body.tables)).toBe(true);
  });

  test('page loads and renders navigation', async ({ page }) => {
    await goto(page, '/');
    await expect(page.locator('body')).toBeAttached();
    await expect(page.locator('.skip-link, [href="#main-content"]').first()).toBeAttached();
    await expect(page.getByRole('heading', { name: 'Research workspace', exact: true })).toBeVisible();
    await expect(page.locator('#health-status')).toHaveText('Database healthy');
    await expect(page.locator('body')).toContainText('Local review');
    await expect(page.getByRole('heading', { name: 'Home', exact: true })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toHaveText('Home');
    await expect(page.locator('.rw-context-panel')).toBeHidden();
    await expect(page.locator('.rw-primary-nav')).toBeHidden();
  });

  test('primary navigation links are present', async ({ page }) => {
    await goto(page, contextURL());
    const nav = page.getByRole('navigation', { name: 'Deepdive navigation' });
    const links = ['Overview', 'Corpus', 'Relationships', 'Provenance', 'Evaluation', 'Advanced'];
    for (const text of links) {
      await expect(nav.getByText(text).first()).toBeAttached();
    }
    await expect(nav.getByText('Trash')).toHaveCount(0);
  });

  test('primary navigation preserves the selected research context', async ({ page }) => {
    await goto(page, contextURL({ view: 'overview' }));
    const href = await page.getByRole('link', { name: 'Corpus', exact: true }).getAttribute('href');
    if (!href) throw new Error('Corpus navigation link has no href');
    const target = new URL(href, page.url());
    expect(target.searchParams.get('view')).toBe('corpus');
    expect(target.searchParams.get('search_id')).toBe(SEARCH_DL);
    expect(target.searchParams.get('search_revision_id')).toBe(REV_DL_R1);
    expect(target.searchParams.get('plan_id')).toBe(PLAN_DL_R1);
    expect(target.searchParams.get('run_id')).toBe(RUN_1_COMPLETED);
  });

  test('primary navigation loads view pages and browser history returns to the prior document', async ({ page }) => {
    await goto(page, contextURL({ view: 'overview' }));
    const priorURL = page.url();

    await page.getByRole('link', { name: 'Corpus', exact: true }).click();
    await page.waitForLoadState('networkidle');

    const corpusURL = new URL(page.url());
    expect(corpusURL.pathname).toBe('/corpus.html');
    expect(corpusURL.searchParams.get('view')).toBe('corpus');
    await expect(page.locator('meta[name="rw-page"]')).toHaveAttribute('content', 'corpus');
    await expect(page.getByRole('heading', { name: 'Corpus', exact: true })).toBeFocused();

    await page.goBack();
    await page.waitForLoadState('networkidle');
    expect(page.url()).toBe(priorURL);
    await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible();
  });

  test('cancelable draft protection blocks a native view-page transition', async ({ page }) => {
    await goto(page, contextURL({ view: 'overview' }));
    const priorURL = page.url();
    await page.evaluate(() => {
      document.addEventListener('rw-before-navigate', (event) => {
        event.preventDefault();
      }, { once: true });
    });

    await page.getByRole('link', { name: 'Corpus', exact: true }).click();

    await expect(page).toHaveURL(priorURL);
    await expect(page.locator('meta[name="rw-page"]')).toHaveAttribute('content', 'home');
    await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible();
  });

  test('a view page URL loads its identified document directly', async ({ page }) => {
    const query = new URLSearchParams({
      view: 'overview',
      search_id: SEARCH_DL,
      search_revision_id: REV_DL_R1,
      plan_id: PLAN_DL_R1,
      run_id: RUN_1_COMPLETED,
    });
    await goto(page, `/overview.html?${query.toString()}`);

    await expect(page.locator('meta[name="rw-page"]')).toHaveAttribute('content', 'overview');
    await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible();
    await expect(page).toHaveTitle('Overview · Research workspace');
  });

  test('research context is displayed after selecting a run', async ({ page }) => {
    await goto(page, contextURL());
    await page.waitForLoadState('networkidle');
    const context = page.locator('.rw-context-panel');
    await expect(context).toContainText(/Search revision|Execution plan|Run attempt/i);
    await expect(context).not.toContainText('Research context');
    await expect(context).toContainText(/deep-learning-nlp|Run 1/i);
    await expect(context).not.toContainText(/Select a captured run|Inspecting run attempt/i);
    const contextGap = await context.evaluate((element) => {
      const tabs = document.querySelector('.rw-primary-nav');
      if (!tabs) throw new Error('Deepdive navigation is unavailable');
      return tabs.getBoundingClientRect().top - element.getBoundingClientRect().bottom;
    });
    expect(contextGap).toBeGreaterThanOrEqual(20);
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toContainText(/Home.*Deepdive.*Overview/i);
  });
});

test.describe('Research-context canonicalization', () => {
  test('a selected run owns the complete displayed ancestry across crossed URLs and reload', async ({ page }) => {
    const contextRequest = page.waitForRequest((request) => request.url().includes('/api/runs/5/context'));
    await goto(page, contextURL({ search_id: SEARCH_DL, search_revision_id: REV_DL_R1, plan_id: PLAN_DL_R1, run_id: RUN_5_QC }));
    await contextRequest;
    await expect.poll(() => new URL(page.url()).searchParams.get('search_id')).toBe(SEARCH_QC);
    const current = new URL(page.url());
    expect(current.searchParams.get('search_revision_id')).toBe(REV_QC_R1);
    expect(current.searchParams.get('plan_id')).toBe(PLAN_QC_R1);
    expect(current.searchParams.get('run_id')).toBe(RUN_5_QC);
    await expect(page.locator('#search-select-trigger')).toContainText('quantum-computing');
    await expect(page.locator('#revision-select-trigger')).toContainText('r1');
    await expect(page.locator('#plan-select-trigger')).toContainText('Plan 4');
    await expect(page.locator('#run-select-trigger')).toContainText('Run 5');

    await page.reload();
    await page.waitForLoadState('networkidle');
    expect(new URL(page.url()).searchParams.get('run_id')).toBe(RUN_5_QC);
    await expect(page.locator('#search-select-trigger')).toContainText('quantum-computing');
  });

  test('run-only, stale descendant, trashed, and history states remain deterministic and visibly focused', async ({ page }) => {
    await goto(page, `/?view=overview&run_id=${RUN_5_QC}`);
    await expect.poll(() => new URL(page.url()).searchParams.get('plan_id')).toBe(PLAN_QC_R1);
    expect(new URL(page.url()).searchParams.get('search_id')).toBe(SEARCH_QC);

    await goto(page, `/?view=overview&search_id=${SEARCH_QC}&search_revision_id=${REV_DL_R1}&plan_id=${PLAN_DL_R1}`);
    await expect.poll(() => new URL(page.url()).searchParams.get('search_revision_id')).toBe(REV_QC_R1);
    const stale = new URL(page.url());
    expect(stale.searchParams.get('search_id')).toBe(SEARCH_QC);
    expect(stale.searchParams.get('plan_id')).toBe(PLAN_QC_R1);
    expect(stale.searchParams.get('run_id')).toBe(RUN_5_QC);
    await expect(page.locator('#search-select-trigger')).toContainText('quantum-computing');
    await expect(page.locator('#revision-select-trigger')).toContainText('r1');

    await goto(page, `/?view=overview&run_id=${RUN_3_TRASHED}`);
    await expect.poll(() => new URL(page.url()).searchParams.get('plan_id')).toBe(PLAN_DL_R2);
    expect(new URL(page.url()).searchParams.get('search_revision_id')).toBe(REV_DL_R2);
    await expect(page.locator('#run-select-trigger')).toContainText('Run 3');

    await goto(page, contextURL({ run_id: RUN_1_COMPLETED }));
    await goto(page, contextURL({ search_id: SEARCH_QC, search_revision_id: REV_QC_R1, plan_id: PLAN_QC_R1, run_id: RUN_5_QC }));
    await page.goBack();
    await page.waitForLoadState('networkidle');
    await expect.poll(() => new URL(page.url()).searchParams.get('run_id')).toBe(RUN_1_COMPLETED);
    await expect(page.locator('#run-select-trigger')).toContainText('Run 1');
    await page.goForward();
    await page.waitForLoadState('networkidle');
    await expect.poll(() => new URL(page.url()).searchParams.get('run_id')).toBe(RUN_5_QC);
    await expect(page.locator('#search-select-trigger')).toContainText('quantum-computing');
  });
});

// ── 2. Overview view ──────────────────────────────────────────────────

test.describe('Overview view', () => {
  test('displays run metrics for a completed run', async ({ page }) => {
    await goto(page, contextURL({ view: 'overview' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Input Records|Parsed Articles|Deduplicated Articles|Valid Articles/);
  });

  test('displays retention funnel for completed run', async ({ page }) => {
    await goto(page, contextURL({ view: 'overview' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Input records|Parsed articles|Deduplicated|Valid articles|Retention flow/i);
  });

  test('uses one initial raw-result baseline across source and pipeline retention stages', async ({ page }) => {
    await page.route('**/api/overview?*', async (route) => {
      const response = await route.fetch();
      const body = await response.json();
      body.source_filter_counts = [
        { source: 'alpha', filters: ['NO_FILTER'], count: 100 },
        { source: 'alpha', filters: ['NO_FILTER', 'RANGE_10_YEARS'], count: 60 },
        { source: 'alpha', filters: ['NO_FILTER', 'RANGE_10_YEARS', 'ARTICLE_ONLY'], count: 30 },
        { source: 'alpha', filters: ['NO_FILTER', 'RANGE_10_YEARS', 'ARTICLE_ONLY', 'ENGLISH_ONLY'], count: 25 },
        { source: 'beta', filters: ['NO_FILTER'], count: 200 },
        { source: 'beta', filters: ['NO_FILTER', 'RANGE_10_YEARS'], count: 100 },
        { source: 'beta', filters: ['NO_FILTER', 'RANGE_10_YEARS', 'ARTICLE_ONLY'], count: 50 },
        { source: 'beta', filters: ['NO_FILTER', 'RANGE_10_YEARS', 'ARTICLE_ONLY', 'ENGLISH_ONLY'], count: 45 },
      ];
      body.retention_funnel = {
        input_records: 68,
        parsed_articles: 60,
        deduplicated_articles: 50,
        valid_articles: 45,
        discarded_articles: 5,
      };
      body.enrichment_breakdown = { ...body.enrichment_breakdown, enrichment_candidates: 50 };
      body.normalization_breakdown = { ...body.normalization_breakdown, normalized_articles_processed: 45 };
      await route.fulfill({ response, json: body });
    });

    await goto(page, contextURL({ view: 'overview' }));
    await expect(page.locator('.rw-retention__phase')).toHaveCount(3);
    await expect(page.locator('.rw-retention__phase-header h4')).toHaveText(['Source selection', 'Pipeline processing', 'Corpus enrichment']);
    await expect(page.locator('.rw-flow--source > .rw-flow__step')).toHaveCount(4);
    await expect(page.locator(`[data-retention-phase="pipeline"] > .rw-flow > .rw-flow__step`)).toHaveCount(3);
    await expect(page.locator(`[data-retention-phase="corpus"] > .rw-flow > .rw-flow__step`)).toHaveCount(3);
    await expect(page.locator('[data-flow-stage="input_records"] .rw-flow__percentage')).toHaveText('22.67%');
    await expect(page.locator('[data-flow-stage="enrichment_candidates"] .rw-flow__percentage')).toHaveText('16.67%');
    await expect(page.locator('[data-flow-stage="validation_outcomes"] .rw-flow__percentage')).toHaveText('16.67%');
    await expect(page.locator('[data-flow-stage="validation_outcomes"] .rw-flow__outcome-values')).toContainText('45 accepted');
    await expect(page.locator('[data-flow-stage="validation_outcomes"] .rw-flow__outcome-values')).toContainText('5 discarded');
    await expect(page.locator('[data-flow-stage="normalized_articles_processed"] .rw-flow__percentage')).toHaveText('15.00%');
    await expect(page.locator('[data-flow-stage="parsed_articles"] a')).toHaveAttribute('href', /stage_q=parse/);
    const sourceStageTops = await page.locator('.rw-flow--source .rw-flow__step').evaluateAll((stages) => (
      stages.map((stage) => stage.getBoundingClientRect().top)
    ));
    expect(Math.max(...sourceStageTops) - Math.min(...sourceStageTops)).toBeLessThan(2);
  });

  test('displays informational source export-count comparisons', async ({ page }) => {
    await goto(page, contextURL({ view: 'overview' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Source export counts|Expected initial count|Observed raw records/i);
    await expect(body).toContainText(/scopus-export|below|ieee-export|match/i);
  });

  test('shows no enrichment badge for enrichment-disabled run', async ({ page }) => {
    await goto(page, contextURL({ search_id: SEARCH_DL, search_revision_id: REV_DL_R3, plan_id: PLAN_DL_R3, run_id: RUN_4_NO_ENRICH, view: 'overview' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/enrichment|skipped|disabled|Not recorded/i);
  });

  test('displays enrichment field and provider breakdowns', async ({ page }) => {
    await goto(page, contextURL({ view: 'overview' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Enriched fields|Enrichment by provider/i);
    await expect(body).toContainText(/title|abstract|publisher|citation_count|references|authors/i);
    await expect(body).toContainText(/crossref|openalex/i);
  });
});

// ── 3. Corpus views ───────────────────────────────────────────────────

test.describe('Corpus view', () => {
  test('articles section loads and shows article rows', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Attention Mechanisms|Deep Reinforcement Learning|Convolutional Neural/i);
    await expect(page.locator('#corpus-section-select')).toBeVisible();
    await expect(page.locator(`[data-table-owner="work_revisions"] .ui.tabular.menu`)).toHaveCount(0);
    await expect(page.locator('.rw-corpus-table thead th').filter({ hasText: /DOI|Title|Year|Journal|Source/ })).toHaveCount(5);
    await expect(page.getByRole('columnheader', { name: 'ID', exact: true })).toHaveCount(0);
    await expect(page.getByRole('columnheader', { name: 'Work', exact: true })).toHaveCount(0);
  });

  test('articles section supports search filtering', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles', q: 'Attention' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText('Attention Mechanisms');
    await expect(body).not.toContainText('Deep Reinforcement Learning');
  });

  test('articles expansion shows matched search terms', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles' }));
    await page.waitForLoadState('networkidle');
    await page.locator(".rw-corpus-table .expand-toggle").first().click();
    const expansion = page.locator(".rw-corpus-table tr.expansion-row").first();
    await expect(expansion).toContainText('Matched search terms');
    await expect(expansion).toContainText('3 of 6 search terms matched');
    await expect(expansion).toContainText('transformer');
    await expect(expansion).toContainText('attention');
    await expect(expansion).toContainText('deep learning');
  });

  test('articles expansion shows no search terms recorded for a run without queries', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles', run_id: RUN_4_NO_ENRICH }));
    await page.waitForLoadState('networkidle');
    await page.locator(".rw-corpus-table .expand-toggle").first().click();
    const expansion = page.locator(".rw-corpus-table tr.expansion-row").first();
    await expect(expansion).toContainText('Matched search terms');
    await expect(expansion).toContainText('No search terms recorded');
  });

  test('authors section loads and shows author rows', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'authors' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Ada Lovelace|Charles Babbage|Alan Turing/i);
  });

  test('references section loads and shows reference rows', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'references' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/10\.1000|10\.external/);
    const referenceTable = page.locator('.rw-corpus-table--references');
    const citingArticle = referenceTable.locator('tbody tr:not(.expansion-row)').first().locator('td.col-citing-title');
    await expect(citingArticle.locator('details')).toHaveCount(0);
    await expect(citingArticle.locator('.rw-table-title')).toBeVisible();
    await expect(referenceTable.locator('td.col-reference-author .rw-table-title').first()).toBeVisible();
    await expect(referenceTable.locator('.rw-cell').first()).toHaveCSS('max-width', /\d+px|100%/);
    await expect(referenceTable.locator('xpath=ancestor::div[contains(@class, "table-wrap")]')).toHaveCSS('overflow-x', 'auto');
  });

  test('sources section loads and shows source records', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'sources' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/scopus|ieee|wos/i);
    await expect(body).toContainText(/Source export counts|Expected initial count|Observed raw records/i);
    await expect(body).toContainText(/below|match/i);
    await page.locator('.rw-corpus-table--sources .expand-toggle').first().click();
    const facts = page.locator('.rw-corpus-table--sources tr.expansion-row').first().locator('.rw-property-grid > div');
    await expect(facts).toHaveCount(3);
    const positions = await facts.evaluateAll(function(items) {
      return items.map(function(item) { return item.getBoundingClientRect().top; });
    });
    expect(Math.max(...positions) - Math.min(...positions)).toBeLessThan(2);
  });

  test('identity evidence links to author-scoped candidate details without expanding candidates inside the table', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'identity_evidence' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Author identity.*ORCID evidence|Unclear ORCID matches/i);
    await expect(body).toContainText(/Provider failures/i);
    await expect(body).toContainText(/Charles Babbage|orcid_is_unclear/i);
    await expect(body).not.toContainText('0000-0001-2345-6789');
    await expect(page.getByRole('columnheader', { name: /Candidate evidence/i })).toHaveCount(0);
    await page.getByRole('link', { name: 'Charles Babbage' }).click();
    await expect(page).toHaveURL(/view=author/);
    await expect(page.locator('body')).toContainText(/ORCID candidate evidence|0000-0001-2345-6789|Provider query/i);
  });

  test('corpus supports pagination', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles', per_page: '20', search_id: SEARCH_CR, search_revision_id: REV_CR_R1, plan_id: PLAN_CR_R1, run_id: RUN_6_CR }));
    await page.waitForLoadState('networkidle');
    const pagination = page.getByRole('navigation', { name: 'Result pages' });
    await expect(pagination).toContainText(/Page 1 of \d+/i);
    await expect(pagination.getByRole('button', { name: 'First page' })).toBeVisible();
    await expect(pagination.getByRole('button', { name: 'Previous page' })).toBeVisible();
    await expect(pagination.getByRole('button', { name: 'Next page' })).toBeVisible();
    await expect(pagination.getByRole('button', { name: 'Last page' })).toBeVisible();
    const firstPageIDs = await page.locator('.rw-corpus-table tbody tr[data-row-key]').evaluateAll((rows) => rows.map((row) => row.dataset.rowKey));
    await pagination.getByRole('button', { name: 'Next page' }).click();
    await expect(page).toHaveURL(/(?:\?|&)page=2(?:&|$)/);
    const secondPageIDs = await page.locator('.rw-corpus-table tbody tr[data-row-key]').evaluateAll((rows) => rows.map((row) => row.dataset.rowKey));
    expect(secondPageIDs.length).toBeGreaterThan(0);
    expect(secondPageIDs).not.toEqual(firstPageIDs);
    expect(secondPageIDs.filter((id) => firstPageIDs.includes(id))).toEqual([]);
  });

  test('corpus ignores a sort field that is unsupported by its selected section', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles', sort: 'work_id', order: 'asc' }));
    await expect(page.locator('#notice')).toBeHidden();
    await expect(page.locator('body')).toContainText(/Attention Mechanisms|Deep Reinforcement Learning|Convolutional Neural/i);
    await expect(page.getByRole('columnheader', { name: 'Work' }).locator('button')).toHaveCount(0);
  });
});

// ── 4. Relationships / Graph view ─────────────────────────────────────

test.describe('Relationships (graph) view', () => {
  test('graph loads and renders for a completed run', async ({ page }) => {
    await goto(page, contextURL({ view: 'relationships' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Research network|Relationship table|Article revision|Author occurrence|Reference mention/i);
    await expect(page.getByLabel('Visible graph encodings')).toContainText(/Entities|Relationships/i);
    await expect(body).toContainText(/Connected clusters/i);
  });

  test('graph supports mode switching to citation', async ({ page }) => {
    await goto(page, contextURL({ view: 'relationships', mode: 'citation' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Internal citations|Relationship table|Internal citation/i);
  });

  test('graph supports text search filter', async ({ page }) => {
    await goto(page, contextURL({ view: 'relationships', q: 'Attention' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Relationship table|Title or DOI: Attention/i);
  });

  test('graph filters are applied explicitly and remain visible in the URL', async ({ page }) => {
    await goto(page, contextURL({ view: 'relationships' }));
    await page.getByLabel('Title or DOI').fill('Attention');
    await page.getByRole('button', { name: 'Apply filters' }).click();
    await expect(page).toHaveURL(/(?:\?|&)q=Attention(?:&|$)/);
    await expect(page.locator('body')).toContainText('Title or DOI: Attention');
  });

  test('graph node search input is present and functional', async ({ page }) => {
    await goto(page, contextURL({ view: 'relationships' }));
    const searchInput = page.locator('#graph-node-search');
    await expect(searchInput).toBeVisible();
    await expect(searchInput).toHaveAttribute('placeholder', /Search nodes/i);
    await searchInput.fill('Attention');
    await expect(page.locator('#graph-search-summary')).toHaveText(/3 matching nodes/i);
    const results = page.locator('#graph-search-results [data-graph-search-node]');
    await expect(results).toHaveCount(3);
    const result = results.filter({ hasText: 'Attention Mechanisms' }).first();
    await expect(result).toContainText('Attention Mechanisms');
    await result.click();
    await expect(page.locator('#graph-selection')).toBeFocused();
    await expect(page.locator('#graph-selection')).toContainText('Attention Mechanisms');
  });

  test('graph export downloads a valid PNG', async ({ page }) => {
    await goto(page, contextURL({ view: 'relationships' }));
    const exportBtn = page.locator('#graph-export-png');
    await expect(exportBtn).toBeVisible();
    await expect(exportBtn).toContainText(/Export|PNG/i);
    const downloadPromise = page.waitForEvent('download');
    await exportBtn.click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe('graph-export.png');
    const path = await download.path();
    expect(path).not.toBeNull();
    if (!path) throw new Error('Downloaded graph path is unavailable');
    const bytes = await readFile(path);
    expect(bytes.subarray(0, 8)).toEqual(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]));
  });

  test('graph legend is anchored at the bottom left of the canvas', async ({ page }) => {
    await goto(page, contextURL({ view: 'relationships' }));
    const legend = page.getByLabel('Visible graph encodings');
    const canvas = page.locator('.rw-graph__canvas');
    const legendBox = await legend.boundingBox();
    const canvasBox = await canvas.boundingBox();
    expect(legendBox).not.toBeNull();
    expect(canvasBox).not.toBeNull();
    if (!legendBox || !canvasBox) throw new Error('Graph legend or canvas bounds are unavailable');
    expect(Math.abs(legendBox.x - canvasBox.x)).toBeLessThan(24);
    expect(canvasBox.y + canvasBox.height - legendBox.y - legendBox.height).toBeLessThan(24);
  });

  test('background click clears a selected graph node', async ({ page }) => {
    await goto(page, contextURL({ view: 'relationships', node: 'article:1' }));
    const clear = page.locator('#graph-clear-selection');
    await expect(clear).toBeEnabled();
    await page.locator('.rw-graph__canvas').click({ position: { x: 3, y: 3 } });
    await expect(clear).toBeDisabled();
    await expect(page).not.toHaveURL(/(?:\?|&)node=/);
  });

  test('clicking the selected graph node again clears it', async ({ page }) => {
    await page.route('**/api/graph?*', function(route) {
      return route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ data: {
          nodes: [{ id: 'article:test', type: 'article', label: 'Test article', revision_id: '1' }],
          edges: [],
          counts: { article_matches: 1, article_rendered: 1, nodes_rendered: 1, edges_rendered: 0, node_types: { article: 1 }, edge_types: {} },
          limits: { article_limit: 2000 }
        } })
      });
    });
    await goto(page, contextURL({ view: 'relationships' }));
    await expect(page.locator('#graph-layout-status')).toContainText(/settled|placed/i);
    await page.locator('#graph-fit').click();
    const canvas = page.locator('.rw-graph__canvas');
    const box = await canvas.boundingBox();
    expect(box).not.toBeNull();
    if (!box) throw new Error('Graph canvas bounds are unavailable');
    const position = { x: box.width / 2, y: box.height / 2 };
    await canvas.click({ position });
    await expect(page.locator('#graph-clear-selection')).toBeEnabled();
    await canvas.click({ position });
    await expect(page.locator('#graph-clear-selection')).toBeDisabled();
    await expect(page).not.toHaveURL(/(?:\?|&)node=/);
  });

  test('secondary-button drag pans without changing graph selection', async ({ page }) => {
    await goto(page, contextURL({ view: 'relationships', node: 'article:1' }));
    const canvas = page.locator('.rw-graph__canvas');
    const box = await canvas.boundingBox();
    expect(box).not.toBeNull();
    if (!box) throw new Error('Graph canvas bounds are unavailable');
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down({ button: 'right' });
    await page.mouse.move(box.x + box.width / 2 + 60, box.y + box.height / 2 + 35, { steps: 4 });
    await page.mouse.up({ button: 'right' });
    await expect(page).toHaveURL(/(?:\?|&)node=article%3A1(?:&|$)/);
    await expect(page.locator('#graph-clear-selection')).toBeEnabled();
  });
});

// ── 5. Provenance views ───────────────────────────────────────────────

test.describe('Provenance view', () => {
  test('audit section loads and shows events', async ({ page }) => {
    await goto(page, contextURL({ view: 'provenance', section: 'audit' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Audit|timeline|events|recorded actions/i);
    await page.getByText('Stage and outcome filters').click();
    const advancedTops = await page.locator('.rw-filter-field-grid--advanced > label').evaluateAll(function(labels) {
      return labels.slice(0, 2).map(function(label) { return Math.round(label.getBoundingClientRect().top); });
    });
    expect(Math.abs(advancedTops[0] - advancedTops[1])).toBeLessThan(2);
  });

  test('audit stream filters events by category on the server', async ({ page }) => {
    await goto(page, contextURL({ view: 'provenance', section: 'audit' }));
    const categoryFilter = page.locator('details.rw-multi-select').filter({ hasText: 'Category' });
    await categoryFilter.locator('summary').click();
    const categoryCheckbox = page.locator('input[name="audit_category"][value="enrichment"]');
    await expect(categoryCheckbox).toHaveCSS('width', '16px');
    await expect(categoryCheckbox).toHaveCSS('height', '16px');
    const summaryBox = await categoryFilter.locator('summary').boundingBox();
    const checkboxBox = await categoryCheckbox.boundingBox();
    if (!summaryBox || !checkboxBox) throw new Error('Audit category control bounds are unavailable');
    expect(Math.abs(checkboxBox.x - summaryBox.x)).toBeLessThan(14);
    await categoryCheckbox.check();
    await page.getByRole('button', { name: 'Apply filters' }).click();
    await expect(page).toHaveURL(/(?:\?|&)audit_category=enrichment(?:&|$)/);
    const stream = page.locator('#audit-event-stream');
    await expect(stream.locator('.rw-audit-event--enrichment').first()).toBeVisible();
    await expect(stream.locator('.rw-audit-event--pipeline')).toHaveCount(0);
    await expect(stream.locator('.rw-audit-event--validation')).toHaveCount(0);
  });

  test('audit stream exposes mirrored PDF events', async ({ page }) => {
    await goto(page, contextURL({ view: 'provenance', section: 'audit', audit_category: 'pdf' }));
    const stream = page.locator('#audit-event-stream');
    await expect(stream.locator('.rw-audit-event--pdf').first()).toBeVisible();
    await expect(stream).toContainText(/Pdf (Inventory Registered|Document Inventoried)/i);
    await expect(stream.locator('.rw-audit-event--pipeline')).toHaveCount(0);
  });

  test('artifacts section identifies configuration snapshots and offers downloads', async ({ page }) => {
    await goto(page, contextURL({ view: 'provenance', section: 'artifacts' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Artifacts|artifact/i);
    await expect(body).toContainText(/Search|Revision|Execution plan|Run attempt/i);
    await expect(body).toContainText(/Workspace Config|Resolved Manifest|Input Manifest/i);
    await expect(page.getByRole('link', { name: 'Download' }).first()).toHaveAttribute('href', /\/api\/artifacts\/\d+\/content/);
    const pagination = page.getByRole('navigation', { name: 'Result pages' });
    await expect(pagination.getByRole('button', { name: 'First page' })).toBeVisible();
    await expect(pagination.getByRole('button', { name: 'Previous page' })).toBeVisible();
    await expect(pagination.getByRole('button', { name: 'Next page' })).toBeVisible();
    await expect(pagination.getByRole('button', { name: 'Last page' })).toBeVisible();
    const artifactControlBottoms = await page.locator('[data-artifact-filters]').evaluate(function(form) {
      return Array.from(form.children).slice(0, 4).map(function(control) { return Math.round(control.getBoundingClientRect().bottom); });
    });
    expect(Math.max(...artifactControlBottoms) - Math.min(...artifactControlBottoms)).toBeLessThan(2);
    await page.getByRole('button', { name: 'Inspect preview' }).first().click();
    await expect(body).toContainText(/Artifact preview|artifact-config-1|Bytes shown/i);
    await expect(page.getByRole('button', { name: 'Copy displayed text' })).toBeVisible();
  });

  test('cache uses section supports shared filtering and pagination controls', async ({ page }) => {
    await goto(page, contextURL({ view: 'provenance', section: 'cache' }));
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toContainText(/Cache use records|hit|negative|stale/i);
    const pagination = page.getByRole('navigation', { name: 'Result pages' });
    await expect(pagination.getByRole('button', { name: 'First page' })).toBeVisible();
    await expect(pagination.getByRole('button', { name: 'Last page' })).toBeVisible();
    await page.locator('#cache-query').fill('openalex');
    await page.locator('#cache-controls').getByRole('button', { name: 'Search' }).click();
    await expect(page).toHaveURL(/(?:\?|&)cache_q=openalex(?:&|$)/);
    await expect(page.locator(`[data-table-scope="Cache uses"]`)).toContainText("openalex");
    const cacheControlBottoms = await page.locator('#cache-controls').evaluate(function(form) {
      return Array.from(form.children).slice(0, 3).map(function(control) { return Math.round(control.getBoundingClientRect().bottom); });
    });
    expect(Math.max(...cacheControlBottoms) - Math.min(...cacheControlBottoms)).toBeLessThan(2);
  });

  test('stages section loads', async ({ page }) => {
    await goto(page, contextURL({ view: 'provenance', section: 'stages' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Stage outcomes and progression|Preflight|Parse|Enrich|Detailed stage outcomes/i);
    await expect(page.locator('.rw-stage-flow')).toBeVisible();
    const stageControlBottoms = await page.locator('#stage-controls').evaluate(function(form) {
      return Array.from(form.children).slice(0, 3).map(function(control) { return Math.round(control.getBoundingClientRect().bottom); });
    });
    expect(Math.max(...stageControlBottoms) - Math.min(...stageControlBottoms)).toBeLessThan(2);
  });

  test('run details section loads', async ({ page }) => {
    await goto(page, contextURL({ view: 'provenance', section: 'run' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Run details|Plan|attempt|status|Execution plan/i);
    await expect(body).toContainText(/Resolved manifest hash|Input manifest hash|Configuration snapshots/i);
  });
});

// ── 6. Evaluation inventory ───────────────────────────────────────────

test.describe('Evaluation view', () => {
  test('lists only normalized articles with manual inventory status', async ({ page }) => {
    await goto(page, contextURL({ view: 'evaluation' }));
    const table = page.locator('.rw-evaluation-table');
    await expect(table).toBeVisible();
    await expect(table.locator('thead')).toContainText('Title');
    await expect(table.locator('thead')).toContainText('DOI');
    await expect(table.locator('thead')).toContainText('PDF');
    await expect(table.locator('thead')).toContainText('Inventoried at');
    await expect(table.getByRole('columnheader', { name: 'Source', exact: true })).toHaveCount(0);
    await expect(table.getByRole('columnheader', { name: 'Qualifiers', exact: true })).toHaveCount(0);
    await expect(table.locator('tbody time').first()).toHaveText(/^[A-Z][a-z]{2} \d{1,2}, \d{4}$/);
    await expect(table.locator('xpath=ancestor::div[contains(@class, "table-wrap")]')).toHaveCSS('overflow-x', 'auto');
    await expect(table.locator('.ui.green.label', { hasText: 'Available' })).toHaveCount(1);
    await expect(table.locator('.ui.orange.label', { hasText: 'Not Available' }).first()).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Result pages' })).toContainText(/9 normalized articles/i);
  });

  test('preserves research context in the Evaluation navigation link', async ({ page }) => {
    await goto(page, contextURL({ view: 'provenance' }));
    const evaluation = page.getByRole('link', { name: 'Evaluation', exact: true });
    const href = await evaluation.getAttribute('href');
    if (!href) throw new Error('Evaluation navigation link has no href');
    const target = new URL(href, page.url());
    expect(target.searchParams.get('view')).toBe('evaluation');
    expect(target.searchParams.get('run_id')).toBe(RUN_1_COMPLETED);
  });
});

// ── 7. Advanced table browser ─────────────────────────────────────────

test.describe('Advanced (table browser) view', () => {
  test('lists available tables', async ({ page }) => {
    await goto(page, '/?view=advanced');
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/work_revisions|pipeline_runs|authorships|searches/i);
  });

  test('displays rows from a selected table', async ({ page }) => {
    await goto(page, '/?view=advanced&table=work_revisions');
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Attention Mechanisms|Deep Reinforcement Learning|Convolutional Neural/i);
  });

  test('table browser supports pagination', async ({ page }) => {
    await goto(page, '/?view=advanced&table=work_revisions&per_page=20');
    await page.waitForLoadState('networkidle');
    const pagination = page.getByRole('navigation', { name: 'Result pages' });
    await expect(pagination).toContainText(/Page 1 of \d+/i);
    await expect(pagination.getByRole('button', { name: 'First page' })).toBeVisible();
    await expect(pagination.getByRole('button', { name: 'Last page' })).toBeVisible();
  });

  test('ignores a sort field that does not belong to the selected table', async ({ page }) => {
    await goto(page, '/?view=advanced&table=work_revisions&sort=citation_name&order=asc');
    await expect(page.locator('#notice')).toBeHidden();
    await expect(page.locator('body')).toContainText(/Attention Mechanisms|Deep Reinforcement Learning|Convolutional Neural/i);
  });
});

// ── 7. Home lifecycle management ─────────────────────────────────────

test.describe('Home lifecycle management', () => {
  test('Home shows all research hierarchy totals and manages trashed runs through a modal', async ({ page }) => {
    await goto(page, '/');
    await expect(page.locator('.rw-home-kpis')).toContainText(/Search terms|Search revisions|Execution plans|Run attempts/i);
    await expect(page.locator('.rw-home-runs')).toContainText(/Run 1|Move to trash/i);
    const explore = page.locator('.rw-home-runs').getByRole('link', { name: 'Explore' }).first();
    await expect(explore).toHaveAttribute('href', /view=overview/);
    await page.locator('[data-home-filters] summary').click();
    await page.locator('[data-home-filters] select[name="visibility"]').selectOption('trashed');
    await page.locator('[data-home-filters]').getByRole('button', { name: 'Apply filters' }).click();
    await expect(page).toHaveURL(/(?:\?|&)home_visibility=trashed(?:&|$)/);
    await expect(page.locator('.rw-home-runs')).toContainText(/Run 3|Restore/i);
    await page.getByRole('button', { name: 'Restore' }).first().click();
    const dialog = page.getByRole('dialog', { name: /Restore run/ });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText(/captured outcome|execution evidence/i);
    await dialog.getByRole('button', { name: 'Cancel' }).click();
    await expect(dialog).toBeHidden();
  });
});

// ── 8. Detail views ───────────────────────────────────────────────────

test.describe('Detail views', () => {
  test('article detail shows revision metadata', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Attention Mechanisms in Transformer Models/i);
    await expect(body).toContainText(/2024/i);
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toContainText(/Home.*Deepdive.*Corpus.*Analysis-ready articles.*10\.1000\/1/i);
    await expect(page.getByRole('link', { name: 'Back to Corpus' })).toHaveCount(0);
    await expect(page.locator('[data-record-audit-search]')).toHaveCSS('height', '38px');
    await expect(page.locator('[data-record-audit-category]')).toHaveCSS('height', '38px');
    await expect(page.locator('[data-record-audit-action]')).toHaveCSS('height', '38px');
  });

  test('article detail shows the search term coverage panel', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('heading', { name: 'Search term coverage' })).toBeVisible();
    const body = page.locator('body');
    await expect(body).toContainText('3 of 6 search terms matched this article.');
    await expect(body).toContainText('All search terms');
    await expect(body).toContainText('Matched terms');
    await expect(body).toContainText('Unmatched terms');
    await expect(body).toContainText('reinforcement');
  });

  test('article detail shows no search terms recorded for a run without queries', async ({ page }) => {
    await goto(page, contextURL({
      view: 'article',
      search_revision_id: REV_DL_R3,
      plan_id: PLAN_DL_R3,
      run_id: RUN_4_NO_ENRICH,
      article_id: '17',
    }));
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('heading', { name: 'Search term coverage' })).toBeVisible();
    await expect(page.locator('body')).toContainText('No search terms recorded for this run.');
  });

  test('article detail opens the bound PDF without discarding research context', async ({ page, request }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await expect(page.locator('body')).toContainText(/Full-text PDF|Available|Inventoried/i);
    const pdfLink = page.getByRole('link', { name: 'Download PDF' });
    await expect(pdfLink).toHaveAttribute('href', '/api/pdf/1');
    await expect(pdfLink).toHaveAttribute('download', '');
    await expect(page).toHaveURL(/(?:\?|&)search_id=1(?:&|$)/);
    await expect(page).toHaveURL(/(?:\?|&)run_id=1(?:&|$)/);
    const response = await request.get('/api/pdf/1', { headers: { Range: 'bytes=0-4' } });
    expect(response.status()).toBe(206);
    expect(response.headers()['content-type']).toContain('application/pdf');
    expect((await response.body()).toString()).toBe('%PDF-');
  });

  test('article detail shows an absent PDF without an open action', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_2_ID }));
    await expect(page.locator('body')).toContainText(/Full-text PDF|Not stored|No stored PDF/i);
    await expect(page.getByRole('link', { name: 'Download PDF' })).toHaveCount(0);
  });

  test('article detail preserves the originating corpus state', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles', q: 'Attention', page: '1', sort: 'title', order: 'asc', expanded: '1' }));
    await page.getByRole('link', { name: /Attention Mechanisms/i }).first().click();
    await expect(page).toHaveURL(/view=article/);
    const origin = new URL(page.url()).searchParams.get('origin');
    expect(origin).toContain('q=Attention');
    expect(origin).toContain('expanded=1');
    await page.getByRole('navigation', { name: 'Detail record navigation' }).getByRole('link', { name: 'Return to Corpus' }).click();
    await expect(page).toHaveURL(/view=corpus/);
    await expect(page).toHaveURL(/q=Attention/);
    await expect(page).toHaveURL(/expanded=1/);
  });

  test('article detail shows authors and references', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Ada Lovelace|Charles Babbage/i);
    await expect(body).toContainText(/10\.1000|10\.external/);
  });

  test('author detail shows author information', async ({ page }) => {
    await goto(page, contextURL({ view: 'author', author_id: AUTHOR_1_ID }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/Ada Lovelace/i);
    await expect(body).toContainText(/0000/);
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toContainText(/Home.*Deepdive.*Corpus.*Author/i);
    await expect(page.locator('[data-record-audit-search]')).toHaveCSS('height', '38px');
    await expect(page.locator('[data-record-audit-category]')).toHaveCSS('height', '38px');
    await expect(page.locator('[data-record-audit-action]')).toHaveCSS('height', '38px');
  });

  test('reference detail shows reference information', async ({ page }) => {
    await goto(page, contextURL({ view: 'reference', reference_id: REF_1_ID }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/10\.1000|Deep Reinforcement Learning/i);
  });
});

// ── 8b. Fullscreen PDF reader ─────────────────────────────────────────

test.describe('Fullscreen PDF reader', () => {

  /** Returns whether the reading workspace is expanded by either the Fullscreen API or the fallback class. */
  async function workspaceExpanded(page: Page): Promise<boolean> {
    return page.locator('.rw-reading-workspace').evaluate((workspace) => {
      return document.fullscreenElement === workspace || workspace.classList.contains('rw-reading-workspace--expanded');
    });
  }

  /** Selects the fixture methods text and hands it to the review selection flow. */
  async function selectFixtureText(page: Page): Promise<void> {
    await page.evaluate(() => {
      const layer = document.querySelector('.rw-pdf-page .textLayer');
      if (!layer) throw new Error('PDF text layer is unavailable');
      const text = Array.from(layer.querySelectorAll('span')).find((span) => span.textContent?.includes('Selectable fixture methods'));
      if (!text) throw new Error('Selectable fixture methods text is unavailable');
      const range = document.createRange();
      range.selectNodeContents(text);
      const selection = window.getSelection();
      if (!selection) throw new Error('Document selection is unavailable');
      selection.removeAllRanges();
      selection.addRange(range);
      const viewer = document.querySelector('.rw-pdf-viewer');
      if (!viewer) throw new Error('PDF viewer is unavailable');
      viewer.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
    });
  }

  test('enters fullscreen from the toolbar with the drawer expanded by default', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await page.waitForLoadState('networkidle');
    const fullscreenButton = page.getByRole('button', { name: 'Fullscreen' });
    await expect(fullscreenButton).toBeVisible();
    await expect(fullscreenButton).toHaveAttribute('aria-pressed', 'false');

    await fullscreenButton.click();
    await expect(page.getByRole('button', { name: 'Exit fullscreen' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Exit fullscreen' })).toHaveAttribute('aria-pressed', 'true');
    expect(await workspaceExpanded(page)).toBe(true);
    await expect(page.locator('[data-drawer-edge]')).toBeVisible();
    await expect(page.locator('[data-review-host]')).toBeVisible();
  });

  test('collapses and expands the review drawer through the edge control', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await page.waitForLoadState('networkidle');
    await page.getByRole('button', { name: 'Fullscreen' }).click();
    await expect(page.locator('[data-drawer-edge]')).toBeVisible();

    await page.locator('[data-drawer-edge]').click();
    await expect(page.locator('[data-review-host]')).toBeHidden();
    await expect(page.locator('[data-drawer-edge]')).toHaveAttribute('aria-expanded', 'false');

    await page.locator('[data-drawer-edge]').click();
    await expect(page.locator('[data-review-host]')).toBeVisible();
    await expect(page.locator('[data-drawer-edge]')).toHaveAttribute('aria-expanded', 'true');
  });

  test('selecting PDF text expands a collapsed drawer', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await page.waitForLoadState('networkidle');
    await page.getByRole('button', { name: 'Fullscreen' }).click();
    await page.locator('[data-drawer-edge]').click();
    await expect(page.locator('[data-review-host]')).toBeHidden();

    await selectFixtureText(page);
    await expect(page.getByRole('button', { name: 'Review selection' })).toBeVisible();
    await page.getByRole('button', { name: 'Review selection' }).click();
    await expect(page.locator('[data-review-host]')).toBeVisible();
    await expect(page.locator('[data-drawer-edge]')).toHaveAttribute('aria-expanded', 'true');
  });

  test('exits fullscreen and restores the embedded article layout', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await page.waitForLoadState('networkidle');
    await page.getByRole('button', { name: 'Fullscreen' }).click();
    await expect(page.getByRole('button', { name: 'Exit fullscreen' })).toBeVisible();

    await page.getByRole('button', { name: 'Exit fullscreen' }).click();
    await expect(page.getByRole('button', { name: 'Fullscreen' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Fullscreen' })).toHaveAttribute('aria-pressed', 'false');
    expect(await workspaceExpanded(page)).toBe(false);
    await expect(page.locator('[data-drawer-edge]')).toHaveCount(0);
    await expect(page.locator('[data-review-host]')).toBeVisible();
  });
});

// ── 8c. PDF theme toggle ──────────────────────────────────────────────

test.describe('PDF theme toggle', () => {

  /** Returns the computed filter of the rendered PDF page. */
  async function pageFilter(page: Page): Promise<string> {
    return page.locator('.rw-pdf-page').evaluate((element) => {
      return getComputedStyle(element).filter;
    });
  }

  test('inverts the rendered page through the Dark toggle and restores it', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await page.waitForLoadState('networkidle');
    const themeButton = page.getByRole('button', { name: 'Dark' });
    await expect(themeButton).toBeVisible();
    await expect(themeButton).toHaveAttribute('aria-pressed', 'false');
    await expect(page.locator('.rw-pdf-viewer')).not.toHaveClass(/rw-pdf-viewer--dark/);
    expect(await pageFilter(page)).toBe('none');

    await themeButton.click();
    await expect(page.getByRole('button', { name: 'Light' })).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('.rw-pdf-viewer')).toHaveClass(/rw-pdf-viewer--dark/);
    expect(await pageFilter(page)).toContain('invert(1)');

    await page.getByRole('button', { name: 'Light' }).click();
    await expect(page.getByRole('button', { name: 'Dark' })).toHaveAttribute('aria-pressed', 'false');
    await expect(page.locator('.rw-pdf-viewer')).not.toHaveClass(/rw-pdf-viewer--dark/);
    expect(await pageFilter(page)).toBe('none');
  });

  test('keeps the inverted theme while entering and leaving fullscreen', async ({ page }) => {
    await goto(page, contextURL({ view: 'article', article_id: ARTICLE_1_ID }));
    await page.waitForLoadState('networkidle');
    await page.getByRole('button', { name: 'Dark' }).click();
    await expect(page.locator('.rw-pdf-viewer')).toHaveClass(/rw-pdf-viewer--dark/);

    await page.getByRole('button', { name: 'Fullscreen' }).click();
    await expect(page.getByRole('button', { name: 'Exit fullscreen' })).toBeVisible();
    await expect(page.locator('.rw-pdf-viewer')).toHaveClass(/rw-pdf-viewer--dark/);
    expect(await pageFilter(page)).toContain('invert(1)');

    await page.getByRole('button', { name: 'Exit fullscreen' }).click();
    await expect(page.getByRole('button', { name: 'Fullscreen' })).toBeVisible();
    await expect(page.locator('.rw-pdf-viewer')).toHaveClass(/rw-pdf-viewer--dark/);
    expect(await pageFilter(page)).toContain('invert(1)');
  });
});

// ── 9. Error states ───────────────────────────────────────────────────

test.describe('Error states', () => {
  test('shows error for invalid API route', async ({ page }) => {
    const resp = await page.request.get('/api/unknown');
    expect(resp.status()).toBe(404);
    const body = await resp.json();
    expect(body).toHaveProperty('error');
    expect(body.error).toHaveProperty('code', 'not_found');
  });

  test('shows error for invalid per_page value', async ({ page }) => {
    const resp = await page.request.get('/api/tables/works?per_page=21');
    expect(resp.status()).toBe(400);
    const body = await resp.json();
    expect(body.error.code).toBe('invalid_request');
  });

  test('rejects SQL injection attempts in sort parameter', async ({ page }) => {
    const resp = await page.request.get('/api/tables/works?sort=id;DROP%20TABLE%20works');
    expect(resp.status()).toBe(400);
    const body = await resp.json();
    expect(body.error.code).toBe('invalid_request');
  });

  test('rejects invalid order parameter', async ({ page }) => {
    const resp = await page.request.get('/api/tables/works?order=sideways');
    expect(resp.status()).toBe(400);
    const body = await resp.json();
    expect(body.error.code).toBe('invalid_request');
  });

  test('rejects unknown query parameters', async ({ page }) => {
    const resp = await page.request.get('/api/runs?bad=1');
    expect(resp.status()).toBe(400);
    const body = await resp.json();
    expect(body.error.code).toBe('invalid_request');
  });

  test('returns 404 for nonexistent table', async ({ page }) => {
    const resp = await page.request.get('/api/tables/nope');
    expect(resp.status()).toBe(404);
    const body = await resp.json();
    expect(body.error.code).toBe('not_found');
  });

  test('handles article detail with nonexistent ID gracefully', async ({ page }) => {
    const resp = await page.request.get('/api/articles/99999?run_id=1');
    expect(resp.status()).toBe(404);
  });

  test('frontend renders error state for invalid view', async ({ page }) => {
    await goto(page, '/?view=nonexistent');
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toBeAttached();
  });
});

// ── 10. Responsive layout ─────────────────────────────────────────────

test.describe('Responsive layout', () => {
  test('renders on mobile viewport (375px)', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await goto(page, contextURL({ view: 'overview' }));
    await expect(page.locator('body')).toBeAttached();
    const body = page.locator('body');
    await expect(body).toContainText(/Overview|Corpus|Relationships|Provenance|Evaluation|Advanced/i);
    await expect(body).not.toContainText(/\bTrash\b/);
  });

  test('renders on tablet viewport (768px)', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await goto(page, contextURL({ view: 'overview' }));
    await expect(page.locator('body')).toBeAttached();
  });

  test('renders on desktop viewport (1280px)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await goto(page, contextURL({ view: 'overview' }));
    await expect(page.locator('body')).toBeAttached();
  });
});

// ── 11. Dark mode ─────────────────────────────────────────────────────

test.describe('Dark mode', () => {
  test('respects prefers-color-scheme: dark', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark' });
    await goto(page, contextURL({ view: 'overview' }));
    await expect(page.locator('body')).toBeAttached();
    const bg = await page.evaluate(() => {
      const style = getComputedStyle(document.body);
      return style.backgroundColor;
    });
    expect(bg).not.toBe('rgb(255, 255, 255)');
  });

  test('respects prefers-color-scheme: light', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'light' });
    await goto(page, contextURL({ view: 'overview' }));
    await expect(page.locator('body')).toBeAttached();
  });
});

// ── 13. Accessibility ─────────────────────────────────────────────────

test.describe('Accessibility', () => {
  test('page has a skip-to-content link', async ({ page }) => {
    await goto(page, '/');
    const skipLink = page.locator('.skip-link, a[href="#main-content"], [href="#main"]').first();
    await expect(skipLink).toBeAttached();
  });

  test('main content area has a landmark role or id', async ({ page }) => {
    await goto(page, '/');
    const main = page.locator('main, [role="main"], #main-content, #main');
    await expect(main).toBeAttached();
  });

  test('navigation is a landmark', async ({ page }) => {
    await goto(page, '/');
    const nav = page.getByRole('navigation', { name: 'Breadcrumb' });
    await expect(nav).toBeAttached();
  });

  test('images have alt text', async ({ page }) => {
    await goto(page, '/');
    const images = page.locator('img');
    const count = await images.count();
    for (let i = 0; i < count; i++) {
      const alt = await images.nth(i).getAttribute('alt');
      expect(alt).not.toBeNull();
    }
  });
});

// ── 14. Search context persistence ────────────────────────────────────

test.describe('Search context', () => {
  test('selecting a search revision shows its plans', async ({ page }) => {
    await goto(page, contextURL({ search_id: SEARCH_DL }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/r1|r2|r3|execution plan|revision/i);
  });

  test('viewing a failed run shows failure indicators', async ({ page }) => {
    await goto(page, contextURL({ search_id: SEARCH_DL, search_revision_id: REV_DL_R1, plan_id: PLAN_DL_R1, run_id: RUN_2_FAILED, view: 'overview' }));
    await page.waitForLoadState('networkidle');
    const body = page.locator('body');
    await expect(body).toContainText(/fail|error|incomplete/i);
  });
});

// ── 15. Expandable rows ─────────────────────────────────────────────────

test.describe('Expandable article rows', () => {
  test('uses an unlabeled, plain disclosure column', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles' }));
    const disclosureHeader = page.locator('.rw-corpus-table thead th').first();
    await expect(disclosureHeader).toHaveText('');
    const toggle = page.locator('.expand-toggle').first();
    await expect(toggle).toHaveCSS('border-top-style', 'none');
  });

  test('toggle arrow expands a row showing property grid', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles' }));
    await page.waitForLoadState('networkidle');
    const toggle = page.locator('.expand-toggle').first();
    await toggle.click();
    const expandRow = page.locator('tr.expansion-row').first();
    await expect(expandRow).not.toHaveAttribute('hidden');
    await expect(expandRow.locator('.rw-property-grid')).toBeAttached();
    await expect(expandRow).toContainText(/title|journal|publisher|work_id|year|source|doi|validation_status|citation_count|reference_count|producer_stage|created_at/i);
  });

  test('clicking anywhere on the row expands it', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles' }));
    await page.waitForLoadState('networkidle');
    const dataCell = page.locator('table tbody tr').first().locator('td').nth(1);
    await dataCell.click();
    const expandRow = page.locator('tr.expansion-row').first();
    await expect(expandRow).not.toHaveAttribute('hidden');
  });

  test('clicking toggle again collapses the row', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles' }));
    await page.waitForLoadState('networkidle');
    const toggle = page.locator('.expand-toggle').first();
    await toggle.click();
    await toggle.click();
    const expandRow = page.locator('tr.expansion-row').first();
    await expect(expandRow).toHaveAttribute('hidden');
  });

  test('toggle arrow and aria-expanded update on click', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles' }));
    await page.waitForLoadState('networkidle');
    const toggle = page.locator('.expand-toggle').first();
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(await toggle.textContent()).toBe('▶');
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(await toggle.textContent()).toBe('▼');
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(await toggle.textContent()).toBe('▶');
  });

  test('multiple rows can be expanded simultaneously', async ({ page }) => {
    await goto(page, contextURL({ view: 'corpus', section: 'articles' }));
    await page.waitForLoadState('networkidle');
    await page.locator('.expand-toggle').nth(0).click();
    await page.locator('.expand-toggle').nth(1).click();
    const expandRows = page.locator('tr.expansion-row');
    await expect(expandRows.nth(0)).not.toHaveAttribute('hidden');
    await expect(expandRows.nth(1)).not.toHaveAttribute('hidden');
  });
});

// ── 16. Dismissible messages ───────────────────────────────────────────

test.describe('Dismissible messages', () => {
  test('clicking close button hides the message', async ({ page }) => {
    await goto(page, contextURL({ view: 'overview' }));
    await page.evaluate(() => {
      const message = document.createElement('div');
      message.className = 'ui info message';
      message.dataset.testDismissible = '';
      message.innerHTML = '<button type="button" class="close" aria-label="Dismiss test message">×</button><p>Deterministic dismissible message.</p>';
      const main = document.querySelector('#main-content');
      if (!main) throw new Error('Main content is unavailable');
      main.prepend(message);
    });
    const message = page.locator('[data-test-dismissible]');
    await expect(message).toBeVisible();
    await message.getByRole('button', { name: 'Dismiss test message' }).click();
    await expect(message).toBeHidden();
  });
});

// ── 17. Mobile navigation toggle ──────────────────────────────────────

test.describe('Mobile navigation toggle', () => {
  test('mobile nav toggle shows and hides navigation links', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await goto(page, contextURL({ view: 'overview' }));
    const toggle = page.locator('#mobile-nav-toggle');
    await expect(toggle).toBeVisible();
    const nav = page.locator('.rw-primary-nav');
    // On mobile, the nav should not have the open class initially
    const initialClass = await nav.getAttribute('class');
    expect(initialClass).not.toContain('rw-mobile-nav-open');
    await toggle.click();
    // After click, the nav gets the open class
    const afterClickClass = await nav.getAttribute('class');
    expect(afterClickClass).toContain('rw-mobile-nav-open');
    // Clicking the toggle again closes the menu
    await toggle.click();
    const afterSecondClass = await nav.getAttribute('class');
    expect(afterSecondClass).not.toContain('rw-mobile-nav-open');
  });
});
