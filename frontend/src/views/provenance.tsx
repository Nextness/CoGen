// Read-only execution evidence: audit events, artifacts, cache uses, stages, and run details.
import {
  app, value, link, stateFor, state, provenanceSections, section,
  Cell, list, selectedRun, pickID, formatTime, formatBytes,
  formatNumber, humanLabel, pageSizes, PageHeader, Subnav, EmptyPanel, Panel, StatusChip, FilterChips,
} from "../state.tsx";
import { h, Fragment, render as renderTree, cx, classToggle, classAdd, classRemove } from "../jsx/jsx-runtime.ts";
import type { ClassName } from "../jsx/classes.ts";
import { api, errorMessage } from "../api.tsx";
import type {
  APIQuery,
  ArtifactContext as ArtifactContextRecord,
  ArtifactInspectionResponse,
  ArtifactRecord,
  ArtifactsResponse,
  AuditFacet,
  AuditResponse,
  CacheUsesResponse,
  RunStep,
  StageSummary,
  StagesResponse,
  WireRecord,
} from "../api/types.ts";
import { setURL, bindFocusContext, replaceState } from "../router.tsx";
import { DataTable, bindTableControls } from "../components/data-table.tsx";
import type { DataTableContext } from "../components/data-table.tsx";
import { Pagination } from "../components/pagination.tsx";
import type { PaginationOptions } from "../components/pagination.tsx";
import { AuditStream, bindAuditRecordedData } from "../components/audit-events.tsx";
import type { AuditEventRecord } from "../components/audit-events.tsx";

/** Typed compound class names used by this module. */
const classNames = {
  rwFilterPanelFieldsRwFilterFieldGrid: cx("rw-filter-panel__fields", "rw-filter-field-grid"),
  rwFilterPanelFieldsRwFilterFieldGridRwFilterFieldGridAdvanced: cx("rw-filter-panel__fields", "rw-filter-field-grid", "rw-filter-field-grid--advanced"),
  rwPropertyGridRwPropertyGridCompact: cx("rw-property-grid", "rw-property-grid--compact"),
  uiBasicButton: cx("ui", "basic", "button"),
  uiButton: cx("ui", "button"),
  uiErrorMessage: cx("ui", "error", "message"),
  uiFadedText: cx("ui", "faded", "text"),
  uiFormRwFilterBar: cx("ui", "form", "rw-filter-bar"),
  uiFormRwFilterPanel: cx("ui", "form", "rw-filter-panel"),
  uiInfoMessage: cx("ui", "info", "message"),
  uiLabel: cx("ui", "label"),
  uiPrimaryButton: cx("ui", "primary", "button"),
  uiSegment: cx("ui", "segment"),
  uiSegmentRwAuditFilters: cx("ui", "segment", "rw-audit-filters"),
  uiSuccessMessage: cx("ui", "success", "message"),
  uiTableRwArtifactTable: cx("ui", "table", "rw-artifact-table"),
  uiTopAttachedHeader: cx("ui", "top", "attached", "header"),
  uiWarningMessage: cx("ui", "warning", "message"),
};

/** Defined stage modifiers keyed by the displayed execution status. */
const stageStatusClasses: Record<string, ClassName | undefined> = {
  failed: "rw-stage-step--failed",
  warning: "rw-stage-step--warning",
  skipped: "rw-stage-step--skipped",
  "not-recorded": "rw-stage-step--not-recorded",
};

const auditFilterKeys: Record<string, string> = {
  audit_q: "Search",
  audit_category: "Category",
  audit_action: "Event type",
  audit_actor: "Source system",
  audit_entity: "Entity type",
  audit_stage: "Pipeline stage",
  audit_outcome: "Outcome",
  audit_review_status: "Review status",
  audit_review_reason: "Review reason",
  audit_review_substatus: "Review subclassification",
};

/** Client presentation state derived from one artifact inspection response. */
interface ArtifactPreviewState extends ArtifactInspectionResponse {
  raw: string;
  formatted: string;
  formatError: boolean;
  mode: "raw" | "formatted";
  wrap: boolean;
}

let activeArtifactPreview: ArtifactPreviewState | null = null;
let activeArtifactRow: HTMLElement | null = null;
let artifactInspectionSequence = 0;
let auditEvents: AuditEventRecord[] = [];
let auditKnownEventIDs = new Set<string>();
let auditLoadedCount = 0;
let auditCursor = "";
let auditHasMore = false;
export const auditVisibleEventLimit = 200;

/** Renders a formatted timestamp cell for a data-table column. */
function renderTime(row: WireRecord, raw: unknown): JSX.Element {
  return <>{formatTime(raw)}</>;
}

/** Returns the selected comma-separated values for an audit facet. */
function selectedValues(raw: unknown): string[] {
  const parts = String(raw || "").split(",");
  const trimmed = parts.map((item) => {
    return item.trim();
  });
  return trimmed.filter(Boolean);
}

/** Renders a multi-select control for one audit facet. */
function AuditMultiSelect(props: { name: string; label: string; options: Array<string | AuditFacet>; selectedRaw: unknown }): JSX.Element {
  const selected = new Set(selectedValues(props.selectedRaw));
  var summary = "Any";
  if (selected.size) summary = `${selected.size} selected`;
  const choices = (props.options || []).map((item) => {
    const optionValue = typeof item === "string" ? item : item.value;
    return (
      <label className="rw-check-option">
        <input type="checkbox" name={props.name} value={optionValue} checked={selected.has(optionValue)} />
        <span>{humanLabel(optionValue)}</span>
      </label>
    );
  });
  var empty: JSX.Element = <p className={classNames.uiFadedText}>No recorded values are available for this run.</p>;
  if (choices.length) empty = <Fragment>{choices}</Fragment>;
  return (
    <details className="rw-multi-select" data-multi-select>
      <summary>
        <span>{props.label}</span>
        <strong>{summary}</strong>
      </summary>
      <div className="rw-multi-select__menu">
        <div className="rw-multi-select__actions">
          <button type="button" className={classNames.uiBasicButton} data-multi-select-all>Select all</button>
          <button type="button" className={classNames.uiBasicButton} data-multi-select-clear>Clear</button>
        </div>
        {empty}
      </div>
    </details>
  );
}

/** Builds API query parameters from the active audit filters. */
function auditQuery(cursor: string): APIQuery {
  return {
    run_id: value("run_id"),
    q: value("audit_q"),
    category: value("audit_category"),
    action: value("audit_action"),
    actor: value("audit_actor"),
    entity_type: value("audit_entity"),
    stage: value("audit_stage"),
    outcome: value("audit_outcome"),
    review_status: value("audit_review_status"),
    review_reason: value("audit_review_reason"),
    review_substatus: value("audit_review_substatus"),
    limit: 25,
    cursor: cursor || "",
  };
}

