// Evaluation: normalized articles and their manual PDF inventory state.
import {
  app, value, link, pageSizes, PageHeader, EmptyState, FilterChips,
  humanLabel, formatTime, StatusChip
} from '../state.tsx';
import { h, Fragment, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api, mutate } from '../api.tsx';
import { DataTable, bindTableControls } from '../components/data-table.tsx';
import { bindFocusContext } from '../router.tsx';

const evaluationSortFields = ['title', 'doi'];

/** Renders a context-preserving article link for an evaluation row. */
function titleLink(row: any): JSX.Element {
  return <a className="rw-table-title" href={link({ view: 'article', article_id: row.work_revision_id })} title={row.title || 'Not recorded'}><span>{row.title || 'Not recorded'}</span></a>;
}

/** Renders the recorded PDF inventory time or an unavailable label. */
function inventoriedTime(row: any): JSX.Element {
  if (!row.inventoried_at) {
    return <span className="ui faded text">{"\u2014"}</span>;
  }
  return <>{formatTime(row.inventoried_at)}</>;
}

/** Asynchronously implements evaluation view for the viewer. */
export async function evaluationView(): Promise<void> {
  const runID = value('run_id');
  if (!runID) {
    renderTree(
      <EmptyState title="Evaluation" detail="Select a run attempt to evaluate its normalized article inventory." action={<button type="button" className="ui primary button" data-focus-context>Select a run attempt</button>} />,
      app
    );
    bindFocusContext();
    return;
  }

  const page = Math.max(1, Number(value('page') || 1));
  var perPage = Number(value('per_page'));
  if (!pageSizes.includes(perPage)) {
    perPage = 50;
  }
  var sort = value('sort');
  if (!evaluationSortFields.includes(sort)) {
    sort = '';
  }
  const order = value('order').toLowerCase() === 'desc' ? 'desc' : 'asc';
  const query = value('q');
  const data = await api('/api/runs/' + encodeURIComponent(runID) + '/evaluation', {
    page: page,
    per_page: perPage,
    sort: sort,
    order: order,
    q: query
  }, { method: 'GET', headers: { Accept: 'application/json' } });

  const controls = (
    <form className="ui form rw-table-controls" data-table-search>
      <label className="rw-table-controls__search"><span>Search normalized articles</span><span className="ui input">
        <input id="corpus-query" type="search" value={query} placeholder="Title or DOI" />
        <button type="button" className="clear" data-clear-query disabled={!query} aria-label="Clear search">{"\u00D7"}</button>
      </span></label>
      <label className="rw-table-controls__size">Rows per page<select id="per-page">
        {pageSizes.map(function(size) {
          return <option value={size} selected={size === perPage}>{size}</option>;
        })}
      </select></label>
      <button type="button" data-search-query className="ui primary button">Search</button>
    </form>
  );

  const filters = query
    ? <FilterChips filters={{ q: query }} labels={{ q: 'Search' }} options={{ clearUpdates: { q: '', page: 1 } }} />
    : null;
  const table = <DataTable tableName="evaluation" result={data} context={{
    page: page,
    perPage: perPage,
    query: query,
    sortFields: evaluationSortFields,
    rowKey: 'work_revision_id',
    itemLabel: 'normalized articles',
    tableClass: 'rw-evaluation-table',
    columnConfig: {
      title: { label: 'Title', className: 'col-title', render: titleLink },
      doi: { label: 'DOI', className: 'col-doi' },
      source: { label: 'Source', className: 'col-source' },
      inventory_status: {
        label: 'Inventory Status',
        render: function(row) { return <StatusChip raw={humanLabel(row.inventory_status)} />; }
      },
      inventoried_at: { label: 'Inventoried at', className: 'col-captured-at', render: inventoriedTime },
      review_status: { label: 'Review status', render: function(row) { return <StatusChip raw={humanLabel(row.review_status || 'not_evaluated')} />; } },
      review_inherited: { label: 'Review source', render: function(row) { return row.review_inherited ? <span className="ui label">Inherited</span> : <span className="ui faded text">This context</span>; } }
    }
  }} />;

  const reviewInitialized = (data.rows || []).some(function(row: any) { return row.review_context_initialized; });
  const startReview = reviewInitialized ? null : <button type="button" className="ui primary button" data-start-evaluation-review>Start review</button>;

  renderTree(
    <Fragment>
      <PageHeader kicker="Reading inventory" title="Evaluation" description="Review every normalized article in the selected run and whether its PDF has been manually inventoried." />
      <section className="ui segment rw-data-section">
        <div className="ui top attached header"><div><h3>Normalized article inventory</h3><p>Available PDFs were validated and added through the manual PDF-store tool.</p></div>{startReview}</div>
        <div className="content">{controls}{filters}{table}</div>
      </section>
    </Fragment>,
    app
  );

  bindTableControls('evaluation', page);
  document.querySelector('[data-start-evaluation-review]')?.addEventListener('click', async function() {
    const context = await api('/api/runs/' + encodeURIComponent(runID) + '/review-context', {}, { method: 'GET', headers: { Accept: 'application/json' } });
    const proposed = context.proposed_parent;
    const prompt = proposed ? `Start review by inheriting ${proposed.inherited_work_count} matching work heads from run ${proposed.pipeline_run_id}?` : 'Start an empty review context for this run?';
    if (!window.confirm(prompt)) return;
    await mutate('/api/runs/' + encodeURIComponent(runID) + '/review-context', 'POST', { parent_context_id: proposed?.context_id || null });
    await evaluationView();
  });
}