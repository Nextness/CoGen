// Home: workspace history, aggregate context metrics, and reversible run lifecycle controls.
import {
  app, state, esc, list, pickID, formatNumber, formatTime, formatDuration, statusChip,
  pageHeader, panel, link, params
} from '../state.ts';
import { api, mutate } from '../api.ts';
import { render } from '../router.ts';

/** Returns a clean Deepdive URL for one complete research context. */
function deepdiveLink(searchID: any, revisionID: any, planID: any, runID: any): string {
  const updates: Record<string, any> = {};
  for (const key of params().keys()) updates[key] = '';
  return link({ ...updates, view: 'overview', search_id: searchID, search_revision_id: revisionID, plan_id: planID, run_id: runID });
}

/** Returns one compact search-history card with revision, plan, and attempt counts. */
function searchCard(search: any, plans: any[], runs: any[]): string {
  const revisions = list(search, ['revisions', 'search_revisions']);
  const revisionIDs = new Set(revisions.map(function(revision) { return String(pickID(revision)); }));
  const searchPlans = plans.filter(function(plan) { return revisionIDs.has(String(plan.search_revision_id)); });
  const planIDs = new Set(searchPlans.map(function(plan) { return String(pickID(plan)); }));
  const searchRuns = runs.filter(function(run) { return revisionIDs.has(String(run.search_revision_id)) || planIDs.has(String(run.execution_plan_id)); });
  const latest = searchRuns.reduce(function(current: any, run) {
    if (!current || String(run.started_at || '') > String(current.started_at || '')) return run;
    return current;
  }, null);
  return '<article class="rw-home-search-card"><header><div><p class="rw-eyebrow">Search term</p><h3>' + esc(search.search_id || pickID(search)) + '</h3></div>'
    + '<span class="ui basic label">Created ' + esc(formatTime(search.created_at)) + '</span></header>'
    + '<dl class="rw-home-search-card__metrics"><div><dt>Search revisions</dt><dd>' + formatNumber(revisions.length) + '</dd></div>'
    + '<div><dt>Execution plans</dt><dd>' + formatNumber(searchPlans.length) + '</dd></div>'
    + '<div><dt>Run attempts</dt><dd>' + formatNumber(searchRuns.length) + '</dd></div>'
    + '<div><dt>Latest execution</dt><dd>' + esc(latest ? formatTime(latest.started_at) : '—') + '</dd></div></dl>'
    + '<div class="rw-home-revisions">' + (revisions.map(function(revision) {
      const revisionPlans = searchPlans.filter(function(plan) { return String(plan.search_revision_id) === String(pickID(revision)); });
      const revisionPlanIDs = new Set(revisionPlans.map(function(plan) { return String(pickID(plan)); }));
      const revisionRuns = searchRuns.filter(function(run) { return revisionPlanIDs.has(String(run.execution_plan_id)); });
      return '<div><strong>' + esc(revision.label || ('Revision ' + pickID(revision))) + '</strong><span>'
        + formatNumber(revisionPlans.length) + ' plans · ' + formatNumber(revisionRuns.length) + ' attempts</span></div>';
    }).join('') || '<p class="ui faded text">No search revisions are recorded.</p>') + '</div></article>';
}

