// Shared state, DOM references, and utility functions.
// This module is imported by every other module. It avoids circular dependencies
// by importing only the leaf JSX runtime.
import { h, Fragment, render as renderTree, cx, classAdd } from "./jsx/jsx-runtime.ts";
import type { ClassName } from "./jsx/classes.ts";
import type {
  HierarchyPlan,
  HierarchyAttempt,
  HierarchyRun,
  HierarchySearch,
  MetricEvidence,
  MetricValue,
  OverviewResponse,
  SourceFilterCount,
  SourceResultCount,
  TableInfo,
  WireRecord,
} from "./api/types.ts";

/** Typed compound class names used by this module. */
const classNames = {
  sectionCurrent: cx("section", "current"),
  uiBasicSegment: cx("ui", "basic", "segment"),
  uiButton: cx("ui", "button"),
  uiFadedText: cx("ui", "faded", "text"),
  uiFeed: cx("ui", "feed"),
  uiLabel: cx("ui", "label"),
  uiNegativeText: cx("ui", "negative", "text"),
  uiProgress: cx("ui", "progress"),
  uiSegmentRwEmptyState: cx("ui", "segment", "rw-empty-state"),
  uiStatisticRwKpi: cx("ui", "statistic", "rw-kpi"),
  uiStepsRwFlow: cx("ui", "steps", "rw-flow"),
  uiStepsRwFlowRwFlowSource: cx("ui", "steps", "rw-flow", "rw-flow--source"),
  uiTable: cx("ui", "table"),
  uiTabularMenu: cx("ui", "tabular", "menu"),
  uiTopAttachedHeader: cx("ui", "top", "attached", "header"),
};

// The viewer shell guarantees these elements exist (index.html), so the DOM
// roots are declared with non-null assertions at module scope.
export const app = document.querySelector<HTMLElement>("#app")!;
export const notice = document.querySelector<HTMLElement>("#notice")!;
export const loading = document.querySelector<HTMLElement>("#loading")!;
export const breadcrumbHost = document.querySelector<HTMLElement>("#workspace-breadcrumb")!;

/** The shared viewer state: discovered context options, tables, and request lifecycle. */
export interface ViewerState {
  searches: HierarchySearch[];
  plans: HierarchyPlan[];
  runs: Array<HierarchyRun | HierarchyAttempt>;
  tables: TableInfo[];
  request: number;
  controller: AbortController | null;
}

export const state: ViewerState = {
  searches: [],
  plans: [],
  runs: [],
  tables: [],
  request: 0,
  controller: null,
};

export const pageSizes = [20, 50, 100, 200, 500];
export const corpusSections: Record<string, { table: string; title: string; description: string }> = {
  articles: {
    table: "work_revisions",
    title: "Analysis-ready articles",
    description: "Valid normalized work revisions captured by the selected run.",
  },
  authors: {
    table: "author_occurrences",
    title: "Author occurrences",
    description: "Observed author records. Matching names do not imply the same person.",
  },
  references: {
    table: "reference_mentions",
    title: "Reference mentions",
    description: "Ordered citation mentions retained for each article revision.",
  },
  identity_evidence: {
    table: "author_identity_resolutions",
    title: "Author identity / ORCID evidence",
    description: "Name-search results are review candidates, never confirmed author identities.",
  },
  sources: {
    table: "source_records",
    title: "Source records",
    description: "Captured input records and parse outcomes.",
  },
};

export const provenanceSections: Record<string, [string, string]> = {
  audit: ["Audit timeline", "Recorded actions for the selected historical run."],
  artifacts: ["Artifacts", "Inspect and download locally stored artifacts for the selected run."],
  cache: ["Cache uses", "Provider response and computation reuse recorded for this run."],
  stages: ["Stage outcomes", "Per-work outcomes recorded for parse, deduplication, validation, enrichment, and normalization."],
  run: ["Run details", "The stored execution attempt and its read-only plan context."],
};

export const graphFilters = ["mode", "q", "author", "orcid", "reference", "source", "year_min", "year_max", "citation_min", "citation_max", "reference_min", "reference_max", "article_limit"];

/** The intentionally English, UTC presentation policy used by every recorded timestamp. */
export const interfaceLocale = "en-US";
export const interfaceTimeZone = "UTC";
const dateTimeFormatter = new Intl.DateTimeFormat(interfaceLocale, {
  dateStyle: "medium",
  timeStyle: "medium",
  timeZone: interfaceTimeZone,
});
const dateFormatter = new Intl.DateTimeFormat(interfaceLocale, {
  dateStyle: "medium",
  timeZone: interfaceTimeZone,
});

/** The complete ancestry keys shared by Deepdive routes. */
export const canonicalContextKeys = ["search_id", "search_revision_id", "plan_id", "run_id"];

/** URL keys owned by each route in addition to canonical research context. */
export const routeOwnedKeys: Record<string, string[]> = {
  home: ["home_q", "home_visibility", "home_status", "home_started_after", "home_started_before", "home_search_cursor", "home_run_cursor"],
  trash: [],
  overview: [],
  corpus: ["section", "q", "sort", "order", "page", "per_page", "expanded"],
  relationships: [...graphFilters, "node"],
  provenance: [
    "section", "artifact_id", "artifact_q", "artifact_role", "artifact_page", "artifact_per_page",
    "audit_q", "audit_category", "audit_action", "audit_actor", "audit_entity", "audit_stage", "audit_outcome",
    "audit_review_status", "audit_review_reason", "audit_review_substatus",
    "cache_q", "cache_page", "cache_per_page", "cache_sort", "cache_order", "cache_expanded", "cache_layer",
    "stage_q", "stage_page", "stage_per_page", "stage_sort", "stage_order", "stage_expanded",
  ],
  evaluation: ["q", "sort", "order", "page", "per_page", "pdf_status", "review_status", "review_source", "qualifier", "source", "reviewed"],
  advanced: ["table", "q", "sort", "order", "page", "per_page", "expanded"],
  article: ["article_id", "note_id", "anchor_id", "pdf_page", "origin", "detail_authors_cursor", "detail_references_cursor", "detail_stages_cursor", "detail_audit_cursor"],
  author: ["author_id", "origin", "detail_articles_cursor", "detail_identity_cursor", "detail_audit_cursor"],
  reference: ["reference_id", "origin"],
};

/** Maps every supported view to its clean extensionless application path. */
export const viewPage: Record<string, string> = {
  home: "/",
  trash: "/trash",
  overview: "/overview",
  corpus: "/corpus",
  relationships: "/relationships",
  provenance: "/provenance",
  evaluation: "/evaluation",
  advanced: "/advanced",
  article: "/article",
  author: "/author",
  reference: "/reference",
};

const detailOriginViews = new Set(["evaluation", "corpus", "relationships", "provenance"]);
const detailOriginLabels: Record<string, string> = {
  evaluation: "Evaluation queue",
  corpus: "Corpus",
  relationships: "Relationships",
  provenance: "Provenance",
};

/** One validated origin route used to return from a detail record. */
export interface DetailOrigin {
  view: string;
  label: string;
  href: string;
  params: URLSearchParams;
  state: Record<string, string>;
}

/** The single source of truth for viewer route state, mirrored to sessionStorage and history.state. */
export let viewerState: Record<string, string> = {};

/** The sessionStorage key that mirrors viewerState across full page loads. */
const viewerStateKey = "rw-viewer-state";

