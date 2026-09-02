// Home: bounded workspace history, aggregate context metrics, and reversible run lifecycle controls.
import {
  app,
  value,
  formatNumber,
  formatTime,
  formatDuration,
  StatusChip,
  link,
  stateFor,
  params,
  PageHeader,
  Panel,
  FilterChips,
} from "../state.tsx";
import { h, Fragment, render as renderTree, cx, classToggle, classAdd, classRemove } from "../jsx/jsx-runtime.ts";
import { api, mutate, errorMessage } from "../api.tsx";
import type {
  Identifier,
  HierarchyPage,
  HierarchyRevision,
  HierarchyRun,
  HierarchySearch,
  HierarchySummaryResponse,
  RunVisibilityResponse,
} from "../api/types.ts";
import { setURL } from "../router.tsx";

/** Typed compound class names used by this module. */
const classNames = {
  fieldRwHomeFiltersSearch: cx("field", "rw-home-filters__search"),
  rwFilterDisclosureRwHomeFiltersAdvanced: cx("rw-filter-disclosure", "rw-home-filters__advanced"),
  rwFilterPanelFieldsRwHomeFiltersAdvancedFields: cx("rw-filter-panel__fields", "rw-home-filters__advanced-fields"),
  rwKpiGridRwHomeKpis: cx("rw-kpi-grid", "rw-home-kpis"),
  uiBasicButton: cx("ui", "basic", "button"),
  uiDangerBasicButton: cx("ui", "danger", "basic", "button"),
  uiDangerButton: cx("ui", "danger", "button"),
  uiErrorMessage: cx("ui", "error", "message"),
  uiFadedText: cx("ui", "faded", "text"),
  uiFormRwFilterPanel: cx("ui", "form", "rw-filter-panel"),
  uiFormRwReviewDialogForm: cx("ui", "form", "rw-review-dialog__form"),
  uiIconBasicButtonRwReviewDialogClose: cx("ui", "icon", "basic", "button", "rw-review-dialog__close"),
  uiInfoMessage: cx("ui", "info", "message"),
  uiPrimaryBasicButton: cx("ui", "primary", "basic", "button"),
  uiPrimaryButton: cx("ui", "primary", "button"),
  uiTableRwHomeRuns: cx("ui", "table", "rw-home-runs"),
};

/** Returns a clean Deepdive link target for one complete research context. */
function deepdiveLink(searchID: Identifier, revisionID: Identifier, planID: Identifier, runID: Identifier): { href: string; state: Record<string, string> } {
  const updates: Record<string, unknown> = {};
  for (const key of params().keys()) updates[key] = "";
  const destination = {
    ...updates,
    view: "overview",
    search_id: searchID,
    search_revision_id: revisionID,
    plan_id: planID,
    run_id: runID,
  };
  return { href: link(destination), state: stateFor(destination) };
}

/** Returns whether one hierarchy item contains a complete planned-run context. */
function hasContext(item: HierarchyRun): boolean {
  return Boolean(item.search_id && item.search_revision_id && item.execution_plan_id && item.id);
}

/** Renders one direct action for a search's latest complete run. */
function ContinueAction(props: { searchID: Identifier | null; revisionID: Identifier | null; planID: Identifier | null; runID: Identifier | null }): JSX.Element | null {
  if (!props.searchID || !props.revisionID || !props.planID || !props.runID) return null;
  const target = deepdiveLink(props.searchID, props.revisionID, props.planID, props.runID);
  return (
    <a
      className={classNames.uiPrimaryBasicButton}
      href={target.href}
      data-state={JSON.stringify(target.state)}
    >
      Continue
    </a>
  );
}

/** Renders one bounded search-history summary with lazy revision discovery. */
function SearchCard(props: { search: HierarchySearch }): JSX.Element {
  const search = props.search;
  const continueAction = (
    <ContinueAction
      searchID={search.id}
      revisionID={search.latest_revision_id}
      planID={search.latest_plan_id}
      runID={search.latest_run_id}
    />
  );
  return (
    <article className="rw-home-search-card">
      <header>
        <div>
          <p className="rw-eyebrow">Search term</p>
          <h3>{search.search_id}</h3>
          <p>Created {formatTime(search.created_at)}</p>
        </div>
        {continueAction}
      </header>
      <dl className="rw-home-search-card__metrics">
        <div>
          <dt>Search revisions</dt>
          <dd>{formatNumber(search.revision_count)}</dd>
        </div>
        <div>
          <dt>Execution plans</dt>
          <dd>{formatNumber(search.plan_count)}</dd>
        </div>
        <div>
          <dt>Run attempts</dt>
          <dd>{formatNumber(search.run_count)}</dd>
        </div>
      </dl>
      <details data-home-search={search.id}>
        <summary>Browse revision history</summary>
        <div className="rw-home-revisions" data-home-revisions>
          <p className={classNames.uiFadedText}>Open this disclosure to load a bounded revision page.</p>
        </div>
      </details>
    </article>
  );
}

