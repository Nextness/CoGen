// Evaluation: normalized articles and their manual PDF inventory state.
import {
  app, value, link, pageSizes, PageHeader, EmptyState, FilterChips,
  humanLabel, formatTime, StatusChip
} from '../state.tsx';
import { h, Fragment, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api, mutate } from '../api.tsx';
import { DataTable, bindTableControls } from '../components/data-table.tsx';
import type { DataTableContext } from '../components/data-table.tsx';
import { bindFocusContext } from '../router.tsx';

const evaluationSortFields = ["title", "doi"];

/** Renders a context-preserving article link for an evaluation row. */
function titleLink(row: any): JSX.Element {
  const updates: Record<string, any> = {
    view: "article",
    article_id: row.work_revision_id,
  };
  const recordHref = link(updates);
  return <a className="rw-table-title" href={recordHref} title={row.title || "Not recorded"}><span>{row.title || "Not recorded"}</span></a>;
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
  const runID = value("run_id");
  if (!runID) {
    const emptyAction = <button type="button" className="ui primary button" data-focus-context>Select a run attempt</button>;
    const emptyStateMarkup = (
      <EmptyState title="Evaluation" detail="Select a run attempt to evaluate its normalized article inventory." action={emptyAction} />
    );
    renderTree(emptyStateMarkup, app);
    bindFocusContext();
    return;
  }

  const page = Math.max(1, Number(value("page") || 1));
  const requestedPerPage = Number(value("per_page"));
  var perPage = 50;
  if (pageSizes.includes(requestedPerPage)) perPage = requestedPerPage;
  const requestedSort = value("sort");
  var sort = "";
  if (evaluationSortFields.includes(requestedSort)) sort = requestedSort;
  var order = "asc";
  if (value("order").toLowerCase() === "desc") order = "desc";
  const query = value("q");
  const data = await api(`/api/runs/${encodeURIComponent(runID)}/evaluation`, {
    page: page,
    per_page: perPage,
    sort: sort,
    order: order,
    q: query,
  }, {
    method: "GET",
    headers: { Accept: "application/json" },
  });

  const pageSizeOptions = pageSizes.map((size) => {
    return <option value={size} selected={size === perPage}>{size}</option>;
  });
  const controls = (
    <form className="ui form rw-table-controls" data-table-search>
      <label className="rw-table-controls__search">
        <span>Search normalized articles</span>
        <span className="ui input">
          <input id="corpus-query" type="search" value={query} placeholder="Title or DOI" />
          <button type="button" className="clear" data-clear-query disabled={!query} aria-label="Clear search">{"\u00D7"}</button>
        </span>
      </label>
      <label className="rw-table-controls__size">
        Rows per page
        <select id="per-page">{pageSizeOptions}</select>
      </label>
      <button type="button" data-search-query className="ui primary button">Search</button>
    </form>
  );

  const clearUpdates = {
    q: "",
    page: 1,
  };
  var filters: JSX.Element | null = null;
  if (query) {
    filters = <FilterChips filters={{ q: query }} labels={{ q: "Search" }} options={{ clearUpdates: clearUpdates }} />;
  }

  const columnConfig: Record<string, { label?: string; className?: string; render?: (row: any, value: any) => JSX.Element }> = {
    title: {
      label: "Title",
      className: "col-title",
      render: titleLink,
    },
    doi: {
      label: "DOI",
      className: "col-doi",
    },
    source: {
      label: "Source",
      className: "col-source",
    },
    inventory_status: {
      label: "Inventory Status",
      render: (row) => {
        return <StatusChip raw={humanLabel(row.inventory_status)} />;
      },
    },
    inventoried_at: {
      label: "Inventoried at",
      className: "col-captured-at",
      render: inventoriedTime,
    },
    review_status: {
      label: "Review status",
      render: (row) => {
        return <StatusChip raw={humanLabel(row.review_status || "not_evaluated")} />;
      },
    },
    review_inherited: {
      label: "Review source",
      render: (row) => {
        if (row.review_inherited) {
          return <span className="ui label">Inherited</span>;
        }
        return <span className="ui faded text">This context</span>;
      },
    },
  };
  const context: DataTableContext = {
    page: page,
    perPage: perPage,
    query: query,
    sortFields: evaluationSortFields,
    rowKey: "work_revision_id",
    itemLabel: "normalized articles",
    tableClass: "rw-evaluation-table",
    columnConfig: columnConfig,
  };
  const table = <DataTable tableName="evaluation" result={data} context={context} />;

  const reviewInitialized = (data.rows || []).some((row: any) => {
    return row.review_context_initialized;
  });
  var startReview: JSX.Element | null = null;
  if (!reviewInitialized) {
    startReview = <button type="button" className="ui primary button" data-start-evaluation-review>Start review</button>;
  }

  const pageMarkup = (
    <Fragment>
      <PageHeader kicker="Reading inventory" title="Evaluation" description="Review every normalized article in the selected run and whether its PDF has been manually inventoried." />
      <section className="ui segment rw-data-section">
        <div className="ui top attached header">
          <div>
            <h3>Normalized article inventory</h3>
            <p>Available PDFs were validated and added through the manual PDF-store tool.</p>
          </div>
          {startReview}
        </div>
        <div className="content">
          {controls}
          {filters}
          {table}
        </div>
      </section>
    </Fragment>
  );
  renderTree(pageMarkup, app);

  bindTableControls("evaluation", page);
  const startReviewButton = document.querySelector("[data-start-evaluation-review]");
  startReviewButton?.addEventListener("click", async () => {
    const reviewContext = await api(`/api/runs/${encodeURIComponent(runID)}/review-context`, {}, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    const proposed = reviewContext.proposed_parent;
    var prompt = "Start an empty review context for this run?";
    if (proposed) prompt = `Start review by inheriting ${proposed.inherited_work_count} matching work heads from run ${proposed.pipeline_run_id}?`;
    if (!window.confirm(prompt)) return;
    await mutate(`/api/runs/${encodeURIComponent(runID)}/review-context`, "POST", { parent_context_id: proposed?.context_id || null });
    await evaluationView();
  });
}