/** Renders markup summarizing active audit filters and their removal links. */
function AuditFilterSummary(): JSX.Element {
  const filters: Record<string, unknown> = {};
  const filterKeys = Object.keys(auditFilterKeys);
  filterKeys.forEach((key) => {
    if (!value(key)) return;
    if (["audit_category", "audit_action", "audit_actor", "audit_entity"].includes(key)) {
      filters[key] = selectedValues(value(key));
    } else {
      filters[key] = value(key);
    }
  });
  const clearUpdates: Record<string, string> = {};
  filterKeys.forEach((key) => {
    clearUpdates[key] = "";
  });
  return <FilterChips filters={filters} labels={auditFilterKeys} options={{ clearUpdates: clearUpdates }} />;
}

/** Renders the complete audit filter form. */
function AuditFilters(props: { facets: AuditResponse["facets"] }): JSX.Element {
  const actors = list<string | AuditFacet>(props.facets, ["actors"]);
  const actions = list<string | AuditFacet>(props.facets, ["actions"]);
  const entityTypes = list<string | AuditFacet>(props.facets, ["entity_types"]);
  const resetUpdates = Object.fromEntries(Object.keys(auditFilterKeys).map((key) => {
    return [key, ""];
  }));
  const resetLink = link(resetUpdates);
  const resetState = stateFor(resetUpdates);
  return (
    <aside className={classNames.uiSegmentRwAuditFilters}>
      <div className={classNames.uiTopAttachedHeader}>
        <div>
          <h3>Filter audit evidence</h3>
          <p>Filters are applied by the server and remain visible in the URL.</p>
        </div>
      </div>
      <div className="content">
        <form id="audit-filter-form" className={classNames.uiFormRwFilterPanel}>
          <label>
            Search events
            <input name="audit_q" type="search" value={value("audit_q")} placeholder="Action, entity, source, or metadata" />
          </label>
          <div className={classNames.rwFilterPanelFieldsRwFilterFieldGrid}>
            <AuditMultiSelect name="audit_category" label="Category" options={["pipeline", "enrichment", "validation", "review", "pdf"]} selectedRaw={value("audit_category")} />
            <AuditMultiSelect name="audit_action" label="Event type" options={actions} selectedRaw={value("audit_action")} />
            <AuditMultiSelect name="audit_actor" label="Source system" options={actors} selectedRaw={value("audit_actor")} />
            <AuditMultiSelect name="audit_entity" label="Entity type" options={entityTypes} selectedRaw={value("audit_entity")} />
          </div>
          <details className="rw-filter-disclosure">
            <summary>Stage and outcome filters</summary>
            <div className={classNames.rwFilterPanelFieldsRwFilterFieldGridRwFilterFieldGridAdvanced}>
              <label>
                Pipeline stage
                <input name="audit_stage" value={value("audit_stage")} placeholder="For example, normalize" />
              </label>
              <label>
                Outcome
                <input name="audit_outcome" value={value("audit_outcome")} placeholder="For example, completed" />
              </label>
              <label>
                Review status
                <input name="audit_review_status" value={value("audit_review_status")} placeholder="For example, approved" />
              </label>
              <label>
                Review reason
                <input name="audit_review_reason" value={value("audit_review_reason")} />
              </label>
              <label>
                Review subclassification
                <input name="audit_review_substatus" value={value("audit_review_substatus")} placeholder="For example, duplicate" />
              </label>
            </div>
          </details>
          <AuditFilterSummary />
          <div className="rw-filter-panel__actions">
            <button type="submit" className={classNames.uiPrimaryButton}>Apply filters</button>
            <a className={classNames.uiBasicButton} href={resetLink} data-state={JSON.stringify(resetState)}>Reset</a>
          </div>
        </form>
      </div>
    </aside>
  );
}

/** Renders summary cards for the filtered audit result. */
function AuditSummary(props: { data: AuditResponse }): JSX.Element {
  const summary = props.data.summary;
  const actions = summary?.actions || [];
  var scope = "All recorded runs";
  if (value("run_id")) {
    var scopeSuffix = "";
    if (selectedValues(value("audit_category")).includes("pdf")) scopeSuffix = " plus global PDF evidence";
    scope = `Run ${value("run_id")}${scopeSuffix}`;
  }
  return (
    <dl className="rw-summary-strip">
      <div>
        <dt>Matching events</dt>
        <dd>{formatNumber(summary?.total_events || auditEvents.length)}</dd>
      </div>
      <div>
        <dt>Events loaded</dt>
        <dd data-audit-loaded-count>{formatNumber(auditLoadedCount)}</dd>
      </div>
      <div>
        <dt>Event types</dt>
        <dd>{formatNumber(actions.length)}</dd>
      </div>
      <div>
        <dt>Scope</dt>
        <dd>{scope}</dd>
      </div>
    </dl>
  );
}

/** Renders the audit timeline and pagination markup. */
function AuditView(props: { data: AuditResponse }): JSX.Element {
  auditEvents = list<AuditEventRecord>(props.data, ["events", "items"]);
  auditKnownEventIDs = new Set(auditEvents.map((event) => String(event.id)));
  auditLoadedCount = auditEvents.length;
  auditCursor = String(props.data.next_cursor || "");
  auditHasMore = Boolean(props.data.has_more);
  return (
    <div className="rw-audit-layout">
      <AuditFilters facets={props.data.facets || { actors: [], actions: [], entity_types: [] }} />
      <section className={classNames.uiSegment}>
        <div className={classNames.uiTopAttachedHeader}>
          <div>
            <h3>Audit timeline</h3>
            <p>Newest evidence appears first. Open Recorded data only when identifiers or payload changes are needed.</p>
          </div>
        </div>
        <div className="content">
          <AuditSummary data={props.data} />
          <div id="audit-event-stream" aria-busy="false"><AuditStream events={auditEvents} /></div>
          <p className={classNames.uiInfoMessage} data-audit-window-status role="status" hidden></p>
          <div className="rw-load-more" hidden={!auditHasMore}>
            <p className={classNames.uiFadedText} role="status" aria-live="polite" data-audit-page-status>{formatNumber(auditEvents.length)} events loaded.</p>
            <button type="button" className={classNames.uiButton} data-audit-load-more>Load 25 older events</button>
          </div>
          <p className={classNames.uiSuccessMessage} data-audit-end hidden={auditHasMore}>The beginning of the recorded history has been reached.</p>
          <div className={classNames.uiErrorMessage} data-audit-page-error role="alert" hidden></div>
        </div>
      </section>
    </div>
  );
}

/** Appends audit events into stable date groups without replacing existing event nodes. */
export function appendAuditEvents(stream: HTMLElement, events: AuditEventRecord[]): number {
  if (!events.length) return 0;
  const staging = document.createElement("div");
  const auditStreamMarkup = <AuditStream events={events} />;
  renderTree(auditStreamMarkup, staging);
  const incomingDays = Array.from(staging.querySelectorAll<HTMLElement>(".rw-audit-day"));
  incomingDays.forEach((incomingDay) => {
    const date = incomingDay.dataset.auditDate || "";
    const existingDay = Array.from(stream.querySelectorAll<HTMLElement>(".rw-audit-day")).find((day) => {
      return day.dataset.auditDate === date;
    });
    if (!existingDay) {
      stream.append(incomingDay);
      return;
    }
    const existingList = existingDay.querySelector(".rw-audit-events")!;
    const incomingItems = Array.from(incomingDay.querySelectorAll(":scope > .rw-audit-events > li"));
    incomingItems.forEach((item) => existingList.append(item));
  });
  return events.length;
}