/** Renders one hierarchy API failure without hiding successful sibling sections. */
function SectionError(props: { title: string; failure: unknown }): JSX.Element {
  const message = errorMessage(props.failure, "The section could not be loaded.");
  return (
    <p className={classNames.uiErrorMessage} role="alert">
      <span className="header">{props.title}</span>
      {message}
    </p>
  );
}

/** Renders one bounded page of run attempts and lifecycle controls. */
function RunTable(props: { runs: HierarchyRun[]; hasMore: boolean }): JSX.Element {
  const rows = props.runs.map((run) => {
    const canExplore = hasContext(run);
    const visibility = run.visibility_state || "active";
    var lifecycleAction: JSX.Element = (
      <button
        type="button"
        className={classNames.uiDangerBasicButton}
        data-run-visibility="trashed"
        data-run-id={run.id}
      >
        Move to trash
      </button>
    );
    if (run.status === "running") {
      lifecycleAction = <button type="button" className={classNames.uiBasicButton} disabled>Running</button>;
    } else if (visibility === "trashed") {
      lifecycleAction = (
        <button
          type="button"
          className={classNames.uiBasicButton}
          data-run-visibility="active"
          data-run-id={run.id}
        >
          Restore
        </button>
      );
    }
    var explore: JSX.Element = <span className={classNames.uiFadedText}>Context unavailable</span>;
    if (canExplore) {
      const target = deepdiveLink(run.search_id!, run.search_revision_id!, run.execution_plan_id!, run.id);
      explore = (
        <a
          className={classNames.uiPrimaryButton}
          href={target.href}
          data-state={JSON.stringify(target.state)}
        >
          Explore
        </a>
      );
    }
    var planLabel = "Not recorded";
    if (run.execution_plan_id) planLabel = `Plan ${run.execution_plan_id}`;
    return (
      <tr data-home-run={run.id}>
        <td data-label="Run attempt">
          <strong>Run {run.id}</strong>
          <small>Attempt {run.attempt_number || "Not recorded"}</small>
        </td>
        <td data-label="Search">{run.search_name || "Unplanned"}</td>
        <td data-label="Revision">{run.revision_label || "Not recorded"}</td>
        <td data-label="Execution plan">{planLabel}</td>
        <td data-label="Started">{formatTime(run.started_at)}</td>
        <td data-label="Duration">{formatDuration(run.started_at, run.finished_at)}</td>
        <td data-label="Outcome"><StatusChip raw={run.status} /></td>
        <td data-label="Visibility"><StatusChip raw={visibility} /></td>
        <td data-label="Actions">
          <div className="rw-inline-group">
            {explore}
            {lifecycleAction}
          </div>
        </td>
      </tr>
    );
  });
  var body: JSX.Element[] = [<tr><td colSpan={9} className="rw-table-empty">No run attempts match these filters.</td></tr>];
  if (rows.length) body = rows;
  var resultLabel = `Showing ${formatNumber(rows.length)} run attempts.`;
  if (props.hasMore) resultLabel = `Showing ${formatNumber(rows.length)} run attempts. More results are available.`;
  return (
    <Fragment>
      <p aria-live="polite">{resultLabel}</p>
      <div className="table-wrap" aria-label="Run attempts table">
        <table className={classNames.uiTableRwHomeRuns}>
          <thead>
            <tr>
              <th>Run attempt</th>
              <th>Search</th>
              <th>Revision</th>
              <th>Execution plan</th>
              <th>Started</th>
              <th>Duration</th>
              <th>Outcome</th>
              <th>Visibility</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>{body}</tbody>
        </table>
      </div>
    </Fragment>
  );
}