/** Returns the run-management table for all planned attempts in the workspace. */
function runTable(searches: any[], plans: any[], runs: any[]): string {
  const revisionContext = new Map<string, { search: any; revision: any }>();
  searches.forEach(function(search) {
    list(search, ['revisions', 'search_revisions']).forEach(function(revision) {
      revisionContext.set(String(pickID(revision)), { search: search, revision: revision });
    });
  });
  const plansByID = new Map(plans.map(function(plan) { return [String(pickID(plan)), plan]; }));
  const rows = runs.slice().sort(function(a, b) { return Number(pickID(b)) - Number(pickID(a)); }).map(function(run) {
    const plan = plansByID.get(String(run.execution_plan_id));
    const context = revisionContext.get(String(run.search_revision_id || plan?.search_revision_id));
    const canExplore = Boolean(context && plan);
    const visibility = run.visibility_state || 'active';
    var lifecycleAction;
    if (run.status === 'running') {
      lifecycleAction = '<button type="button" class="ui basic button" disabled>Running</button>';
    } else if (visibility === 'trashed') {
      lifecycleAction = '<button type="button" class="ui basic button" data-run-visibility="active" data-run-id="' + esc(pickID(run)) + '">Restore</button>';
    } else {
      lifecycleAction = '<button type="button" class="ui danger basic button" data-run-visibility="trashed" data-run-id="' + esc(pickID(run)) + '">Move to trash</button>';
    }
    const explore = canExplore
      ? '<a class="ui primary button" href="' + deepdiveLink(pickID(context!.search), pickID(context!.revision), pickID(plan!), pickID(run)) + '">Explore</a>'
      : '<span class="ui faded text">Context unavailable</span>';
    return '<tr data-home-run="' + esc(pickID(run)) + '"><td><strong>Run ' + esc(pickID(run)) + '</strong><small>Attempt ' + esc(run.attempt_number || '—') + '</small></td>'
      + '<td>' + esc(context?.search?.search_id || 'Unplanned') + '</td><td>' + esc(context?.revision?.label || '—') + '</td>'
      + '<td>Plan ' + esc(run.execution_plan_id || '—') + '</td><td>' + esc(formatTime(run.started_at)) + '</td>'
      + '<td>' + esc(formatDuration(run.started_at, run.finished_at)) + '</td><td>' + statusChip(run.status) + '</td>'
      + '<td>' + statusChip(visibility) + '</td><td><div class="rw-inline-group">' + explore + lifecycleAction + '</div></td></tr>';
  }).join('');
  return '<div class="table-wrap" aria-label="Run attempts table"><table class="ui table rw-home-runs"><thead><tr><th>Run attempt</th><th>Search</th><th>Revision</th><th>Execution plan</th><th>Started</th><th>Duration</th><th>Outcome</th><th>Visibility</th><th>Actions</th></tr></thead>'
    + '<tbody>' + (rows || '<tr><td colspan="9" class="empty">No run attempts are recorded.</td></tr>') + '</tbody></table></div>';
}

/** Returns the confirmation dialog used for reversible run lifecycle changes. */
function runDialog(): string {
  return '<div class="rw-modal-backdrop" data-run-dialog hidden><section class="rw-modal" role="dialog" aria-modal="true" aria-labelledby="run-dialog-title" aria-describedby="run-dialog-description">'
    + '<header class="rw-modal__header"><div><p class="rw-eyebrow">Run lifecycle</p><h3 id="run-dialog-title">Change run visibility</h3></div><button type="button" class="ui icon basic button" data-run-dialog-close aria-label="Close run lifecycle dialog">×</button></header>'
    + '<form class="ui form rw-modal__body" data-run-dialog-form><p id="run-dialog-description"></p><label class="field" data-run-reason-field>Reason<textarea name="reason" maxlength="1000" rows="3" placeholder="Why should this run move out of the active workspace?"></textarea></label>'
    + '<p class="ui info message" data-run-dialog-guidance></p><p class="ui error message" data-run-dialog-error role="alert" hidden></p>'
    + '<div class="rw-modal__actions"><button type="button" class="ui basic button" data-run-dialog-close>Cancel</button><button type="submit" class="ui danger button" data-run-dialog-submit>Confirm</button></div></form></section></div>';
}

/** Binds confirmation and mutation behavior for Home run lifecycle controls. */
function bindRunLifecycle(runs: any[]): void {
  const dialogElement = document.querySelector<HTMLElement>('[data-run-dialog]');
  if (!dialogElement) return;
  const dialog = dialogElement;
  const form = dialog.querySelector('[data-run-dialog-form]') as HTMLFormElement;
  const title = dialog.querySelector('#run-dialog-title') as HTMLElement;
  const description = dialog.querySelector('#run-dialog-description') as HTMLElement;
  const guidance = dialog.querySelector('[data-run-dialog-guidance]') as HTMLElement;
  const reasonField = dialog.querySelector('[data-run-reason-field]') as HTMLElement;
  const reason = form.elements.namedItem('reason') as HTMLTextAreaElement;
  const submit = dialog.querySelector('[data-run-dialog-submit]') as HTMLButtonElement;
  const error = dialog.querySelector('[data-run-dialog-error]') as HTMLElement;

  /** Dismisses the lifecycle dialog and clears its transient form state. */
  function close(): void {
    dialog.hidden = true;
    document.body.classList.remove('rw-modal-open');
    error.hidden = true;
    form.reset();
  }
  /** Configures and opens the lifecycle dialog for the selected run action. */
  function open(button: HTMLButtonElement): void {
    const run = runs.find(function(item) { return String(pickID(item)) === button.dataset.runId; });
    if (!run) return;
    dialog.dataset.runId = button.dataset.runId;
    dialog.dataset.visibilityState = button.dataset.runVisibility;
    const trashing = button.dataset.runVisibility === 'trashed';
    title.textContent = trashing ? 'Move run ' + button.dataset.runId + ' to trash?' : 'Restore run ' + button.dataset.runId + '?';
    description.textContent = trashing
      ? 'This removes the attempt from active workspace results without deleting immutable evidence.'
      : 'This returns the attempt to active workspace results.';
    guidance.textContent = trashing
      ? 'The run, its provenance, and any review history remain stored and can be restored from Home.'
      : 'Restoring does not change the captured outcome or execution evidence.';
    reasonField.hidden = !trashing;
    submit.textContent = trashing ? 'Move to trash' : 'Restore run';
    submit.classList.toggle('danger', trashing);
    submit.classList.toggle('primary', !trashing);
    dialog.hidden = false;
    document.body.classList.add('rw-modal-open');
    (trashing ? reason : submit).focus();
  }

  document.querySelectorAll<HTMLButtonElement>('[data-run-visibility]').forEach(function(button) {
    button.addEventListener('click', function() { open(button); });
  });
  dialog.querySelectorAll<HTMLButtonElement>('[data-run-dialog-close]').forEach(function(button) { button.addEventListener('click', close); });
  dialog.addEventListener('click', function(event) { if (event.target === dialog) close(); });
  dialog.addEventListener('keydown', function(event) { if (event.key === 'Escape' && !dialog.hidden) close(); });
  form.addEventListener('submit', async function(event) {
    event.preventDefault();
    error.hidden = true;
    submit.disabled = true;
    submit.classList.add('loading');
    try {
      await mutate('/api/runs/' + encodeURIComponent(dialog.dataset.runId!) + '/visibility', 'PUT', {
        visibility_state: dialog.dataset.visibilityState,
        reason: dialog.dataset.visibilityState === 'trashed' ? reason.value : ''
      });
      state.runs = [];
      close();
      await render();
    } catch (failure: any) {
      error.textContent = failure.message;
      error.hidden = false;
    } finally {
      submit.disabled = false;
      submit.classList.remove('loading');
    }
  });
}

