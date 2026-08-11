// Shared state, DOM references, and utility functions.
// This module is imported by every other module. It avoids circular dependencies
// by importing only the leaf JSX runtime.
import { h, Fragment, renderToString, raw } from "./jsx/jsx-runtime.ts";

// The viewer shell guarantees these elements exist (index.html), so the DOM
// roots are declared with non-null assertions at module scope.
export const app = document.querySelector<HTMLElement>("#app")!;
export const notice = document.querySelector<HTMLElement>("#notice")!;
export const loading = document.querySelector<HTMLElement>("#loading")!;
export const breadcrumbHost = document.querySelector<HTMLElement>("#workspace-breadcrumb")!;

/** The shared viewer state: discovered context options, tables, and request lifecycle. */
export interface ViewerState {
  searches: any[];
  plans: any[];
  runs: any[];
  tables: any[];
  request: number;
  controller: AbortController | null;
}

export const state: ViewerState = {
  searches: [],
  plans: [],
  runs: [],
  tables: [],
  request: 0,
  controller: null
};

export const pageSizes = [20, 50, 100, 200, 500];
export const corpusSections: Record<string, { table: string; title: string; description: string }> = {
  articles: {
    table: "work_revisions",
    title: "Analysis-ready articles",
    description: "Valid normalized work revisions captured by the selected run."
  },
  authors: {
    table: "author_occurrences",
    title: "Author occurrences",
    description: "Observed author records. Matching names do not imply the same person."
  },
  references: {
    table: "reference_mentions",
    title: "Reference mentions",
    description: "Ordered citation mentions retained for each article revision."
  },
  identity_evidence: {
    table: "author_identity_resolutions",
    title: "Author identity / ORCID evidence",
    description: "Name-search results are review candidates, never confirmed author identities."
  },
  sources: {
    table: "source_records",
    title: "Source records",
    description: "Captured input records and parse outcomes."
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

/** Returns the current URL search parameters. */
export function params(): URLSearchParams {
  return new URLSearchParams(location.search);
}

/** Returns a named URL parameter or an empty string. */
export function value(name: string): string {
  return params().get(name) || "";
}

/** Returns the selected viewer view. */
export function view(): string {
  return value("view") || "home";
}

/** Returns a named section parameter or its fallback. */
export function section(name: string, fallback: string): string {
  return value(name) || fallback;
}

/** Escapes a value for safe HTML text insertion. */
export function esc(raw: any): string {
  const element = document.createElement("span");
  element.textContent = raw == null ? "" : String(raw);
  return element.innerHTML;
}

/** Formats a value for JSON-oriented display. */
export function asJSON(item: any): string {
  if (typeof item === "string") return item;
  return JSON.stringify(item, null, 2);
}

/** Returns the first matching array in an API response. */
export function list(data: any, keys?: string[]): any[] {
  if (keys) {
    for (const key of keys) {
      if (Array.isArray(data?.[key])) return data[key];
    }
  }
  if (Array.isArray(data)) return data;
  return [];
}

/** Returns the first supported identifier present on an item. */
export function pickID(item: any): any {
  if (item?.id)        return item.id;
  if (item?.search_id) return item.search_id;
  if (item?.run_id)    return item.run_id;
  if (item?.plan_id)   return item.plan_id;
  return "";
}

/** Returns the first non-empty display field on an item. */
export function text(item: any, fields: string[]): string {
  for (const field of fields) {
    if (item?.[field] !== undefined && item?.[field] !== null && item[field] !== "") {
      return String(item[field]);
    }
  }
  return "Unnamed";
}

/** Converts a value to a finite number or zero. */
export function number(raw: any): number {
  const parsed = Number(raw?.value ?? raw);
  if (Number.isFinite(parsed)) return parsed;
  return 0;
}

/** Formats number. */
export function formatNumber(raw: any): string {
  return number(raw).toLocaleString();
}

/** Formats a count as a percentage of its denominator. */
export function percent(raw: any, denominator: any): string {
  const count = number(raw);
  const base = number(denominator);
  if (base > 0) {
    return `${(count * 100 / base).toFixed(1)}%`;
  }
  return "—";
}

/** Formats time. */
export function formatTime(raw: any): string {
  if (!raw) {
    return "—";
  }
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return String(raw);
  }
  return date.toLocaleString();
}

/** Formats the elapsed time between two recorded timestamps. */
export function formatDuration(startedAt: any, finishedAt: any): string {
  if (!startedAt || !finishedAt) return "—";
  const started = new Date(startedAt).getTime();
  const finished = new Date(finishedAt).getTime();
  if (!Number.isFinite(started) || !Number.isFinite(finished) || finished < started) return "—";
  var seconds = Math.round((finished - started) / 1000);
  const hours = Math.floor(seconds / 3600);
  seconds -= hours * 3600;
  const minutes = Math.floor(seconds / 60);
  seconds -= minutes * 60;
  const parts = [];
  if (hours) parts.push(hours + "h");
  if (minutes) parts.push(minutes + "m");
  if (seconds || !parts.length) parts.push(seconds + "s");
  return parts.join(" ");
}

/** Formats bytes. */
export function formatBytes(raw: any): string {
  const bytes = Math.max(0, number(raw));
  if (bytes < 1024) {
    return bytes.toLocaleString() + " B";
  }
  const units = ["KB", "MB", "GB", "TB"];
  var value = bytes;
  var unit = -1;
  do {
    value /= 1024;
    unit += 1;
  } while (value >= 1024 && unit < units.length - 1);
  return value.toLocaleString(undefined, { maximumFractionDigits: value >= 10 ? 1 : 2 }) + " " + units[unit];
}

/** Converts a machine-oriented identifier to a title-cased display label. */
export function humanLabel(raw: any): string {
  return String(raw || "")
    .replace(/_/g, " ")
    .replace(/\b\w/g, function(character) {
      return character.toUpperCase();
    });
}

/** Parses object. */
export function parseObject(raw: any): Record<string, any> {
  if (raw && typeof raw === "object") {
    return raw;
  }
  if (!raw || typeof raw !== "string") {
    return {};
  }
  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === "object") {
      return parsed;
    }
  } catch (_) {
    return {};
  }
  return {};
}

