// Evaluation: a filtered, progress-aware queue of normalized articles.
import {
  app,
  value,
  link,
  pageSizes,
  PageHeader,
  EmptyState,
  FilterChips,
  humanLabel,
  formatNumber,
  formatDate,
  StatusChip,
  currentDetailOrigin,
} from "../state.tsx";
import { h, Fragment, render as renderTree } from "../jsx/jsx-runtime.ts";
import { api } from "../api.tsx";
import { DataTable, bindTableControls } from "../components/data-table.tsx";
import type { DataTableContext } from "../components/data-table.tsx";
import {
  bindReviewContextInitializer,
  ReviewContextDialog,
} from "../components/review-context-dialog.tsx";
import { bindFocusContext, setURL } from "../router.tsx";
import { mountRunNotesIndex } from "../components/run-notes-index.tsx";

const evaluationSortFields = ["title", "doi"];
const evaluationFilterKeys = ["q", "pdf_status", "review_status", "review_source", "qualifier", "source", "reviewed"];

/** Renders a queue-preserving article link for an evaluation row. */
function titleLink(row: any): JSX.Element {
  const recordHref = link({
    view: "article",
    article_id: row.work_revision_id,
    origin: currentDetailOrigin(),
  });
  return (
    <a className="rw-table-title" href={recordHref} title={row.title || "Not recorded"}>
      <span>{row.title || "Not recorded"}</span>
    </a>
  );
}

/** Renders the recorded PDF inventory date or an unavailable label. */
function inventoriedDate(row: any): JSX.Element {
  if (!row.inventoried_at) {
    return <span className="ui faded text">{"\u2014"}</span>;
  }
  return <time datetime={row.inventoried_at}>{formatDate(row.inventoried_at)}</time>;
}

/** Renders explicit review-lineage state from the invariant server response. */
function reviewSource(row: any, initialized: boolean): JSX.Element {
  if (!initialized || !row.review_version_id) {
    return <span className="ui orange label">Not started</span>;
  }
  if (row.review_inherited) {
    return <span className="ui violet label">Inherited</span>;
  }
  return <span className="ui neutral label">This context</span>;
}

/** Renders one select option from an aggregate facet value. */
function facetOptions(items: any[], selected: string): JSX.Element[] {
  return (items || []).map((item) => {
    const label = `${humanLabel(item.value)} (${formatNumber(item.count)})`;
    return <option value={item.value} selected={String(item.value) === selected}>{label}</option>;
  });
}

