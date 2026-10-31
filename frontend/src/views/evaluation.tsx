// Corpus articles: metadata, PDF inventory, and review in one collection.
import {
  app,
  value,
  linkTargetFor,
  detailLinkFor,
  pageSizes,
  EmptyState,
  FilterChips,
  humanLabel,
  formatNumber,
  formatDate,
  StatusChip,
} from "../state.tsx";
import { h, Fragment, render as renderTree, cx, classAdd, classRemove } from "../jsx/jsx-runtime.ts";
import { APIError, api, downloadExport, errorMessage } from "../api.tsx";
import type { EvaluationFacet, EvaluationResponse, EvaluationRow, WireRecord } from "../api/types.ts";
import { articlesColumns, articlesExpandFields } from "../components/article-table.tsx";
import { DataTable, bindTableControls } from "../components/data-table.tsx";
import type { DataTableContext } from "../components/data-table.tsx";
import {
  bindReviewContextInitializer,
  ReviewContextDialog,
} from "../components/review-context-dialog.tsx";
import { bindFocusContext, setURL } from "../router.tsx";
import { mountRunNotesIndex } from "../components/run-notes-index.tsx";

/** Typed compound class names used by this module. */
const classNames = {
  rwFilterDisclosureRwEvaluationFiltersAdvanced: cx("rw-filter-disclosure", "rw-evaluation-filters__advanced"),
  rwFilterPanelFieldsRwEvaluationFiltersAdvancedFields: cx("rw-filter-panel__fields", "rw-evaluation-filters__advanced-fields"),
  uiBasicButton: cx("ui", "basic", "button"),
  uiErrorMessage: cx("ui", "error", "message"),
  uiFadedText: cx("ui", "faded", "text"),
  uiFormRwFilterPanel: cx("ui", "form", "rw-filter-panel"),
  uiInput: cx("ui", "input"),
  uiNeutralLabel: cx("ui", "neutral", "label"),
  uiOrangeLabel: cx("ui", "orange", "label"),
  uiPrimaryButton: cx("ui", "primary", "button"),
  uiSegment: cx("ui", "segment"),
  uiSuccessMessage: cx("ui", "success", "message"),
  uiTopAttachedHeader: cx("ui", "top", "attached", "header"),
  uiVioletLabel: cx("ui", "violet", "label"),
  uiWarningMessage: cx("ui", "warning", "message"),
};

const evaluationSortFields = ["id", "title", "doi", "year", "journal", "publisher", "source", "validation_status", "citation_count", "reference_count", "created_at"];
const evaluationFilterKeys = ["q", "pdf_status", "review_status", "review_source", "qualifier", "source", "reviewed"];

/** Renders a queue-preserving article link for an evaluation row. */
function titleLink(row: WireRecord): JSX.Element {
  const record = row as EvaluationRow;
  const target = detailLinkFor("article", record.work_revision_id);
  return (
    <a className="rw-table-title" href={target.href} data-state={JSON.stringify(target.state)} title={record.title || "Not recorded"}>
      <span>{record.title || "Not recorded"}</span>
    </a>
  );
}

/** Renders the recorded PDF inventory date or an unavailable label. */
function inventoriedDate(row: WireRecord): JSX.Element {
  const record = row as EvaluationRow;
  if (!record.inventoried_at) {
    return <span className={classNames.uiFadedText}>{"\u2014"}</span>;
  }
  return <time dateTime={record.inventoried_at}>{formatDate(record.inventoried_at)}</time>;
}

/** Renders explicit review-lineage state from the invariant server response. */
function reviewSource(row: WireRecord, initialized: boolean): JSX.Element {
  const record = row as EvaluationRow;
  if (!initialized || !record.review_version_id) {
    return <span className={classNames.uiOrangeLabel}>Not started</span>;
  }
  if (record.review_inherited) {
    return <span className={classNames.uiVioletLabel}>Inherited</span>;
  }
  return <span className={classNames.uiNeutralLabel}>This context</span>;
}

/** Renders one select option from an aggregate facet value. */
function facetOptions(items: EvaluationFacet[], selected: string): JSX.Element[] {
  return (items || []).map((item) => {
    const label = `${humanLabel(item.value)} (${formatNumber(item.count)})`;
    return <option value={item.value} selected={String(item.value) === selected}>{label}</option>;
  });
}

/** Reports whether one current-page article has readable exportable bytes. */
function exportableRow(row: WireRecord): boolean {
  const record = row as EvaluationRow;
  return record.inventory_status === "available" && typeof record.content_hash === "string" && record.content_hash.length === 64;
}

