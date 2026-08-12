// Home: workspace history, aggregate context metrics, and reversible run lifecycle controls.
import {
  app, state, list, pickID, formatNumber, formatTime, formatDuration,
  StatusChip, link, params, PageHeader, Panel
} from '../state.tsx';
import { h, Fragment, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api, mutate } from '../api.tsx';
import { render } from '../router.tsx';

/** Returns a clean Deepdive URL for one complete research context. */
function deepdiveLink(searchID: any, revisionID: any, planID: any, runID: any): string {
  const updates: Record<string, any> = {};
  for (const key of params().keys()) updates[key] = '';
  return link({ ...updates, view: 'overview', search_id: searchID, search_revision_id: revisionID, plan_id: planID, run_id: runID });
}

/** Renders one compact search-history card with revision, plan, and attempt counts. */
function SearchCard(props: { search: any; plans: any[]; runs: any[] }): JSX.Element {
  const search = props.search;
  const revisions = list(search, ['revisions', 'search_revisions']);
  const revisionIDs = new Set(revisions.map(function(revision) { return String(pickID(revision)); }));
  const searchPlans = props.plans.filter(function(plan) { return revisionIDs.has(String(plan.search_revision_id)); });
  const planIDs = new Set(searchPlans.map(function(plan) { return String(pickID(plan)); }));
  const searchRuns = props.runs.filter(function(run) { return revisionIDs.has(String(run.search_revision_id)) || planIDs.has(String(run.execution_plan_id)); });
  const latest = searchRuns.reduce(function(current: any, run) {
    if (!current || String(run.started_at || '') > String(current.started_at || '')) return run;
    return current;
  }, null);
  return (
    <article className="rw-home-search-card">
      <header><div><p className="rw-eyebrow">Search term</p><h3>{search.search_id || pickID(search)}</h3></div><span className="ui basic label">Created {formatTime(search.created_at)}</span></header>
      <dl className="rw-home-search-card__metrics">
        <div><dt>Search revisions</dt><dd>{formatNumber(revisions.length)}</dd></div>
        <div><dt>Execution plans</dt><dd>{formatNumber(searchPlans.length)}</dd></div>
        <div><dt>Run attempts</dt><dd>{formatNumber(searchRuns.length)}</dd></div>
        <div><dt>Latest execution</dt><dd>{latest ? formatTime(latest.started_at) : '—'}</dd></div>
      </dl>
      <div className="rw-home-revisions">{revisions.length ? revisions.map(function(revision) {
        const revisionPlans = searchPlans.filter(function(plan) { return String(plan.search_revision_id) === String(pickID(revision)); });
        const revisionPlanIDs = new Set(revisionPlans.map(function(plan) { return String(pickID(plan)); }));
        const revisionRuns = searchRuns.filter(function(run) { return revisionPlanIDs.has(String(run.execution_plan_id)); });
        return <div><strong>{revision.label || ('Revision ' + pickID(revision))}</strong><span>{formatNumber(revisionPlans.length)} plans · {formatNumber(revisionRuns.length)} attempts</span></div>;
      }) : <p className="ui faded text">No search revisions are recorded.</p>}</div>
    </article>
  );
}

/** Renders the run-management table for all planned attempts in the workspace. */
function RunTable(props: { searches: any[]; plans: any[]; runs: any[] }): JSX.Element {
  const revisionContext = new Map<string, { search: any; revision: any }>();
  props.searches.forEach(function(search) {
    list(search, ['revisions', 'search_revisions']).forEach(function(revision) {
      revisionContext.set(String(pickID(revision)), { search: search, revision: revision });
    });
  });
  const plansByID = new Map(props.plans.map(function(plan) { return [String(pickID(plan)), plan]; }));
  const rows = props.runs.slice().sort(function(a, b) { return Number(pickID(b)) - Number(pickID(a)); }).map(function(run) {
    const plan = plansByID.get(String(run.execution_plan_id));
    const context = revisionContext.get(String(run.search_revision_id || plan?.search_revision_id));
    const canExplore = Boolean(context && plan);
    const visibility = run.visibility_state || 'active';
    var lifecycleAction: JSX.Element;
    if (run.status === 'running') {
      lifecycleAction = <button type="button" className="ui basic button" disabled>Running</button>;
    } else if (visibility === 'trashed') {
      lifecycleAction = <button type="button" className="ui basic button" data-run-visibility="active" data-run-id={pickID(run)}>Restore</button>;
    } else {
      lifecycleAction = <button type="button" className="ui danger basic button" data-run-visibility="trashed" data-run-id={pickID(run)}>Move to trash</button>;
    }
    const explore = canExplore
      ? <a className="ui primary button" href={deepdiveLink(pickID(context!.search), pickID(context!.revision), pickID(plan!), pickID(run))}>Explore</a>
      : <span className="ui faded text">Context unavailable</span>;
    return <tr data-home-run={pickID(run)}><td><strong>Run {pickID(run)}</strong><small>Attempt {run.attempt_number || '—'}</small></td>
      <td>{context?.search?.search_id || 'Unplanned'}</td><td>{context?.revision?.label || '—'}</td>
      <td>Plan {run.execution_plan_id || '—'}</td><td>{formatTime(run.started_at)}</td>
      <td>{formatDuration(run.started_at, run.finished_at)}</td><td><StatusChip raw={run.status} /></td>
      <td><StatusChip raw={visibility} /></td><td><div className="rw-inline-group">{explore}{lifecycleAction}</div></td></tr>;
  });
  return (
    <div className="table-wrap" aria-label="Run attempts table">
      <table className="ui table rw-home-runs">
        <thead><tr><th>Run attempt</th><th>Search</th><th>Revision</th><th>Execution plan</th><th>Started</th><th>Duration</th><th>Outcome</th><th>Visibility</th><th>Actions</th></tr></thead>
        <tbody>{rows.length ? rows : <tr><td colspan={9} className="empty">No run attempts are recorded.</td></tr>}</tbody>
      </table>
    </div>
  );
}