/** Renders server-backed Home filters with explicit visibility and calendar scope. */
function HomeFilters(): JSX.Element {
  const visibility = value("home_visibility") || "active";
  const status = value("home_status") || "all";
  const query = value("home_q");
  const startedAfter = value("home_started_after");
  const startedBefore = value("home_started_before");
  var advancedFilterCount = 0;
  if (visibility !== "active") advancedFilterCount += 1;
  if (status !== "all") advancedFilterCount += 1;
  if (startedAfter) advancedFilterCount += 1;
  if (startedBefore) advancedFilterCount += 1;
  var advancedSummary = "Active runs, any outcome or start date";
  if (advancedFilterCount) advancedSummary = `${advancedFilterCount} additional filters applied`;
  const advancedOpen = advancedFilterCount > 0;
  const activeFilters: Record<string, unknown> = {};
  if (query) activeFilters.home_q = query;
  if (visibility !== "active") activeFilters.home_visibility = visibility;
  if (status !== "all") activeFilters.home_status = status;
  if (startedAfter) activeFilters.home_started_after = startedAfter;
  if (startedBefore) activeFilters.home_started_before = startedBefore;
  const filterLabels = {
    home_q: "Search",
    home_visibility: "Visibility",
    home_status: "Outcome",
    home_started_after: "Started after",
    home_started_before: "Started before",
  };
  const clearUpdates = {
    home_q: "",
    home_visibility: "",
    home_status: "",
    home_started_after: "",
    home_started_before: "",
    home_search_cursor: "",
    home_run_cursor: "",
  };
  const filterOptions = { clearUpdates: clearUpdates };
  var filterSummary: JSX.Element | null = null;
  if (Object.keys(activeFilters).length) {
    filterSummary = <FilterChips filters={activeFilters} labels={filterLabels} options={filterOptions} />;
  }
  return (
    <form className={classNames.uiFormRwFilterPanel} data-home-filters>
      <div className="rw-home-filters__primary">
        <label className={classNames.fieldRwHomeFiltersSearch}>
          Search runs and history
          <input name="q" type="search" value={query} placeholder="Search term, revision, or run ID" />
        </label>
        <div className="rw-filter-panel__actions">
          <button type="submit" className={classNames.uiPrimaryButton}>Apply filters</button>
          <button type="button" className={classNames.uiBasicButton} data-home-clear>Clear filters</button>
        </div>
      </div>
      <details className={classNames.rwFilterDisclosureRwHomeFiltersAdvanced} open={advancedOpen}>
        <summary>
          <span>Visibility, outcome, and dates</span>
          <small>{advancedSummary}</small>
        </summary>
        <div className={classNames.rwFilterPanelFieldsRwHomeFiltersAdvancedFields}>
          <label className="field">
            Visibility
            <select name="visibility">
              <option value="active" selected={visibility === "active"}>Active</option>
              <option value="trashed" selected={visibility === "trashed"}>Trashed</option>
              <option value="all" selected={visibility === "all"}>Active and trashed</option>
            </select>
          </label>
          <label className="field">
            Outcome
            <select name="status">
              <option value="all" selected={status === "all"}>All outcomes</option>
              <option value="running" selected={status === "running"}>Running</option>
              <option value="completed" selected={status === "completed"}>Completed</option>
              <option value="failed" selected={status === "failed"}>Failed</option>
            </select>
          </label>
          <label className="field">
            Started on or after
            <input type="date" name="started_after" value={startedAfter} />
          </label>
          <label className="field">
            Started on or before
            <input type="date" name="started_before" value={startedBefore} />
          </label>
        </div>
      </details>
      {filterSummary}
    </form>
  );
}

/** Renders the native lifecycle confirmation dialog. */
function RunDialog(): JSX.Element {
  return (
    <dialog
      className="rw-review-dialog"
      data-run-dialog
      aria-labelledby="run-dialog-title"
      aria-describedby="run-dialog-description"
    >
      <form className={classNames.uiFormRwReviewDialogForm} data-run-dialog-form>
        <div className="rw-review-dialog__header">
          <div>
            <p className="rw-review-dialog__eyebrow">Run lifecycle</p>
            <h3 id="run-dialog-title">Change run visibility</h3>
            <p id="run-dialog-description"></p>
          </div>
          <button
            type="button"
            className={classNames.uiIconBasicButtonRwReviewDialogClose}
            data-run-dialog-close
            aria-label="Close run lifecycle dialog"
          >
            {"\u00D7"}
          </button>
        </div>
        <div className="rw-review-dialog__body">
          <label className="field" data-run-reason-field>
            Reason
            <textarea name="reason" maxLength={1000} rows={3} placeholder="Why should this run move out of the active workspace?"></textarea>
          </label>
          <p className={classNames.uiInfoMessage} data-run-dialog-guidance></p>
          <p className={classNames.uiErrorMessage} data-run-dialog-error role="alert" hidden></p>
        </div>
        <div className="rw-review-dialog__actions">
          <button type="button" className={classNames.uiBasicButton} data-run-dialog-close>Cancel</button>
          <button type="submit" className={classNames.uiDangerButton} data-run-dialog-submit>Confirm</button>
        </div>
      </form>
    </dialog>
  );
}

