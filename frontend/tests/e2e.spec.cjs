// @ts-check
const { test, expect } = require('@playwright/test');

test.skip(process.env.E2E_SPEC !== '1', 'Run through make test-e2e with a generated pipeline database.');

/** Resolves the generated database's persistent URL-state identifiers through public APIs. */
async function generatedContext(request) {
  const searchResponse = await request.get('/api/searches');
  expect(searchResponse.ok()).toBeTruthy();
  const searchBody = await searchResponse.json();
  const search = searchBody.searches.find((item) => item.search_id === 'e2e-deterministic');
  expect(search).toBeTruthy();
  expect(search.revisions).toHaveLength(1);
  const revision = search.revisions[0];

  const planResponse = await request.get('/api/plans', { params: { search_revision_id: revision.id } });
  expect(planResponse.ok()).toBeTruthy();
  const planBody = await planResponse.json();
  expect(planBody.plans).toHaveLength(1);
  const plan = planBody.plans[0];

  const runResponse = await request.get('/api/runs', { params: { plan_id: plan.id } });
  expect(runResponse.ok()).toBeTruthy();
  const runBody = await runResponse.json();
  const run = runBody.runs.find((item) => item.status === 'completed');
  expect(run).toBeTruthy();

  return {
    search_id: search.id,
    search_revision_id: revision.id,
    plan_id: plan.id,
    run_id: run.id,
  };
}

/** Builds a context-preserving application URL for one generated viewer route. */
function generatedURL(context, updates) {
  const params = new URLSearchParams({
    search_id: String(context.search_id),
    search_revision_id: String(context.search_revision_id),
    plan_id: String(context.plan_id),
    run_id: String(context.run_id),
    ...updates,
  });
  return `/?${params.toString()}`;
}

test('pipeline evidence is consistent across Corpus, Provenance, and Evaluation', async ({ page, request }) => {
  const context = await generatedContext(request);

  await page.goto(generatedURL(context, { view: 'corpus', section: 'articles' }));
  await page.waitForLoadState('networkidle');
  const corpus = page.locator('.rw-corpus-table--articles');
  await expect(corpus).toBeVisible();
  await expect(corpus).toContainText('Offline Complete One');
  await expect(corpus).toContainText('Offline Complete Two');
  await expect(corpus).not.toContainText('Provider Enrichment Candidate');
  await expect(corpus).not.toContainText('Validation Discarded Invalid DOI');

  await page.goto(generatedURL(context, { view: 'provenance', section: 'audit' }));
  await page.waitForLoadState('networkidle');
  const audit = page.locator('#audit-event-stream');
  await expect(audit).toBeVisible();
  await expect(audit).toContainText('Run Started');
  await expect(audit).toContainText('Validation Changed');
  await expect(audit).toContainText('Pdf Inventory Registered');
  await expect(audit).toContainText('Run Completed');

  await page.goto(generatedURL(context, { view: 'evaluation' }));
  await page.waitForLoadState('networkidle');
  const evaluation = page.locator('.rw-evaluation-table');
  await expect(evaluation).toBeVisible();
  await expect(evaluation).toContainText('Offline Complete One');
  await expect(evaluation).toContainText('Offline Complete Two');
  await expect(evaluation.locator('.ui.red.label', { hasText: 'Not Available' })).toHaveCount(2);
});