/** Derives the view from a pathname: strips slashes and a trailing .html suffix. */
export function pathView(pathname?: string): string {
  const raw = pathname === undefined ? location.pathname : pathname;
  var viewName = raw.replace(/^\/+/, "").replace(/\/+$/, "");
  if (viewName.endsWith(".html")) viewName = viewName.slice(0, -5);
  if (!viewName) return "home";
  return viewName;
}

/** Validates an unknown value as a plain string-valued state object. */
export function isStateObject(raw: unknown): raw is Record<string, string> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return false;
  return Object.values(raw).every((item) => typeof item === "string");
}

/** Persists the current viewerState to sessionStorage when storage is available. */
export function saveState(): void {
  try {
    sessionStorage.setItem(viewerStateKey, JSON.stringify(viewerState));
  } catch (_) {
    // Storage may be unavailable; history.state still carries the state.
  }
}

/** Reads and validates the persisted viewerState, returning null when absent or invalid. */
export function loadState(): Record<string, string> | null {
  try {
    const raw = sessionStorage.getItem(viewerStateKey);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (!isStateObject(parsed)) return null;
    return parsed;
  } catch (_) {
    return null;
  }
}

/** Assigns the viewerState and mirrors it to sessionStorage. */
export function restoreState(state: Record<string, string>): void {
  viewerState = state;
  saveState();
}

/** Returns the clean application path for one state object's view. */
export function pathFor(state: Record<string, string>): string {
  return viewPage[state.view || "home"] || viewPage.home;
}

/** Returns the current URL search parameters backed by the viewerState. */
export function params(): URLSearchParams {
  return new URLSearchParams(viewerState);
}

/** Returns a named URL parameter or an empty string. */
export function value(name: string): string {
  return params().get(name) || "";
}

/** Returns the selected viewer view. */
export function view(): string {
  return viewerState.view || pathView();
}

/** Returns a named section parameter or its fallback. */
export function section(name: string, fallback: string): string {
  return value(name) || fallback;
}

/** Builds the filtered destination state from updates applied to the current viewerState. */
export function stateFor(updates?: Record<string, unknown>): Record<string, string> {
  if (!updates) updates = {};
  const next: Record<string, string> = { ...viewerState };
  const previousArticleID = next.article_id || "";
  Object.entries(updates).forEach(([key, raw]) => {
    if (raw === "" || raw === null || raw === undefined) {
      delete next[key];
    } else {
      next[key] = String(raw);
    }
  });
  if (!next.view) {
    next.view = "home";
  }

  const destination = next.view;
  if (destination === "article" && Object.hasOwn(updates, "article_id") && next.article_id !== previousArticleID) {
    delete next.note_id;
    delete next.anchor_id;
    delete next.pdf_page;
    delete next.detail_authors_cursor;
    delete next.detail_references_cursor;
    delete next.detail_stages_cursor;
    delete next.detail_audit_cursor;
  }
  const allowed = new Set<string>(["view", ...(routeOwnedKeys[destination] || [])]);
  if (destination !== "home" && destination !== "trash") {
    canonicalContextKeys.forEach((key) => {
      allowed.add(key);
    });
  }
  for (const key of Object.keys(next)) {
    if (!allowed.has(key)) delete next[key];
  }
  return next;
}

/** Builds an internal path-only URL from canonical context and destination-owned state only. */
export function link(updates?: Record<string, unknown>): string {
  return pathFor(stateFor(updates));
}

/** Returns the filtered destination state object for anchor state carrying. */
export function linkState(updates?: Record<string, unknown>): Record<string, string> {
  return stateFor(updates);
}

/** Adopts persisted state at boot, corrects the view from the pathname, and attaches it to the initial entry. */
export function initViewerState(): void {
  const adopted = isStateObject(history.state) ? history.state : loadState() || {};
  viewerState = adopted;
  viewerState = stateFor({ view: pathView() });
  saveState();
  history.replaceState(viewerState, "", pathFor(viewerState));
}

/** Serializes the current supported collection route for use by a detail link. */
export function currentDetailOrigin(): string {
  const currentView = view();
  if (["article", "author", "reference"].includes(currentView)) {
    return value("origin");
  }
  if (!detailOriginViews.has(currentView)) return "";
  const current = params();
  const allowed = new Set(["view", ...canonicalContextKeys, ...(routeOwnedKeys[currentView] || [])]);
  Array.from(current.keys()).forEach((key) => {
    if (!allowed.has(key)) current.delete(key);
  });
  return current.toString();
}

/** Validates the stored detail origin against route ownership and visible canonical context. */
export function detailOrigin(): DetailOrigin | null {
  const raw = value("origin");
  if (!raw) return null;
  const origin = new URLSearchParams(raw);
  const originView = origin.get("view") || "";
  if (!detailOriginViews.has(originView)) return null;
  for (const key of canonicalContextKeys) {
    if ((origin.get(key) || "") !== value(key)) return null;
  }
  const allowed = new Set(["view", ...canonicalContextKeys, ...(routeOwnedKeys[originView] || [])]);
  Array.from(origin.keys()).forEach((key) => {
    if (!allowed.has(key)) origin.delete(key);
  });
  const state = Object.fromEntries(origin);
  return {
    view: originView,
    label: detailOriginLabels[originView],
    href: pathFor(state),
    params: origin,
    state: state,
  };
}

/** Escapes a value for safe HTML text insertion. */
export function esc(raw: unknown): string {
  const element = document.createElement("span");
  element.textContent = raw == null ? "" : String(raw);
  return element.innerHTML;
}

/** Formats a value for JSON-oriented display. */
export function asJSON(item: unknown): string {
  if (typeof item === "string") return item;
  return JSON.stringify(item, null, 2);
}

/** Returns the first matching array in an API response. */
export function list<T = WireRecord>(data: unknown, keys?: string[]): T[] {
  const record = data as WireRecord | null;
  if (keys) {
    for (const key of keys) {
      if (Array.isArray(record?.[key])) return record[key] as T[];
    }
  }
  if (Array.isArray(data)) return data as T[];
  return [];
}

/** Returns the first supported identifier present on an item. */
export function pickID(item: object | null | undefined): unknown {
  const record = item as WireRecord | null | undefined;
  if (record?.id)        return record.id;
  if (record?.search_id) return record.search_id;
  if (record?.run_id)    return record.run_id;
  if (record?.plan_id)   return record.plan_id;
  return "";
}

/** Returns the first non-empty display field on an item. */
export function text(item: object | null | undefined, fields: string[]): string {
  const record = item as WireRecord | null | undefined;
  for (const field of fields) {
    if (record?.[field] !== undefined && record?.[field] !== null && record[field] !== "") {
      return String(record[field]);
    }
  }
  return "Unnamed";
}

/** Classifies numeric evidence without conflating missing or malformed values with recorded zero. */
export function numericEvidence(raw: unknown): { state: "recorded" | "derived" | "unavailable" | "invalid"; value: number | null } {
  const evidence = raw as Partial<MetricEvidence> | null;
  if (evidence?.state === "invalid") return { state: "invalid", value: null };
  if (evidence?.available === false || raw == null || raw === "") return { state: "unavailable", value: null };
  const parsed = Number(evidence?.value ?? raw);
  if (!Number.isFinite(parsed)) return { state: "invalid", value: null };
  if (evidence?.state === "derived") return { state: "derived", value: parsed };
  return { state: "recorded", value: parsed };
}

/** Converts numeric evidence to a number, returning NaN when it is unavailable or invalid. */
export function number(raw: unknown): number {
  return numericEvidence(raw).value ?? Number.NaN;
}