/** Returns one stable selection key for an evaluation row. */
function exportRowKey(row: WireRecord): string {
  return String((row as EvaluationRow).work_revision_id || "");
}

/** Binds current-page PDF selection, count feedback, and selected-set download. */
function bindExportSelection(runID: number, rows: WireRecord[], selected: Set<string>): void {
  let includeComments = false;
  const scope = app.querySelector<HTMLElement>('[data-table-scope="work_revisions"]');
  const button = app.querySelector<HTMLButtonElement>("[data-export-selected]");
  const count = app.querySelector<HTMLElement>("[data-export-count]");
  const unavailable = app.querySelector<HTMLElement>("[data-export-unavailable]");
  const feedback = app.querySelector<HTMLElement>("[data-export-feedback]");
  const selectAll = app.querySelector<HTMLInputElement>("[data-select-all]");
  if (!scope || !button || !count || !feedback) return;

  const byKey = new Map<string, WireRecord>();
  rows.forEach((row) => {
    byKey.set(exportRowKey(row), row);
  });
  const selectableKeys = rows.filter(exportableRow).map(exportRowKey);
  const unavailableCount = rows.length - selectableKeys.length;
  if (unavailable) {
    var unavailableText = "All PDFs on this page can be selected.";
    if (unavailableCount > 0) unavailableText = `${formatNumber(unavailableCount)} PDF${unavailableCount === 1 ? " is" : "s are"} unavailable or lacks readable bytes on this page.`;
    unavailable.textContent = unavailableText;
  }

  /** Synchronizes counts, selection controls, and feedback from the current set. */
  function update(): void {
    count!.textContent = `${formatNumber(selected.size)} selected`;
    button!.disabled = selected.size === 0;
    scope!.querySelectorAll<HTMLInputElement>("[data-select-row]").forEach((input) => {
      input.checked = selected.has(input.dataset.selectRow || "");
    });
    if (selectAll) {
      const allSelected = selectableKeys.length > 0 && selectableKeys.every((key) => selected.has(key));
      const someSelected = selectableKeys.some((key) => selected.has(key));
      selectAll.checked = allSelected;
      selectAll.indeterminate = someSelected && !allSelected;
    }
  }

  /** Replaces one feedback element's semantic classes with the supplied result kind. */
  function applyFeedbackClass(node: HTMLElement, kind: "info" | "warning" | "error" | "success"): void {
    classRemove(node, "ui");
    classRemove(node, "faded");
    classRemove(node, "text");
    classRemove(node, "info");
    classRemove(node, "success");
    classRemove(node, "warning");
    classRemove(node, "error");
    classRemove(node, "message");
    classAdd(node, ["ui", kind, "message"]);
  }

  /** Sets one bounded selection status message. */
  function setFeedback(message: string, kind: "info" | "warning" | "error" | "success" = "info"): void {
    feedback!.textContent = message;
    feedback!.setAttribute("role", kind === "error" ? "alert" : "status");
    applyFeedbackClass(feedback!, kind);
  }

  scope.addEventListener("change", (event) => {
    const input = event.target as HTMLInputElement;
    const key = input.dataset.selectRow || "";
    if (input.matches("[data-select-all]")) {
      if (input.checked) {
        selected.clear();
        selectableKeys.slice(0, 20).forEach((item) => selected.add(item));
        if (selectableKeys.length > 20) setFeedback("Selected the first 20 exportable PDFs on this page. The limit is 20 per download.", "warning");
        else setFeedback("");
      } else {
        selected.clear();
        setFeedback("");
      }
      update();
      return;
    }
    if (!input.matches("[data-select-row]") || !key) return;
    if (input.checked) {
      if (selected.size >= 20) {
        input.checked = false;
        setFeedback("A download can include at most 20 PDFs. Clear a selection before adding another.", "warning");
        return;
      }
      selected.add(key);
    } else {
      selected.delete(key);
    }
    setFeedback("");
    update();
  });

  const comments = app.querySelector<HTMLInputElement>("[data-export-include-comments]");
  comments?.addEventListener("change", () => {
    includeComments = comments.checked;
  });

  button.addEventListener("click", async () => {
    const articles = Array.from(selected).map((key) => {
      const row = byKey.get(key) as EvaluationRow;
      return { work_revision_id: row.work_revision_id, content_hash: row.content_hash || "" };
    }).filter((item) => item.content_hash);
    if (!articles.length) {
      setFeedback("Select at least one PDF with readable bytes.", "warning");
      return;
    }
    button.disabled = true;
    setFeedback("Preparing the selected PDF export.", "info");
    try {
      const result = await downloadExport(`/api/runs/${encodeURIComponent(runID)}/pdf-exports`, {
        articles: articles,
        include_comments: includeComments,
      }, {
        contentType: "application/zip",
        maxBytes: 101_000_000,
        signal: null,
      });
      var message = `Download prepared: ${result.filename}.`;
      if (result.included !== null && result.skipped !== null) {
        message = `Download prepared: ${result.filename}. ${formatNumber(result.included)} included, ${formatNumber(result.skipped)} skipped.`;
      }
      if (includeComments) {
        const count = result.comments || 0;
        if (count > 0) message += ` ${formatNumber(count)} saved comment${count === 1 ? "" : "s"} included in ${formatNumber(result.annotated || 0)} file${result.annotated === 1 ? "" : "s"}.`;
        else message += " No matching saved comments were found, so original PDFs were downloaded.";
      }
      setFeedback(message, "success");
    } catch (error) {
      if (error instanceof APIError && error.code === "pdf_export_empty") {
        setFeedback("No selected PDF could be exported. Refresh the page to review current PDF availability.", "warning");
      } else {
        setFeedback(errorMessage(error, "The selected PDF export could not be prepared."), "error");
      }
    } finally {
      button.disabled = selected.size === 0;
    }
  });

  update();
}