/** Bounds visible audit-event nodes while preserving disclosures the reviewer has opened. */
export function boundAuditWindow(stream: HTMLElement, limit: number = auditVisibleEventLimit): number {
  const events = Array.from(stream.querySelectorAll<HTMLElement>(".rw-audit-event"));
  var removeCount = Math.max(0, events.length - limit);
  var removed = 0;
  for (const event of events) {
    if (!removeCount) break;
    if (event.querySelector<HTMLDetailsElement>(".rw-event-details[open]")) continue;
    event.closest("li")?.remove();
    removeCount -= 1;
    removed += 1;
  }
  stream.querySelectorAll<HTMLElement>(".rw-audit-day").forEach((day) => {
    if (!day.querySelector(".rw-audit-event")) day.remove();
  });
  return removed;
}

/** Renders the research-context fields displayed for an artifact. */
function ArtifactContext(props: { context: ArtifactContextRecord }): JSX.Element {
  var planLabel = props.context.execution_plan_id || "Not recorded";
  if (props.context.execution_fingerprint) planLabel = String(props.context.execution_fingerprint).slice(0, 16);
  var runLabel = "Not recorded";
  if (props.context.run_id) runLabel = `Run ${props.context.run_id}, attempt ${props.context.attempt_number}`;
  const entries = [
    ["Search", props.context.search_id || "Not recorded"],
    ["Revision", props.context.search_revision_label || props.context.search_revision_id || "Not recorded"],
    ["Execution plan", planLabel],
    ["Run attempt", runLabel],
  ];
  const entryItems = entries.map(([label, entryValue]) => {
    return (
      <div>
        <dt>{label}</dt>
        <dd>{entryValue}</dd>
      </div>
    );
  });
  return <dl className="rw-context-strip">{entryItems}</dl>;
}

/** Renders safe inspect and download actions for an artifact. */
function ArtifactActions(props: { row: ArtifactRecord }): JSX.Element {
  if (!props.row.has_blob) {
    return <span className={classNames.uiFadedText}>Payload not stored</span>;
  }
  var inspect: JSX.Element = <span className={classNames.uiLabel}>Download only</span>;
  if (props.row.preview_available) {
    inspect = <button type="button" className={classNames.uiButton} data-inspect-artifact={props.row.id}>Inspect preview</button>;
  }
  return (
    <div className="artifact-actions">
      {inspect}
      <a className={classNames.uiBasicButton} href={`/api/artifacts/${encodeURIComponent(props.row.id)}/content`} download>Download</a>
    </div>
  );
}

/** Renders the run artifact inventory markup. */
function ArtifactsView(props: { data: ArtifactsResponse }): JSX.Element {
  const artifacts = list<ArtifactRecord>(props.data, ["artifacts"]);
  const page = Math.max(1, Number(value("artifact_page") || props.data.pagination?.page || 1));
  const perPage = Number(value("artifact_per_page") || props.data.pagination?.per_page || 50);
  const pagination = props.data.pagination || {
    page: page,
    per_page: perPage,
    total_rows: artifacts.length,
    total_pages: 1,
  };
  activeArtifactPreview = null;
  activeArtifactRow = null;
  var rows: JSX.Element[] = [<tr><td colSpan={6} className="rw-table-empty">No artifacts were recorded for this run.</td></tr>];
  if (artifacts.length) {
    rows = artifacts.map((row) => {
      const role = row.artifact_roles || row.relationship_roles || "Stage artifact";
      var producedPart = "";
      if (row.produced_by_steps) producedPart = `Produced: ${row.produced_by_steps}`;
      var consumedPart = "";
      if (row.consumed_by_steps) consumedPart = `Consumed: ${row.consumed_by_steps}`;
      const stageParts = [producedPart, consumedPart];
      const stages = stageParts.filter(Boolean).join(" / ");
      var stageCell: JSX.Element = <span className={classNames.uiFadedText}>Not linked to a recorded step</span>;
      if (stages) stageCell = <>{stages}</>;
      const focused = String(row.id) === value("artifact_id");
      var rowClass: ClassName | undefined;
      var rowTabIndex: number | undefined;
      if (focused) {
        rowClass = "selected";
        rowTabIndex = -1;
      }
      return (
        <tr data-artifact-row={row.id} className={rowClass} tabIndex={rowTabIndex}>
          <td><strong>{humanLabel(role)}</strong><small className="rw-cell-note">Artifact {String(row.id)}</small></td>
          <td>{stageCell}</td>
          <td><span className="rw-mono">{String(row.content_type)}</span><small className="rw-cell-note">{formatBytes(row.byte_size)}</small></td>
          <td><span className="rw-cell" title={String(row.content_hash)}>{String(row.content_hash)}</span></td>
          <td>{formatTime(row.created_at)}</td>
          <td><ArtifactActions row={row} /></td>
        </tr>
      );
    });
  }
  const role = value("artifact_role");
  var filterSummary: JSX.Element | null = null;
  if (value("artifact_q") || role) {
    const activeFilters = {
      artifact_q: value("artifact_q"),
      artifact_role: role,
    };
    const filterLabels = {
      artifact_q: "Search",
      artifact_role: "Relationship",
    };
    const filterOptions = {
      removeUpdates: { artifact_page: 1, artifact_id: "" },
      clearUpdates: { artifact_q: "", artifact_role: "", artifact_page: 1, artifact_id: "" },
    };
    filterSummary = <FilterChips filters={activeFilters} labels={filterLabels} options={filterOptions} />;
  }
  const controls = (
    <form className={classNames.uiFormRwFilterBar} data-artifact-filters>
      <label className="rw-filter-bar__search">
        Search artifacts
        <input name="artifact_q" type="search" value={value("artifact_q")} placeholder="Hash, format, stage, provider, or role" />
      </label>
      <label>
        Relationship
        <select name="artifact_role">
          <option value="" selected={!role}>All relationships</option>
          <option value="run_role" selected={role === "run_role"}>Run configuration role</option>
          <option value="step_input" selected={role === "step_input"}>Step input</option>
          <option value="step_output" selected={role === "step_output"}>Step output</option>
          <option value="cache_payload" selected={role === "cache_payload"}>Cache payload</option>
          <option value="identity_candidate_payload" selected={role === "identity_candidate_payload"}>Identity candidate payload</option>
        </select>
      </label>
      <label>
        Rows per page
        <select name="artifact_per_page"><PageSizeOptions current={perPage} /></select>
      </label>
      <button type="submit" className={classNames.uiPrimaryButton}>Apply filters</button>
      {filterSummary}
    </form>
  );
  const paginationOptions: PaginationOptions = {
    page: page,
    perPage: perPage,
    itemLabel: "artifacts",
    pageAttribute: "data-artifact-page",
  };
  const paginationMarkup = <Pagination result={pagination} options={paginationOptions} />;
  const artifactCount = formatNumber(pagination.total_rows);
  return (
    <Fragment>
      <ArtifactContext context={props.data.context} />
      <p className={classNames.uiInfoMessage}>Artifact inspection is read-only. Text previews are bounded before they reach the browser; download the original file for complete inspection.</p>
      <section className={classNames.uiSegment}>
        <div className={classNames.uiTopAttachedHeader}>
          <div>
            <h3>Recorded artifacts</h3>
            <p>Content-addressed files linked to this run through configuration roles or execution steps.</p>
          </div>
          <span className={classNames.uiLabel}>{artifactCount} artifacts</span>
        </div>
        <div className="content">
          {controls}
          <div className="table-wrap">
            <table className={classNames.uiTableRwArtifactTable}>
              <thead>
                <tr>
                  <th>Role</th>
                  <th>Execution use</th>
                  <th>Format and size</th>
                  <th>Content hash</th>
                  <th>Created</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>{rows}</tbody>
            </table>
          </div>
          {paginationMarkup}
        </div>
      </section>
      <section className={classNames.uiSegment} id="artifact-inspector">
        <div className={classNames.uiTopAttachedHeader}>
          <div>
            <h3 tabIndex={-1} data-artifact-inspector-title>Artifact preview</h3>
            <p>Inspect a safe prefix of a text-based artifact without loading the full file into the document.</p>
          </div>
        </div>
        <div className="content"><EmptyPanel title="No artifact selected" detail="Choose Inspect preview from the artifact list above." /></div>
      </section>
    </Fragment>
  );
}