/** Formats number. */
export function formatNumber(raw: unknown): string {
  const evidence = numericEvidence(raw);
  if (evidence.state === "unavailable") return "Not recorded";
  if (evidence.state === "invalid") return "Invalid value";
  return evidence.value!.toLocaleString();
}

/** Formats a count as a percentage of its denominator. */
export function percent(raw: unknown, denominator: unknown): string {
  const count = number(raw);
  const base = number(denominator);
  if (base > 0) {
    return `${(count * 100 / base).toFixed(1)}%`;
  }
  return "—";
}

/** Formats time. */
export function formatTime(raw: unknown): string {
  if (!raw) return "—";
  const date = new Date(String(raw));
  if (Number.isNaN(date.getTime())) return String(raw);
  return dateTimeFormatter.format(date);
}

/** Formats a timestamp as one UTC calendar date for grouping and display. */
export function formatDate(raw: unknown): string {
  if (!raw) return "Not recorded";
  const date = new Date(String(raw));
  if (Number.isNaN(date.getTime())) return String(raw);
  return dateFormatter.format(date);
}

/** Formats the elapsed time between two recorded timestamps. */
export function formatDuration(startedAt: unknown, finishedAt: unknown): string {
  if (!startedAt || !finishedAt) return "—";
  const started = new Date(String(startedAt)).getTime();
  const finished = new Date(String(finishedAt)).getTime();
  if (!Number.isFinite(started) || !Number.isFinite(finished) || finished < started) return "—";
  var seconds = Math.round((finished - started) / 1000);
  const hours = Math.floor(seconds / 3600);
  seconds -= hours * 3600;
  const minutes = Math.floor(seconds / 60);
  seconds -= minutes * 60;
  const parts: string[] = [];
  if (hours) parts.push(`${hours}h`);
  if (minutes) parts.push(`${minutes}m`);
  if (seconds || !parts.length) parts.push(`${seconds}s`);
  return parts.join(" ");
}

/** Formats bytes. */
export function formatBytes(raw: unknown): string {
  const rawBytes = number(raw);
  if (!Number.isFinite(rawBytes)) return formatNumber(raw);
  const bytes = Math.max(0, rawBytes);
  if (bytes < 1024) return `${bytes.toLocaleString()} B`;
  const units = ["KB", "MB", "GB", "TB"];
  var value = bytes;
  var unit = -1;
  do {
    value /= 1024;
    unit += 1;
  } while (value >= 1024 && unit < units.length - 1);
  return `${value.toLocaleString(undefined, { maximumFractionDigits: value >= 10 ? 1 : 2 })} ${units[unit]}`;
}

/** Converts a machine-oriented identifier to a title-cased display label. */
export function humanLabel(raw: unknown): string {
  const spaced = String(raw || "").replace(/_/g, " ");
  return spaced.replace(/\b\w/g, (character) => {
    return character.toUpperCase();
  });
}

/** Parses object. */
export function parseObject(raw: unknown): WireRecord {
  if (raw && typeof raw === "object") {
    return raw as WireRecord;
  }
  if (!raw || typeof raw !== "string") {
    return {};
  }
  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === "object") {
      return parsed as WireRecord;
    }
  } catch (_) {
    return {};
  }
  return {};
}

/** Maps a recorded status to its semantic color class. */
export function statusClass(raw: unknown): ClassName {
  const normalized = String(raw || "").trim().toLowerCase();
  const status = normalized.replace(/[ -]+/g, "_");
  const danger = new Set(["fail", "failed", "parse_failed", "provider_failed", "network_failed", "discard", "discarded", "error", "errored", "trash", "trashed", "purge", "purged", "reject", "rejected", "invalid", "removed"]);
  const warning = new Set(["warning", "skip", "skipped", "stale", "negative", "unresolved", "unclear", "orcid_is_unclear", "disabled", "incomplete", "below", "above", "unmatched", "not_available", "unavailable", "not_approved", "not_evaluated"]);
  const success = new Set(["complete", "completed", "valid", "success", "successful", "hit", "cache_hit", "ready", "available", "approved", "resolved", "resolved_internally", "enriched", "normalized", "linked", "linked_global_person", "match", "matched"]);
  const info = new Set(["pending", "running", "recorded", "active", "visible", "inventoried", "observed_occurrence_only"]);
  const review = new Set(["inherited", "reviewed", "review"]);
  const neutral = new Set(["no_orcid_candidate", "no_candidate", "no_match", "unknown", "not_recorded"]);

  if (danger.has(status)) return "red";
  if (warning.has(status)) return "orange";
  if (success.has(status)) return "green";
  if (info.has(status)) return "blue";
  if (review.has(status)) return "violet";
  if (neutral.has(status)) return "grey";
  return "grey";
}

/** Renders one status chip with its semantic color class. */
export function StatusChip(props: { raw: unknown }): JSX.Element {
  const chipClass = cx("ui", statusClass(props.raw), "label");
  return <span className={chipClass}>{props.raw == null || props.raw === "" ? "Not recorded" : String(props.raw)}</span>;
}

/** Normalizes array- or object-backed metrics to display-name and value pairs. */
export function metricEntries(group: MetricValue[] | Record<string, MetricValue> | null | undefined): Array<[string, MetricValue]> {
  if (Array.isArray(group)) {
    return group.map((item) => {
      var suffix = "";
      if (typeof item === "object" && item !== null && item.source) suffix = ` (${item.source})`;
      const metric = typeof item === "object" && item !== null ? item.metric : "";
      return [`${metric || "Metric"}${suffix}`, item];
    });
  }
  return Object.entries(group || {});
}

/** Returns the pipeline run selected by the current URL context. */
export function selectedRun(): HierarchyRun | HierarchyAttempt | undefined {
  const runId = value("run_id");
  return state.runs.find((run) => {
    return String(pickID(run)) === runId;
  });
}

/** Shows error. */
export function showError(error: unknown): void {
  notice.textContent = error instanceof Error ? error.message : String(error);
  notice.hidden = false;
}

/** Clears error. */
export function clearError(): void {
  notice.hidden = true;
  notice.textContent = "";
}

/** Shows or hides the global loading indicator. */
export function busy(isBusy: boolean): void {
  loading.hidden = !isBusy;
  app.setAttribute("aria-busy", String(isBusy));
  app.inert = isBusy;
}

/** Adds route and focus cleanup required when a parent research context changes. */
export function contextChange(updates: Record<string, unknown>): Record<string, unknown> {
  const cleaned: Record<string, unknown> = {
    ...updates,
    article_id: "",
    author_id: "",
    reference_id: "",
    note_id: "",
    anchor_id: "",
    pdf_page: "",
    origin: "",
  };
  const current = view();
  if (current === "article" || current === "author" || current === "reference") {
    cleaned.view = "corpus";
    cleaned.section = `${current}s`;
    if (current === "reference") cleaned.section = "references";
  }
  return cleaned;
}

/** Renders the standard page header with escaped copy and optional actions. */
export function PageHeader(props: { kicker: string; title: string; description: string; extra?: JSX.Element }): JSX.Element {
  return (
    <header className="rw-page-header">
      <div className="rw-page-header__main">
        {props.kicker ? <p className="rw-page-header__kicker">{props.kicker}</p> : null}
        <h2 id="page-title">{props.title}</h2>
        {props.description ? <p className="rw-page-header__description">{props.description}</p> : null}
      </div>
      {props.extra ? <div className="rw-page-header__actions">{props.extra}</div> : null}
    </header>
  );
}