/** Loads and renders one bounded revision page inside an open search disclosure. */
async function loadRevisions(searchID: string, cursor: string, host: HTMLElement): Promise<void> {
  const loadingMarkup = <p className={classNames.uiInfoMessage}>Loading revision history.</p>;
  renderTree(loadingMarkup, host);
  try {
    const result = await api<HierarchyPage<HierarchyRevision>>("/api/hierarchy", {
      section: "revisions",
      search_id: searchID,
      cursor: cursor,
    }, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    const rows = result.items.map((revision) => {
      return (
        <div>
          <div>
            <strong>{revision.label || `Revision ${revision.id}`}</strong>
            <span>{formatNumber(revision.plan_count)} plans · {formatNumber(revision.run_count)} attempts</span>
          </div>
        </div>
      );
    });
    var empty: JSX.Element | null = null;
    if (!rows.length) empty = <p className={classNames.uiFadedText}>No search revisions are recorded.</p>;
    var more: JSX.Element | null = null;
    if (result.has_more) {
      more = (
        <button type="button" className={classNames.uiBasicButton} data-more-revisions={result.next_cursor}>
          Next revision page
        </button>
      );
    }
    const pageMarkup = (
      <Fragment>
        {empty}
        {rows}
        {more}
      </Fragment>
    );
    renderTree(pageMarkup, host);
    host.querySelector<HTMLButtonElement>("[data-more-revisions]")?.addEventListener("click", (event) => {
      const button = event.currentTarget as HTMLButtonElement;
      void loadRevisions(searchID, button.dataset.moreRevisions || "", host);
    });
  } catch (failure) {
    const errorMarkup = <SectionError title="Revision history unavailable" failure={failure} />;
    renderTree(errorMarkup, host);
  }
}

/** Binds search disclosures and cursor paging for the bounded Home sections. */
function bindHomeDiscovery(): void {
  app.querySelectorAll<HTMLDetailsElement>("[data-home-search]").forEach((details) => {
    let loaded = false;
    details.addEventListener("toggle", () => {
      if (!details.open || loaded) return;
      loaded = true;
      const host = details.querySelector<HTMLElement>("[data-home-revisions]")!;
      void loadRevisions(details.dataset.homeSearch || "", "", host);
    });
  });
  app.querySelector<HTMLButtonElement>("[data-more-searches]")?.addEventListener("click", (event) => {
    setURL({ home_search_cursor: (event.currentTarget as HTMLButtonElement).dataset.moreSearches }, false);
  });
  app.querySelector<HTMLButtonElement>("[data-more-runs]")?.addEventListener("click", (event) => {
    setURL({ home_run_cursor: (event.currentTarget as HTMLButtonElement).dataset.moreRuns }, false);
  });
  app.querySelector<HTMLFormElement>("[data-home-filters]")?.addEventListener("submit", (event) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget as HTMLFormElement);
    setURL({
      home_q: form.get("q"),
      home_visibility: form.get("visibility"),
      home_status: form.get("status"),
      home_started_after: form.get("started_after"),
      home_started_before: form.get("started_before"),
      home_search_cursor: "",
      home_run_cursor: "",
    }, false);
  });
  app.querySelector<HTMLButtonElement>("[data-home-clear]")?.addEventListener("click", () => {
    setURL({
      home_q: "",
      home_visibility: "",
      home_status: "",
      home_started_after: "",
      home_started_before: "",
      home_search_cursor: "",
      home_run_cursor: "",
    }, false);
  });
}