/** Renders page-size option markup with the current value selected. */
function PageSizeOptions(props: { current: number | string }): JSX.Element {
  const sizeOptions = pageSizes.map((size) => {
    return <option value={size} selected={Number(props.current) === size}>{size}</option>;
  });
  return <Fragment>{sizeOptions}</Fragment>;
}

/** Renders cache-use evidence and pagination markup. */
function CacheView(props: { data: CacheUsesResponse }): JSX.Element {
  const rows = list<WireRecord>(props.data, ["rows", "cache_uses"]);
  const columns = list<string>(props.data, ["columns"]);
  const page = Math.max(1, Number(value("cache_page") || props.data.pagination?.page || 1));
  const perPage = Number(value("cache_per_page") || props.data.pagination?.per_page || 50);
  const pagination = props.data.pagination || {
    page: page,
    per_page: perPage,
    total_rows: rows.length,
    total_pages: Math.max(1, Math.ceil(rows.length / perPage)),
  };
  var resultColumns = Object.keys(rows[0] || {});
  if (columns.length) resultColumns = columns;
  const result = {
    columns: resultColumns,
    rows: rows,
    pagination: pagination,
  };
  var filterSummary: JSX.Element | null = null;
  if (value("cache_q")) {
    filterSummary = <FilterChips filters={{ cache_q: value("cache_q") }} labels={{ cache_q: "Search" }} options={{
      removeUpdates: { cache_page: 1 },
      clearUpdates: { cache_q: "", cache_page: 1 },
    }} />;
  }
  const controls = (
    <form className={classNames.uiFormRwFilterBar} id="cache-controls">
      <label className="rw-filter-bar__search">
          Search cache evidence
          <input id="cache-query" type="search" value={value("cache_q")} placeholder="Provider, namespace, outcome, or fingerprint" />
        </label>
      <label>
        Rows per page
        <select id="cache-per-page"><PageSizeOptions current={perPage} /></select>
      </label>
      <button type="submit" className={classNames.uiPrimaryButton} data-cache-search>Search</button>
      {filterSummary}
    </form>
  );
  const columnConfig: NonNullable<DataTableContext["columnConfig"]> = {
    provider: { label: "Provider" },
    namespace: { label: "Namespace" },
    outcome: {
      label: "Outcome",
      render: (row, raw) => {
        return <StatusChip raw={raw} />;
      },
    },
    cache_layer: { label: "Cache layer" },
    response_status: { label: "Response" },
    used_at: {
      label: "Used",
      render: renderTime,
    },
    expires_at: {
      label: "Expires",
      render: renderTime,
    },
    payload_artifact_id: {
      label: "Payload artifact",
      render: (row, raw) => {
        if (raw) {
          return <a href={link({ section: "artifacts", artifact_id: raw, artifact_page: 1 })} data-state={JSON.stringify(stateFor({ section: "artifacts", artifact_id: raw, artifact_page: 1 }))}>Artifact {String(raw)}</a>;
        }
        return <span className={classNames.uiFadedText}>None</span>;
      },
    },
    request_fingerprint: { label: "Request fingerprint" },
  };
  const context: DataTableContext = {
    page: page,
    perPage: perPage,
    pageKey: "cache_page",
    perPageKey: "cache_per_page",
    sortKey: "cache_sort",
    orderKey: "cache_order",
    queryKey: "cache_q",
    expandedKey: "cache_expanded",
    query: value("cache_q"),
    itemLabel: "cache uses",
    columnsWhitelist: ["provider", "namespace", "outcome", "cache_layer", "response_status", "used_at", "expires_at", "payload_artifact_id", "request_fingerprint"],
    columnConfig: columnConfig,
  };
  const tableMarkup = <DataTable tableName="Cache uses" result={result} context={context} />;
  const body = (
    <div data-table-scope="Cache uses">
      {controls}
      {tableMarkup}
    </div>
  );
  return <Panel title="Cache use records" description="Recorded provider-response reuse and negative or stale outcomes for this run." body={body} />;
}

/** Returns the effective display status for a work-stage record. */
function stageStatus(summary: StageSummary | undefined, step: RunStep | undefined): string {
  const recorded = String(step?.step_status || "").toLocaleLowerCase();
  if (recorded) return recorded;
  const outcomes = summary?.outcomes || {};
  const names = Object.keys(outcomes).map((item) => {
    return item.toLocaleLowerCase();
  });
  if (names.some((item) => {
    return item.includes("fail") || item.includes("error");
  })) {
    return "failed";
  }
  if (names.length && names.every((item) => {
    return item.includes("skip");
  })) {
    return "skipped";
  }
  if (names.some((item) => {
    return item.includes("discard") || item.includes("warning");
  })) {
    return "warning";
  }
  if (summary) return "completed";
  return "not-recorded";
}

