// Bounded relationship exploration with common and advanced graph filters.
import { app, esc, value, graphFilters, pageHeader, emptyState, list, filterChips, formatNumber } from '../state.ts';
import { api } from '../api.ts';
import { graphClusters, graphField, graphQuery, graphResult, mountGraph } from '../components/graph.ts';
import { setURL, bindFocusContext } from '../router.ts';

const graphModes: Record<string, { label: string; description: string }> = {
  research_network: {
    label: 'Research network',
    description: 'Articles, observed authors, reference mentions, raw referenced-author strings, internal citations, co-authorships, and shared references.'
  },
  article_author: {
    label: 'Articles and authors',
    description: 'Authorship relationships between immutable article revisions and observed author occurrences.'
  },
  citation: {
    label: 'Internal citations',
    description: 'Directional citations that resolve to another selected article in this historical run.'
  },
  article_reference: {
    label: 'Articles and references',
    description: 'Article revisions connected to their captured reference mentions.'
  },
};

const filterLabels: Record<string, string> = {
  q: 'Title or DOI', author: 'Author', orcid: 'ORCID', reference: 'Reference', source: 'Source',
  year_min: 'Year from', year_max: 'Year to', citation_min: 'Citations from', citation_max: 'Citations to',
  reference_min: 'References from', reference_max: 'References to', article_limit: 'Article limit'
};

/** Returns markup for selecting a relationship graph mode. */
function modeControl(current: string): string {
  return '<section class="rw-graph-model"><div class="rw-graph-model__heading"><h4>Graph model</h4><p>Changing the model updates the network immediately and preserves the current filters.</p></div>'
    + '<div class="rw-segmented-control" role="group" aria-label="Graph model">'
    + Object.entries(graphModes).map(function([id, mode]) {
      const checked = id === current ? ' checked' : '';
      const active = id === current ? ' active' : '';
      return '<label class="item' + active + '"><input type="radio" name="graph_mode" value="' + id + '"' + checked + '>'
        + '<span>' + esc(mode.label) + '</span></label>';
    }).join('') + '</div></section>';
}

/** Returns markup summarizing the active relationship filters. */
function appliedFilters(): string {
  const filters: Record<string, string> = {};
  Object.keys(filterLabels).forEach(function(key) {
    if (value(key)) {
      filters[key] = value(key);
    }
  });
  const clear: Record<string, any> = { view: 'relationships', mode: value('mode') || 'research_network', node: '' };
  Object.keys(filterLabels).forEach(function(key) { clear[key] = ''; });
  return filterChips(filters, filterLabels, { clearUpdates: clear });
}

/** Returns markup summarizing connected graph clusters. */
function clusterSummary(data: any): string {
  const counts = data.counts || {};
  const clusters = graphClusters(list(data, ['nodes']), list(data, ['edges'])).components;
  const largest = clusters.reduce(function(size, cluster) { return Math.max(size, cluster.size); }, 0);
  const items = [
    ['Connected clusters', clusters.length],
    ['Largest cluster size', largest],
    ['Rendered entities', counts.nodes_rendered || 0],
    ['Rendered relationships', counts.edges_rendered || 0],
  ];
  return '<dl class="rw-graph-cluster-summary">' + items.map(function([label, count]) {
    return '<div><dt>' + esc(label) + '</dt><dd>' + formatNumber(count) + '</dd></div>';
  }).join('') + '</dl>';
}

