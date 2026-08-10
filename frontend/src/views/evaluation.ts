// Evaluation: normalized articles and their manual PDF inventory state.
import {
  app, esc, value, link, pageSizes, pageHeader, emptyState,
  statusChip, humanLabel, formatTime, filterChips
} from '../state.ts';
import { api, mutate } from '../api.ts';
import { dataTable, bindTableControls } from '../components/data-table.ts';
import { bindFocusContext } from '../router.ts';

const evaluationSortFields = ['title', 'doi'];

/** Returns a context-preserving article link for an evaluation row. */
function titleLink(row: any): string {
  return '<a class="rw-table-title" href="' + link({ view: 'article', article_id: row.work_revision_id })
    + '" title="' + esc(row.title || 'Not recorded') + '"><span>'
    + esc(row.title || 'Not recorded') + '</span></a>';
}

/** Returns the recorded PDF inventory time or an unavailable label. */
function inventoriedTime(row: any): string {
  if (!row.inventoried_at) {
    return '<span class="ui faded text">&mdash;</span>';
  }
  return formatTime(row.inventoried_at);
}

/** Asynchronously implements evaluation view for the viewer. */
export async function evaluationView(): Promise<void> {
  const runID = value('run_id');
  if (!runID) {
    app.innerHTML = emptyState(
      'Evaluation',
      'Select a run attempt to evaluate its normalized article inventory.',
      '<button type="button" class="ui primary button" data-focus-context>Select a run attempt</button>'
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

  const controls = '<form class="ui form rw-table-controls" data-table-search>'
    + '<label class="rw-table-controls__search"><span>Search normalized articles</span><span class="ui input">'
    + '<input id="corpus-query" type="search" value="' + esc(query) + '" placeholder="Title or DOI">'
    + '<button type="button" class="clear" data-clear-query' + (query ? '' : ' disabled') + ' aria-label="Clear search">&times;</button></span></label>'
    + '<label class="rw-table-controls__size">Rows per page<select id="per-page">'
    + pageSizes.map(function(size) {
      return '<option value="' + size + '"' + (size === perPage ? ' selected' : '') + '>' + size + '</option>';
    }).join('')
    + '</select></label><button type="button" data-search-query class="ui primary button">Search</button></form>';

  const filters = query
    ? filterChips({ q: query }, { q: 'Search' }, { clearUpdates: { q: '', page: 1 } })
    : '';
  const table = dataTable('evaluation', data, {
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
        render: function(row) { return statusChip(humanLabel(row.inventory_status)); }
      },
      inventoried_at: { label: 'Inventoried at', className: 'col-captured-at', render: inventoriedTime },
      review_status: { label: 'Review status', render: function(row) { return statusChip(humanLabel(row.review_status || 'not_evaluated')); } },
      review_inherited: { label: 'Review source', render: function(row) { return row.review_inherited ? '<span class="ui label">Inherited</span>' : '<span class="ui faded text">This context</span>'; } }
    }
  });

  const reviewInitialized = (data.rows || []).some(function(row: any) { return row.review_context_initialized; });
  const startReview = reviewInitialized ? '' : '<button type="button" class="ui primary button" data-start-evaluation-review>Start review</button>';

  app.innerHTML = pageHeader(
    'Reading inventory',
    'Evaluation',
    'Review every normalized article in the selected run and whether its PDF has been manually inventoried.'
  ) + '<section class="ui segment rw-data-section"><div class="ui top attached header"><div>'
    + '<h3>Normalized article inventory</h3><p>Available PDFs were validated and added through the manual PDF-store tool.</p>'
    + '</div>' + startReview + '</div><div class="content">' + controls + filters + table + '</div></section>';

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