/** One breadcrumb item with an optional state-carrying destination. */
export interface BreadcrumbItem {
  href?: string;
  label: string;
  state?: Record<string, string>;
}

/** Renders escaped breadcrumb markup for an ordered page hierarchy. */
export function Breadcrumb(props: { items: BreadcrumbItem[] }): JSX.Element | null {
  var parts: BreadcrumbItem[] = [];
  if (Array.isArray(props.items)) parts = props.items;
  if (!parts.length) {
    return null;
  }
  const children: JSX.Element[] = [];
  parts.forEach((item, index) => {
    if (index > 0) {
      children.push(<span className="divider" aria-hidden="true">/</span>);
    }
    if (item.href && index < parts.length - 1) {
      children.push(<a className="section" href={item.href} data-state={item.state ? JSON.stringify(item.state) : undefined}>{item.label}</a>);
    } else {
      var ariaCurrent: string | undefined;
      if (index === parts.length - 1) ariaCurrent = "page";
      children.push(<span className={classNames.sectionCurrent} aria-current={ariaCurrent}>{item.label}</span>);
    }
  });
  return <nav className="rw-breadcrumb" aria-label="Breadcrumb">{children}</nav>;
}

/** Replaces the shell breadcrumb with the supplied ordered page hierarchy. */
export function setBreadcrumb(items: BreadcrumbItem[]): void {
  const breadcrumbMarkup = <Breadcrumb items={items} />;
  if (breadcrumbHost) renderTree(breadcrumbMarkup, breadcrumbHost);
}

/** Renders a complete empty-view state with the standard page header. */
export function EmptyState(props: { title: string; detail: string; action?: JSX.Element }): JSX.Element {
  return (
    <>
      <PageHeader kicker="Read-only workspace" title={props.title} description={props.detail} />
      <section className={classNames.uiBasicSegment}>
        <p>{props.detail}</p>
        {props.action}
      </section>
    </>
  );
}

/** Renders a compact empty-state panel. */
export function EmptyPanel(props: { title: string; detail: string; action?: JSX.Element }): JSX.Element {
  return (
    <section className={classNames.uiSegmentRwEmptyState}>
      <h3>{props.title}</h3>
      <p>{props.detail}</p>
      {props.action}
    </section>
  );
}

/** Renders the standard titled content panel. */
export function Panel(props: { title: string; description: string; body: JSX.Element; classes?: readonly ClassName[] }): JSX.Element {
  const panelClass = cx("ui", "segment", ...(props.classes || []));
  return (
    <section className={panelClass}>
      <div className={classNames.uiTopAttachedHeader}>
        <div>
          <h3>{props.title}</h3>
          {props.description ? <p>{props.description}</p> : null}
        </div>
      </div>
      <div className="content">{props.body}</div>
    </section>
  );
}

/** One table column definition: a field name or a labeled renderer. */
export type TableColumn = string | { label: string; render: (row: WireRecord) => JSX.Element };

/** Renders an escaped data table inside the standard panel wrapper. */
export function Table(props: { title: string; description: string; columns: TableColumn[]; rows: WireRecord[]; classes?: readonly ClassName[] }): JSX.Element {
  const header = props.columns.map((column) => {
    var label = "";
    if (typeof column === "string") {
      label = column;
    } else {
      label = column.label;
    }
    return <th scope="col">{label}</th>;
  });
  var body: JSX.Element[] = [<tr><td colSpan={Math.max(props.columns.length, 1)} className="rw-table-empty">No records.</td></tr>];
  if (props.rows.length) {
    body = props.rows.map((row) => {
      const cells = props.columns.map((column) => {
        var content: JSX.Element;
        if (typeof column === "string") {
          content = <Cell item={row[column]} column={column} />;
        } else {
          content = column.render(row);
        }
        return <td>{content}</td>;
      });
      return <tr>{cells}</tr>;
    });
  }
  const tableWrap = (
    <div className="table-wrap" aria-label={`${props.title} table`}>
      <table className={classNames.uiTable}>
        <thead><tr>{header}</tr></thead>
        <tbody>{body}</tbody>
      </table>
    </div>
  );
  return <Panel title={props.title} description={props.description} body={tableWrap} classes={props.classes} />;
}

/** Renders context-preserving tab navigation for a keyed section. */
export function Subnav(props: { items: Array<[string, string]>; current: string; key: string }): JSX.Element {
  const links = props.items.map(([id, label]) => {
    const updates = { [props.key]: id };
    const href = link(updates);
    const state = linkState(updates);
    const active = id === props.current;
    var ariaCurrent: string | undefined;
    if (id === props.current) ariaCurrent = "page";
    const itemClass = cx("item", active && "active");
    return <a href={href} className={itemClass} aria-current={ariaCurrent} data-state={JSON.stringify(state)}>{label}</a>;
  });
  return <nav className={classNames.uiTabularMenu} aria-label="Section navigation">{links}</nav>;
}

/** One filter summary rendering option set. */
export interface FilterChipOptions {
  removeUpdates?: Record<string, unknown>;
  clearUpdates?: Record<string, unknown>;
}

/** Renders removable filter chips with a clear-all action. */
export function FilterChips(props: { filters: Record<string, unknown> | null; labels?: Record<string, string>; options?: FilterChipOptions }): JSX.Element {
  const options = props.options || {};
  const filterEntries = Object.entries(props.filters || {});
  const entries = filterEntries.filter(([, raw]) => {
    return raw !== "" && raw !== null && raw !== undefined;
  });
  if (!entries.length) {
    return <div className="rw-filter-summary"><span className={classNames.uiFadedText}>No filters applied.</span></div>;
  }
  const chips = entries.flatMap(([key, raw]) => {
    var values = [raw];
    if (Array.isArray(raw)) values = raw;
    return values.map((item) => {
      var remaining = "";
      if (Array.isArray(raw)) {
        remaining = raw.filter((candidate) => {
          return candidate !== item;
        }).join(",");
      }
      const updates = { ...(options.removeUpdates || { page: 1 }), [key]: remaining };
      const href = link(updates);
      const state = linkState(updates);
      return (
        <a className="rw-filter-chip" href={href} title="Remove filter" data-state={JSON.stringify(state)}>
          <span>{props.labels?.[key] || humanLabel(key)}:</span>
          {" "}
          {String(item)}
          {" "}
          <b aria-hidden="true">{"\u00D7"}</b>
        </a>
      );
    });
  });
  var clear: JSX.Element | null = null;
  if (options.clearUpdates) {
    const clearHref = link(options.clearUpdates);
    const clearState = linkState(options.clearUpdates);
    clear = <a className="rw-filter-clear" href={clearHref} data-state={JSON.stringify(clearState)}>Clear all</a>;
  }
  return (
    <div className="rw-filter-summary">
      <strong>Applied filters</strong>
      <div className="rw-filter-chips">
        {chips}
        {clear}
      </div>
    </div>
  );
}