/** Asynchronously implements the Evaluation review queue. */
export async function evaluationView(): Promise<void> {
  const runID = value("run_id");
  if (!runID) {
    const emptyAction = <button type="button" className="ui primary button" data-focus-context>Select a run attempt</button>;
    const emptyStateMarkup = (
      <EmptyState
        title="Evaluation"
        detail="Select a run attempt to evaluate its normalized article inventory."
        action={emptyAction}
      />
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

  const filters: Record<string, string> = {};
  evaluationFilterKeys.forEach((key) => {
    filters[key] = value(key);
  });
  const data = await api(`/api/runs/${encodeURIComponent(runID)}/evaluation`, {
    page: page,
    per_page: perPage,
    sort: sort,
    order: order,
    ...filters,
  }, {
    method: "GET",
    headers: { Accept: "application/json" },
  });

  const summary = data.review_summary || {};
  const facets = summary.facets || {};
  const pageSizeOptions = pageSizes.map((size) => {
    return <option value={size} selected={size === perPage}>{size}</option>;
  });
  const pdfOptions = facetOptions(facets.pdf_status, filters.pdf_status);
  const reviewStatusOptions = facetOptions(facets.review_status, filters.review_status);
  const reviewSourceOptions = facetOptions(facets.review_source, filters.review_source);
  const qualifierOptions = facetOptions(facets.qualifier, filters.qualifier);
  const sourceOptions = facetOptions(facets.source, filters.source);
  const activeFilters: Record<string, string> = {};
  evaluationFilterKeys.forEach((key) => {
    if (filters[key]) activeFilters[key] = filters[key];
  });
  const filterLabels = {
    q: "Search",
    pdf_status: "PDF",
    review_status: "Review status",
    review_source: "Review source",
    qualifier: "Qualifier",
    source: "Source",
    reviewed: "Progress",
  };
  const clearUpdates = { q: "", pdf_status: "", review_status: "", review_source: "", qualifier: "", source: "", reviewed: "", page: 1 };
  const filterOptions = { clearUpdates: clearUpdates };
  var filterSummary: JSX.Element | null = null;
  if (Object.keys(activeFilters).length) {
    filterSummary = <FilterChips filters={activeFilters} labels={filterLabels} options={filterOptions} />;
  }
  const advancedFilterCount = evaluationFilterKeys.filter((key) => {
    return key !== "q" && Boolean(filters[key]);
  }).length;
  const advancedOpen = advancedFilterCount > 0;
  var advancedSummary = "Any PDF, review, source, or progress state";
  if (advancedFilterCount) advancedSummary = `${advancedFilterCount} additional filters applied`;
  const clearFilterHref = link(clearUpdates);
  const controls = (
    <form className="ui form rw-filter-panel rw-evaluation-filters" data-evaluation-filters>
      <div className="rw-evaluation-filters__primary">
        <label className="rw-evaluation-filters__search">
          <span>Search normalized articles</span>
          <span className="ui input">
            <input id="evaluation-query" name="q" type="search" value={filters.q} placeholder="Title or DOI" />
          </span>
        </label>
        <label className="rw-evaluation-filters__size">
          <span>Rows per page</span>
          <select id="evaluation-per-page">{pageSizeOptions}</select>
        </label>
        <div className="rw-filter-panel__actions">
          <button type="submit" className="ui primary button">Apply filters</button>
          <a className="ui basic button" href={clearFilterHref}>Clear filters</a>
        </div>
      </div>
      <details className="rw-filter-disclosure rw-evaluation-filters__advanced" open={advancedOpen}>
        <summary>
          <span>PDF, review, source, and progress filters</span>
          <small>{advancedSummary}</small>
        </summary>
        <div className="rw-filter-panel__fields rw-evaluation-filters__advanced-fields">
          <label>
            <span>PDF availability</span>
            <select name="pdf_status">
              <option value="">All PDF states</option>
              {pdfOptions}
            </select>
          </label>
          <label>
            <span>Review status</span>
            <select name="review_status">
              <option value="">All review statuses</option>
              {reviewStatusOptions}
            </select>
          </label>
          <label>
            <span>Review source</span>
            <select name="review_source">
              <option value="">All review sources</option>
              {reviewSourceOptions}
            </select>
          </label>
          <label>
            <span>Qualifier</span>
            <select name="qualifier">
              <option value="">All qualifiers</option>
              {qualifierOptions}
            </select>
          </label>
          <label>
            <span>Source</span>
            <select name="source">
              <option value="">All article sources</option>
              {sourceOptions}
            </select>
          </label>
          <label>
            <span>Progress</span>
            <select name="reviewed">
              <option value="" selected={!filters.reviewed}>All progress states</option>
              <option value="unreviewed" selected={filters.reviewed === "unreviewed"}>Unreviewed</option>
              <option value="reviewed" selected={filters.reviewed === "reviewed"}>Reviewed</option>
            </select>
          </label>
        </div>
      </details>
      {filterSummary}
    </form>
  );

  const columnConfig: DataTableContext["columnConfig"] = {
    title: {
      label: "Title",
      className: "col-title",
      render: titleLink,
    },
    doi: {
      label: "DOI",
      className: "col-doi",
    },
    inventory_status: {
      label: "PDF",
      className: "col-inventory-status",
      render: (row) => {
        return <StatusChip raw={humanLabel(row.inventory_status)} />;
      },
    },
    inventoried_at: {
      label: "Inventoried at",
      className: "col-inventoried-date",
      render: inventoriedDate,
    },
    review_status: {
      label: "Review status",
      className: "col-review-status",
      render: (row) => {
        return <StatusChip raw={humanLabel(row.review_status || "not_evaluated")} />;
      },
    },
    review_inherited: {
      label: "Review source",
      className: "col-review-source",
      render: (row) => {
        return reviewSource(row, data.review_context_initialized === true);
      },
    },
  };
  const tableContext: DataTableContext = {
    page: page,
    perPage: perPage,
    query: "",
    sortFields: evaluationSortFields,
    rowKey: "work_revision_id",
    itemLabel: "normalized articles",
    tableClass: "rw-evaluation-table",
    columnConfig: columnConfig,
    columnsWhitelist: ["title", "doi", "inventory_status", "inventoried_at", "review_status", "review_inherited"],
    perPageSelector: "#evaluation-per-page",
    querySelector: "#evaluation-unused-query",
    searchButtonSelector: "[data-evaluation-unused-search]",
    clearButtonSelector: "[data-evaluation-unused-clear]",
  };
  const table = <DataTable tableName="evaluation" result={data} context={tableContext} />;

  var contextAction: JSX.Element | null = null;
  if (!data.review_context_initialized && data.run_writable) {
    contextAction = <button type="button" className="ui primary button" data-start-review>Start review</button>;
  }
  var contextStatus: JSX.Element = <span className="ui orange label">Not started</span>;
  if (data.review_context_initialized) contextStatus = <span className="ui violet label">Review initialized</span>;
  const progressWidth = Math.max(0, Math.min(100, Number(summary.percent_reviewed) || 0));
  const navigation = data.queue_navigation || {};
  var previousUnreviewed: JSX.Element | null = null;
  var nextUnreviewed: JSX.Element | null = null;
  if (navigation.previous_work_revision_id) {
    previousUnreviewed = (
      <a className="ui basic button" href={link({ view: "article", article_id: navigation.previous_work_revision_id, origin: currentDetailOrigin() })}>
        Previous unreviewed
      </a>
    );
  }
  if (navigation.next_work_revision_id) {
    nextUnreviewed = (
      <a className="ui primary button" href={link({ view: "article", article_id: navigation.next_work_revision_id, origin: currentDetailOrigin() })}>
        Next unreviewed
      </a>
    );
  }
  const progressMarkup = (
    <section className="rw-review-progress" aria-labelledby="evaluation-progress-title">
      <div>
        <h3 id="evaluation-progress-title">Review progress</h3>
        {contextStatus}
      </div>
      <div className="rw-review-progress__counts">
        <span><strong>{formatNumber(summary.reviewed)}</strong> reviewed</span>
        <span><strong>{formatNumber(summary.unreviewed)}</strong> unreviewed</span>
        <span><strong>{formatNumber(summary.pdf_available)}</strong> PDFs available</span>
      </div>
      <div className="rw-review-progress__track" role="progressbar" aria-label="Articles reviewed" aria-valuemin="0" aria-valuemax="100" aria-valuenow={progressWidth}>
        <span style={`width:${progressWidth}%`}></span>
      </div>
      <div className="rw-review-progress__actions">
        {previousUnreviewed}
        {nextUnreviewed}
      </div>
    </section>
  );

  const pageMarkup = (
    <Fragment>
      <PageHeader
        kicker="Review queue"
        title="Evaluation"
        description="Review normalized articles with explicit PDF, decision, lineage, and progress filters."
      />
      {progressMarkup}
      <section className="ui segment rw-data-section">
        <div className="ui top attached header">
          <div>
            <h3>Normalized article queue</h3>
            <p>The queue and its return links preserve the selected filters, sorting, and page.</p>
          </div>
          {contextAction}
        </div>
        <div className="content">
          <div className="rw-content-stack rw-evaluation-queue" data-table-scope="evaluation">
            {controls}
            {table}
          </div>
        </div>
      </section>
      <details className="ui segment rw-run-notes-index">
        <summary>Browse all Notes in this review context</summary>
        <div className="content">
          <button type="button" className="ui basic button" data-run-notes-open disabled={!data.review_context_initialized}>Load run Notes index</button>
          <div data-run-notes-host></div>
        </div>
      </details>
      <ReviewContextDialog proposed={data.proposed_parent || null} />
    </Fragment>
  );
  renderTree(pageMarkup, app);

  bindTableControls("evaluation", page, tableContext);
  app.querySelector<HTMLFormElement>("[data-evaluation-filters]")?.addEventListener("submit", (event) => {
    event.preventDefault();
    const formElement = event.currentTarget as HTMLFormElement;
    const form = new FormData(formElement);
    const updates: Record<string, any> = { page: 1 };
    evaluationFilterKeys.forEach((key) => {
      updates[key] = String(form.get(key) || "");
    });
    setURL(updates, false);
  });
  bindReviewContextInitializer(app, {
    runID: Number(runID),
    proposed: data.proposed_parent || null,
    onInitialized: evaluationView,
  });
  app.querySelector<HTMLButtonElement>("[data-run-notes-open]")?.addEventListener("click", async (event) => {
    const button = event.currentTarget as HTMLButtonElement;
    const host = app.querySelector<HTMLElement>("[data-run-notes-host]")!;
    button.disabled = true;
    await mountRunNotesIndex(host, Number(runID));
    button.remove();
  });
}