/** Renders ordered stage-flow markup for one work. */
function StageFlow(props: { summaries: StageSummary[]; steps: RunStep[] }): JSX.Element {
  const summaryByName = new Map(props.summaries.map((item) => {
    return [item.stage_name, item];
  }));
  const stepByName = new Map(props.steps.map((item) => {
    return [item.step_name, item];
  }));
  const standard = ["preflight", "load", "parse", "deduplicate", "enrich", "enrich_metadata", "enrich_identity", "validate", "normalize", "finalize"];
  const available = new Set<string>([...summaryByName.keys(), ...stepByName.keys()]);
  const standardNames = standard.filter((name) => {
    return available.delete(name);
  });
  const names = standardNames.concat(Array.from(available).sort());
  if (!names.length) {
    return (
      <div className="rw-empty-inline">
        <strong>No stage progression was recorded.</strong>
        <p>Detailed stage records are also unavailable for this run.</p>
      </div>
    );
  }
  const stageItems = names.map((name, index) => {
    const summary = summaryByName.get(name);
    const step = stepByName.get(name);
    const status = stageStatus(summary, step);
    const outcomes = summary?.outcomes || {};
    const outcomeEntries = Object.entries(outcomes);
    const outcomeText = outcomeEntries.map(([outcome, count]) => {
      return `${formatNumber(count)} ${humanLabel(outcome).toLocaleLowerCase()}`;
    }).join(", ");
    var outcomeFallback = "Stage summary recorded.";
    if (step) outcomeFallback = "Execution step recorded.";
    const outcomeDisplay = outcomeText || outcomeFallback;
    const artifacts: JSX.Element[] = [];
    if (step?.input_artifact_id) {
      artifacts.push(<a href={link({ section: "artifacts", artifact_id: step.input_artifact_id, artifact_page: 1 })} data-state={JSON.stringify(stateFor({ section: "artifacts", artifact_id: step.input_artifact_id, artifact_page: 1 }))}>Input artifact {step.input_artifact_id}</a>);
    }
    if (step?.output_artifact_id) {
      artifacts.push(<a href={link({ section: "artifacts", artifact_id: step.output_artifact_id, artifact_page: 1 })} data-state={JSON.stringify(stateFor({ section: "artifacts", artifact_id: step.output_artifact_id, artifact_page: 1 }))}>Output artifact {step.output_artifact_id}</a>);
    }
    var duration = "Not recorded";
    if (step?.duration_seconds != null) {
      const seconds = Number(step.duration_seconds);
      if (seconds === 0 && step.started_at && step.finished_at) {
        duration = "Less than 1 second";
      } else {
        var unit = " seconds";
        if (seconds === 1) unit = " second";
        duration = `${seconds.toLocaleString(undefined, { maximumFractionDigits: 3 })}${unit}`;
      }
    }
    var records: JSX.Element = <span className={classNames.uiFadedText}>Not applicable</span>;
    if (summary) {
      records = <>{formatNumber(summary.total_records)}</>;
    }
    var artifactMarkup: JSX.Element | null = null;
    if (artifacts.length) {
      artifactMarkup = <div className="rw-stage-step__artifacts">{artifacts}</div>;
    }
    const stepClass = cx("rw-stage-step", stageStatusClasses[status]);
    return (
      <li className={stepClass}>
        <div className="rw-stage-step__marker"><span>{index + 1}</span></div>
        <div>
          <div className="rw-stage-step__heading">
            <h4>{humanLabel(name)}</h4>
            <StatusChip raw={humanLabel(status)} />
          </div>
          <p>{outcomeDisplay}</p>
          <dl>
            <div>
              <dt>Work outcome records</dt>
              <dd>{records}</dd>
            </div>
            <div>
              <dt>Duration</dt>
              <dd>{duration}</dd>
            </div>
          </dl>
          {artifactMarkup}
        </div>
      </li>
    );
  });
  return (
    <ol className="rw-stage-flow">{stageItems}</ol>
  );
}

/** Renders work-stage evidence and pagination markup. */
function StagesView(props: { data: StagesResponse }): JSX.Element {
  const rows = list<WireRecord>(props.data, ["rows"]);
  const summaries = list<StageSummary>(props.data, ["stage_summaries"]);
  const steps = list<RunStep>(props.data, ["run_steps"]);
  const page = Math.max(1, Number(value("stage_page") || props.data.pagination?.page || 1));
  const perPage = Number(value("stage_per_page") || props.data.pagination?.per_page || 50);
  const result = {
    columns: list(props.data, ["columns"]),
    rows: rows,
    pagination: props.data.pagination || {
      page: page,
      per_page: perPage,
      total_rows: rows.length,
    },
  };
  var stageFilterSummary: JSX.Element | null = null;
  if (value("stage_q")) {
    stageFilterSummary = <FilterChips filters={{ stage_q: value("stage_q") }} labels={{ stage_q: "Search" }} options={{
      removeUpdates: { stage_page: 1 },
      clearUpdates: { stage_q: "", stage_page: 1 },
    }} />;
  }
  const controls = (
    <form className={classNames.uiFormRwFilterBar} id="stage-controls">
      <label className="rw-filter-bar__search">
        Search detailed outcomes
        <input id="stage-query" type="search" value={value("stage_q")} placeholder="Stage, outcome, reason, or work ID" />
      </label>
      <label>
        Rows per page
        <select id="stage-per-page"><PageSizeOptions current={perPage} /></select>
      </label>
      <button type="submit" className={classNames.uiPrimaryButton} data-stage-search>Search</button>
      {stageFilterSummary}
    </form>
  );
  const columnConfig: NonNullable<DataTableContext["columnConfig"]> = {
    work_id: { label: "Work" },
    stage_name: { label: "Stage" },
    outcome: {
      label: "Outcome",
      render: (row, raw) => {
        return <StatusChip raw={raw} />;
      },
    },
    reason: { label: "Reason" },
    created_at: {
      label: "Recorded",
      render: renderTime,
    },
    updated_at: {
      label: "Updated",
      render: renderTime,
    },
  };
  const context: DataTableContext = {
    page: page,
    perPage: perPage,
    query: value("stage_q"),
    itemLabel: "stage outcomes",
    pageKey: "stage_page",
    perPageKey: "stage_per_page",
    sortKey: "stage_sort",
    orderKey: "stage_order",
    queryKey: "stage_q",
    expandedKey: "stage_expanded",
    columnsWhitelist: ["work_id", "stage_name", "outcome", "reason", "created_at", "updated_at"],
    columnConfig: columnConfig,
  };
  const details = <DataTable tableName="Detailed stage outcomes" result={result} context={context} />;
  const body = (
    <div data-table-scope="Detailed stage outcomes">
      {controls}
      {details}
    </div>
  );
  return (
    <Fragment>
      <section className={classNames.uiSegment}>
        <div className={classNames.uiTopAttachedHeader}>
          <div>
            <h3>Stage outcomes and progression</h3>
            <p>This view explains what happened during the run. It does not provide pipeline controls.</p>
          </div>
        </div>
        <p className={classNames.uiInfoMessage}>Counts describe stored per-work outcome rows, not raw input records or field updates. A stage without per-work evidence is marked not applicable. Durations use recorded step timestamps; steps inside one timestamp tick are shown as less than one second.</p>
        <div className="content"><StageFlow summaries={summaries} steps={steps} /></div>
      </section>
      <Panel title="Detailed stage outcomes" description="Per-work evidence remains available below the run-level progression." body={body} />
    </Fragment>
  );
}