/** Renders a metric card with availability, denominator, and optional navigation. */
export function MetricCard(props: { name: string; metric: MetricValue | null | undefined; href?: string; state?: Record<string, string> }): JSX.Element {
  const evidence = numericEvidence(props.metric);
  const metric = typeof props.metric === "object" && props.metric !== null ? props.metric : null;
  const unavailable = evidence.state === "unavailable";
  var content: JSX.Element = (
    <>
      <span className="label">{humanLabel(props.name)}</span>
      <span className="value">Not recorded</span>
      <small>Not captured for this run</small>
    </>
  );
  if (evidence.state === "invalid") {
    content = (
      <>
        <span className="label">{humanLabel(props.name)}</span>
        <span className="value">Invalid value</span>
        <small>Stored evidence could not be interpreted</small>
      </>
    );
  } else if (!unavailable) {
    const value = formatNumber(metric?.value ?? props.metric);
    var detail: JSX.Element | null = null;
    if (metric?.denominator != null) {
      const pct = metric.percentage ?? percent(metric.value, metric.denominator);
      detail = <small>{formatNumber(metric.value)} of {formatNumber(metric.denominator)} ({pct})</small>;
    } else if (metric?.basis || metric?.unit) {
      detail = <small>{metric.basis || metric.unit}</small>;
    }
    content = (
      <>
        <span className="label">{humanLabel(props.name)}</span>
        <span className="value">{value}</span>
        {detail}
      </>
    );
  }
  if (props.href) {
    return <div className={classNames.uiStatisticRwKpi}><a href={props.href} data-state={props.state ? JSON.stringify(props.state) : undefined}>{content}</a></div>;
  }
  return <div className={classNames.uiStatisticRwKpi}>{content}</div>;
}

/** One retention-flow stage option set. */
export interface FlowStageOptions {
  description?: string;
  denominatorLabel?: string;
  baselineLabel?: string;
  href?: string;
  state?: Record<string, string>;
  outcomes?: Array<{ label: string; value: unknown }>;
}

/** Renders one retention-flow stage with counts, percentages, and optional links. */
export function FlowStage(props: { label: string; raw: unknown; base: unknown; previous: unknown; modifier?: ClassName; stageKey: string; options: FlowStageOptions }): JSX.Element {
  const options = props.options || {};
  const stageClass = cx("ui", "step", "rw-flow__step", props.modifier);
  var info: JSX.Element | null = null;
  if (options.description) {
    info = (
      <details className="rw-flow__info">
        <summary aria-label={`About ${props.label}`}>i</summary>
        <div>
          <strong>{props.label}</strong>
          <p>{options.description}</p>
        </div>
      </details>
    );
  }

  const evidence = numericEvidence(props.raw);
  if (evidence.state === "unavailable" || evidence.state === "invalid") {
    const articleClass = cx("ui", "step", "rw-flow__step", props.modifier, "disabled");
    const stateLabel = evidence.state === "invalid" ? "Invalid value" : "Not recorded";
    const stateDetail = evidence.state === "invalid" ? "Stored evidence could not be interpreted." : "This stage was not captured.";
    return (
      <article className={articleClass} data-flow-stage={props.stageKey || undefined}>
        {info}
        <div className="rw-flow__content">
          <h5>{props.label}</h5>
          <strong className="rw-flow__unavailable">{stateLabel}</strong>
          <small>{stateDetail}</small>
        </div>
      </article>
    );
  }

  const count = evidence.value!;
  const baseCount = number(props.base);
  var percentage: number | null = null;
  if (baseCount > 0) percentage = count * 100 / baseCount;
  var percentageText = "\u2014";
  if (percentage != null) percentageText = `${percentage.toFixed(2)}%`;
  var progressWidth = 0;
  if (percentage != null) progressWidth = Math.max(0, Math.min(100, percentage));
  const denominatorText = options.denominatorLabel || "input records";
  var change: string;
  if (props.previous == null) {
    change = options.baselineLabel || "Input baseline";
  } else {
    const diff = number(props.previous) - count;
    if (diff === 0) {
      change = "No change from prior";
    } else {
      var sign = "+";
      if (diff > 0) sign = "\u2212";
      change = `${sign}${formatNumber(Math.abs(diff))} from prior`;
    }
  }

  var outcomes: JSX.Element | null = null;
  if (Array.isArray(options.outcomes)) {
    const outcomeValues = options.outcomes.map((outcome) => {
      return (
        <span>
          <b>{formatNumber(outcome.value)}</b>
          {" "}
          {outcome.label}
        </span>
      );
    });
    outcomes = <div className="rw-flow__outcome-values">{outcomeValues}</div>;
  }

  const content = (
    <div className="rw-flow__content">
      <h5>{props.label}</h5>
      <div className="rw-flow__value">
        <strong>{formatNumber(count)}</strong>
        <span className="rw-flow__percentage">{percentageText}</span>
      </div>
      <span className="rw-flow__progress" role="img" aria-label={`${props.label}: ${formatNumber(count)} of ${formatNumber(baseCount)} ${denominatorText} (${percentageText})`}>
        <span style={`width:${progressWidth.toFixed(2)}%`}></span>
      </span>
      <small className="rw-flow__delta">{change}</small>
      {outcomes}
    </div>
  );

  var linkedContent: JSX.Element = content;
  if (options.href) {
    linkedContent = <a className="rw-flow__link" href={options.href} data-state={options.state ? JSON.stringify(options.state) : undefined}>{content}</a>;
  }
  var linkedModifier: ClassName | undefined;
  if (options.href) linkedModifier = "linked";
  const linkedStageClass = cx("ui", "step", "rw-flow__step", props.modifier, linkedModifier);
  return (
    <article className={linkedStageClass} data-flow-stage={props.stageKey || undefined}>
      {info}
      {linkedContent}
    </article>
  );
}

const filterPresentations: Record<string, { groupLabel: string; label: string; description: string }> = {
  "NO_FILTER": {
    groupLabel: "Unfiltered Source Data",
    label: "Initial raw results",
    description: "Unfiltered results reported by the configured sources.",
  },
  "RANGE_10_YEARS": {
    groupLabel: "Publication Range",
    label: "Publication range",
    description: "Results retained within the declared 10-year range.",
  },
  "ARTICLE_ONLY": {
    groupLabel: "Document Type",
    label: "Document type",
    description: "Results retained after applying the article-only filter.",
  },
  "ENGLISH_ONLY": {
    groupLabel: "Language",
    label: "Language",
    description: "Results retained after the language filter.",
  },
};

/** Combines cumulative source filter counts into ordered cross-source stages. */
function sourceFilterStageSummary(items: SourceFilterCount[]): { stages: Array<{ count: number | MetricEvidence; groupLabel: string; label: string; description: string }>; sourceCount: number } {
  var bySource = new Map<string, Map<string, { count: number; filters: string[] }>>();
  (items || []).forEach((item) => {
    if (!Array.isArray(item?.filters) || item.filters.length === 0) {
      return;
    }
    const sourceName = String(item.source || "source");
    const filters = item.filters.map(String);
    const count = number(item.count);
    if (!Number.isFinite(count) || count < 0) return;
    if (!bySource.has(sourceName)) bySource.set(sourceName, new Map());
    bySource.get(sourceName)!.set(filters.join("\u001f"), { count: count, filters: filters });
  });

  const sources = Array.from(bySource.values()).filter((stages) => {
    return stages.size > 0;
  });
  const identities = new Map<string, string[]>();
  sources.forEach((sourceStages) => {
    sourceStages.forEach((stage, identity) => {
      identities.set(identity, stage.filters);
    });
  });
  const orderedIdentities = Array.from(identities.entries()).sort((left, right) => {
    return left[1].length - right[1].length || left[0].localeCompare(right[0]);
  });
  var stages: Array<{ count: number | MetricEvidence; groupLabel: string; label: string; description: string }> = [];
  orderedIdentities.forEach(([identity, filters], index) => {
    const matching = sources.map((sourceStages) => {
      return sourceStages.get(identity);
    });
    var count: number | MetricEvidence = { available: false, state: "unavailable" };
    if (matching.every(Boolean)) {
      count = matching.reduce((sum, stage) => {
        return sum + stage!.count;
      }, 0);
    }
    const appliedFilters = filters.slice(-1);
    var presentation: { groupLabel: string; label: string; description: string } | null = null;
    if (appliedFilters.length === 1) presentation = filterPresentations[appliedFilters[0]];
    const filterLabels = appliedFilters.map((name) => {
      return humanLabel(name);
    });
    var groupLabel = presentation?.groupLabel || "Unfiltered Source Data";
    if (index !== 0 && !presentation?.groupLabel) groupLabel = `Source Filter ${index + 1}`;
    var label = presentation?.label || "Initial raw results";
    if (index !== 0 && !presentation?.label) label = `Source filter stage ${index + 1}`;
    var description = presentation?.description || filterLabels.join(", ");
    stages.push({
      count: count,
      groupLabel: groupLabel,
      label: label,
      description: description,
    });
  });
  return { stages: stages, sourceCount: sources.length };
}