/** Asynchronously implements relationships view for the viewer. */
export async function relationshipsView(): Promise<void> {
  if (!value('run_id')) {
    app.innerHTML = emptyState('Relationships', 'Select a run attempt to explore its bounded authorship, citation, and reference-mention relationships.', '<button type="button" data-focus-context>Focus context selector</button>');
    bindFocusContext();
    return;
  }

  var mode = value('mode');
  if (!graphModes[mode]) {
    mode = 'research_network';
  }
  const queryParams: Record<string, any> = graphQuery();
  queryParams.run_id = value('run_id');
  queryParams.mode = mode;
  if (!queryParams.article_limit) {
    queryParams.article_limit = 2000;
  }
  const data = await api('/api/graph', queryParams, { method: 'GET', headers: { Accept: 'application/json' } });
  const modeDefinition = graphModes[mode];

  const filterPanel = '<aside class="ui segment rw-relationship-filters">'
    + '<div class="ui top attached header"><div><h3>Explore a bounded network</h3><p>Choose a graph model, then refine the matching article set independently.</p></div></div>'
    + '<div class="content">'
    + modeControl(mode)
    + '<form id="graph-form" class="rw-graph__controls"><div class="rw-graph-filter-heading"><h4>Article filters</h4><p>Filters apply to the selected run without changing the graph model.</p></div>'
    + '<div class="rw-graph__common-filters">'
    + graphField('q', 'Title or DOI')
    + graphField('year_min', 'Year from', 'number')
    + graphField('year_max', 'Year to', 'number')
    + graphField('source', 'Source')
    + '</div>'
    + '<details class="rw-filter-disclosure"><summary>Advanced filters</summary><div class="rw-graph__advanced-filters">'
    + graphField('author', 'Author')
    + graphField('orcid', 'ORCID')
    + graphField('reference', 'Reference')
    + graphField('citation_min', 'Citations from', 'number')
    + graphField('citation_max', 'Citations to', 'number')
    + graphField('reference_min', 'References from', 'number')
    + graphField('reference_max', 'References to', 'number')
    + '<label>Article limit (1\u20132,000)<input name="article_limit" type="number" min="1" max="2000" value="' + esc(value('article_limit') || '2000') + '"><small>If lowered, the earliest matching normalized revisions by revision ID are retained.</small></label>'
    + '</div></details>'
    + appliedFilters()
    + '<div class="rw-graph__filter-actions"><button type="submit" class="ui primary button">Apply filters</button>'
    + '<button type="button" id="graph-reset" class="ui basic button">Reset filters</button></div>'
    + '</form></div></aside>';

  const networkPanel = '<section class="ui segment rw-relationship-network">'
    + '<div class="ui top attached header"><div><h3>' + esc(modeDefinition.label) + '</h3><p>' + esc(modeDefinition.description) + '</p></div>'
    + '<span class="ui label">' + formatNumber(list(data, ['edges']).length) + ' relationships</span></div>'
    + '<div class="content">' + clusterSummary(data) + graphResult(data) + '</div></section>';

  app.innerHTML = pageHeader('Bounded relationship exploration', 'Relationships', 'Investigate how articles, authors, and captured references connect inside the selected historical run.')
    + '<div class="rw-relationship-layout">' + filterPanel + networkPanel + '</div>';

  document.querySelector('#graph-form')!.addEventListener('submit', function(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget as HTMLFormElement);
    const updates: Record<string, any> = { view: 'relationships', node: '' };
    graphFilters.filter(function(name) { return name !== 'mode'; }).forEach(function(name) {
      const raw = String(form.get(name) || '');
      updates[name] = name === 'article_limit' && raw === '2000' ? '' : raw;
    });
    updates.mode = mode;
    setURL(updates);
  });

  document.querySelectorAll<HTMLInputElement>('input[name="graph_mode"]').forEach(function(input) {
    input.addEventListener('change', function() {
      if (input.checked) {
        setURL({ view: 'relationships', mode: input.value, node: '' });
      }
    });
  });

  document.querySelector('#graph-reset')!.addEventListener('click', function() {
    const updates: Record<string, any> = { view: 'relationships', mode: 'research_network', node: '' };
    graphFilters.forEach(function(name) {
      if (name !== 'mode') {
        updates[name] = '';
      }
    });
    setURL(updates);
  });

  mountGraph(data);
}