/** Renders stored run details and exact configuration links. */
function RunView(props: { artifactData: ArtifactsResponse }): JSX.Element {
  const run = selectedRun();
  const plan = state.plans.find((item) => {
    return String(pickID(item)) === String(run?.execution_plan_id || value("plan_id"));
  });
  const snapshots = list<ArtifactRecord>(props.artifactData, ["artifacts"]);
  const snapshotArtifacts = snapshots.filter((artifact) => {
    return artifact.artifact_roles;
  });
  var started: Date | null = null;
  if (run?.started_at) started = new Date(run.started_at);
  var finished: Date | null = null;
  if (run?.finished_at) finished = new Date(run.finished_at);
  var duration = "Not recorded";
  if (started && finished && !Number.isNaN(started.getTime()) && !Number.isNaN(finished.getTime())) {
    duration = `${Math.max(0, (finished.getTime() - started.getTime()) / 1000).toLocaleString()} seconds`;
  }
  const summary = (
    <dl className="rw-summary-strip">
      <div>
        <dt>Run attempt</dt>
        <dd>{run?.attempt_number || value("run_id")}</dd>
      </div>
      <div>
        <dt>Status</dt>
        <dd><StatusChip raw={run?.status} /></dd>
      </div>
      <div>
        <dt>Started</dt>
        <dd>{formatTime(run?.started_at)}</dd>
      </div>
      <div>
        <dt>Duration</dt>
        <dd>{duration}</dd>
      </div>
    </dl>
  );
  const fields: Record<string, unknown> = {
    run_id: run?.id || value("run_id"),
    execution_plan_id: run?.execution_plan_id || value("plan_id"),
    attempt_number: run?.attempt_number,
    status: run?.status,
    visibility: run?.visibility_state,
    started_at: run?.started_at,
    finished_at: run?.finished_at,
    summary: run && "summary" in run ? run.summary : undefined,
    execution_fingerprint: plan?.execution_fingerprint,
    resolved_manifest_hash: plan?.resolved_manifest_hash,
    input_manifest_hash: plan?.input_manifest_hash,
    enrichment_enabled: plan?.enrichment_enabled,
  };
  const propertyItems = Object.entries(fields).map(([field, fieldValue]) => {
    return (
      <div>
        <dt>{humanLabel(field)}</dt>
        <dd><Cell item={fieldValue} column={field} /></dd>
      </div>
    );
  });
  const properties = (
    <dl className={classNames.rwPropertyGridRwPropertyGridCompact}>{propertyItems}</dl>
  );
  var snapshotRows: JSX.Element = <p className={classNames.uiFadedText}>No configuration snapshots were recorded for this legacy run.</p>;
  if (snapshotArtifacts.length) {
    const snapshotItems = snapshotArtifacts.map((artifact) => {
      var action: JSX.Element = <span className={classNames.uiLabel}>Payload unavailable</span>;
      if (artifact.has_blob) {
        action = <a className={classNames.uiBasicButton} href={`/api/artifacts/${encodeURIComponent(String(artifact.id))}/content`} download>Download</a>;
      }
      return (
        <li>
          <div>
            <strong>{humanLabel(artifact.artifact_roles)}</strong>
            <span>{formatBytes(artifact.byte_size)} / {String(artifact.content_type)}</span>
          </div>
          {action}
        </li>
      );
    });
    snapshotRows = <ul className="rw-file-list">{snapshotItems}</ul>;
  }
  return (
    <Fragment>
      {summary}
      <Panel title="Stored run attempt" description="Immutable execution identity, lifecycle, and plan fingerprints." body={properties} />
      <Panel title="Configuration snapshots" description="Exact workspace configuration, resolved manifest, and input manifest captured for this attempt." body={snapshotRows} />
    </Fragment>
  );
}

/** Asynchronously implements provenance view for the viewer. */
export async function provenanceView(): Promise<void> {
  const requestedSection = section("section", "audit");
  var current = "audit";
  if (provenanceSections[requestedSection]) current = requestedSection;
  const [title] = provenanceSections[current];
  const navItems: Array<[string, string]> = Object.entries(provenanceSections).map(([id, [label]]) => {
    return [id, label];
  });
  if (!value("run_id") && current !== "audit") {
    const focusContextAction = <button type="button" className={classNames.uiButton} data-focus-context>Focus context selector</button>;
    const emptyStateMarkup = (
      <Fragment>
        <PageHeader kicker="Execution evidence" title="Provenance" description="Inspect append-only evidence without changing the workspace." />
        <Subnav items={navItems} current={current} key="section" />
        <EmptyPanel title={title} detail="Select a run attempt to inspect this provenance view." action={focusContextAction} />
      </Fragment>
    );
    renderTree(emptyStateMarkup, app);
    bindFocusContext();
    return;
  }

  var content: JSX.Element;
  if (current === "audit") {
    content = <AuditView data={await api<AuditResponse>("/api/audit", auditQuery(""), { method: "GET", headers: { Accept: "application/json" } })} />;
  } else if (current === "artifacts") {
    content = <ArtifactsView data={await api<ArtifactsResponse>(`/api/runs/${encodeURIComponent(value("run_id"))}/artifacts`, {
      q: value("artifact_q"),
      role: value("artifact_role"),
      page: value("artifact_page") || 1,
      per_page: value("artifact_per_page") || 50,
      artifact_id: value("artifact_id"),
    }, { method: "GET", headers: { Accept: "application/json" } })} />;
  } else if (current === "cache") {
    content = <CacheView data={await api<CacheUsesResponse>(`/api/runs/${encodeURIComponent(value("run_id"))}/cache-uses`, {
      page: value("cache_page") || 1,
      per_page: value("cache_per_page") || 50,
      sort: value("cache_sort") || "id",
      order: value("cache_order") || "asc",
      q: value("cache_q"),
    }, { method: "GET", headers: { Accept: "application/json" } })} />;
  } else if (current === "stages") {
    content = <StagesView data={await api<StagesResponse>(`/api/runs/${encodeURIComponent(value("run_id"))}/stages`, {
      page: value("stage_page") || 1,
      per_page: value("stage_per_page") || 50,
      sort: value("stage_sort") || "id",
      order: value("stage_order") || "asc",
      q: value("stage_q"),
    }, { method: "GET", headers: { Accept: "application/json" } })} />;
  } else {
    content = <RunView artifactData={await api<ArtifactsResponse>(`/api/runs/${encodeURIComponent(value("run_id"))}/artifacts`, { role: "run_role", limit: 100 }, { method: "GET", headers: { Accept: "application/json" } })} />;
  }

  const pageMarkup = (
    <Fragment>
      <PageHeader kicker="Execution evidence" title="Provenance" description="Inspect append-only audit events, artifacts, cache decisions, and stage outcomes without changing the selected run." />
      <Subnav items={navItems} current={current} key="section" />
      {content}
    </Fragment>
  );
  renderTree(pageMarkup, app);

  if (current === "audit") {
    bindAuditControls();
  } else if (current === "artifacts") {
    bindArtifactInspection();
  } else if (current === "cache") {
    bindTableControls("Cache uses", Number(value("cache_page") || 1), {
      pageKey: "cache_page",
      perPageKey: "cache_per_page",
      sortKey: "cache_sort",
      orderKey: "cache_order",
      queryKey: "cache_q",
      expandedKey: "cache_expanded",
      querySelector: "#cache-query",
      perPageSelector: "#cache-per-page",
      searchButtonSelector: "[data-cache-search]",
    });
  } else if (current === "stages") {
    bindTableControls("Detailed stage outcomes", Number(value("stage_page") || 1), {
      pageKey: "stage_page",
      perPageKey: "stage_per_page",
      sortKey: "stage_sort",
      orderKey: "stage_order",
      queryKey: "stage_q",
      expandedKey: "stage_expanded",
      querySelector: "#stage-query",
      perPageSelector: "#stage-per-page",
      searchButtonSelector: "[data-stage-search]",
    });
  }
}