/** Renders the confirmation dialog used for reversible run lifecycle changes. */
function RunDialog(): JSX.Element {
  return (
    <div className="rw-modal-backdrop" data-run-dialog hidden>
      <section className="rw-modal" role="dialog" aria-modal="true" aria-labelledby="run-dialog-title" aria-describedby="run-dialog-description">
        <header className="rw-modal__header"><div><p className="rw-eyebrow">Run lifecycle</p><h3 id="run-dialog-title">Change run visibility</h3></div><button type="button" className="ui icon basic button" data-run-dialog-close aria-label="Close run lifecycle dialog">{"\u00D7"}</button></header>
        <form className="ui form rw-modal__body" data-run-dialog-form>
          <p id="run-dialog-description"></p>
          <label className="field" data-run-reason-field>Reason<textarea name="reason" maxlength={1000} rows={3} placeholder="Why should this run move out of the active workspace?"></textarea></label>
          <p className="ui info message" data-run-dialog-guidance></p>
          <p className="ui error message" data-run-dialog-error role="alert" hidden></p>
          <div className="rw-modal__actions"><button type="button" className="ui basic button" data-run-dialog-close>Cancel</button><button type="submit" className="ui danger button" data-run-dialog-submit>Confirm</button></div>
        </form>
      </section>
    </div>
  );
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
  const searchResponse = await api('/api/searches', {}, { method: 'GET', headers: { Accept: 'application/json' } });
  const searches = list(searchResponse, ['searches', 'items']);
  state.searches = searches;
  const revisions = searches.flatMap(function(search) { return list(search, ['revisions', 'search_revisions']); });
  const [planResponses, runResponse] = await Promise.all([
    Promise.all(revisions.map(function(revision) { return api('/api/plans', { search_revision_id: pickID(revision) }, { method: 'GET', headers: { Accept: 'application/json' } }); })),
    api('/api/runs', { include_trashed: 'true' }, { method: 'GET', headers: { Accept: 'application/json' } })
  ]);
  const plans = planResponses.flatMap(function(response) { return list(response, ['plans', 'items']); });
  const runs = list(runResponse, ['runs', 'items']);
  const latest = runs.reduce(function(current: any, run) {
    if (!current || String(run.started_at || '') > String(current.started_at || '')) return run;
    return current;
  }, null);
  const completed = runs.filter(function(run) { return run.status === 'completed'; }).length;

  const metrics = (
    <div className="rw-kpi-grid rw-home-kpis">
      <div className="rw-kpi"><span className="label">Search terms</span><span className="value">{formatNumber(searches.length)}</span><small>Declared research scopes</small></div>
      <div className="rw-kpi"><span className="label">Search revisions</span><span className="value">{formatNumber(revisions.length)}</span><small>Immutable configurations</small></div>
      <div className="rw-kpi"><span className="label">Execution plans</span><span className="value">{formatNumber(plans.length)}</span><small>Resolved pipeline plans</small></div>
      <div className="rw-kpi"><span className="label">Run attempts</span><span className="value">{formatNumber(runs.length)}</span><small>{formatNumber(completed)} completed</small></div>
    </div>
  );
  const latestMessage = latest
    ? <p className="ui info message"><span className="header">Latest execution</span>Run {pickID(latest)} started {formatTime(latest.started_at)} and took {formatDuration(latest.started_at, latest.finished_at)}.</p>
    : <p className="ui neutral message">No pipeline run has been recorded yet.</p>;
  const searchHistory = searches.length
    ? <div className="rw-home-searches">{searches.map(function(search) { return <SearchCard search={search} plans={plans} runs={runs} />; })}</div>
    : <p className="empty">No search terms are recorded.</p>;

  renderTree(
    <Fragment>
      <PageHeader kicker="" title="Home" description="Choose a captured run for Deepdive analysis or manage which completed attempts remain active." />
      {metrics}
      {latestMessage}
      <Panel title="Research history" description="Search terms organize immutable revisions, execution plans, and recorded run attempts." body={searchHistory} classes="rw-home-history" />
      <Panel title="Run attempts" description="Explore a complete context or move a terminal run into the reversible trash lifecycle." body={<RunTable searches={searches} plans={plans} runs={runs} />} classes="rw-home-run-panel" />
      <RunDialog />
    </Fragment>,
    app
  );
  bindRunLifecycle(runs);
}