/** Renders one titled phase in the retention-flow presentation. */
function RetentionPhase(props: { title: string; description: string; summary: string; children: JSX.Element; phase: "source" | "pipeline" | "corpus" }): JSX.Element {
  var summaryMarkup: JSX.Element | null = null;
  if (props.summary) summaryMarkup = <span className={classNames.uiLabel}>{props.summary}</span>;
  return (
    <section className="rw-retention__phase" data-retention-phase={props.phase}>
      <header className="rw-retention__phase-header">
        <div>
          <h4>{props.title}</h4>
          <p>{props.description}</p>
        </div>
        {summaryMarkup}
      </header>
      {props.children}
    </section>
  );
}

/** Renders the three-phase source-selection, pipeline-processing, and corpus-enrichment flow for an overview payload. */
export function RetentionFlow(props: { overview: OverviewResponse }): JSX.Element {
  const source = props.overview.retention_funnel || {};
  const input = source.input_records;
  const filterSummary = sourceFilterStageSummary(props.overview.source_filter_counts);
  const filterStages = filterSummary.stages;
  const hasFilterStages = filterStages.length > 0;
  const inputCount = number(input);
  var initialCount = inputCount;
  if (hasFilterStages && Number.isFinite(number(filterStages[0].count))) initialCount = number(filterStages[0].count);
  var denominatorLabel = "input records";
  if (hasFilterStages) denominatorLabel = "initial raw results";
  const sourceDefinitions = [
    ["Initial raw results", "Unfiltered results reported by the configured sources."],
    ["Publication range", "Results retained within the declared publication window."],
    ["Document type", "Results retained after applying the declared document-type filter."],
    ["Language", "Results retained after applying the declared language filter."]
  ];
  var previousFilterCount: number | null = null;
  const sourceSteps = sourceDefinitions.map(([label, description], index) => {
    const recordedStage = filterStages[index];
    const stageOptions = {
      description: recordedStage?.description || description,
      denominatorLabel: denominatorLabel,
      baselineLabel: "Initial raw-data baseline",
    };
    const stage = <FlowStage label={label} raw={recordedStage?.count} base={initialCount} previous={previousFilterCount} modifier="rw-flow__step--source" stageKey={`filter_${index}`} options={stageOptions} />;
    if (recordedStage && Number.isFinite(number(recordedStage.count))) previousFilterCount = number(recordedStage.count);
    return stage;
  });
  var sourceSummary = "Aggregate source-filter counts were not recorded";
  if (hasFilterStages) {
    sourceSummary = `${formatNumber(initialCount)} initial raw results`;
    if (filterSummary.sourceCount > 1) sourceSummary += ` across ${formatNumber(filterSummary.sourceCount)} sources`;
  }
  const phases: JSX.Element[] = [
    <RetentionPhase title="Source selection" description="Cumulative filters applied before source export." summary={sourceSummary} phase="source">
      <div className={classNames.uiStepsRwFlowRwFlowSource}>{sourceSteps}</div>
    </RetentionPhase>
  ];

  if (numericEvidence(input).value == null) {
    phases.push(
      <RetentionPhase title="Pipeline processing" description="Records loaded, parsed, and deduplicated by the pipeline." summary="Pipeline counts not recorded" phase="pipeline">
        <div className={classNames.uiStepsRwFlow}>
          <FlowStage label="Input records" raw={input} base={initialCount} previous={null} stageKey="input_records" options={{ description: "Records read from exported source files." }} />
          <FlowStage label="Parsed articles" raw={null} base={initialCount} previous={null} stageKey="parsed_articles" options={{ description: "Source records converted into article metadata." }} />
          <FlowStage label="Deduplicated articles" raw={null} base={initialCount} previous={null} stageKey="deduplicated_articles" options={{ description: "Unique articles retained after source merging." }} />
        </div>
      </RetentionPhase>
    );
    phases.push(
      <RetentionPhase title="Corpus enrichment" description="Candidate articles continue through enrichment, validation, and normalization." summary="Corpus counts not recorded" phase="corpus">
        <div className={classNames.uiStepsRwFlow}>
          <FlowStage label="Candidate articles" raw={null} base={initialCount} previous={null} stageKey="enrichment_candidates" options={{ description: "Deduplicated articles considered for provider enrichment." }} />
          <FlowStage label="Accepted + Discarded" raw={null} base={initialCount} previous={null} stageKey="validation_outcomes" options={{ description: "Validation divides candidate articles into accepted and discarded outcomes." }} />
          <FlowStage label="Normalization" raw={null} base={initialCount} previous={null} stageKey="normalized_articles_processed" options={{ description: "Accepted articles processed into canonical forms." }} />
        </div>
      </RetentionPhase>
    );
    const retentionBody = <div className="rw-retention">{phases}</div>;
    return <Panel title="Retention flow" description="Three phases connect source selection to the analysis-ready corpus." body={retentionBody} classes={["rw-grid-span-all", "rw-panel--no-separator"]} />;
  }

  const parsed = source.parsed_articles;
  const deduped = source.deduplicated_articles;
  const parsedCount = number(parsed);
  const dedupedCount = number(deduped);
  const validCount = number(source.valid_articles);
  const enrichmentCandidates = props.overview.enrichment_breakdown?.enrichment_candidates;
  const normalizedArticles = props.overview.normalization_breakdown?.normalized_articles_processed;
  var enrichmentCount = dedupedCount;
  if (enrichmentCandidates != null && enrichmentCandidates?.available !== false) {
    enrichmentCount = number(enrichmentCandidates);
  }
  var pipelinePrevious: number | null = null;
  if (hasFilterStages && Number.isFinite(number(filterStages[filterStages.length - 1].count))) pipelinePrevious = number(filterStages[filterStages.length - 1].count);
  const stageHref = (stage: string): { href: string; state: Record<string, string> } => {
    if (stage === "input") {
      const updates = { view: "corpus", section: "sources", q: "", page: 1 };
      return { href: link(updates), state: linkState(updates) };
    }
    const updates = { view: "provenance", section: "stages", stage_q: stage, stage_page: 1 };
    return { href: link(updates), state: linkState(updates) };
  };
  const stageOptions = (description: string, target: { href: string; state: Record<string, string> }): FlowStageOptions => {
    return {
      description: description,
      href: target.href,
      state: target.state,
      denominatorLabel: denominatorLabel,
      baselineLabel: "Input baseline",
    };
  };
  const pipelineSteps = [
    <FlowStage label="Input records" raw={input} base={initialCount} previous={pipelinePrevious} stageKey="input_records" options={stageOptions("Records read from the exported source files.", stageHref("input"))} />,
    <FlowStage label="Parsed articles" raw={parsed} base={initialCount} previous={inputCount} stageKey="parsed_articles" options={stageOptions("Source records converted into article metadata.", stageHref("parse"))} />,
    <FlowStage label="Deduplicated articles" raw={deduped} base={initialCount} previous={parsedCount} stageKey="deduplicated_articles" options={stageOptions("Unique articles retained after source merging.", stageHref("deduplicate"))} />,
  ];
  phases.push(
    <RetentionPhase title="Pipeline processing" description="Records move from source loading through deduplication." summary={`${formatNumber(inputCount)} captured input records`} phase="pipeline">
      <div className={classNames.uiStepsRwFlow}>{pipelineSteps}</div>
    </RetentionPhase>
  );

  const discardedCount = number(source.discarded_articles);
  var validationTotal: MetricEvidence | number = { available: false, state: "unavailable" };
  if (Number.isFinite(validCount) && Number.isFinite(discardedCount)) validationTotal = validCount + discardedCount;
  const corpusSteps = [
    <FlowStage label="Candidate articles" raw={enrichmentCandidates} base={initialCount} previous={dedupedCount} stageKey="enrichment_candidates" options={stageOptions("Deduplicated articles considered for provider enrichment.", stageHref("enrich"))} />,
    <FlowStage label="Accepted + Discarded" raw={validationTotal} base={initialCount} previous={enrichmentCount} stageKey="validation_outcomes" options={{
      ...stageOptions("Validation divides candidate articles into analysis-ready and discarded outcomes.", stageHref("validate")),
      outcomes: [{ label: "accepted", value: validCount }, { label: "discarded", value: discardedCount }]
    }} />,
    <FlowStage label="Normalization" raw={normalizedArticles} base={initialCount} previous={validCount} stageKey="normalized_articles_processed" options={stageOptions("Accepted articles processed into canonical forms.", stageHref("normalize"))} />,
  ];
  phases.push(
    <RetentionPhase title="Corpus enrichment" description="Candidate articles continue through enrichment, validation, and normalization." summary={`${formatNumber(validCount)} accepted articles`} phase="corpus">
      <div className={classNames.uiStepsRwFlow}>{corpusSteps}</div>
    </RetentionPhase>
  );

  const retentionBody = <div className="rw-retention">{phases}</div>;
  return <Panel title="Retention flow" description="Three phases connect source selection to the analysis-ready corpus." body={retentionBody} classes={["rw-grid-span-all", "rw-panel--no-separator"]} />;
}