/** Binds native-dialog confirmation and mutation behavior for Home lifecycle controls. */
function bindRunLifecycle(): void {
  const dialog = app.querySelector<HTMLDialogElement>("[data-run-dialog]");
  if (!dialog) return;
  const form = dialog.querySelector<HTMLFormElement>("[data-run-dialog-form]")!;
  const title = dialog.querySelector<HTMLElement>("#run-dialog-title")!;
  const description = dialog.querySelector<HTMLElement>("#run-dialog-description")!;
  const guidance = dialog.querySelector<HTMLElement>("[data-run-dialog-guidance]")!;
  const reasonField = dialog.querySelector<HTMLElement>("[data-run-reason-field]")!;
  const reason = form.elements.namedItem("reason") as HTMLTextAreaElement;
  const submit = dialog.querySelector<HTMLButtonElement>("[data-run-dialog-submit]")!;
  const error = dialog.querySelector<HTMLElement>("[data-run-dialog-error]")!;
  const closeButtons = dialog.querySelectorAll<HTMLButtonElement>("[data-run-dialog-close]");
  let mutating = false;
  let opener: HTMLButtonElement | null = null;

  /** Dismisses the lifecycle dialog and restores focus to its exact opener. */
  function close(): void {
    if (mutating) return;
    if (typeof dialog!.close === "function") dialog!.close();
    else dialog!.removeAttribute("open");
    error.hidden = true;
    form.reset();
    opener?.focus();
  }

  /** Configures and opens the lifecycle dialog for the selected run action. */
  function open(button: HTMLButtonElement): void {
    opener = button;
    dialog!.dataset.runId = button.dataset.runId;
    dialog!.dataset.visibilityState = button.dataset.runVisibility;
    const trashing = button.dataset.runVisibility === "trashed";
    var titleText = `Restore run ${button.dataset.runId}?`;
    if (trashing) titleText = `Move run ${button.dataset.runId} to trash?`;
    title.textContent = titleText;
    var descriptionText = "This returns the attempt to active workspace results.";
    if (trashing) descriptionText = "This removes the attempt from active workspace results without deleting immutable evidence.";
    description.textContent = descriptionText;
    var guidanceText = "Restoring does not change the captured outcome or execution evidence.";
    if (trashing) guidanceText = "The run, its provenance, and any review history remain stored and can be restored from Home.";
    guidance.textContent = guidanceText;
    reasonField.hidden = !trashing;
    var submitText = "Restore run";
    if (trashing) submitText = "Move to trash";
    submit.textContent = submitText;
    classToggle(submit, "danger", trashing);
    classToggle(submit, "primary", !trashing);
    dialog!.showModal?.();
    if (!dialog!.open) dialog!.setAttribute("open", "");
    if (trashing) reason.focus();
    else submit.focus();
  }

  app.querySelectorAll<HTMLButtonElement>("[data-run-visibility]").forEach((button) => {
    button.addEventListener("click", () => {
      open(button);
    });
  });
  closeButtons.forEach((button) => {
    button.addEventListener("click", close);
  });
  dialog.addEventListener("click", (event) => {
    if (event.target === dialog) close();
  });
  dialog.addEventListener("cancel", (event) => {
    if (mutating) event.preventDefault();
  });
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    error.hidden = true;
    mutating = true;
    submit.disabled = true;
    classAdd(submit, ["loading"]);
    closeButtons.forEach((button) => {
      button.disabled = true;
    });
    const runID = dialog.dataset.runId || "";
    const visibilityState = dialog.dataset.visibilityState || "";
    try {
      var reasonValue = "";
      if (visibilityState === "trashed") reasonValue = reason.value;
      await mutate<RunVisibilityResponse>(`/api/runs/${encodeURIComponent(runID)}/visibility`, "PUT", {
        visibility_state: visibilityState,
        reason: reasonValue,
      });
    } catch (failure) {
      error.textContent = errorMessage(failure, "The run could not be updated.");
      error.hidden = false;
      mutating = false;
      submit.disabled = false;
      classRemove(submit, "loading");
      closeButtons.forEach((button) => {
        button.disabled = false;
      });
      return;
    }
    mutating = false;
    submit.disabled = false;
    classRemove(submit, "loading");
    closeButtons.forEach((button) => {
      button.disabled = false;
    });
    close();
    try {
      var actionLabel = "Restored";
      if (visibilityState === "trashed") actionLabel = "Moved";
      await homeView(`${actionLabel} run ${runID}.`);
    } catch (failure) {
      const status = app.querySelector<HTMLElement>("[data-home-lifecycle-status]");
      if (status) status.textContent = `Run ${runID} was updated, but Home could not refresh: ${errorMessage(failure, "Unknown error")}`;
    }
  });
}

