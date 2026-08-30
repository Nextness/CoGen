import { test, expect } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';

/** Persistent identifier shape returned by generated fixture APIs. */
type Identifier = string | number;

/** Complete URL context for one generated pipeline run. */
type GeneratedContext = {
  search_id: Identifier;
  search_revision_id: Identifier;
  plan_id: Identifier;
  run_id: Identifier;
};

/** Search fields used to locate the deterministic generated fixture. */
type SearchRecord = {
  id: Identifier;
  search_id: string;
  revisions: Array<{ id: Identifier }>;
};

/** Plan fields used to locate the deterministic generated fixture. */
type PlanRecord = {
  id: Identifier;
};

/** Run fields used by generated-context and lineage checks. */
type RunRecord = {
  id: Identifier;
  status: string;
  attempt_number: number;
};

/** Evaluation fields used to resolve one generated work revision. */
type EvaluationRecord = {
  title: string;
  work_revision_id: Identifier;
};

test.skip(process.env.E2E_SPEC !== '1', 'Run through make test-e2e with a generated pipeline database.');
test.describe.configure({ mode: 'serial' });

/** Resolves the generated database's persistent URL-state identifiers through public APIs. */
async function generatedContext(request: APIRequestContext): Promise<GeneratedContext> {
  const searchResponse = await request.get('/api/searches');
  expect(searchResponse.ok()).toBeTruthy();
  const searchBody = await searchResponse.json();
  const searches = searchBody.searches as SearchRecord[];
  const search = searches.find((item) => item.search_id === 'e2e-deterministic');
  expect(search).toBeTruthy();
  if (!search) throw new Error('Generated E2E search is unavailable');
  expect(search.revisions).toHaveLength(1);
  const revision = search.revisions[0];

  const planResponse = await request.get('/api/plans', { params: { search_revision_id: revision.id } });
  expect(planResponse.ok()).toBeTruthy();
  const planBody = await planResponse.json();
  expect(planBody.plans).toHaveLength(1);
  const plan = (planBody.plans as PlanRecord[])[0];

  const runResponse = await request.get('/api/runs', { params: { plan_id: plan.id } });
  expect(runResponse.ok()).toBeTruthy();
  const runBody = await runResponse.json();
  const runs = runBody.runs as RunRecord[];
  const run = runs.find((item) => item.status === 'completed' && item.attempt_number === 1);
  expect(run).toBeTruthy();
  if (!run) throw new Error('Generated E2E run is unavailable');

  return {
    search_id: search.id,
    search_revision_id: revision.id,
    plan_id: plan.id,
    run_id: run.id,
  };
}

/** Builds a context-preserving viewer state for one generated route. */
function generatedState(context: GeneratedContext, updates: Record<string, Identifier>): Record<string, string> {
  const state: Record<string, string> = {
    search_id: String(context.search_id),
    search_revision_id: String(context.search_revision_id),
    plan_id: String(context.plan_id),
    run_id: String(context.run_id),
  };
  for (const [key, value] of Object.entries(updates)) state[key] = String(value);
  return state;
}

/** Seeds viewer state through sessionStorage and navigates to the clean generated route. */
async function visitGenerated(page: Page, context: GeneratedContext, updates: Record<string, Identifier>): Promise<void> {
  const state = generatedState(context, updates);
  const path = state.view === 'home' ? '/' : `/${state.view}`;
  await page.evaluate(({ seed, seedPath }: { seed: Record<string, string>; seedPath: string }) => {
    window.name = JSON.stringify(seed);
    try {
      sessionStorage.removeItem("rw-viewer-state");
      if (location.pathname === seedPath) history.replaceState(null, "", location.pathname);
    } catch (_) {
      // The initial about:blank document may deny sessionStorage access.
    }
  }, { seed: state, seedPath: path });
  await page.addInitScript(() => {
    const seed = window.name ? JSON.parse(window.name) : null;
    if (seed && !sessionStorage.getItem("rw-viewer-state")) sessionStorage.setItem("rw-viewer-state", JSON.stringify(seed));
  });
  await page.goto(path);
  await page.waitForLoadState('networkidle');
}

test('pipeline evidence is consistent across Corpus, Provenance, and Evaluation', async ({ page, request }) => {
  const context = await generatedContext(request);

  await visitGenerated(page, context, { view: 'corpus', section: 'articles' });
  await page.waitForLoadState('networkidle');
  const corpus = page.locator(`[data-table-owner="work_revisions"]`);
  await expect(corpus).toBeVisible();
  await expect(corpus).toContainText('Offline Complete One');
  await expect(corpus).toContainText('Offline Complete Two');
  await expect(corpus).not.toContainText('Provider Enrichment Candidate');
  await expect(corpus).not.toContainText('Validation Discarded Invalid DOI');

  await visitGenerated(page, context, { view: 'provenance', section: 'audit' });
  await page.waitForLoadState('networkidle');
  const audit = page.locator('#audit-event-stream');
  await expect(audit).toBeVisible();
  await expect(audit).toContainText('Run Started');
  await expect(audit).toContainText('Validation Changed');
  await expect(audit).toContainText('Pdf Inventory Registered');
  await expect(audit).toContainText('Run Completed');

  await visitGenerated(page, context, { view: 'evaluation' });
  await page.waitForLoadState('networkidle');
  const evaluation = page.locator('.rw-evaluation-table');
  await expect(evaluation).toBeVisible();
  await expect(evaluation).toContainText('Offline Complete One');
  await expect(evaluation).toContainText('Offline Complete Two');
  await expect(evaluation.locator('.ui.orange.label', { hasText: 'Not Available' })).toHaveCount(1);
  await expect(evaluation.locator('.ui.green.label', { hasText: 'Available' })).toHaveCount(1);
});