/** Maps a recorded status to its semantic color class. */
export function statusClass(raw: any): string {
  const status = String(raw || "").trim().toLowerCase().replace(/[ -]+/g, "_");
  const danger = new Set(["fail", "failed", "parse_failed", "provider_failed", "network_failed", "discard", "discarded", "error", "errored", "trash", "trashed", "purge", "purged", "reject", "rejected", "invalid", "removed"]);
  const warning = new Set(["warning", "skip", "skipped", "stale", "negative", "unresolved", "unclear", "orcid_is_unclear", "disabled", "incomplete", "below", "above", "unmatched", "not_available", "unavailable", "not_approved"]);
  const success = new Set(["complete", "completed", "valid", "success", "successful", "hit", "cache_hit", "ready", "available", "approved", "resolved", "resolved_internally", "enriched", "normalized", "linked", "linked_global_person", "match", "matched"]);
  const info = new Set(["pending", "running", "recorded", "active", "visible", "inventoried", "not_evaluated", "observed_occurrence_only"]);
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
export function StatusChip(props: { raw: any }): JSX.Element {
  return <span className={"ui " + statusClass(props.raw) + " label"}>{props.raw || "Not recorded"}</span>;
}

/** Returns escaped label markup for a recorded status. */
export function statusChip(raw: any): string {
  return renderToString(<StatusChip raw={raw} />);
}

/** Normalizes array- or object-backed metrics to display-name and value pairs. */
export function metricEntries(group: any): Array<[string, any]> {
  if (Array.isArray(group)) {
    return group.map(function(item) {
      const suffix = item.source ? ` (${item.source})` : "";
      return [`${item.metric || "Metric"}${suffix}`, item];
    });
  }
  return Object.entries(group || {});
}

/** Returns the pipeline run selected by the current URL context. */
export function selectedRun(): any {
  const runId = value("run_id");
  return state.runs.find(function(run) {
    return String(pickID(run)) === runId;
  });
}

/** Shows error. */
export function showError(error: any): void {
  notice.textContent = error?.message || String(error);
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
}

/** Builds an internal URL that preserves current research context and applies supplied updates. */
export function link(updates?: Record<string, any>): string {
  if (!updates) {
    updates = {};
  }
  const next = params();
  Object.entries(updates).forEach(function([key, raw]) {
    if (raw === "" || raw === null || raw === undefined) {
      next.delete(key);
    } else {
      next.set(key, String(raw));
    }
  });
  if (!next.get("view")) {
    next.set("view", "home");
  }
  return `?${next.toString()}`;
}

/** Renders the standard page header with escaped copy and optional actions. */
export function PageHeader(props: { kicker: string; title: string; description: string; extra?: string }): JSX.Element {
  return (
    <header className="rw-page-header">
      <div className="rw-page-header__main">
        {props.kicker ? <p className="rw-page-header__kicker">{props.kicker}</p> : null}
        <h2 id="page-title">{props.title}</h2>
        {props.description ? <p className="rw-page-header__description">{props.description}</p> : null}
      </div>
      {props.extra ? <div className="rw-page-header__actions">{raw(props.extra)}</div> : null}
    </header>
  );
}

/** Returns the standard page header with escaped copy and optional actions. */
export function pageHeader(kicker: string, title: string, description: string, extra?: string): string {
  return renderToString(<PageHeader kicker={kicker} title={title} description={description} extra={extra} />);
}

/** Renders escaped breadcrumb markup for an ordered page hierarchy. */
export function Breadcrumb(props: { items: Array<{ href?: string; label: string }> }): JSX.Element | null {
  const parts = Array.isArray(props.items) ? props.items : [];
  if (!parts.length) {
    return null;
  }
  const children: JSX.Element[] = [];
  parts.forEach(function(item, index) {
    if (index > 0) {
      children.push(<span className="divider" aria-hidden="true">/</span>);
    }
    if (item.href && index < parts.length - 1) {
      children.push(<a className="section" href={item.href}>{item.label}</a>);
    } else {
      children.push(<span className="section current" aria-current={index === parts.length - 1 ? "page" : undefined}>{item.label}</span>);
    }
  });
  return <nav className="ui breadcrumb" aria-label="Breadcrumb">{children}</nav>;
}

/** Returns escaped breadcrumb markup for an ordered page hierarchy. */
export function breadcrumb(items: Array<{ href?: string; label: string }>): string {
  return renderToString(<Breadcrumb items={items} />);
}

/** Replaces the shell breadcrumb with the supplied ordered page hierarchy. */
export function setBreadcrumb(items: Array<{ href?: string; label: string }>): void {
  if (breadcrumbHost) breadcrumbHost.innerHTML = breadcrumb(items);
}

/** Renders a complete empty-view state with the standard page header. */
export function EmptyState(props: { title: string; detail: string; action?: string }): JSX.Element {
  return (
    <>
      <PageHeader kicker="Read-only workspace" title={props.title} description={props.detail} />
      <section className="ui basic segment">
        <p>{props.detail}</p>
        {props.action ? raw(props.action) : null}
      </section>
    </>
  );
}

/** Returns a complete empty-view state with the standard page header. */
export function emptyState(title: string, detail: string, action?: string): string {
  return renderToString(<EmptyState title={title} detail={detail} action={action} />);
}

/** Renders a compact empty-state panel. */
export function EmptyPanel(props: { title: string; detail: string; action?: string }): JSX.Element {
  return (
    <section className="ui segment rw-empty-state">
      <h3>{props.title}</h3>
      <p>{props.detail}</p>
      {props.action ? raw(props.action) : null}
    </section>
  );
}

/** Returns compact empty-state panel markup. */
export function emptyPanel(title: string, detail: string, action?: string): string {
  return renderToString(<EmptyPanel title={title} detail={detail} action={action} />);
}

/** Renders the standard titled content panel. */
export function Panel(props: { title: string; description: string; body: string; classes?: string }): JSX.Element {
  return (
    <section className={"ui segment rw-panel " + (props.classes || "")}>
      <div className="ui top attached header rw-panel__header">
        <div>
          <h3>{props.title}</h3>
          {props.description ? <p>{props.description}</p> : null}
        </div>
      </div>
      <div className="content rw-panel__body">{raw(props.body)}</div>
    </section>
  );
}

/** Returns the standard titled content-panel markup. */
export function panel(title: string, description: string, body: string, classes?: string): string {
  return renderToString(<Panel title={title} description={description} body={body} classes={classes} />);
}

/** One table column definition: a field name or a labeled renderer. */
export type TableColumn = string | { label: string; render: (row: any) => string };

/** Renders an escaped data table inside the standard panel wrapper. */
export function Table(props: { title: string; description: string; columns: TableColumn[]; rows: any[]; classes?: string }): JSX.Element {
  const header = props.columns.map(function(column) {
    return <th scope="col">{typeof column === "string" ? column : column.label}</th>;
  });
  let body: JSX.Element[];
  if (props.rows.length) {
    body = props.rows.map(function(row) {
      const cells = props.columns.map(function(column) {
        const content = typeof column === "string" ? cell(row[column], column) : column.render(row);
        return <td>{raw(content)}</td>;
      });
      return <tr>{cells}</tr>;
    });
  } else {
    body = [<tr><td colspan={Math.max(props.columns.length, 1)} className="empty">No records.</td></tr>];
  }
  const tableWrap = (
    <div className="table-wrap" aria-label={props.title + " table"}>
      <table className="ui table">
        <thead><tr>{header}</tr></thead>
        <tbody>{body}</tbody>
      </table>
    </div>
  );
  return <Panel title={props.title} description={props.description} body={renderToString(tableWrap)} classes={props.classes} />;
}

/** Returns an escaped data table inside the standard panel wrapper. */
export function table(title: string, description: string, columns: TableColumn[], rows: any[], classes?: string): string {
  return renderToString(<Table title={title} description={description} columns={columns} rows={rows} classes={classes} />);
}

/** Renders context-preserving tab navigation for a keyed section. */
export function Subnav(props: { items: Array<[string, string]>; current: string; key: string }): JSX.Element {
  const links = props.items.map(function([id, label]) {
    const href = link({ [props.key]: id });
    const active = id === props.current ? " active" : "";
    return <a href={href} className={"item" + active} aria-current={id === props.current ? "page" : undefined}>{label}</a>;
  });
  return <nav className="ui tabular menu rw-section-tabs" aria-label="Section navigation">{links}</nav>;
}

/** Returns context-preserving tab navigation for a keyed section. */
export function subnav(items: Array<[string, string]>, current: string, key: string): string {
  return renderToString(<Subnav items={items} current={current} key={key} />);
}

/** One filter summary rendering option set. */
export interface FilterChipOptions {
  removeUpdates?: Record<string, any>;
  clearUpdates?: Record<string, any>;
}

/** Renders removable filter chips with a clear-all action. */
export function FilterChips(props: { filters: Record<string, any> | null; labels?: Record<string, string>; options?: FilterChipOptions }): JSX.Element {
  const options = props.options || {};
  const entries = Object.entries(props.filters || {}).filter(function([, raw]) {
    return raw !== "" && raw !== null && raw !== undefined;
  });
  if (!entries.length) {
    return <div className="rw-filter-summary"><span className="ui faded text">No filters applied.</span></div>;
  }
  const chips = entries.map(function([key, raw]) {
    const updates = { ...(options.removeUpdates || { page: 1 }), [key]: "" };
    const href = link(updates);
    return (
      <a className="rw-filter-chip" href={href} title="Remove filter">
        <span>{props.labels?.[key] || humanLabel(key)}:</span> {raw} <b aria-hidden="true">{"\u00D7"}</b>
      </a>
    );
  });
  let clear: JSX.Element | null = null;
  if (options.clearUpdates) {
    clear = <a className="rw-filter-clear" href={link(options.clearUpdates)}>Clear all</a>;
  }
  return (
    <div className="rw-filter-summary">
      <strong>Applied filters</strong>
      <div className="rw-filter-chips">{chips}{clear}</div>
    </div>
  );
}

/** Filters chips. */
export function filterChips(filters: Record<string, any> | null, labels?: Record<string, string>, options?: FilterChipOptions): string {
  return renderToString(<FilterChips filters={filters} labels={labels} options={options} />);
}

/** Renders a metric card with availability, denominator, and optional navigation. */
export function MetricCard(props: { name: string; metric: any; href?: string }): JSX.Element {
  const unavailable = props.metric?.available === false;
  let content: JSX.Element;
  if (unavailable) {
    content = (
      <>
        <span className="label">{humanLabel(props.name)}</span>
        <span className="value">Not recorded</span>
        <small>Not captured for this run</small>
      </>
    );
  } else {
    const value = formatNumber(props.metric?.value ?? props.metric);
    let detail: JSX.Element | null = null;
    if (props.metric?.denominator != null) {
      const pct = props.metric.percentage ?? percent(props.metric.value, props.metric.denominator);
      detail = <small>{formatNumber(props.metric.value)} of {formatNumber(props.metric.denominator)} ({pct})</small>;
    } else if (props.metric?.basis || props.metric?.unit) {
      detail = <small>{props.metric.basis || props.metric.unit}</small>;
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
    return <div className="ui statistic rw-kpi"><a href={props.href}>{content}</a></div>;
  }
  return <div className="ui statistic rw-kpi">{content}</div>;
}

/** Returns a metric card with availability, denominator, and optional navigation. */
export function metricCard(name: string, metric: any, href?: string): string {
  return renderToString(<MetricCard name={name} metric={metric} href={href} />);
}

/** One retention-flow stage option set. */
export interface FlowStageOptions {
  description?: string;
  denominatorLabel?: string;
  baselineLabel?: string;
  href?: string;
  outcomes?: Array<{ label: string; value: any }>;
}

/** Renders one retention-flow stage with counts, percentages, and optional links. */
export function FlowStage(props: { label: string; raw: any; base: any; previous: any; extraClass: string; stageKey: string; options: FlowStageOptions }): JSX.Element {
  const extraClass = props.extraClass || "";
  const options = props.options || {};
  const stageClass = "ui step rw-flow__step" + (extraClass ? " rw-flow__step--" + extraClass.replace(/\s+/g, "-") : "");
  const info = options.description ? (
    <details className="rw-flow__info">
      <summary aria-label={"About " + props.label}>i</summary>
      <div><strong>{props.label}</strong><p>{options.description}</p></div>
    </details>
  ) : null;

  if (props.raw?.available === false || props.raw == null) {
    return (
      <article className={stageClass + " disabled"} data-flow-stage={props.stageKey || undefined}>
        {info}
        <div className="rw-flow__content">
          <h5>{props.label}</h5>
          <strong className="rw-flow__unavailable">Not recorded</strong>
          <small>This stage was not captured.</small>
        </div>
      </article>
    );
  }

  const count = number(props.raw);
  const baseCount = number(props.base);
  const percentage = baseCount > 0 ? count * 100 / baseCount : null;
  const percentageText = percentage == null ? "\u2014" : percentage.toFixed(2) + "%";
  const progressWidth = percentage == null ? 0 : Math.max(0, Math.min(100, percentage));
  const denominatorText = options.denominatorLabel || "input records";
  let change: string;
  if (props.previous == null) {
    change = options.baselineLabel || "Input baseline";
  } else {
    const diff = props.previous - count;
    if (diff === 0) {
      change = "No change from prior";
    } else {
      const sign = diff > 0 ? "\u2212" : "+";
      change = sign + formatNumber(Math.abs(diff)) + " from prior";
    }
  }

  let outcomes: JSX.Element | null = null;
  if (Array.isArray(options.outcomes)) {
    outcomes = (
      <div className="rw-flow__outcome-values">
        {options.outcomes.map(function(outcome) {
          return <span><b>{formatNumber(outcome.value)}</b> {outcome.label}</span>;
        })}
      </div>
    );
  }

  const content = (
    <div className="rw-flow__content">
      <h5>{props.label}</h5>
      <div className="rw-flow__value">
        <strong>{formatNumber(count)}</strong>
        <span className="rw-flow__percentage">{percentageText}</span>
      </div>
      <span className="rw-flow__progress" role="img" aria-label={props.label + ": " + formatNumber(count) + " of " + formatNumber(baseCount) + " " + denominatorText + " (" + percentageText + ")"}>
        <span style={"width:" + progressWidth.toFixed(2) + "%"}></span>
      </span>
      <small className="rw-flow__delta">{change}</small>
      {outcomes}
    </div>
  );

  const linkedContent = options.href ? <a className="rw-flow__link" href={options.href}>{content}</a> : content;
  return (
    <article className={stageClass + (options.href ? " linked" : "")} data-flow-stage={props.stageKey || undefined}>
      {info}
      {linkedContent}
    </article>
  );
}

/** Returns one retention-flow stage with counts, percentages, and optional links. */
export function flowStage(label: string, raw: any, base: any, previous: any, extraClass: string, stageKey: string, options: FlowStageOptions): string {
  return renderToString(<FlowStage label={label} raw={raw} base={base} previous={previous} extraClass={extraClass} stageKey={stageKey} options={options} />);
}

const filterPresentations: Record<string, { groupLabel: string; label: string; description: string }> = {
  "NO_FILTER": {
    groupLabel: "Unfiltered Source Data",
    label: "Initial raw results",
    description: "Unfiltered results reported by the configured sources."
  },
  "RANGE_10_YEARS": {
    groupLabel: "Publication Range",
    label: "Publication range",
    description: "Results retained within the declared 10-year range."
  },
  "ARTICLE_ONLY": {
    groupLabel: "Document Type",
    label: "Document type",
    description: "Results retained after applying the article-only filter."
  },
  "ENGLISH_ONLY": {
    groupLabel: "Language",
    label: "Language",
    description: "Results retained after the language filter."
  }
};

/** Combines cumulative source filter counts into ordered cross-source stages. */
function sourceFilterStageSummary(items: any[]): { stages: Array<{ count: number; groupLabel: string; label: string; description: string }>; sourceCount: number } {
  var bySource = new Map<string, Array<{ count: number; filters: string[] }>>();
  (items || []).forEach(function(item) {
    if (!Array.isArray(item?.filters) || item.filters.length === 0) {
      return;
    }
    const sourceName = String(item.source || "source");
    if (!bySource.has(sourceName)) {
      bySource.set(sourceName, []);
    }
    bySource.get(sourceName)!.push({
      count: Math.max(0, number(item.count)),
      filters: item.filters.map(String)
    });
  });

  const sources = Array.from(bySource.values()).filter(function(stages) { return stages.length > 0; });
  const stageCount = sources.reduce(function(maximum, stages) {
    return Math.max(maximum, stages.length);
  }, 0);
  var stages: Array<{ count: number; groupLabel: string; label: string; description: string }> = [];
  for (var index = 0; index < stageCount; index += 1) {
    var count = 0;
    var appliedFilters = new Set<string>();
    sources.forEach(function(sourceStages) {
      const stage = sourceStages[Math.min(index, sourceStages.length - 1)];
      count += stage.count;
      if (index < sourceStages.length) {
        const currentFilter = stage.filters[stage.filters.length - 1];
        if (currentFilter) {
          appliedFilters.add(currentFilter);
        }
      }
    });
    const filters = Array.from(appliedFilters);
    const presentation = filters.length === 1 ? filterPresentations[filters[0]] : null;
    stages.push({
      count: count,
      groupLabel: presentation?.groupLabel || (index === 0 ? "Unfiltered Source Data" : "Source Filter " + (index + 1)),
      label: presentation?.label || (index === 0 ? "Initial raw results" : "Source filter stage " + (index + 1)),
      description: presentation?.description || filters.map(function(name) { return humanLabel(name); }).join(", ")
    });
  }
  return { stages: stages, sourceCount: sources.length };
}

/** Renders one titled phase in the retention-flow presentation. */
function RetentionPhase(props: { title: string; description: string; summary: string; content: string; className: string }): JSX.Element {
  return (
    <section className={"rw-retention__phase " + (props.className || "")}>
      <header className="rw-retention__phase-header">
        <div>
          <h4>{props.title}</h4>
          <p>{props.description}</p>
        </div>
        {props.summary ? <span className="ui label">{props.summary}</span> : null}
      </header>
      {raw(props.content)}
    </section>
  );
}

/** Returns one titled phase in the retention-flow presentation. */
function retentionPhase(title: string, description: string, summary: string, content: string, className: string): string {
  return renderToString(<RetentionPhase title={title} description={description} summary={summary} content={content} className={className} />);
}

/** Renders the three-phase source-selection, pipeline-processing, and corpus-enrichment flow for an overview payload. */
export function RetentionFlow(props: { overview: any }): JSX.Element {
  const overview = props.overview;
  const source = overview.retention_funnel || {};
  const input = source.input_records;
  const filterSummary = sourceFilterStageSummary(overview.source_filter_counts);
  const filterStages = filterSummary.stages;
  const hasFilterStages = filterStages.length > 0;
  const inputCount = number(input);
  const initialCount = hasFilterStages ? filterStages[0].count : inputCount;
  const denominatorLabel = hasFilterStages ? "initial raw results" : "input records";
  const sourceDefinitions = [
    ["Initial raw results", "Unfiltered results reported by the configured sources."],
    ["Publication range", "Results retained within the declared publication window."],
    ["Document type", "Results retained after applying the declared document-type filter."],
    ["Language", "Results retained after applying the declared language filter."]
  ];
  var previousFilterCount: any = null;
  const sourceSteps = sourceDefinitions.map(function(definition, index) {
    const recordedStage = filterStages[index];
    const markup = flowStage(definition[0], recordedStage?.count, initialCount, previousFilterCount, "source", "filter_" + index, {
      description: recordedStage?.description || definition[1], denominatorLabel: denominatorLabel, baselineLabel: "Initial raw-data baseline"
    });
    if (recordedStage) previousFilterCount = recordedStage.count;
    return markup;
  }).join("");
  const sourceSummary = hasFilterStages
    ? formatNumber(initialCount) + " initial raw results" + (filterSummary.sourceCount > 1 ? " across " + formatNumber(filterSummary.sourceCount) + " sources" : "")
    : "Aggregate source-filter counts were not recorded";
  var phases = retentionPhase("Source selection", "Cumulative filters applied before source export.", sourceSummary,
    `<div class="ui fluid steps rw-flow rw-flow--source">` + sourceSteps + "</div>", "rw-retention__phase--source");

  if (!input || input.available === false) {
    phases += retentionPhase("Pipeline processing", "Records loaded, parsed, and deduplicated by the pipeline.", "Pipeline counts not recorded",
      `<div class="ui fluid steps rw-flow">` + flowStage("Input records", input, initialCount, null, "", "input_records", { description: "Records read from exported source files." })
      + flowStage("Parsed articles", null, initialCount, null, "", "parsed_articles", { description: "Source records converted into article metadata." })
      + flowStage("Deduplicated articles", null, initialCount, null, "", "deduplicated_articles", { description: "Unique articles retained after source merging." }) + "</div>", "rw-retention__phase--pipeline");
    phases += retentionPhase("Corpus enrichment", "Candidate articles continue through enrichment, validation, and normalization.", "Corpus counts not recorded",
      `<div class="ui fluid steps rw-flow">` + flowStage("Candidate articles", null, initialCount, null, "", "enrichment_candidates", { description: "Deduplicated articles considered for provider enrichment." })
      + flowStage("Accepted + Discarded", null, initialCount, null, "", "validation_outcomes", { description: "Validation divides candidate articles into accepted and discarded outcomes." })
      + flowStage("Normalization", null, initialCount, null, "", "normalized_articles_processed", { description: "Accepted articles processed into canonical forms." }) + "</div>", "rw-retention__phase--corpus");
    return <Panel title="Retention flow" description="Three phases connect source selection to the analysis-ready corpus." body={'<div class="rw-retention">' + phases + "</div>"} classes="span-all rw-panel--no-separator" />;
  }

  const parsed = source.parsed_articles;
  const deduped = source.deduplicated_articles;
  const parsedCount = number(parsed);
  const dedupedCount = number(deduped);
  const validCount = number(source.valid_articles);
  const enrichmentCandidates = overview.enrichment_breakdown?.enrichment_candidates;
  const normalizedArticles = overview.normalization_breakdown?.normalized_articles_processed;
  const enrichmentCount = enrichmentCandidates == null || enrichmentCandidates?.available === false
    ? dedupedCount
    : number(enrichmentCandidates);
  const pipelinePrevious = hasFilterStages ? filterStages[filterStages.length - 1].count : null;
  const stageHref = function(stage: string) {
    if (stage === "input") return link({ view: "corpus", section: "sources", q: "", page: 1 });
    return link({ view: "provenance", section: "stages", stage_q: stage, stage_page: 1 });
  };
  const stageOptions = function(description: string, href: string): FlowStageOptions {
    return { description: description, href: href, denominatorLabel: denominatorLabel, baselineLabel: "Input baseline" };
  };
  const pipelineSteps = flowStage("Input records", input, initialCount, pipelinePrevious, "", "input_records", stageOptions("Records read from the exported source files.", stageHref("input")))
    + flowStage("Parsed articles", parsed, initialCount, inputCount, "", "parsed_articles", stageOptions("Source records converted into article metadata.", stageHref("parse")))
    + flowStage("Deduplicated articles", deduped, initialCount, parsedCount, "", "deduplicated_articles", stageOptions("Unique articles retained after source merging.", stageHref("deduplicate")));
  phases += retentionPhase("Pipeline processing", "Records move from source loading through deduplication.", formatNumber(inputCount) + " captured input records",
    `<div class="ui fluid steps rw-flow rw-flow--pipeline">` + pipelineSteps + "</div>", "rw-retention__phase--pipeline");

  const discardedCount = number(source.discarded_articles);
  const validationTotal = validCount + discardedCount;
  const corpusSteps = flowStage("Candidate articles", enrichmentCandidates, initialCount, dedupedCount, "", "enrichment_candidates", stageOptions("Deduplicated articles considered for provider enrichment.", stageHref("enrich")))
    + flowStage("Accepted + Discarded", validationTotal, initialCount, enrichmentCount, "", "validation_outcomes", {
      ...stageOptions("Validation divides candidate articles into analysis-ready and discarded outcomes.", stageHref("validate")),
      outcomes: [{ label: "accepted", value: validCount }, { label: "discarded", value: discardedCount }]
    })
    + flowStage("Normalization", normalizedArticles, initialCount, validCount, "", "normalized_articles_processed", stageOptions("Accepted articles processed into canonical forms.", stageHref("normalize")));
  phases += retentionPhase("Corpus enrichment", "Candidate articles continue through enrichment, validation, and normalization.", formatNumber(validCount) + " accepted articles",
    `<div class="ui fluid steps rw-flow rw-flow--corpus">` + corpusSteps + "</div>", "rw-retention__phase--corpus");

  return <Panel title="Retention flow" description="Three phases connect source selection to the analysis-ready corpus." body={'<div class="rw-retention">' + phases + "</div>"} classes="span-all rw-panel--no-separator" />;
}

/** Returns the three-phase source-selection, pipeline-processing, and corpus-enrichment flow for an overview payload. */
export function retentionFlow(overview: any): string {
  return renderToString(<RetentionFlow overview={overview} />);
}

/** Renders a metric breakdown table with relative bars and optional total percentages. */
export function Breakdown(props: { title: string; source: any; valueLabel?: string; useTotal?: boolean }): JSX.Element {
  const valueLabel = props.valueLabel || "Count";
  const entries = metricEntries(props.source);
  if (!entries.length) {
    return <Panel title={props.title} description="Recorded activity for this run." body={'<p class="empty">Not recorded for this run.</p>'} />;
  }

  var total: number | null;
  if (props.useTotal) {
    total = entries.reduce(function(sum, [, entry]) {
      return sum + number(entry);
    }, 0);
  } else {
    total = null;
  }

  const max = total || Math.max.apply(null, entries.map(function([, entry]) {
    return number(entry);
  }).concat([1]));

  /** Formats one breakdown value with availability and optional percentage. */
  function valueRender(row: any): string {
    if (row.raw?.available === false) {
      return `<span class="ui faded text">Not recorded</span>`;
    }
    if (!props.useTotal) {
      return `<strong class="rw-metric-value">` + formatNumber(row.raw) + "</strong>";
    }
    var pct;
    if (total! > 0) {
      pct = (number(row.raw) * 100 / total!).toFixed(2) + "%";
    } else {
      pct = "—";
    }
    return `<strong class="rw-metric-value">` + formatNumber(row.raw) + `</strong><span class="rw-metric-percent">` + pct + "</span>";
  }

  /** Returns an accessible relative-volume bar for one breakdown row. */
  function barRender(row: any): string {
    if (row.raw?.available === false) {
      return `<span class="ui faded text">—</span>`;
    }
    const pct = Math.min(100, number(row.raw) / max * 100);
    const basis = props.useTotal ? "share of total" : "relative to the largest recorded value";
    return `<span class="ui progress" role="img" aria-label="` + esc(humanLabel(row.name)) + ": " + pct.toFixed(1) + "% " + basis + `">`
      + `<span class="bar" style="width:` + pct + `%"></span></span>`;
  }

  const rows = entries.map(function([name, raw]) {
    return { name: name, raw: raw };
  });

  return <Table title={props.title} description="Recorded activity for this run." columns={[
    { label: "Metric", render: function(row) { return `<strong class="rw-metric-name">` + esc(humanLabel(row.name)) + "</strong>"; } },
    { label: valueLabel, render: valueRender },
    { label: props.useTotal ? "Share of total" : "Relative to largest", render: barRender }
  ]} rows={rows} classes="rw-metric-table" />;
}

/** Returns a metric breakdown table with relative bars and optional total percentages. */
export function breakdown(title: string, source: any, valueLabel?: string, useTotal?: boolean): string {
  return renderToString(<Breakdown title={title} source={source} valueLabel={valueLabel} useTotal={useTotal} />);
}

/** Renders the expected-versus-observed source export count table. */
export function SourceResultCountSummary(props: { items: any[] | null; classes?: string }): JSX.Element {
  const classes = props.classes || "";

  /** Formats a source count or its unavailable state. */
  function count(raw: any): string {
    if (raw == null) {
      return `<span class="muted">Not recorded</span>`;
    }
    return formatNumber(raw);
  }

  /** Returns a status chip for a source-count comparison. */
  function comparison(raw: any): string {
    if (raw) {
      return statusChip(raw);
    }
    return `<span class="muted">Not recorded</span>`;
  }

  /** Escapes an export date or returns its unavailable state. */
  function date(raw: any): string {
    if (raw) {
      return esc(raw);
    }
    return `<span class="muted">Not recorded</span>`;
  }

  const rows = (props.items || []).map(function(item) {
    return {
      source: item.source_name || "Unnamed source",
      expected: item.expected_result_count,
      observed: item.observed_result_count,
      comparison: item.result_count_comparison,
      exportDate: item.export_date,
    };
  });

  return <Table title="Source export counts"
    description="Expected is the count recorded when metadata was originally downloaded. Observed is the raw export count read by this run. The comparison is informational and never makes a run fail."
    columns={[
      { label: "Source", render: function(row) { return esc(row.source); } },
      { label: "Export date", render: function(row) { return date(row.exportDate); } },
      { label: "Expected initial count", render: function(row) { return count(row.expected); } },
      { label: "Observed raw records", render: function(row) { return count(row.observed); } },
      { label: "Comparison", render: function(row) { return comparison(row.comparison); } },
    ]} rows={rows} classes={classes} />;
}

/** Returns the expected-versus-observed source export count table. */
export function sourceResultCountSummary(items: any[] | null, classes?: string): string {
  return renderToString(<SourceResultCountSummary items={items} classes={classes} />);
}

/** Renders expandable exact-query markup for source exports. */
export function SourceSearchQueries(props: { items: any[] | null; classes?: string }): JSX.Element | null {
  const classes = props.classes || "";
  if (!props.items || !props.items.length) {
    return null;
  }

  const rows = props.items.map(function(item) {
    return (
      <details className="query-row">
        <summary>
          <span className="query-source">{item.source_name || "Unnamed source"}</span>
          <span className="ui faded text">Inspect exact query</span>
        </summary>
        <div className="query-content">
          <code className="query-code">{item.query || ""}</code>
          <button type="button" className="ui button" data-copy-text={item.query || ""}>Copy query</button>
        </div>
      </details>
    );
  });

  return <Panel title="Search queries"
    description="The exact search terms used to obtain each source export. Expand a source to inspect or copy its query."
    body={renderToString(<>{rows}</>)} classes={classes} />;
}

/** Returns expandable exact-query markup for source exports. */
export function sourceSearchQueries(items: any[] | null, classes?: string): string {
  return renderToString(<SourceSearchQueries items={items} classes={classes} />);
}

/** Renders chronological audit feed markup for generic event rows. */
export function Timeline(props: { rows: any[] }): JSX.Element {
  if (!props.rows.length) {
    return <p className="ui faded text">No records.</p>;
  }

  const items = props.rows.map(function(event) {
    const action = text(event, ["action", "event_type", "type"]);
    const entityType = text(event, ["entity_type", "entity", "source"]);
    const actor = event.actor;
    const entityId = event.entity_id;
    const meta = event.metadata_json;

    var detail = "";
    if (meta && typeof meta === "object") {
      if (meta.field && meta.provider) {
        detail = "Field <strong>" + esc(meta.field) + "</strong> enriched by <strong>" + esc(meta.provider) + "</strong>";
      } else if (meta.reasons) {
        detail = esc(Array.isArray(meta.reasons) ? meta.reasons.join("; ") : String(meta.reasons));
      } else if (meta.error) {
        detail = "Error: " + esc(meta.error);
      } else if (meta.status) {
        detail = "Status: " + esc(meta.status);
      } else if (meta.identity) {
        detail = esc(meta.identity);
      } else if (meta.reason) {
        detail = esc(meta.reason);
      } else if (meta.search_id) {
        detail = "Search: " + esc(meta.search_id);
        if (meta.revision) {
          detail = detail + " / revision " + esc(meta.revision);
        }
      }
    }

    var dotClass;
    if (action.startsWith("pipeline_")) {
      dotClass = "pipeline-dot";
    } else if (action === "field_enriched") {
      dotClass = "enrich-dot";
    } else if (action.startsWith("validation_")) {
      dotClass = "validation-dot";
    } else {
      dotClass = "default-dot";
    }

    const timestamp = formatTime(event.occurred_at || event.created_at || event.at || event.timestamp);

    return (
      <li className={"event " + dotClass}>
        {actor ? <span className="user">{actor}</span> : null}
        {entityId ? <span className="extra">{entityType} #{entityId}</span> : null}
        <strong>{action}</strong><br />
        {detail ? <span className="summary">{raw(detail)}</span> : null}<br />
        <time>{timestamp}</time>
      </li>
    );
  });

  return <ol className="ui feed">{items}</ol>;
}

/** Returns chronological audit feed markup for generic event rows. */
export function timeline(rows: any[]): string {
  return renderToString(<Timeline rows={rows} />);
}

/** Renders a table whose columns are derived from the supplied detail records. */
export function DetailTable(props: { title: string; rows: any }): JSX.Element {
  const records = list(props.rows, ["items", "rows"]);
  const columns = [...new Set(records.flatMap(function(record) {
    return Object.keys(record);
  }))];
  const colDefs = columns.map(function(key) {
    return {
      label: key,
      render: function(row: any) {
        return cell(row[key], key);
      }
    };
  });
  return <Table title={props.title} description="" columns={colDefs} rows={records} />;
}

/** Builds a table whose columns are derived from the supplied detail records. */
export function detailTable(title: string, rows: any): string {
  return renderToString(<DetailTable title={title} rows={rows} />);
}

/** One cell rendering option set. */
export interface CellOptions {
  expandLong?: boolean;
}

/** Renders and links a table cell according to its column and table context. */
export function Cell(props: { item: any; column: string; tableName?: string; options?: CellOptions }): JSX.Element {
  const tableName = props.tableName || "";
  const options = props.options || {};
  if (props.item === null || props.item === undefined) {
    return <span className="ui faded text">NULL</span>;
  }

  const full = asJSON(props.item);
  const display = full.length > 140 ? full.slice(0, 137) + "…" : full;

  var href = "";
  if (props.column === "article_id" || props.column === "work_revision_id" || (tableName === "work_revisions" && props.column === "id")) {
    href = link({ view: "article", article_id: props.item });
  }
  if (props.column === "author_id" || props.column === "author_occurrence_id" || (tableName === "author_occurrences" && props.column === "id")) {
    href = link({ view: "author", author_id: props.item });
  }
  if (props.column === "reference_id" || (tableName === "reference_mentions" && props.column === "id")) {
    href = link({ view: "reference", reference_id: props.item });
  }

  const shown = <span className="rw-cell" title={full}>{display}</span>;
  if (href) {
    return <a href={href}>{shown}</a>;
  }
  if (full.length > 140 && options.expandLong !== false) {
    return <details><summary>{shown}</summary><pre>{full}</pre></details>;
  }
  return shown;
}

/** Formats and links a table cell according to its column and table context. */
export function cell(item: any, column: string, tableName?: string, options?: CellOptions): string {
  return renderToString(<Cell item={item} column={column} tableName={tableName} options={options} />);
}

/**
 * Bind copy-to-clipboard behavior for [data-copy-text] buttons.
 * Shows "Copied!" feedback for 2 seconds, falls back to prompt().
 */
export function bindCopyButtons(): void {
  document.querySelectorAll<HTMLButtonElement>("[data-copy-text]").forEach(function(button) {
    button.addEventListener("click", async function() {
      var text = this.getAttribute("data-copy-text") || "";
      try {
        await navigator.clipboard.writeText(text);
        this.textContent = "Copied!";
        setTimeout(function(this: HTMLButtonElement) { this.textContent = "Copy"; }.bind(this), 2000);
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
  document.querySelectorAll<HTMLElement>(".ui.message > .close").forEach(function(button) {
    button.addEventListener("click", function() {
      const message = this.closest<HTMLElement>(".ui.message");
      if (message) {
        message.style.opacity = "0";
        setTimeout(function() { message.hidden = true; }, 150);
      }
    });
  });
}

/**
 * Bind loading state for buttons with [data-loading].
 * On click, the button shows a spinner and disables itself.
 */
export function bindLoadingButtons(): void {
  document.querySelectorAll<HTMLButtonElement>("[data-loading]").forEach(function(button) {
    button.addEventListener("click", function() {
      button.classList.add("loading");
      button.disabled = true;
    });
  });
}
