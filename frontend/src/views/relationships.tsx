// Bounded relationship exploration with common and advanced graph filters.
import { app, value, graphFilters, PageHeader, EmptyState, list, filterChips, formatNumber, humanLabel } from '../state.tsx';
import { h, Fragment, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api } from '../api.tsx';
import { graphClusters, GraphField, graphQuery, GraphResult, mountGraph } from '../components/graph.tsx';
import { setURL, bindFocusContext } from '../router.tsx';

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

/** Renders markup for selecting a relationship graph mode. */
function ModeControl(props: { current: string }): JSX.Element {
  return (
    <section className="rw-graph-model">
      <div className="rw-graph-model__heading"><h4>Graph model</h4><p>Changing the model updates the network immediately and preserves the current filters.</p></div>
      <div className="rw-segmented-control" role="group" aria-label="Graph model">
        {Object.entries(graphModes).map(function([id, mode]) {
          const checked = id === props.current;
          const active = id === props.current ? ' active' : '';
          return (
            <label className={'item' + active}>
              <input type="radio" name="graph_mode" value={id} checked={checked} />
              <span>{mode.label}</span>
            </label>
          );
        })}
      </div>
    </section>
  );
}

/** Renders markup summarizing the active relationship filters. */
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

/** Renders markup summarizing connected graph clusters. */
function ClusterSummary(props: { data: any }): JSX.Element {
  const data = props.data;
  const counts = data.counts || {};
  const clusters = graphClusters(list(data, ['nodes']), list(data, ['edges'])).components;
  const largest = clusters.reduce(function(size, cluster) { return Math.max(size, cluster.size); }, 0);
  const items = [
    ['Connected clusters', clusters.length],
    ['Largest cluster size', largest],
    ['Rendered entities', counts.nodes_rendered || 0],
    ['Rendered relationships', counts.edges_rendered || 0],
  ];
  return (
    <dl className="rw-graph-cluster-summary">
      {items.map(function([label, count]) {
        return <div><dt>{label}</dt><dd>{formatNumber(count)}</dd></div>;
      })}
    </dl>
  );
}

/** Asynchronously implements relationships view for the viewer. */
export async function relationshipsView(): Promise<void> {
  if (!value('run_id')) {
    renderTree(<EmptyState title="Relationships" detail="Select a run attempt to explore its bounded authorship, citation, and reference-mention relationships." action={'<button type="button" data-focus-context>Focus context selector</button>'} />, app);
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

  const filterPanel = (
    <aside className="ui segment rw-relationship-filters">
      <div className="ui top attached header"><div><h3>Explore a bounded network</h3><p>Choose a graph model, then refine the matching article set independently.</p></div></div>
      <div className="content">
        <ModeControl current={mode} />
        <form id="graph-form" className="rw-graph__controls">
          <div className="rw-graph-filter-heading"><h4>Article filters</h4><p>Filters apply to the selected run without changing the graph model.</p></div>
          <div className="rw-graph__common-filters">
            <GraphField name="q" label="Title or DOI" />
            <GraphField name="year_min" label="Year from" type="number" />
            <GraphField name="year_max" label="Year to" type="number" />
            <GraphField name="source" label="Source" />
          </div>
          <details className="rw-filter-disclosure">
            <summary>Advanced filters</summary>
            <div className="rw-graph__advanced-filters">
              <GraphField name="author" label="Author" />
              <GraphField name="orcid" label="ORCID" />
              <GraphField name="reference" label="Reference" />
              <GraphField name="citation_min" label="Citations from" type="number" />
              <GraphField name="citation_max" label="Citations to" type="number" />
              <GraphField name="reference_min" label="References from" type="number" />
              <GraphField name="reference_max" label="References to" type="number" />
              <label>Article limit (1\u20132,000)<input name="article_limit" type="number" min="1" max="2000" value={value('article_limit') || '2000'} /><small>If lowered, the earliest matching normalized revisions by revision ID are retained.</small></label>
            </div>
          </details>
          {appliedFilters()}
          <div className="rw-graph__filter-actions">
            <button type="submit" className="ui primary button">Apply filters</button>
            <button type="button" id="graph-reset" className="ui basic button">Reset filters</button>
          </div>
        </form>
      </div>
    </aside>
  );

  const networkPanel = (
    <section className="ui segment rw-relationship-network">
      <div className="ui top attached header"><div><h3>{modeDefinition.label}</h3><p>{modeDefinition.description}</p></div>
      <span className="ui label">{formatNumber(list(data, ['edges']).length)} relationships</span></div>
      <div className="content"><ClusterSummary data={data} /><GraphResult data={data} /></div>
    </section>
  );

  renderTree(
    <Fragment>
      <PageHeader kicker="Bounded relationship exploration" title="Relationships" description="Investigate how articles, authors, and captured references connect inside the selected historical run." />
      <div className="rw-relationship-layout">{filterPanel}{networkPanel}</div>
    </Fragment>,
    app
  );

  document.querySelector('#graph-form')!.addEventListener('submit', function(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget as HTMLFormElement);
    const updates: Record<string, any> = { view: 'relationships', node: '' };
    graphFilters.filter(function(name) { return name !== 'mode'; }).forEach(function(name) {
      const raw = String(form.get(name) || '');
      updates[name] = name === 'article_limit' && raw === '2000' ? '' : raw;
    });
    updates.mode = mode;
    setURL(updates, false);
  });

  document.querySelectorAll<HTMLInputElement>('input[name="graph_mode"]').forEach(function(input) {
    input.addEventListener('change', function() {
      if (input.checked) {
        setURL({ view: 'relationships', mode: input.value, node: '' }, false);
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
    setURL(updates, false);
  });

  mountGraph(data);
}