/** Binds DOM behavior for audit controls. */
function bindAuditControls(): void {
  bindAuditRecordedData();
  const form = document.querySelector<HTMLFormElement>("#audit-filter-form");
  if (form) {
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      const values = new FormData(form);
      const updates: Record<string, unknown> = {};
      const multiValueKeys = new Set(["audit_category", "audit_action", "audit_actor", "audit_entity"]);
      const filterKeys = Object.keys(auditFilterKeys);
      filterKeys.forEach((key) => {
        var keyValue = values.get(key) || "";
        if (multiValueKeys.has(key)) keyValue = values.getAll(key).join(",");
        updates[key] = keyValue;
      });
      setURL(updates, false);
    });
    const multiSelectControls = form.querySelectorAll("[data-multi-select]");
    multiSelectControls.forEach((control) => {
      const checkboxes = Array.from(control.querySelectorAll<HTMLInputElement>("input[type=\"checkbox\"]"));
      const refresh = () => {
        const count = checkboxes.filter((input) => {
          return input.checked;
        }).length;
        var summaryText = "Any";
        if (count) summaryText = `${count} selected`;
        (control.querySelector("summary strong") as HTMLElement).textContent = summaryText;
      };
      (control.querySelector("[data-multi-select-all]") as HTMLButtonElement).addEventListener("click", () => {
        checkboxes.forEach((input) => {
          input.checked = true;
        });
        refresh();
      });
      (control.querySelector("[data-multi-select-clear]") as HTMLButtonElement).addEventListener("click", () => {
        checkboxes.forEach((input) => {
          input.checked = false;
        });
        refresh();
      });
      checkboxes.forEach((input) => {
        input.addEventListener("change", refresh);
      });
      control.addEventListener("toggle", () => {
        if ((control as HTMLDetailsElement).open) {
          const openControls = form.querySelectorAll<HTMLDetailsElement>("[data-multi-select][open]");
          openControls.forEach((other) => {
            if (other !== control) other.open = false;
          });
        }
      });
    });
  }
  const button = document.querySelector<HTMLButtonElement>("[data-audit-load-more]");
  if (button) {
    button.addEventListener("click", async () => {
      const stream = document.querySelector<HTMLElement>("#audit-event-stream")!;
      const pageStatus = document.querySelector<HTMLElement>("[data-audit-page-status]")!;
      const pageError = document.querySelector<HTMLElement>("[data-audit-page-error]")!;
      pageError.hidden = true;
      pageError.textContent = "";
      stream.setAttribute("aria-busy", "true");
      button.disabled = true;
      classAdd(button, ["loading"]);
      try {
        const data = await api<AuditResponse>("/api/audit", auditQuery(auditCursor), {
          method: "GET",
          headers: { Accept: "application/json" },
        });
        const newEvents = list(data, ["events", "items"]).filter((event) => {
          const id = String(event.id);
          if (auditKnownEventIDs.has(id)) return false;
          auditKnownEventIDs.add(id);
          return true;
        });
        auditLoadedCount += newEvents.length;
        auditCursor = String(data.next_cursor || "");
        auditHasMore = Boolean(data.has_more);
        appendAuditEvents(stream, newEvents as AuditEventRecord[]);
        bindAuditRecordedData(stream);
        const removed = boundAuditWindow(stream);
        const visible = stream.querySelectorAll(".rw-audit-event").length;
        const windowStatus = document.querySelector<HTMLElement>("[data-audit-window-status]")!;
        windowStatus.hidden = removed === 0;
        if (removed) windowStatus.textContent = `${formatNumber(visible)} events remain visible to keep this long timeline responsive. Reset or change the filters to return to the newest matching evidence.`;
        (document.querySelector("[data-audit-loaded-count]") as HTMLElement).textContent = formatNumber(auditLoadedCount);
        pageStatus.textContent = `${formatNumber(auditLoadedCount)} events loaded; ${formatNumber(visible)} currently visible.`;
        (document.querySelector(".rw-load-more") as HTMLElement).hidden = !auditHasMore;
        (document.querySelector("[data-audit-end]") as HTMLElement).hidden = auditHasMore;
      } catch (error) {
        pageError.textContent = errorMessage(error, "Unable to load older audit events.");
        pageError.hidden = false;
        pageStatus.textContent = "Older events were not loaded. The current timeline is unchanged.";
      } finally {
        stream.setAttribute("aria-busy", "false");
        button.disabled = false;
        classRemove(button, "loading");
      }
    });
  }
}