/** Renders the workspace Home page from independently recoverable bounded hierarchy requests. */
export async function homeView(lifecycleMessage = ""): Promise<void> {
  const query = value("home_q");
  const visibility = value("home_visibility") || "active";
  const status = value("home_status") || "all";
  const dateQuery = {
    q: query,
    visibility: visibility,
    status: status,
    started_after: value("home_started_after"),
    started_before: value("home_started_before"),
  };
  const results = await Promise.allSettled([
    api<HierarchySummaryResponse>("/api/hierarchy", { section: "summary" }, { method: "GET", headers: { Accept: "application/json" } }),
    api<HierarchyPage<HierarchySearch>>("/api/hierarchy", { section: "searches", q: query, cursor: value("home_search_cursor") }, { method: "GET", headers: { Accept: "application/json" } }),
    api<HierarchyPage<HierarchyRun>>("/api/hierarchy", { section: "runs", ...dateQuery, cursor: value("home_run_cursor") }, { method: "GET", headers: { Accept: "application/json" } }),
  ]);
  const summaryResult = results[0];
  const searchesResult = results[1];
  const runsResult = results[2];

  var metrics: JSX.Element = <SectionError title="Workspace totals unavailable" failure={(summaryResult as PromiseRejectedResult).reason} />;
  if (summaryResult.status === "fulfilled") {
    const totals = summaryResult.value.totals;
    metrics = (
      <div className={classNames.rwKpiGridRwHomeKpis}>
        <div className="rw-kpi">
          <span className="label">Search terms</span>
          <span className="value">{formatNumber(totals.searches)}</span>
          <small>Declared research scopes</small>
        </div>
        <div className="rw-kpi">
          <span className="label">Search revisions</span>
          <span className="value">{formatNumber(totals.revisions)}</span>
          <small>Immutable configurations</small>
        </div>
        <div className="rw-kpi">
          <span className="label">Execution plans</span>
          <span className="value">{formatNumber(totals.plans)}</span>
          <small>Resolved pipeline plans</small>
        </div>
        <div className="rw-kpi">
          <span className="label">Run attempts</span>
          <span className="value">{formatNumber(totals.runs)}</span>
          <small>{formatNumber(totals.completed_runs)} completed</small>
        </div>
      </div>
    );
  }

  var searchHistory: JSX.Element = <SectionError title="Research history unavailable" failure={(searchesResult as PromiseRejectedResult).reason} />;
  if (searchesResult.status === "fulfilled") {
    const cards = searchesResult.value.items.map((search) => {
      return <SearchCard search={search} />;
    });
    var searchEmpty: JSX.Element | null = null;
    if (!cards.length) searchEmpty = <p className={classNames.uiFadedText}>No search terms match this search.</p>;
    var nextSearchPage: JSX.Element | null = null;
    if (searchesResult.value.has_more) {
      nextSearchPage = (
        <button type="button" className={classNames.uiBasicButton} data-more-searches={searchesResult.value.next_cursor}>
          Next search page
        </button>
      );
    }
    searchHistory = (
      <Fragment>
        {searchEmpty}
        <div className="rw-home-searches">{cards}</div>
        {nextSearchPage}
      </Fragment>
    );
  }

  var runTableMarkup: JSX.Element = <SectionError title="Run attempts unavailable" failure={(runsResult as PromiseRejectedResult).reason} />;
  if (runsResult.status === "fulfilled") {
    var nextRunPage: JSX.Element | null = null;
    if (runsResult.value.has_more) {
      nextRunPage = (
        <button type="button" className={classNames.uiBasicButton} data-more-runs={runsResult.value.next_cursor}>
          Next run page
        </button>
      );
    }
    runTableMarkup = (
      <Fragment>
        <RunTable runs={runsResult.value.items} hasMore={runsResult.value.has_more} />
        {nextRunPage}
      </Fragment>
    );
  }

  const pageMarkup = (
    <Fragment>
      <PageHeader
        kicker=""
        title="Home"
        description="Choose a captured run for Deepdive analysis or manage which completed attempts remain active."
      />
      <p data-home-lifecycle-status role="status" aria-live="polite">{lifecycleMessage}</p>
      {metrics}
      <Panel
        title="Research history"
        description="Search terms organize immutable revisions, execution plans, and recorded run attempts."
        body={searchHistory}
      />
      <Panel
        title="Run attempts"
        description="Filter a bounded result page, explore a complete context, or change a terminal run's reversible visibility."
        body={<div className="rw-content-stack"><HomeFilters />{runTableMarkup}</div>}
      />
      <RunDialog />
    </Fragment>
  );
  renderTree(pageMarkup, app);
  bindHomeDiscovery();
  bindRunLifecycle();
}