/** Renders the Corpus article collection with metadata, PDF inventory, and review controls. */
export async function articleCollectionView(heading: JSX.Element, refresh: () => Promise<void>): Promise<void> {
  const runID = value("run_id");
  if (!runID) {
    const emptyAction = <button type="button" className={classNames.uiPrimaryButton} data-focus-context>Select a run attempt</button>;
    const emptyStateMarkup = (
      <EmptyState
        title="Corpus"
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
  const data = await api<EvaluationResponse>(`/api/runs/${encodeURIComponent(runID)}/corpus/articles`, {
    page: page,
    per_page: perPage,
    sort: sort,
    order: order,
    ...filters,
  });

  const selectedRevisions = new Set<string>();
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
  const clearFilterTarget = linkTargetFor(clearUpdates);
  const controls = (
    <form className={classNames.uiFormRwFilterPanel} data-evaluation-filters>
      <div className="rw-evaluation-filters__primary">
        <label className="rw-evaluation-filters__search">
          <span>Search normalized articles</span>
          <span className={classNames.uiInput}>
            <input id="evaluation-query" name="q" type="search" value={filters.q} placeholder="Title, DOI, journal, publisher, or source" />
          </span>
        </label>
        <label className="rw-evaluation-filters__size">
          <span>Rows per page</span>
          <select id="evaluation-per-page">{pageSizeOptions}</select>
        </label>
        <div className="rw-filter-panel__actions">
          <button type="submit" className={classNames.uiPrimaryButton}>Apply filters</button>
          <a className={classNames.uiBasicButton} href={clearFilterTarget.href} data-state={JSON.stringify(clearFilterTarget.state)}>Clear filters</a>
        </div>
      </div>
      <details className={classNames.rwFilterDisclosureRwEvaluationFiltersAdvanced} open={advancedOpen}>
        <summary>
          <span>PDF, review, source, and progress filters</span>
          <small>{advancedSummary}</small>
        </summary>
        <div className={classNames.rwFilterPanelFieldsRwEvaluationFiltersAdvancedFields}>
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
    year: {
      label: "Year",
      className: "col-year",
    },
    journal: {
      label: "Journal",
      className: "col-journal",
    },
    source: {
      label: "Source",
      className: "col-source",
    },
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
    tableClasses: ["rw-corpus-table", "rw-evaluation-table"],
    selection: {
      selected: selectedRevisions,
      label: "Select PDF",
      ariaLabel: "Select all exportable PDFs on this page",
      isSelectable: exportableRow,
      rowLabel: (row) => {
        const record = row as EvaluationRow;
        return String(record.title || record.doi || record.work_revision_id);
      },
    },
    columnConfig: columnConfig,
    columnsWhitelist: [...articlesColumns, "inventory_status", "review_status", "review_inherited"],
    expandableFields: [
      ...articlesExpandFields,
      {
        f: "inventoried_at",
        w: 5,
        label: "Inventoried at",
        render: inventoriedDate,
      },
    ],
    perPageSelector: "#evaluation-per-page",
  };
  const table = <DataTable tableName="work_revisions" result={data} context={tableContext} />;

  const exportToolbar = (
    <section className="rw-export-toolbar" aria-label="PDF export selection">
      <div className="rw-export-toolbar__summary">
        <strong data-export-count>0 selected</strong>
        <span>Limit 20 per download</span>
        <small data-export-unavailable>PDFs unavailable on this page cannot be selected.</small>
      </div>
      <label className="rw-export-toolbar__comments">
        <input type="checkbox" data-export-include-comments />
        <span>Include comments</span>
      </label>
      <small className="rw-export-toolbar__comments-help">Matching saved highlights and comments are embedded in separate copies. Off downloads original PDFs.</small>
      <button type="button" className={classNames.uiPrimaryButton} data-export-selected disabled>Download selected PDFs</button>
      <p className={classNames.uiFadedText} data-export-feedback role="status" aria-live="polite"></p>
    </section>
  );

  var contextAction: JSX.Element | null = null;
  if (!data.review_context_initialized && data.run_writable) {
    contextAction = <button type="button" className={classNames.uiPrimaryButton} data-start-review>Start review</button>;
  }
  var contextStatus: JSX.Element = <span className={classNames.uiOrangeLabel}>Not started</span>;
  if (!data.run_writable && !data.review_context_initialized) contextStatus = <span className={classNames.uiNeutralLabel}>Review unavailable for this run</span>;
  if (data.review_context_initialized) contextStatus = <span className={classNames.uiVioletLabel}>Review initialized</span>;
  const progressWidth = Math.max(0, Math.min(100, Number(summary.percent_reviewed) || 0));
  const navigation = data.queue_navigation || {};
  var previousUnreviewed: JSX.Element | null = null;
  var nextUnreviewed: JSX.Element | null = null;
  if (navigation.previous_work_revision_id) {
    const target = detailLinkFor("article", navigation.previous_work_revision_id);
    previousUnreviewed = (
      <a className={classNames.uiBasicButton} href={target.href} data-state={JSON.stringify(target.state)}>
        Previous unreviewed
      </a>
    );
  }
  if (navigation.next_work_revision_id) {
    const target = detailLinkFor("article", navigation.next_work_revision_id);
    nextUnreviewed = (
      <a className={classNames.uiPrimaryButton} href={target.href} data-state={JSON.stringify(target.state)}>
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
      {heading}
      {progressMarkup}
      <section className={classNames.uiSegment}>
        <div className={classNames.uiTopAttachedHeader}>
          <div>
            <h3>Articles</h3>
            <p>Valid normalized articles with PDF and review status. Discarded works remain in stage outcomes and provenance.</p>
          </div>
          {contextAction}
        </div>
        <div className="content">
          <div className="rw-content-stack" data-table-scope="work_revisions">
            {controls}
            {exportToolbar}
            {table}
          </div>
        </div>
      </section>
      <details className={classNames.uiSegment}>
        <summary>Browse all Notes in this review context</summary>
        <div className="content">
          <button type="button" className={classNames.uiBasicButton} data-run-notes-open disabled={!data.review_context_initialized}>Load run Notes index</button>
          <div data-run-notes-host></div>
        </div>
      </details>
      <ReviewContextDialog proposed={data.proposed_parent || null} />
    </Fragment>
  );
  renderTree(pageMarkup, app);

  bindTableControls("work_revisions", tableContext);
  bindExportSelection(Number(runID), data.rows as WireRecord[], selectedRevisions);
  app.querySelector<HTMLFormElement>("[data-evaluation-filters]")?.addEventListener("submit", (event) => {
    event.preventDefault();
    const formElement = event.currentTarget as HTMLFormElement;
    const form = new FormData(formElement);
    const updates: Record<string, unknown> = { page: 1 };
    evaluationFilterKeys.forEach((key) => {
      updates[key] = String(form.get(key) || "");
    });
    setURL(updates, false);
  });
  bindReviewContextInitializer(app, {
    runID: Number(runID),
    proposed: data.proposed_parent || null,
    onInitialized: refresh,
  });
  app.querySelector<HTMLButtonElement>("[data-run-notes-open]")?.addEventListener("click", async (event) => {
    const button = event.currentTarget as HTMLButtonElement;
    const host = app.querySelector<HTMLElement>("[data-run-notes-host]")!;
    button.disabled = true;
    await mountRunNotesIndex(host, Number(runID));
    button.remove();
  });
}