/** Renders a metric breakdown table with relative bars and optional total percentages. */
export function Breakdown(props: { title: string; source: Record<string, MetricEvidence>; valueLabel?: string; useTotal?: boolean }): JSX.Element {
  const valueLabel = props.valueLabel || "Count";
  const entries = metricEntries(props.source);
  if (!entries.length) {
    const emptyBody = <p className={classNames.uiFadedText}>Not recorded for this run.</p>;
    return <Panel title={props.title} description="Recorded activity for this run." body={emptyBody} />;
  }

  var total: number | null;
  if (props.useTotal) {
    const values = entries.map(([, entry]) => {
      return number(entry);
    });
    total = values.every(Number.isFinite) ? values.reduce((sum, entry) => {
      return sum + entry;
    }, 0) : null;
  } else {
    total = null;
  }

  const entryValues = entries.map(([, entry]) => {
    return number(entry);
  });
  const finiteEntryValues = entryValues.filter(Number.isFinite);
  const max = total || Math.max.apply(null, finiteEntryValues.concat([1]));

  /** Renders one breakdown value with availability and optional percentage. */
  function valueRender(row: WireRecord): JSX.Element {
    const evidence = numericEvidence(row.raw);
    if (evidence.state === "unavailable") {
      return <span className={classNames.uiFadedText}>Not recorded</span>;
    }
    if (evidence.state === "invalid") return <span className={classNames.uiNegativeText}>Invalid value</span>;
    if (!props.useTotal) {
      return <strong className="rw-metric-value">{formatNumber(row.raw)}</strong>;
    }
    var pct = "—";
    if (total! > 0) {
      pct = `${(number(row.raw) * 100 / total!).toFixed(2)}%`;
    }
    return (
      <>
        <strong className="rw-metric-value">{formatNumber(row.raw)}</strong>
        <span className="rw-metric-percent">{pct}</span>
      </>
    );
  }

  /** Renders an accessible relative-volume bar for one breakdown row. */
  function barRender(row: WireRecord): JSX.Element {
    const value = number(row.raw);
    if (!Number.isFinite(value)) {
      return <span className={classNames.uiFadedText}>—</span>;
    }
    const pct = Math.min(100, value / max * 100);
    var basis = "relative to the largest recorded value";
    if (props.useTotal) basis = "share of total";
    return <span className={classNames.uiProgress} role="img" aria-label={`${humanLabel(row.name)}: ${pct.toFixed(1)}% ${basis}`}><span className="rw-progress__bar" style={`width:${pct}%`}></span></span>;
  }

  const rows = entries.map(([name, raw]) => {
    return {
      name: name,
      raw: raw,
    };
  });

  var shareLabel = "Relative to largest";
  if (props.useTotal) shareLabel = "Share of total";
  const columns: TableColumn[] = [
    {
      label: "Metric",
      render: (row) => {
        return <strong className="rw-metric-name">{humanLabel(row.name)}</strong>;
      },
    },
    {
      label: valueLabel,
      render: valueRender,
    },
    {
      label: shareLabel,
      render: barRender,
    },
  ];
  return <Table title={props.title} description="Recorded activity for this run." columns={columns} rows={rows} classes={["rw-metric-table"]} />;
}

/** Renders the expected-versus-observed source export count table. */
export function SourceResultCountSummary(props: { items: SourceResultCount[] | null; classes?: readonly ClassName[] }): JSX.Element {

  /** Formats a source count or its unavailable state. */
  function count(raw: unknown): JSX.Element {
    if (raw == null) {
      return <span className={classNames.uiFadedText}>Not recorded</span>;
    }
    return <>{formatNumber(raw)}</>;
  }

  /** Renders a status chip for a source-count comparison. */
  function comparison(raw: unknown): JSX.Element {
    if (raw) {
      return <StatusChip raw={raw} />;
    }
    return <span className={classNames.uiFadedText}>Not recorded</span>;
  }

  /** Renders an export date or its unavailable state. */
  function date(raw: unknown): JSX.Element {
    if (raw) {
      return <>{String(raw)}</>;
    }
    return <span className={classNames.uiFadedText}>Not recorded</span>;
  }

  const rows = (props.items || []).map((item) => {
    return {
      source: item.source_name || "Unnamed source",
      expected: item.expected_result_count,
      observed: item.observed_result_count,
      comparison: item.result_count_comparison,
      exportDate: item.export_date,
    };
  });

  const columns: TableColumn[] = [
    {
      label: "Source",
      render: (row) => {
        return <>{String(row.source)}</>;
      },
    },
    {
      label: "Export date",
      render: (row) => {
        return date(row.exportDate);
      },
    },
    {
      label: "Expected initial count",
      render: (row) => {
        return count(row.expected);
      },
    },
    {
      label: "Observed raw records",
      render: (row) => {
        return count(row.observed);
      },
    },
    {
      label: "Comparison",
      render: (row) => {
        return comparison(row.comparison);
      },
    },
  ];
  return <Table title="Source export counts"
    description="Expected is the count recorded when metadata was originally downloaded. Observed is the raw export count read by this run. The comparison is informational and never makes a run fail."
    columns={columns} rows={rows} classes={props.classes} />;
}

