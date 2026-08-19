// Bounded relationship exploration with common and advanced graph filters.
import { app, value, graphFilters, PageHeader, EmptyState, list, filterChips, formatNumber, humanLabel } from '../state.tsx';
import { h, Fragment, render as renderTree, raw } from '../jsx/jsx-runtime.ts';
import { api } from '../api.tsx';
import { graphClusters, GraphField, graphQuery, GraphResult, mountGraph } from '../components/graph.tsx';
import { setURL, bindFocusContext } from '../router.tsx';

const graphModes: Record<string, { label: string; description: string }> = {
  research_network: {
    label: "Research network",
    description: "Articles, observed authors, reference mentions, raw referenced-author strings, internal citations, co-authorships, and shared references.",
  },
  article_author: {
    label: "Articles and authors",
    description: "Authorship relationships between immutable article revisions and observed author occurrences.",
  },
  citation: {
    label: "Internal citations",
    description: "Directional citations that resolve to another selected article in this historical run.",
  },
  article_reference: {
    label: "Articles and references",
    description: "Article revisions connected to their captured reference mentions.",
  },
};

const filterLabels: Record<string, string> = {
  q: "Title or DOI",
  author: "Author",
  orcid: "ORCID",
  reference: "Reference",
  source: "Source",
  year_min: "Year from",
  year_max: "Year to",
  citation_min: "Citations from",
  citation_max: "Citations to",
  reference_min: "References from",
  reference_max: "References to",
  article_limit: "Article limit",
};

/** Renders markup for selecting a relationship graph mode. */
function ModeControl(props: { current: string }): JSX.Element {
  const modeItems = Object.entries(graphModes).map(([id, mode]) => {
    const checked = id === props.current;
    var active = "";
    if (id === props.current) active = " active";
    return (
      <label className={`item${active}`}>
        <input type="radio" name="graph_mode" value={id} checked={checked} />
        <span>{mode.label}</span>
      </label>
    );
  });
  return (
    <section className="rw-graph-model">
      <div className="rw-graph-model__heading">
        <h4>Graph model</h4>
        <p>Changing the model updates the network immediately and preserves the current filters.</p>
      </div>
      <div className="rw-segmented-control" role="group" aria-label="Graph model">
        {modeItems}
      </div>
    </section>
  );
}

/** Renders markup summarizing the active relationship filters. */
function appliedFilters(): string {
  const filters: Record<string, string> = {};
  Object.keys(filterLabels).forEach((key) => {
    if (value(key)) {
      filters[key] = value(key);
    }
  });
  const clear: Record<string, any> = {
    view: "relationships",
    mode: value("mode") || "research_network",
    node: "",
  };
  Object.keys(filterLabels).forEach((key) => {
    clear[key] = "";
  });
  return filterChips(filters, filterLabels, { clearUpdates: clear });
}

/** Renders markup summarizing connected graph clusters. */
function ClusterSummary(props: { data: any }): JSX.Element {
  const counts = props.data.counts || {};
  const clusters = graphClusters(list(props.data, ["nodes"]), list(props.data, ["edges"])).components;
  const largest = clusters.reduce((size, cluster) => {
    return Math.max(size, cluster.size);
  }, 0);
  const items = [
    ["Connected clusters", clusters.length],
    ["Largest cluster size", largest],
    ["Rendered entities", counts.nodes_rendered || 0],
    ["Rendered relationships", counts.edges_rendered || 0],
  ];
  const summaryItems = items.map(([label, count]) => {
    return (
      <div>
        <dt>{label}</dt>
        <dd>{formatNumber(count)}</dd>
      </div>
    );
  });
  return (
    <dl className="rw-graph-cluster-summary">
      {summaryItems}
    </dl>
  );
}

/** Asynchronously implements relationships view for the viewer. */
export async function relationshipsView(): Promise<void> {
  if (!value("run_id")) {
    const emptyStateMarkup = (
      <EmptyState title="Relationships" detail="Select a run attempt to explore its bounded authorship, citation, and reference-mention relationships." action={<button type="button" data-focus-context>Focus context selector</button>} />
    );
    renderTree(emptyStateMarkup, app);
    bindFocusContext();
    return;
  }

  const requestedMode = value("mode");
  var mode = "research_network";
  if (graphModes[requestedMode]) mode = requestedMode;
  const queryParams: Record<string, any> = graphQuery();
  queryParams.run_id = value("run_id");
  queryParams.mode = mode;
  if (!queryParams.article_limit) {
    queryParams.article_limit = 2000;
  }
  const data = await api("/api/graph", queryParams, {
    method: "GET",
    headers: { Accept: "application/json" },
  });
  const modeDefinition = graphModes[mode];
  const appliedFilterMarkup = raw(appliedFilters());
  const edgeCount = formatNumber(list(data, ["edges"]).length);

  const filterPanel = (
    <aside className="ui segment rw-relationship-filters">
      <div className="ui top attached header">
        <div>
          <h3>Explore a bounded network</h3>
          <p>Choose a graph model, then refine the matching article set independently.</p>
        </div>
      </div>
      <div className="content">
        <ModeControl current={mode} />
        <form id="graph-form" className="rw-graph__controls">
          <div className="rw-graph-filter-heading">
            <h4>Article filters</h4>
            <p>Filters apply to the selected run without changing the graph model.</p>
          </div>
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
              <label>
                Article limit (1\u20132,000)
                <input name="article_limit" type="number" min="1" max="2000" value={value("article_limit") || "2000"} />
                <small>If lowered, the earliest matching normalized revisions by revision ID are retained.</small>
              </label>
            </div>
          </details>
          {appliedFilterMarkup}
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
      <div className="ui top attached header">
        <div>
          <h3>{modeDefinition.label}</h3>
          <p>{modeDefinition.description}</p>
        </div>
        <span className="ui label">{edgeCount} relationships</span>
      </div>
      <div className="content">
        <ClusterSummary data={data} />
        <GraphResult data={data} />
      </div>
    </section>
  );

  const pageMarkup = (
    <Fragment>
      <PageHeader kicker="Bounded relationship exploration" title="Relationships" description="Investigate how articles, authors, and captured references connect inside the selected historical run." />
      <div className="rw-relationship-layout">{filterPanel}{networkPanel}</div>
    </Fragment>
  );
  renderTree(pageMarkup, app);

  const graphForm = document.querySelector("#graph-form")!;
  graphForm.addEventListener("submit", (event) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget as HTMLFormElement);
    const updates: Record<string, any> = {
      view: "relationships",
      node: "",
    };
    const filterableNames = graphFilters.filter((name) => {
      return name !== "mode";
    });
    filterableNames.forEach((name) => {
      const rawValue = String(form.get(name) || "");
      var updateValue = rawValue;
      if (name === "article_limit" && rawValue === "2000") updateValue = "";
      updates[name] = updateValue;
    });
    updates.mode = mode;
    setURL(updates, false);
  });

  const modeInputs = document.querySelectorAll<HTMLInputElement>("input[name=\"graph_mode\"]");
  modeInputs.forEach((input) => {
    input.addEventListener("change", () => {
      if (input.checked) {
        setURL({
          view: "relationships",
          mode: input.value,
          node: "",
        }, false);
      }
    });
  });

  const resetButton = document.querySelector("#graph-reset")!;
  resetButton.addEventListener("click", () => {
    const updates: Record<string, any> = {
      view: "relationships",
      mode: "research_network",
      node: "",
    };
    graphFilters.forEach((name) => {
      if (name !== "mode") {
        updates[name] = "";
      }
    });
    setURL(updates, false);
  });

  mountGraph(data);
}