/** Renders the workspace Home page and its research-history controls. */
export async function homeView(): Promise<void> {
  const searchResponse = await api('/api/searches');
  const searches = list(searchResponse, ['searches', 'items']);
  state.searches = searches;
  const revisions = searches.flatMap(function(search) { return list(search, ['revisions', 'search_revisions']); });
  const [planResponses, runResponse] = await Promise.all([
    Promise.all(revisions.map(function(revision) { return api('/api/plans', { search_revision_id: pickID(revision) }); })),
    api('/api/runs', { include_trashed: 'true' })
  ]);
  const plans = planResponses.flatMap(function(response) { return list(response, ['plans', 'items']); });
  const runs = list(runResponse, ['runs', 'items']);
  const latest = runs.reduce(function(current: any, run) {
    if (!current || String(run.started_at || '') > String(current.started_at || '')) return run;
    return current;
  }, null);
  const completed = runs.filter(function(run) { return run.status === 'completed'; }).length;

  const metrics = '<div class="rw-kpi-grid rw-home-kpis">'
    + '<div class="rw-kpi"><span class="label">Search terms</span><span class="value">' + formatNumber(searches.length) + '</span><small>Declared research scopes</small></div>'
    + '<div class="rw-kpi"><span class="label">Search revisions</span><span class="value">' + formatNumber(revisions.length) + '</span><small>Immutable configurations</small></div>'
    + '<div class="rw-kpi"><span class="label">Execution plans</span><span class="value">' + formatNumber(plans.length) + '</span><small>Resolved pipeline plans</small></div>'
    + '<div class="rw-kpi"><span class="label">Run attempts</span><span class="value">' + formatNumber(runs.length) + '</span><small>' + formatNumber(completed) + ' completed</small></div></div>';
  const latestMessage = latest
    ? '<p class="ui info message"><span class="header">Latest execution</span>Run ' + esc(pickID(latest)) + ' started ' + esc(formatTime(latest.started_at)) + ' and took ' + esc(formatDuration(latest.started_at, latest.finished_at)) + '.</p>'
    : '<p class="ui neutral message">No pipeline run has been recorded yet.</p>';
  const searchHistory = searches.length
    ? '<div class="rw-home-searches">' + searches.map(function(search) { return searchCard(search, plans, runs); }).join('') + '</div>'
    : '<p class="empty">No search terms are recorded.</p>';

  app.innerHTML = pageHeader('', 'Home', 'Choose a captured run for Deepdive analysis or manage which completed attempts remain active.')
    + metrics + latestMessage
    + panel('Research history', 'Search terms organize immutable revisions, execution plans, and recorded run attempts.', searchHistory, 'rw-home-history')
    + panel('Run attempts', 'Explore a complete context or move a terminal run into the reversible trash lifecycle.', runTable(searches, plans, runs), 'rw-home-run-panel')
    + runDialog();
  bindRunLifecycle(runs);
}