/** Renders expandable exact-query markup for source exports. */
export function SourceSearchQueries(props: { items: SourceResultCount[] | null; classes?: readonly ClassName[] }): JSX.Element | null {
  if (!props.items || !props.items.length) {
    return null;
  }

  const rows = props.items.map((item) => {
    return (
      <details className="rw-query-row">
        <summary>
          <span className="rw-query-row__source">{item.source_name || "Unnamed source"}</span>
          <span className={classNames.uiFadedText}>Inspect exact query</span>
        </summary>
        <div className="rw-query-row__content">
          <code className="rw-query-row__code">{item.query || ""}</code>
          <button type="button" className={classNames.uiButton} data-copy-text={item.query || ""}>Copy query</button>
        </div>
      </details>
    );
  });

  const body = <>{rows}</>;
  return <Panel title="Search queries"
    description="The exact search terms used to obtain each source export. Expand a source to inspect or copy its query."
    body={body} classes={props.classes} />;
}

/** Renders chronological audit feed markup for generic event rows. */
export function Timeline(props: { rows: WireRecord[] }): JSX.Element {
  if (!props.rows.length) {
    return <p className={classNames.uiFadedText}>No records.</p>;
  }

  const items = props.rows.map((event) => {
    const action = text(event, ["action", "event_type", "type"]);
    const entityType = text(event, ["entity_type", "entity", "source"]);
    const actor = event.actor;
    const entityId = event.entity_id;
    const meta = parseObject(event.metadata_json);

    var detail: JSX.Element | null = null;
    if (Object.keys(meta).length) {
      if (meta.field && meta.provider) {
        detail = <>Field <strong>{String(meta.field)}</strong> enriched by <strong>{String(meta.provider)}</strong></>;
      } else if (meta.reasons) {
        var reasonsText = String(meta.reasons);
        if (Array.isArray(meta.reasons)) reasonsText = meta.reasons.join("; ");
        detail = <>{reasonsText}</>;
      } else if (meta.error) {
        detail = <>Error: {String(meta.error)}</>;
      } else if (meta.status) {
        detail = <>Status: {String(meta.status)}</>;
      } else if (meta.identity) {
        detail = <>{String(meta.identity)}</>;
      } else if (meta.reason) {
        detail = <>{String(meta.reason)}</>;
      } else if (meta.search_id) {
        detail = <>Search: {String(meta.search_id)}{meta.revision ? <> / revision {String(meta.revision)}</> : null}</>;
      }
    }

    var dotClass: ClassName = "default-dot";
    if (action.startsWith("pipeline_")) {
      dotClass = "pipeline-dot";
    } else if (action === "field_enriched") {
      dotClass = "enrich-dot";
    } else if (action.startsWith("validation_")) {
      dotClass = "validation-dot";
    }

    const timestamp = formatTime(event.occurred_at || event.created_at || event.at || event.timestamp);
    const eventClass = cx("event", dotClass);

    return (
      <li className={eventClass}>
        {actor ? <span className="user">{String(actor)}</span> : null}
        {entityId ? <span className="extra">{entityType} #{String(entityId)}</span> : null}
        <strong>{action}</strong>
        <br />
        {detail ? <span className="summary">{detail}</span> : null}
        <br />
        <time>{timestamp}</time>
      </li>
    );
  });

  return <ol className={classNames.uiFeed}>{items}</ol>;
}

/** Renders a table whose columns are derived from the supplied detail records. */
export function DetailTable(props: { title: string; rows: unknown }): JSX.Element {
  const records = list(props.rows, ["items", "rows"]);
  const recordKeys = records.flatMap((record) => {
    return Object.keys(record);
  });
  const columns = [...new Set(recordKeys)];
  const colDefs = columns.map((key) => {
    return {
      label: key,
      render: (row: WireRecord) => {
        return <Cell item={row[key]} column={key} />;
      },
    };
  });
  return <Table title={props.title} description="" columns={colDefs} rows={records} />;
}

/** One cell rendering option set. */
export interface CellOptions {
  expandLong?: boolean;
}

/** Renders and links a table cell according to its column and table context. */
export function Cell(props: { item: unknown; column: string; tableName?: string; options?: CellOptions }): JSX.Element {
  const tableName = props.tableName || "";
  const options = props.options || {};
  if (props.item === null || props.item === undefined) {
    return <span className={classNames.uiFadedText}>NULL</span>;
  }

  const full = asJSON(props.item);
  var display = full;
  if (full.length > 140) display = `${full.slice(0, 137)}…`;

  var href = "";
  var state: Record<string, string> | null = null;
  if (props.column === "article_id" || props.column === "work_revision_id" || (tableName === "work_revisions" && props.column === "id")) {
    const updates = { view: "article", article_id: props.item };
    href = link(updates);
    state = linkState(updates);
  }
  if (props.column === "author_id" || props.column === "author_occurrence_id" || (tableName === "author_occurrences" && props.column === "id")) {
    const updates = { view: "author", author_id: props.item };
    href = link(updates);
    state = linkState(updates);
  }
  if (props.column === "reference_id" || (tableName === "reference_mentions" && props.column === "id")) {
    const updates = { view: "reference", reference_id: props.item };
    href = link(updates);
    state = linkState(updates);
  }

  const shown = <span className="rw-cell" title={full}>{display}</span>;
  if (href) {
    return <a href={href} data-state={JSON.stringify(state)}>{shown}</a>;
  }
  if (full.length > 140 && options.expandLong !== false) {
    return <details><summary>{shown}</summary><pre>{full}</pre></details>;
  }
  return shown;
}

/**
 * Bind copy-to-clipboard behavior for [data-copy-text] buttons.
 * Shows "Copied!" feedback for 2 seconds, falls back to prompt().
 */
export function bindCopyButtons(): void {
  const copyButtons = document.querySelectorAll<HTMLButtonElement>("[data-copy-text]");
  copyButtons.forEach((button) => {
    button.addEventListener("click", async () => {
      const text = button.getAttribute("data-copy-text") || "";
      try {
        await navigator.clipboard.writeText(text);
        button.textContent = "Copied!";
        setTimeout(() => { button.textContent = "Copy"; }, 2000);
      } catch (_) {
        prompt("Copy query manually:", text);
      }
    });
  });
}

/**
 * Bind dismissible behavior for .ui.message elements with a .close child.
 * Clicking the close button fades out and removes the message.
 */
export function bindDismissibleMessages(): void {
  const closeButtons = document.querySelectorAll<HTMLElement>(".ui.message > .close");
  closeButtons.forEach((button) => {
    button.addEventListener("click", () => {
      const message = button.closest<HTMLElement>(".ui.message");
      if (message) {
        message.style.opacity = "0";
        setTimeout(() => { message.hidden = true; }, 150);
      }
    });
  });
}

/**
 * Bind loading state for buttons with [data-loading].
 * On click, the button shows a spinner and disables itself.
 */
export function bindLoadingButtons(): void {
  const loadingButtons = document.querySelectorAll<HTMLButtonElement>("[data-loading]");
  loadingButtons.forEach((button) => {
    button.addEventListener("click", () => {
      classAdd(button, ["loading"]);
      button.disabled = true;
    });
  });
}
