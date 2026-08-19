// Read-only execution evidence: audit events, artifacts, cache uses, stages, and run details.
import {
  app, value, link, state, provenanceSections, section,
  cell, list, selectedRun, pickID, formatTime, formatBytes,
  formatNumber, humanLabel, pageSizes, PageHeader, Subnav, EmptyPanel, Panel, StatusChip, FilterChips
} from '../state.tsx';
import { h, Fragment, raw, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api } from '../api.tsx';
import { setURL, bindFocusContext } from '../router.tsx';
import { DataTable, bindTableControls } from '../components/data-table.tsx';
import type { DataTableContext } from '../components/data-table.tsx';
import { AuditStream } from '../components/audit-events.tsx';
import type { AuditEventRecord } from '../components/audit-events.tsx';

const auditFilterKeys: Record<string, string> = {
  audit_q: "Search",
  audit_category: "Category",
  audit_action: "Event type",
  audit_actor: "Source system",
  audit_entity: "Entity type",
  audit_stage: "Pipeline stage",
  audit_outcome: "Outcome",
};

let activeArtifactPreview: any = null;
let activeArtifactRow: HTMLElement | null = null;
let auditEvents: AuditEventRecord[] = [];
let auditCursor = "";
let auditHasMore = false;

/** Renders a formatted timestamp cell for a data-table column. */
function renderTime(row: any, raw: any): JSX.Element {
  return <>{formatTime(raw)}</>;
}

/** Returns the selected comma-separated values for an audit facet. */
function selectedValues(raw: any): string[] {
  return String(raw || "").split(",").map((item) => {
    return item.trim();
  }).filter(Boolean);
}

/** Renders a multi-select control for one audit facet. */
function AuditMultiSelect(props: { name: string; label: string; options: any[]; selectedRaw: any }): JSX.Element {
  const selected = new Set(selectedValues(props.selectedRaw));
  var summary = "Any";
  if (selected.size) summary = selected.size + " selected";
  const choices = (props.options || []).map((item) => {
    return <label className="rw-check-option"><input type="checkbox" name={props.name} value={item} checked={selected.has(String(item))} /><span>{humanLabel(item)}</span></label>;
  });
  var empty: JSX.Element = <p className="ui faded text">No recorded values are available for this run.</p>;
  if (choices.length) {
    empty = <Fragment>{choices}</Fragment>;
  }
  return (
    <details className="rw-multi-select" data-multi-select>
      <summary>
        <span>{props.label}</span>
        <strong>{summary}</strong>
      </summary>
      <div className="rw-multi-select__menu">
        <div className="rw-multi-select__actions">
          <button type="button" className="ui basic button" data-multi-select-all>Select all</button>
          <button type="button" className="ui basic button" data-multi-select-clear>Clear</button>
        </div>
        {empty}
      </div>
    </details>
  );
}

/** Builds API query parameters from the active audit filters. */
function auditQuery(cursor: string): Record<string, any> {
  return {
    run_id: value("run_id"),
    q: value("audit_q"),
    category: value("audit_category"),
    action: value("audit_action"),
    actor: value("audit_actor"),
    entity_type: value("audit_entity"),
    stage: value("audit_stage"),
    outcome: value("audit_outcome"),
    limit: 25,
    cursor: cursor || "",
  };
}

/** Renders markup summarizing active audit filters and their removal links. */
function AuditFilterSummary(): JSX.Element {
  const filters: Record<string, string> = {};
  Object.keys(auditFilterKeys).forEach((key) => {
    if (value(key)) {
      filters[key] = value(key);
    }
  });
  const clearUpdates: Record<string, string> = {};
  Object.keys(auditFilterKeys).forEach((key) => {
    clearUpdates[key] = "";
  });
  return <FilterChips filters={filters} labels={auditFilterKeys} options={{ clearUpdates: clearUpdates }} />;
}