test('A2 inherits immutable A1 review heads and diverges without changing A1', async ({ page, request }) => {
  const context = await generatedContext(request);
  const runsResponse = await request.get('/api/runs', { params: { plan_id: context.plan_id } });
  const runRecords = (await runsResponse.json()).runs as RunRecord[];
  const completed = runRecords.filter((item) => item.status === 'completed').sort((left, right) => left.attempt_number - right.attempt_number);
  expect(completed).toHaveLength(2);
  const [a1Run, a2Run] = completed;
  const revisions: Record<string, Identifier> = {};
  for (const run of completed) {
    const response = await request.get(`/api/runs/${run.id}/evaluation`, { params: { per_page: 100, sort: 'title', order: 'asc' } });
    const rows = (await response.json()).rows as EvaluationRecord[];
    const row = rows.find((item) => item.title === 'Offline Complete One');
    expect(row).toBeTruthy();
    if (!row) throw new Error('Generated E2E work revision is unavailable');
    revisions[run.id] = row.work_revision_id;
  }
  const a1Revision = revisions[a1Run.id];
  const a2Revision = revisions[a2Run.id];

  const a1ContextResponse = await request.post(`/api/runs/${a1Run.id}/review-context`, { data: { parent_context_id: null } });
  expect(a1ContextResponse.status()).toBe(201);
  const a1Context = (await a1ContextResponse.json()).context;
  const a1ReviewResponse = await request.put(`/api/runs/${a1Run.id}/articles/${a1Revision}/review`, { data: { expected_version_id: null, status: 'approved', sub_statuses: [], reason: 'A1 approved evidence' } });
  expect(a1ReviewResponse.ok()).toBeTruthy();
  const a1Review = (await a1ReviewResponse.json()).review;
  const noteResponse = await request.post(`/api/runs/${a1Run.id}/articles/${a1Revision}/notes`, { data: { body: 'A1 future link [[note:2|target note]].' } });
  expect(noteResponse.status()).toBe(201);
  const a1Note = (await noteResponse.json()).note;
  const anchorResponse = await request.post(`/api/runs/${a1Run.id}/articles/${a1Revision}/anchors`, { data: { label: 'e2e-methods', page: 1, selected_text: 'Selectable E2E methods', rectangles: [{ x: 0.1, y: 0.1, width: 0.4, height: 0.08 }] } });
  expect(anchorResponse.status()).toBe(201);

  const a2ContextResponse = await request.post(`/api/runs/${a2Run.id}/review-context`, { data: { parent_context_id: a1Context.id } });
  expect(a2ContextResponse.status()).toBe(201);
  const inheritedReview = await (await request.get(`/api/runs/${a2Run.id}/articles/${a2Revision}/review`)).json();
  expect(inheritedReview.review.version.id).toBe(a1Review.version.id);
  expect(inheritedReview.review.inherited_from_context_id).toBe(a1Context.id);
  const inheritedNotes = await (await request.get(`/api/runs/${a2Run.id}/articles/${a2Revision}/notes`)).json();
  expect(inheritedNotes.notes[0].id).toBe(a1Note.id);
  const inheritedNote = await (await request.get(`/api/runs/${a2Run.id}/notes/${a1Note.id}`)).json();
  expect(inheritedNote.note.version.links[0].resolved).toBeFalsy();
  const targetResponse = await request.post(`/api/runs/${a2Run.id}/articles/${a2Revision}/notes`, { data: { body: 'A2 target note.' } });
  expect((await targetResponse.json()).note.id).toBe(2);
  const resolvedNote = await (await request.get(`/api/runs/${a2Run.id}/notes/${a1Note.id}`)).json();
  expect(resolvedNote.note.version.links[0].resolved).toBeTruthy();
  const editedNote = await request.post(`/api/runs/${a2Run.id}/notes/${a1Note.id}/versions`, { data: { expected_version_id: a1Note.version.id, state: 'active', body: 'A2 edited note.' } });
  expect(editedNote.status()).toBe(201);
  const changedReview = await request.put(`/api/runs/${a2Run.id}/articles/${a2Revision}/review`, { data: { expected_version_id: a1Review.version.id, status: 'not_approved', sub_statuses: ['out_of_scope'], reason: 'A2 changed evidence' } });
  expect(changedReview.ok()).toBeTruthy();

  const stableA1 = await (await request.get(`/api/runs/${a1Run.id}/articles/${a1Revision}/review`)).json();
  expect(stableA1.review.version.status).toBe('approved');
  expect(stableA1.review.version.id).toBe(a1Review.version.id);
  const stableA1Note = await (await request.get(`/api/runs/${a1Run.id}/notes/${a1Note.id}`)).json();
  expect(stableA1Note.note.version.id).toBe(a1Note.version.id);

  await visitGenerated(page, { ...context, run_id: a2Run.id }, { view: 'article', article_id: a2Revision });
  await page.waitForLoadState('networkidle');
  await expect(page.locator('[data-review-host]')).toContainText('A2 changed evidence');
  await page.getByRole('tab', { name: 'PDF anchors' }).click();
  await expect(page.locator('[data-anchor-list]')).toContainText('e2e-methods');
  await expect(page.locator(".rw-pdf-page .textLayer")).toContainText("Selectable E2E methods");
});