/** Binds DOM behavior for artifact inspection. */
function bindArtifactInspection(): void {
  document.querySelector<HTMLFormElement>("[data-artifact-filters]")?.addEventListener("submit", (event) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget as HTMLFormElement);
    setURL({
      artifact_q: form.get("artifact_q"),
      artifact_role: form.get("artifact_role"),
      artifact_per_page: form.get("artifact_per_page"),
      artifact_page: 1,
      artifact_id: "",
    }, false);
  });
  document.querySelectorAll<HTMLButtonElement>("[data-artifact-page]").forEach((button) => {
    button.addEventListener("click", () => {
      setURL({
        artifact_page: button.dataset.artifactPage,
        artifact_id: "",
      }, false);
    });
  });
  const inspectButtons = document.querySelectorAll<HTMLButtonElement>("[data-inspect-artifact]");
  inspectButtons.forEach((button) => {
    button.addEventListener("click", async (event) => {
      const sequence = ++artifactInspectionSequence;
      const explicitSelection = event.isTrusted;
      const id = button.dataset.inspectArtifact as string;
      replaceState({ artifact_id: id });
      activeArtifactRow = button.closest("tr");
      const artifactRows = document.querySelectorAll<HTMLElement>("[data-artifact-row]");
      artifactRows.forEach((row) => {
        classToggle(row, "selected", row === activeArtifactRow);
      });
      const previous = button.textContent;
      button.disabled = true;
      classAdd(button, ["loading"]);
      try {
        const payload = await api<ArtifactInspectionResponse>(`/api/artifacts/${encodeURIComponent(id)}/inspect`, { preview_bytes: 65536 }, {
          method: "GET",
          headers: { Accept: "application/json" },
        });
        if (sequence !== artifactInspectionSequence) return;
        const raw = payload.content || "";
        var formatted = raw;
        var formatError = false;
        if (payload.format === "json") {
          try {
            formatted = JSON.stringify(JSON.parse(raw), null, 2);
          } catch (_) {
            formatted = raw;
            formatError = true;
          }
        }
        var previewMode: ArtifactPreviewState["mode"] = "raw";
        if (payload.format === "json" && !formatError) previewMode = "formatted";
        activeArtifactPreview = {
          ...payload,
          raw: raw,
          formatted: formatted,
          formatError: formatError,
          mode: previewMode,
          wrap: false,
        };
        renderArtifactInspector();
        if (explicitSelection) {
          const title = document.querySelector<HTMLElement>("[data-artifact-inspector-title]");
          title?.scrollIntoView?.({ behavior: "smooth", block: "center" });
          title?.focus({ preventScroll: true });
        }
      } catch (error) {
        if (sequence !== artifactInspectionSequence) return;
        const errorMarkup = (
          <Fragment>
            <div className={classNames.uiErrorMessage}>
              <div className="header">Preview unavailable</div>
              <p>{errorMessage(error, "Unable to inspect this artifact.")}</p>
            </div>
            <a className={classNames.uiButton} href={`/api/artifacts/${encodeURIComponent(id)}/content`} download>Download original</a>
          </Fragment>
        );
        renderTree(errorMarkup, document.querySelector("#artifact-inspector .content") as HTMLElement);
      } finally {
        button.disabled = false;
        classRemove(button, "loading");
        button.textContent = previous;
      }
    });
  });
  const focusedID = value("artifact_id");
  if (focusedID) {
    const focusedRow = Array.from(document.querySelectorAll<HTMLElement>("[data-artifact-row]")).find((row) => {
      return row.dataset.artifactRow === focusedID;
    });
    focusedRow?.focus();
    focusedRow?.scrollIntoView?.({ behavior: "smooth", block: "center" });
    const focusedButton = Array.from(inspectButtons).find((button) => {
      return button.dataset.inspectArtifact === focusedID;
    });
    if (focusedButton) {
      focusedButton.click();
    }
  }
}

/** Renders artifact inspector. */
function renderArtifactInspector(): void {
  const preview = activeArtifactPreview;
  if (!preview) {
    return;
  }
  var shown = preview.raw;
  if (preview.mode === "formatted") shown = preview.formatted;
  const canFormat = preview.format === "json" && preview.formatted !== preview.raw;
  const id = preview.artifact_id;
  var truncation: JSX.Element = <p className={classNames.uiSuccessMessage}>The complete stored text fits within the safe preview limit.</p>;
  if (preview.truncated) {
    truncation = (
      <div className={classNames.uiWarningMessage}>
        <div className="header">Preview truncated</div>
        <p>This page shows the first {formatBytes(preview.preview_byte_size)} of {formatBytes(preview.stored_byte_size || preview.byte_size)}. Download the original file to inspect its complete contents.</p>
      </div>
    );
  }
  var modeButtons: JSX.Element | null = null;
  if (canFormat) {
    modeButtons = (
      <Fragment>
        <button type="button" className={classNames.uiBasicButton} data-artifact-preview-mode="formatted" disabled={preview.mode === "formatted"}>Formatted JSON</button>
        <button type="button" className={classNames.uiBasicButton} data-artifact-preview-mode="raw" disabled={preview.mode === "raw"}>Raw text</button>
      </Fragment>
    );
  }
  var formatNotice: JSX.Element | null = null;
  if (preview.formatError && preview.truncated) {
    formatNotice = <p className={classNames.uiInfoMessage}>Formatted JSON is unavailable because this bounded prefix does not contain the complete document. Raw text is shown.</p>;
  } else if (preview.formatError) {
    formatNotice = <p className={classNames.uiWarningMessage}>The stored content is not valid JSON. Raw text is shown; download the original file for independent inspection.</p>;
  }
  var wrapLabel = "Wrap long lines";
  if (preview.wrap) wrapLabel = "Disable line wrapping";
  const previewClass = cx("rw-artifact-preview", preview.wrap && "rw-artifact-preview--wrap");
  const inspectorMarkup = (
    <Fragment>
      {truncation}
      {formatNotice}
      <dl className="rw-summary-strip">
        <div>
          <dt>Original size</dt>
          <dd>{formatBytes(preview.byte_size)}</dd>
        </div>
        <div>
          <dt>Stored size</dt>
          <dd>{formatBytes(preview.stored_byte_size)}</dd>
        </div>
        <div>
          <dt>Bytes shown</dt>
          <dd>{formatBytes(preview.preview_byte_size)}</dd>
        </div>
        <div>
          <dt>Content type</dt>
          <dd>{preview.content_type}</dd>
        </div>
      </dl>
      <div className="artifact-inspector-toolbar">
        {modeButtons}
        <button type="button" className={classNames.uiBasicButton} data-toggle-artifact-wrap>{wrapLabel}</button>
        <button type="button" className={classNames.uiBasicButton} data-copy-artifact-preview>Copy displayed text</button>
        <a className={classNames.uiPrimaryButton} href={`/api/artifacts/${encodeURIComponent(id)}/content`} download>Download original</a>
      </div>
      <pre className={previewClass}>{shown}</pre>
      <p className={classNames.uiFadedText} data-artifact-copy-status></p>
    </Fragment>
  );
  renderTree(inspectorMarkup, document.querySelector("#artifact-inspector .content") as HTMLElement);
  const previewModeButtons = document.querySelectorAll<HTMLButtonElement>("[data-artifact-preview-mode]");
  previewModeButtons.forEach((button) => {
    button.addEventListener("click", () => {
      preview.mode = button.dataset.artifactPreviewMode === "formatted" ? "formatted" : "raw";
      renderArtifactInspector();
    });
  });
  document.querySelector("[data-toggle-artifact-wrap]")!.addEventListener("click", () => {
    preview.wrap = !preview.wrap;
    renderArtifactInspector();
  });
  document.querySelector("[data-copy-artifact-preview]")!.addEventListener("click", async () => {
    const status = document.querySelector("[data-artifact-copy-status]") as HTMLElement;
    try {
      await copyArtifactText(shown);
      status.textContent = "Displayed preview copied to the clipboard.";
    } catch (_) {
      status.textContent = "Copy failed. Select the displayed text manually.";
    }
  });
}

/** Asynchronously copies artifact text. */
async function copyArtifactText(text: string): Promise<void> {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    return navigator.clipboard.writeText(text);
  }
  const area = document.createElement("textarea");
  area.value = text;
  area.setAttribute("readonly", "");
  area.style.position = "fixed";
  area.style.opacity = "0";
  document.body.append(area);
  area.select();
  const copied = document.execCommand("copy");
  area.remove();
  if (!copied) throw new Error("Clipboard unavailable");
}