/** Renders the complete audit filter form. */
function AuditFilters(props: { facets: any }): JSX.Element {
  const actors = list(props.facets, ["actors"]);
  const actions = list(props.facets, ["actions"]);
  const entityTypes = list(props.facets, ["entity_types"]);
  const resetUpdates = Object.fromEntries(Object.keys(auditFilterKeys).map((key) => {
    return [key, ""];
  }));
  const resetLink = link(resetUpdates);
  return (
    <aside className="ui segment rw-audit-filters">
      <div className="ui top attached header">
        <div>
          <h3>Filter audit evidence</h3>
          <p>Filters are applied by the server and remain visible in the URL.</p>
        </div>
      </div>
      <div className="content">
        <form id="audit-filter-form" className="ui form">
          <label className="rw-filter-search">Search events<input name="audit_q" type="search" value={value("audit_q")} placeholder="Action, entity, source, or metadata" /></label>
          <div className="rw-filter-field-grid">
            <AuditMultiSelect name="audit_category" label="Category" options={["pipeline", "enrichment", "validation", "pdf"]} selectedRaw={value("audit_category")} />
            <AuditMultiSelect name="audit_action" label="Event type" options={actions} selectedRaw={value("audit_action")} />
            <AuditMultiSelect name="audit_actor" label="Source system" options={actors} selectedRaw={value("audit_actor")} />
            <AuditMultiSelect name="audit_entity" label="Entity type" options={entityTypes} selectedRaw={value("audit_entity")} />
          </div>
          <details className="rw-filter-disclosure">
            <summary>Stage and outcome filters</summary>
            <div className="rw-filter-field-grid rw-filter-field-grid--stacked">
              <label>Pipeline stage<input name="audit_stage" value={value("audit_stage")} placeholder="For example, normalize" /></label>
              <label>Outcome<input name="audit_outcome" value={value("audit_outcome")} placeholder="For example, completed" /></label>
            </div>
          </details>
          <AuditFilterSummary />
          <div className="rw-filter-actions">
            <button type="submit" className="ui primary button">Apply filters</button>
            <a className="ui basic button" href={resetLink}>Reset</a>
          </div>
        </form>
      </div>
    </aside>
  );
}

/** Renders summary cards for the filtered audit result. */
function AuditSummary(props: { data: any }): JSX.Element {
  const summary = props.data.summary || {};
  const actions = list(summary, ["actions"]);
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
        <dd>{formatNumber(summary.total_events || auditEvents.length)}</dd>
      </div>
      <div>
        <dt>Events loaded</dt>
        <dd data-audit-loaded-count>{formatNumber(auditEvents.length)}</dd>
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
function AuditView(props: { data: any }): JSX.Element {
  auditEvents = list(props.data, ["events", "items"]);
  auditCursor = props.data.next_cursor || "";
  auditHasMore = Boolean(props.data.has_more);
  return (
    <div className="rw-audit-layout">
      <AuditFilters facets={props.data.facets || {}} />
      <section className="ui segment rw-audit-results">
        <div className="ui top attached header">
          <div>
            <h3>Audit timeline</h3>
            <p>Newest evidence appears first. Open Recorded data only when identifiers or payload changes are needed.</p>
          </div>
        </div>
        <div className="content">
          <AuditSummary data={props.data} />
          <div id="audit-event-stream" aria-busy="false"><AuditStream events={auditEvents} /></div>
          <div className="rw-load-more" hidden={!auditHasMore}>
            <p className="ui faded text" role="status" aria-live="polite" data-audit-page-status>{formatNumber(auditEvents.length)} events loaded.</p>
            <button type="button" className="ui button" data-audit-load-more>Load 25 older events</button>
          </div>
          <p className="rw-audit-end ui success message" data-audit-end hidden={auditHasMore}>The beginning of the recorded history has been reached.</p>
          <div className="ui error message" data-audit-page-error role="alert" hidden></div>
        </div>
      </section>
    </div>
  );
}

/** Renders the research-context fields displayed for an artifact. */
function ArtifactContext(props: { context: any }): JSX.Element {
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
function ArtifactActions(props: { row: any }): JSX.Element {
  if (!props.row.has_blob) {
    return <span className="ui faded text">Payload not stored</span>;
  }
  var inspect: JSX.Element = <span className="ui label">Download only</span>;
  if (props.row.preview_available) {
    inspect = <button type="button" className="ui button" data-inspect-artifact={props.row.id}>Inspect preview</button>;
  }
  return (
    <div className="artifact-actions">
      {inspect}
      <a className="ui basic button" href={`/api/artifacts/${encodeURIComponent(props.row.id)}/content`} download>Download</a>
    </div>
  );
}

/** Renders the run artifact inventory markup. */
function ArtifactsView(props: { data: any }): JSX.Element {
  const artifacts = list(props.data, ["artifacts"]);
  activeArtifactPreview = null;
  activeArtifactRow = null;
  var rows: JSX.Element[] = [<tr><td colspan={6} className="empty">No artifacts were recorded for this run.</td></tr>];
  if (artifacts.length) {
    rows = artifacts.map((row) => {
      const role = row.artifact_roles || "Stage artifact";
      const stageParts = [
        row.produced_by_steps ? `Produced: ${row.produced_by_steps}` : "",
        row.consumed_by_steps ? `Consumed: ${row.consumed_by_steps}` : "",
      ];
      const stages = stageParts.filter(Boolean).join(" / ");
      var stageCell: JSX.Element = <span className="ui faded text">Not linked to a recorded step</span>;
      if (stages) stageCell = <>{stages}</>;
      return (
        <tr data-artifact-row={row.id}>
          <td><strong>{humanLabel(role)}</strong><small className="rw-cell-note">Artifact {row.id}</small></td>
          <td>{stageCell}</td>
          <td><span className="rw-mono">{row.content_type}</span><small className="rw-cell-note">{formatBytes(row.byte_size)}</small></td>
          <td><span className="rw-cell" title={row.content_hash}>{row.content_hash}</span></td>
          <td>{formatTime(row.created_at)}</td>
          <td><ArtifactActions row={row} /></td>
        </tr>
      );
    });
  }
  return (
    <Fragment>
      <ArtifactContext context={props.data.context || {}} />
      <p className="ui info message">Artifact inspection is read-only. Text previews are bounded before they reach the browser; download the original file for complete inspection.</p>
      <section className="ui segment rw-artifact-list">
        <div className="ui top attached header">
          <div>
            <h3>Recorded artifacts</h3>
            <p>Content-addressed files linked to this run through configuration roles or execution steps.</p>
          </div>
          <span className="ui label">{artifacts.length.toLocaleString()} artifacts</span>
        </div>
        <div className="content">
          <div className="table-wrap">
            <table className="ui table rw-artifact-table">
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
        </div>
      </section>
      <section className="ui segment rw-artifact-inspector" id="artifact-inspector">
        <div className="ui top attached header">
          <div>
            <h3>Artifact preview</h3>
            <p>Inspect a safe prefix of a text-based artifact without loading the full file into the document.</p>
          </div>
        </div>
        <div className="content"><EmptyPanel title="No artifact selected" detail="Choose Inspect preview from the artifact list above." /></div>
      </section>
    </Fragment>
  );
}

/** Renders page-size option markup with the current value selected. */
function PageSizeOptions(props: { current: any }): JSX.Element {
  const sizeOptions = pageSizes.map((size) => {
    return <option value={size} selected={Number(props.current) === size}>{size}</option>;
  });
  return <Fragment>{sizeOptions}</Fragment>;
}

/** Renders cache-use evidence and pagination markup. */
function CacheView(props: { data: any }): JSX.Element {
  const rows = list(props.data, ["rows", "cache_uses"]);
  const columns = list(props.data, ["columns"]);
  const page = Math.max(1, Number(value("cache_page") || props.data.pagination?.page || 1));
  const perPage = Number(value("cache_per_page") || props.data.pagination?.per_page || 50);
  const pagination = props.data.pagination || {
    page: page,
    per_page: perPage,
    total_rows: rows.length,
    total_pages: Math.max(1, Math.ceil(rows.length / perPage)),
  };
  const result = {
    columns: columns.length ? columns : Object.keys(rows[0] || {}),
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
    <form className="rw-table-controls" id="cache-controls">
      <label className="rw-table-search">Search cache evidence<input id="cache-query" type="search" value={value("cache_q")} placeholder="Provider, namespace, outcome, or fingerprint" /></label>
      <label>Rows per page<select id="cache-per-page"><PageSizeOptions current={perPage} /></select></label>
      <button type="submit" className="ui primary button" data-cache-search>Search</button>
      {filterSummary}
    </form>
  );
  const columnConfig: Record<string, { label?: string; className?: string; render?: (row: any, value: any) => JSX.Element }> = {
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
          return <a href={link({ section: "artifacts" })}>Artifact {raw}</a>;
        }
        return <span className="ui faded text">None</span>;
      },
    },
    request_fingerprint: { label: "Request fingerprint", className: "rw-column-fingerprint" },
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
    <Fragment>
      {controls}
      {tableMarkup}
    </Fragment>
  );
  return <Panel title="Cache use records" description="Recorded provider-response reuse and negative or stale outcomes for this run." body={body} classes="rw-cache-view" />;
}

/** Returns the effective display status for a work-stage record. */
function stageStatus(summary: any, step: any): string {
  const recorded = String(step?.step_status || "").toLocaleLowerCase();
  if (recorded) {
    return recorded;
  }
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
  return summary ? "completed" : "not-recorded";
}

/** Renders ordered stage-flow markup for one work. */
function StageFlow(props: { summaries: any[]; steps: any[] }): JSX.Element {
  const summaryByName = new Map(props.summaries.map((item) => {
    return [item.stage_name, item];
  }));
  const stepByName = new Map(props.steps.map((item) => {
    return [item.step_name, item];
  }));
  const standard = ["preflight", "load", "parse", "deduplicate", "enrich", "enrich_metadata", "enrich_identity", "validate", "normalize", "finalize"];
  const available = new Set<string>([...summaryByName.keys(), ...stepByName.keys()]);
  const names = standard.filter((name) => {
    return available.delete(name);
  }).concat(Array.from(available).sort());
  if (!names.length) {
    return <div className="rw-empty-inline"><strong>No stage progression was recorded.</strong><p>Detailed stage records are also unavailable for this run.</p></div>;
  }
  const stageItems = names.map((name, index) => {
    const summary = summaryByName.get(name);
    const step = stepByName.get(name);
    const status = stageStatus(summary, step);
    const outcomes = summary?.outcomes || {};
    const outcomeText = Object.entries(outcomes).map(([outcome, count]) => {
      return formatNumber(count) + " " + humanLabel(outcome).toLocaleLowerCase();
    }).join(", ");
    var outcomeFallback = "Stage summary recorded.";
    if (step) outcomeFallback = "Execution step recorded.";
    const outcomeDisplay = outcomeText || outcomeFallback;
    const artifacts: JSX.Element[] = [];
    if (step?.input_artifact_id) {
      artifacts.push(<a href={link({ section: "artifacts" })}>Input artifact {step.input_artifact_id}</a>);
    }
    if (step?.output_artifact_id) {
      artifacts.push(<a href={link({ section: "artifacts" })}>Output artifact {step.output_artifact_id}</a>);
    }
    var duration = "Not recorded";
    if (step?.duration_seconds != null) {
      const seconds = Number(step.duration_seconds);
      if (seconds === 0 && step.started_at && step.finished_at) {
        duration = "Less than 1 second";
      } else {
        var unit = " seconds";
        if (seconds === 1) unit = " second";
        duration = seconds.toLocaleString(undefined, { maximumFractionDigits: 3 }) + unit;
      }
    }
    var records: JSX.Element = <span className="ui faded text">Not applicable</span>;
    if (summary) {
      records = <>{formatNumber(summary.total_records)}</>;
    }
    var artifactMarkup: JSX.Element | null = null;
    if (artifacts.length) {
      artifactMarkup = <div className="rw-stage-step__artifacts">{artifacts}</div>;
    }
    const stepClass = `rw-stage-step rw-stage-step--${status}`;
    return (
      <li className={stepClass}>
        <div className="rw-stage-step__marker"><span>{index + 1}</span></div>
        <div className="rw-stage-step__body">
          <div className="rw-stage-step__heading"><h4>{humanLabel(name)}</h4><StatusChip raw={humanLabel(status)} /></div>
          <p>{outcomeDisplay}</p>
          <dl>
            <div><dt>Work outcome records</dt><dd>{records}</dd></div>
            <div><dt>Duration</dt><dd>{duration}</dd></div>
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
function StagesView(props: { data: any }): JSX.Element {
  const rows = list(props.data, ["rows"]);
  const summaries = list(props.data, ["stage_summaries"]);
  const steps = list(props.data, ["run_steps"]);
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
    <form className="rw-table-controls" id="stage-controls">
      <label className="rw-table-search">Search detailed outcomes<input id="stage-query" type="search" value={value("stage_q")} placeholder="Stage, outcome, reason, or work ID" /></label>
      <label>Rows per page<select id="stage-per-page"><PageSizeOptions current={perPage} /></select></label>
      <button type="submit" className="ui primary button" data-stage-search>Search</button>
      {stageFilterSummary}
    </form>
  );
  const columnConfig: Record<string, { label?: string; className?: string; render?: (row: any, value: any) => JSX.Element }> = {
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
    <Fragment>
      {controls}
      {details}
    </Fragment>
  );
  return (
    <Fragment>
      <section className="ui segment rw-stage-overview">
        <div className="ui top attached header">
          <div>
            <h3>Stage outcomes and progression</h3>
            <p>This view explains what happened during the run. It does not provide pipeline controls.</p>
          </div>
        </div>
        <p className="ui info message">Counts describe stored per-work outcome rows, not raw input records or field updates. A stage without per-work evidence is marked not applicable. Durations use recorded step timestamps; steps inside one timestamp tick are shown as less than one second.</p>
        <div className="content"><StageFlow summaries={summaries} steps={steps} /></div>
      </section>
      <Panel title="Detailed stage outcomes" description="Per-work evidence remains available below the run-level progression." body={body} classes="rw-stage-details" />
    </Fragment>
  );
}

/** Renders stored run details and exact configuration links. */
function RunView(props: { artifactData: any }): JSX.Element {
  const run = selectedRun();
  const plan = state.plans.find((item) => {
    return String(pickID(item)) === String(run?.execution_plan_id || value("plan_id"));
  });
  const snapshots = list(props.artifactData, ["artifacts"]).filter((artifact) => {
    return artifact.artifact_roles;
  });
  const started = run?.started_at ? new Date(run.started_at) : null;
  const finished = run?.finished_at ? new Date(run.finished_at) : null;
  var duration = "Not recorded";
  if (started && finished && !Number.isNaN(started.getTime()) && !Number.isNaN(finished.getTime())) {
    duration = Math.max(0, (finished.getTime() - started.getTime()) / 1000).toLocaleString() + " seconds";
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
  const fields: Record<string, any> = {
    run_id: run?.id || value("run_id"),
    execution_plan_id: run?.execution_plan_id || value("plan_id"),
    attempt_number: run?.attempt_number,
    status: run?.status,
    visibility: run?.visibility_state,
    started_at: run?.started_at,
    finished_at: run?.finished_at,
    summary: run?.summary,
    execution_fingerprint: plan?.execution_fingerprint,
    resolved_manifest_hash: plan?.resolved_manifest_hash,
    input_manifest_hash: plan?.input_manifest_hash,
    enrichment_enabled: plan?.enrichment_enabled,
  };
  const propertyItems = Object.entries(fields).map(([field, fieldValue]) => {
    return (
      <div>
        <dt>{humanLabel(field)}</dt>
        <dd>{raw(cell(fieldValue, field))}</dd>
      </div>
    );
  });
  const properties = (
    <dl className="property-grid property-grid--compact">{propertyItems}</dl>
  );
  var snapshotRows: JSX.Element = <p className="ui faded text">No configuration snapshots were recorded for this legacy run.</p>;
  if (snapshots.length) {
    const snapshotItems = snapshots.map((artifact) => {
      var action: JSX.Element = <span className="ui label">Payload unavailable</span>;
      if (artifact.has_blob) {
        action = <a className="ui basic button" href={`/api/artifacts/${encodeURIComponent(artifact.id)}/content`} download>Download</a>;
      }
      return (
        <li>
          <div>
            <strong>{humanLabel(artifact.artifact_roles)}</strong>
            <span>{formatBytes(artifact.byte_size)} / {artifact.content_type}</span>
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
      <Panel title="Stored run attempt" description="Immutable execution identity, lifecycle, and plan fingerprints." body={properties} classes="rw-run-record" />
      <Panel title="Configuration snapshots" description="Exact workspace configuration, resolved manifest, and input manifest captured for this attempt." body={snapshotRows} classes="rw-run-snapshots" />
    </Fragment>
  );
}

/** Asynchronously implements provenance view for the viewer. */
export async function provenanceView(): Promise<void> {
  const requestedSection = section("section", "audit");
  var current = "audit";
  if (provenanceSections[requestedSection]) current = requestedSection;
  const title = provenanceSections[current][0];
  const navItems: Array<[string, string]> = Object.entries(provenanceSections).map(([id, item]) => {
    return [id, item[0]];
  });
  if (!value("run_id") && current !== "audit") {
    const emptyStateMarkup = (
      <Fragment>
        <PageHeader kicker="Execution evidence" title="Provenance" description="Inspect append-only evidence without changing the workspace." />
        <Subnav items={navItems} current={current} key="section" />
        <EmptyPanel title={title} detail="Select a run attempt to inspect this provenance view." action={<button type="button" className="ui button" data-focus-context>Focus context selector</button>} />
      </Fragment>
    );
    renderTree(emptyStateMarkup, app);
    bindFocusContext();
    return;
  }

  var content: JSX.Element;
  if (current === "audit") {
    content = <AuditView data={await api("/api/audit", auditQuery(""), { method: "GET", headers: { Accept: "application/json" } })} />;
  } else if (current === "artifacts") {
    content = <ArtifactsView data={await api(`/api/runs/${encodeURIComponent(value("run_id"))}/artifacts`, {}, { method: "GET", headers: { Accept: "application/json" } })} />;
  } else if (current === "cache") {
    content = <CacheView data={await api(`/api/runs/${encodeURIComponent(value("run_id"))}/cache-uses`, {
      page: value("cache_page") || 1,
      per_page: value("cache_per_page") || 50,
      sort: value("cache_sort") || "id",
      order: value("cache_order") || "asc",
      q: value("cache_q"),
    }, { method: "GET", headers: { Accept: "application/json" } })} />;
  } else if (current === "stages") {
    content = <StagesView data={await api(`/api/runs/${encodeURIComponent(value("run_id"))}/stages`, {
      page: value("stage_page") || 1,
      per_page: value("stage_per_page") || 50,
      sort: value("stage_sort") || "id",
      order: value("stage_order") || "asc",
      q: value("stage_q"),
    }, { method: "GET", headers: { Accept: "application/json" } })} />;
  } else {
    content = <RunView artifactData={await api(`/api/runs/${encodeURIComponent(value("run_id"))}/artifacts`, {}, { method: "GET", headers: { Accept: "application/json" } })} />;
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
    bindTableControls("cache", Number(value("cache_page") || 1), {
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
    bindTableControls("stages", Number(value("stage_page") || 1), {
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
  const form = document.querySelector<HTMLFormElement>("#audit-filter-form");
  if (form) {
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      const values = new FormData(form);
      const updates: Record<string, any> = {};
      const multiValueKeys = new Set(["audit_category", "audit_action", "audit_actor", "audit_entity"]);
      Object.keys(auditFilterKeys).forEach((key) => {
        var keyValue = values.get(key) || "";
        if (multiValueKeys.has(key)) keyValue = values.getAll(key).join(",");
        updates[key] = keyValue;
      });
      setURL(updates, false);
    });
    form.querySelectorAll("[data-multi-select]").forEach((control) => {
      const checkboxes = Array.from(control.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
      const refresh = () => {
        const count = checkboxes.filter((input) => {
          return input.checked;
        }).length;
        var summaryText = "Any";
        if (count) summaryText = count + " selected";
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
          form.querySelectorAll<HTMLDetailsElement>("[data-multi-select][open]").forEach((other) => {
            if (other !== control) {
              other.open = false;
            }
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
      const openEventIDs = new Set(Array.from(stream.querySelectorAll(".rw-event-details[open]")).map((details) => {
        return (details.closest(".rw-audit-event") as HTMLElement | null)?.dataset.auditEventId;
      }));
      pageError.hidden = true;
      pageError.textContent = "";
      stream.setAttribute("aria-busy", "true");
      button.disabled = true;
      button.classList.add("loading");
      try {
        const data = await api("/api/audit", auditQuery(auditCursor), {
          method: "GET",
          headers: { Accept: "application/json" },
        });
        const knownIDs = new Set(auditEvents.map((event) => {
          return String(event.id);
        }));
        list(data, ["events", "items"]).forEach((event) => {
          if (!knownIDs.has(String(event.id))) {
            knownIDs.add(String(event.id));
            auditEvents.push(event as AuditEventRecord);
          }
        });
        auditCursor = data.next_cursor || "";
        auditHasMore = Boolean(data.has_more);
        const auditStreamMarkup = <AuditStream events={auditEvents} />;
        renderTree(auditStreamMarkup, stream);
        openEventIDs.forEach((id) => {
          const event = Array.from(stream.querySelectorAll(".rw-audit-event[data-audit-event-id]")).find((item) => {
            return (item as HTMLElement).dataset.auditEventId === id;
          });
          const details = event?.querySelector<HTMLDetailsElement>(".rw-event-details");
          if (details) details.open = true;
        });
        (document.querySelector("[data-audit-loaded-count]") as HTMLElement).textContent = formatNumber(auditEvents.length);
        pageStatus.textContent = `${formatNumber(auditEvents.length)} events loaded.`;
        (document.querySelector(".rw-load-more") as HTMLElement).hidden = !auditHasMore;
        (document.querySelector("[data-audit-end]") as HTMLElement).hidden = auditHasMore;
      } catch (error: any) {
        pageError.textContent = error.message || "Unable to load older audit events.";
        pageError.hidden = false;
        pageStatus.textContent = "Older events were not loaded. The current timeline is unchanged.";
      } finally {
        stream.setAttribute("aria-busy", "false");
        button.disabled = false;
        button.classList.remove("loading");
      }
    });
  }
}

/** Binds DOM behavior for artifact inspection. */
function bindArtifactInspection(): void {
  document.querySelectorAll<HTMLButtonElement>("[data-inspect-artifact]").forEach((button) => {
    button.addEventListener("click", async () => {
      const id = button.dataset.inspectArtifact as string;
      activeArtifactRow = button.closest("tr");
      document.querySelectorAll<HTMLElement>("[data-artifact-row]").forEach((row) => {
        row.classList.toggle("selected", row === activeArtifactRow);
      });
      const previous = button.textContent;
      button.disabled = true;
      button.classList.add("loading");
      try {
        const payload = await api(`/api/artifacts/${encodeURIComponent(id)}/inspect`, { preview_bytes: 65536 }, {
          method: "GET",
          headers: { Accept: "application/json" },
        });
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
        var previewMode = "raw";
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
      } catch (error: any) {
        const errorMarkup = (
          <Fragment>
            <div className="ui error message"><div className="header">Preview unavailable</div><p>{error.message || "Unable to inspect this artifact."}</p></div>
            <a className="ui button" href={`/api/artifacts/${encodeURIComponent(id)}/content`} download>Download original</a>
          </Fragment>
        );
        renderTree(errorMarkup, document.querySelector("#artifact-inspector .content") as HTMLElement);
      } finally {
        button.disabled = false;
        button.classList.remove("loading");
        button.textContent = previous;
      }
    });
  });
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
  var truncation: JSX.Element = <p className="ui success message">The complete stored text fits within the safe preview limit.</p>;
  if (preview.truncated) {
    truncation = <div className="ui warning message"><div className="header">Preview truncated</div><p>This page shows the first {formatBytes(preview.preview_byte_size)} of {formatBytes(preview.stored_byte_size || preview.byte_size)}. Download the original file to inspect its complete contents.</p></div>;
  }
  var modeButtons: JSX.Element | null = null;
  if (canFormat) {
    modeButtons = (
      <Fragment>
        <button type="button" className="ui basic button" data-artifact-preview-mode="formatted" disabled={preview.mode === "formatted"}>Formatted JSON</button>
        <button type="button" className="ui basic button" data-artifact-preview-mode="raw" disabled={preview.mode === "raw"}>Raw text</button>
      </Fragment>
    );
  }
  var formatNotice: JSX.Element | null = null;
  if (preview.formatError && preview.truncated) {
    formatNotice = <p className="ui info message">Formatted JSON is unavailable because this bounded prefix does not contain the complete document. Raw text is shown.</p>;
  } else if (preview.formatError) {
    formatNotice = <p className="ui warning message">The stored content is not valid JSON. Raw text is shown; download the original file for independent inspection.</p>;
  }
  var wrapLabel = "Wrap long lines";
  if (preview.wrap) wrapLabel = "Disable line wrapping";
  var previewClass = "rw-artifact-preview";
  if (preview.wrap) previewClass = "rw-artifact-preview rw-artifact-preview--wrap";
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
        <button type="button" className="ui basic button" data-toggle-artifact-wrap>{wrapLabel}</button>
        <button type="button" className="ui basic button" data-copy-artifact-preview>Copy displayed text</button>
        <a className="ui primary button" href={`/api/artifacts/${encodeURIComponent(id)}/content`} download>Download original</a>
      </div>
      <pre className={previewClass}>{shown}</pre>
      <p className="ui faded text" data-artifact-copy-status></p>
    </Fragment>
  );
  renderTree(inspectorMarkup, document.querySelector("#artifact-inspector .content") as HTMLElement);
  document.querySelectorAll<HTMLButtonElement>("[data-artifact-preview-mode]").forEach((button) => {
    button.addEventListener("click", () => {
      activeArtifactPreview.mode = button.dataset.artifactPreviewMode;
      renderArtifactInspector();
    });
  });
  document.querySelector("[data-toggle-artifact-wrap]")!.addEventListener("click", () => {
    activeArtifactPreview.wrap = !activeArtifactPreview.wrap;
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
  if (!copied) {
    throw new Error("Clipboard unavailable");
  }
}
