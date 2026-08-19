// Shared audit-event presentation for Provenance and immutable record details.
import { formatTime, humanLabel, link, parseObject, StatusChip } from "../state.tsx";
import { h, Fragment, render as renderTree, renderToString } from "../jsx/jsx-runtime.ts";

const recordAuditBatchSize = 25;

/** One audit event record returned by the server. */
export interface AuditEventRecord {
  id?: any;
  action?: string;
  entity_type?: string;
  entity_id?: any;
  occurred_at?: any;
  created_at?: any;
  actor?: string;
  metadata_json?: any;
  before_json?: any;
  after_json?: any;
  correlation_id?: any;
  pipeline_run_id?: any;
  [key: string]: any;
}

/** Classifies an audit event into its presentation category. */
export function auditCategory(event: AuditEventRecord): string {
  const action = String(event.action || "");
  if (action.startsWith("review_") || action.startsWith("work_review_") || action.startsWith("note_") || action.startsWith("anchor_")) {
    return "review";
  }
  if (action.startsWith("pdf_")) return "pdf";
  if (action === "field_enriched" || action.startsWith("cache_") || action === "network_fetch") {
    return "enrichment";
  }
  if (action.startsWith("validation_")) return "validation";
  return "pipeline";
}

/** Parses an audit event's stored metadata object. */
function eventMetadata(event: AuditEventRecord): Record<string, any> {
  return parseObject(event.metadata_json);
}

/** Derives the display outcome from recorded metadata and action semantics. */
function auditOutcome(event: AuditEventRecord, metadata: Record<string, any>, after: Record<string, any>): string {
  const recorded = metadata.outcome || metadata.status || metadata.cache_outcome || after.status;
  if (recorded) return String(recorded);
  const action = String(event.action || "").toLocaleLowerCase();
  if (action.includes("failed") || action.includes("error")) return "failed";
  if (action.includes("discarded") || action.includes("trashed")) return "warning";
  if (action.includes("skipped")) return "skipped";
  return "recorded";
}

/** Renders a context-preserving link or label for the affected audit entity. */
function AuditEntity(props: { event: AuditEventRecord }): JSX.Element {
  const type = String(props.event.entity_type || "record");
  const id = props.event.entity_id;
  var href = "";
  if (id && type === "work_revision") {
    href = link({
      view: "article",
      article_id: id,
    });
  } else if (id && type === "author_occurrence") {
    href = link({
      view: "author",
      author_id: id,
    });
  } else if (id && type === "reference_mention") {
    href = link({
      view: "reference",
      reference_id: id,
    });
  } else if (type === "pipeline_run") {
    href = link({
      view: "provenance",
      section: "run",
    });
  }
  var label = humanLabel(type);
  if (id) label += ` #${id}`;
  if (href) {
    return <a href={href}>{label}</a>;
  }
  return <span>{label}</span>;
}

/** Returns a concise human-readable summary of an audit event. */
function eventSummary(event: AuditEventRecord, metadata: Record<string, any>, before: Record<string, any>, after: Record<string, any>): string {
  const action = String(event.action || "").toLocaleLowerCase();
  if (action === "work_review_version_created" && before.status && after.status) {
    return `Review decision changed from ${humanLabel(before.status || "not_evaluated")} to ${humanLabel(after.status || "not_evaluated")}.`;
  }
  if (metadata.field && metadata.provider) {
    return `${humanLabel(metadata.field)} enriched by ${metadata.provider}.`;
  }
  if (metadata.reasons) {
    var reasons = [metadata.reasons];
    if (Array.isArray(metadata.reasons)) reasons = metadata.reasons;
    return reasons.join("; ");
  }
  if (metadata.error) return String(metadata.error);
  if (metadata.reason) return String(metadata.reason);
  if (metadata.search_id) {
    var revisionSuffix = ".";
    if (metadata.revision) revisionSuffix = `, revision ${metadata.revision}.`;
    return `Search ${metadata.search_id}${revisionSuffix}`;
  }
  if (action.startsWith("pdf_inventory")) {
    return "The PDF inventory state was recorded for this work.";
  }
  if (action.startsWith("pdf_document")) {
    return "A validated PDF document was recorded in the companion store.";
  }
  if (action.startsWith("pipeline_")) {
    return "The selected pipeline run changed lifecycle state.";
  }
  if (action.startsWith("cache_")) {
    return "A provider cache decision was recorded with its request evidence.";
  }
  if (action.startsWith("review_") || action.startsWith("work_review_")) {
    return "An immutable local review version was recorded.";
  }
  return "Recorded append-only audit event.";
}

/** Renders one complete previous or new review-decision state. */
function ReviewDecisionState(props: { label: string; state: Record<string, any> }): JSX.Element {
  var substatuses: any[] = [];
  if (Array.isArray(props.state.sub_statuses)) substatuses = props.state.sub_statuses;
  var substatusMarkup: JSX.Element = <span className="ui faded text">None</span>;
  if (substatuses.length) {
    const substatusLabels = substatuses.map((substatus) => {
      return <span className="ui neutral label">{humanLabel(substatus)}</span>;
    });
    substatusMarkup = <div className="rw-review-audit-substatuses">{substatusLabels}</div>;
  }
  var reasonMarkup: JSX.Element = <span className="ui faded text">Not recorded</span>;
  if (props.state.reason) reasonMarkup = <>{props.state.reason}</>;
  return (
    <section className="rw-review-audit-state">
      <h6>{props.label}</h6>
      <dl>
        <div>
          <dt>Status</dt>
          <dd><StatusChip raw={props.state.status || "not_evaluated"} /></dd>
        </div>
        <div>
          <dt>Reason</dt>
          <dd>{reasonMarkup}</dd>
        </div>
        <div>
          <dt>Subclassifications</dt>
          <dd>{substatusMarkup}</dd>
        </div>
      </dl>
    </section>
  );
}

/** Renders the visible before-and-after decision comparison for review audit events. */
function ReviewDecisionChange(props: { event: AuditEventRecord; before: Record<string, any>; after: Record<string, any> }): JSX.Element | null {
  if (String(props.event.action || "") !== "work_review_version_created" || !props.before.status || !props.after.status) return null;
  return (
    <div className="rw-review-audit-change" aria-label="Review decision change">
      <ReviewDecisionState label="Previous decision" state={props.before} />
      <ReviewDecisionState label="New decision" state={props.after} />
    </div>
  );
}

/** Returns metadata fields not already represented in the primary event presentation. */
function additionalMetadata(metadata: Record<string, any>): Record<string, any> {
  const displayed = new Set([
    "stage", "stage_name", "outcome", "status", "provider", "field", "reasons",
    "error", "reason", "search_id", "revision", "duration_seconds", "duration",
    "input_artifact_id", "output_artifact_id",
    "note_body", "body", "selected_text", "reviewer_email", "email"
  ]);
  const remaining = Object.entries(metadata).filter(([key]) => {
    return !displayed.has(key);
  });
  return safeAuditPayload(Object.fromEntries(remaining));
}

/** Removes review prose and reviewer contact fields from generic audit payload inspection. */
function safeAuditPayload(raw: any): any {
  const privateKeys = new Set(["note_body", "body", "selected_text", "reviewer_email", "email"]);
  if (Array.isArray(raw)) return raw.map(safeAuditPayload);
  if (!raw || typeof raw !== "object") return raw;
  const kept = Object.entries(raw).filter(([key]) => {
    return !privateKeys.has(String(key).toLocaleLowerCase());
  });
  const scrubbed = kept.map(([key, value]) => {
    return [key, safeAuditPayload(value)];
  });
  return Object.fromEntries(scrubbed);
}

/** Renders expandable facts and JSON payloads for an audit event. */
function EventDetails(props: { event: AuditEventRecord; metadata: Record<string, any>; before: Record<string, any>; after: Record<string, any> }): JSX.Element | null {
  var duration = props.metadata.duration;
  if (props.metadata.duration_seconds != null) duration = `${props.metadata.duration_seconds} seconds`;
  const facts = [
    ["Event ID", props.event.id],
    ["Correlation ID", props.event.correlation_id],
    ["Duration", duration],
    ["Input artifact", props.metadata.input_artifact_id],
    ["Output artifact", props.metadata.output_artifact_id],
  ].filter(([, value]) => {
    return value !== null && value !== undefined && value !== "";
  }) as Array<[string, any]>;
  const payloads = [
    ["Metadata", additionalMetadata(props.metadata)],
    ["Before", safeAuditPayload(props.before)],
    ["After", safeAuditPayload(props.after)],
  ].filter(([, value]) => {
    return Object.keys(value).length > 0;
  });
  if (!facts.length && !payloads.length) {
    return null;
  }
  var factsMarkup: JSX.Element | null = null;
  if (facts.length) {
    const factRows = facts.map(([label, value]) => {
      var shown: JSX.Element = <>{value}</>;
      if ((label === "Input artifact" || label === "Output artifact") && value) {
        shown = <a href={link({ section: "artifacts" })}>Artifact {value}</a>;
      }
      return (
        <div>
          <dt>{label}</dt>
          <dd>{shown}</dd>
        </div>
      );
    });
    factsMarkup = <dl className="rw-event-facts">{factRows}</dl>;
  }
  const payloadSections = payloads.map(([label, value]) => {
    return (
      <div>
        <h5>{label}</h5>
        <pre>{JSON.stringify(value, null, 2)}</pre>
      </div>
    );
  });
  return (
    <details className="rw-event-details">
      <summary>Recorded data</summary>
      <div className="rw-event-details__body">
        {factsMarkup}
        {payloadSections}
      </div>
    </details>
  );
}

/** Renders the complete escaped markup for one audit event. */
export function AuditEventMarkup(props: { event: AuditEventRecord }): JSX.Element {
  const metadata = eventMetadata(props.event);
  const before = parseObject(props.event.before_json);
  const after = parseObject(props.event.after_json);
  const category = auditCategory(props.event);
  const outcome = auditOutcome(props.event, metadata, after);
  const timestamp = props.event.occurred_at || props.event.created_at;
  const source = props.event.actor || metadata.provider || "Not recorded";
  const stage = metadata.stage || metadata.stage_name;
  const eventID = String(props.event.id || "unrecorded");
  var runContext = "Run not recorded";
  if (props.event.pipeline_run_id) {
    runContext = `Run ${props.event.pipeline_run_id}`;
  } else if (category === "pdf") {
    runContext = "Global PDF evidence";
  }
  const eventClass = `rw-audit-event rw-audit-event--${category}`;
  var stageMarkup: JSX.Element | null = null;
  if (stage) {
    stageMarkup = <span>Stage: <strong>{stage}</strong></span>;
  }
  return (
    <article className={eventClass} data-audit-event-id={eventID}>
      <time datetime={timestamp || ""}><span>{formatTime(timestamp)}</span></time>
      <div className="rw-audit-event__main">
        <div className="rw-audit-event__heading">
          <h5>{humanLabel(props.event.action || "event")}</h5>
          <span className="ui label">{humanLabel(category)}</span>
          <StatusChip raw={outcome} />
        </div>
        <p>{eventSummary(props.event, metadata, before, after)}</p>
        <ReviewDecisionChange event={props.event} before={before} after={after} />
        <div className="rw-audit-event__context">
          <span>Source: <strong>{source}</strong></span>
          <span>Scope: <strong>{runContext}</strong></span>
          {stageMarkup}
        </div>
        <EventDetails event={props.event} metadata={metadata} before={before} after={after} />
      </div>
      <div className="rw-audit-event__entity">
        <small>Affected record</small>
        <AuditEntity event={props.event} />
      </div>
    </article>
  );
}

/** Returns the complete escaped markup for one audit event. */
export function auditEventMarkup(event: AuditEventRecord): string {
  const markup = <AuditEventMarkup event={event} />;
  return renderToString(markup);
}

/** Renders audit events grouped by local date as a timeline. */
export function AuditStream(props: { events: AuditEventRecord[]; emptyMessage?: string }): JSX.Element {
  if (!props.events.length) {
    return (
      <div className="rw-empty-inline">
        <strong>No audit events match these filters.</strong>
        <p>{props.emptyMessage || "Broaden the filters or reset them. The selected run context has not changed."}</p>
      </div>
    );
  }
  const groups = new Map<string, AuditEventRecord[]>();
  props.events.forEach((event) => {
    const timestamp = event.occurred_at || event.created_at;
    var parsed: Date | null = null;
    if (timestamp) parsed = new Date(timestamp);
    var key = "Date not recorded";
    if (parsed && !Number.isNaN(parsed.getTime())) key = parsed.toLocaleDateString();
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key)!.push(event);
  });
  const daySections = Array.from(groups.entries()).map(([date, events], index) => {
    const headingID = `audit-day-${index}`;
    const eventItems = events.map((event) => {
      return <li><AuditEventMarkup event={event} /></li>;
    });
    return (
      <section className="rw-audit-day" aria-labelledby={headingID}>
        <h4 id={headingID}>{date}</h4>
        <ol className="rw-audit-events">{eventItems}</ol>
      </section>
    );
  });
  return <Fragment>{daySections}</Fragment>;
}

/** Groups audit events by local date and returns timeline markup. */
export function auditStream(events: AuditEventRecord[], emptyMessage?: string): string {
  const markup = <AuditStream events={events} emptyMessage={emptyMessage} />;
  return renderToString(markup);
}

/** Renders the record audit investigation controls and initial event batch. */
export function RecordAuditInvestigation(props: { events: AuditEventRecord[] }): JSX.Element {
  const actionNames = props.events.map((event) => {
    return String(event.action || "event");
  });
  const actionSet = new Set(actionNames);
  const actions = Array.from(actionSet).sort();
  const initialEvents = props.events.slice(0, recordAuditBatchSize);
  const actionOptionElements = actions.map((action) => {
    return <option value={action}>{humanLabel(action)}</option>;
  });
  const actionOptions = [
    <option value="">All event types</option>,
    ...actionOptionElements,
  ];
  return (
    <div className="rw-record-audit" data-record-audit>
      <div className="rw-record-audit__controls">
        <label>
          Search events
          <input type="search" data-record-audit-search placeholder="Action, source, identifier, or metadata" />
        </label>
        <label>
          Category
          <select data-record-audit-category>
            <option value="">All categories</option>
            <option value="pipeline">Pipeline</option>
            <option value="enrichment">Enrichment</option>
            <option value="validation">Validation</option>
            <option value="pdf">PDF</option>
          </select>
        </label>
        <label>
          Event type
          <select data-record-audit-action>{actionOptions}</select>
        </label>
      </div>
      <div className="rw-record-audit__count">
        <strong data-record-audit-count>{initialEvents.length.toLocaleString()}</strong> of
        <span data-record-audit-matches>{props.events.length.toLocaleString()}</span> events shown
      </div>
      <div data-record-audit-stream><AuditStream events={initialEvents} emptyMessage="No recorded events match the local detail filters." /></div>
      <div className="rw-record-audit__actions">
        <button type="button" className="ui button" data-record-audit-more hidden={initialEvents.length >= props.events.length}>Load {recordAuditBatchSize} more events</button>
      </div>
    </div>
  );
}

/** Records audit investigation. */
export function recordAuditInvestigation(events: AuditEventRecord[]): string {
  const markup = <RecordAuditInvestigation events={events} />;
  return renderToString(markup);
}

/** Binds DOM behavior for record audit investigation. */
export function bindRecordAuditInvestigation(events: AuditEventRecord[]): void {
  const auditRoots = document.querySelectorAll("[data-record-audit]");
  auditRoots.forEach((root) => {
    const search = root.querySelector("[data-record-audit-search]") as HTMLInputElement;
    const category = root.querySelector("[data-record-audit-category]") as HTMLSelectElement;
    const action = root.querySelector("[data-record-audit-action]") as HTMLSelectElement;
    const stream = root.querySelector("[data-record-audit-stream]") as HTMLElement;
    const count = root.querySelector("[data-record-audit-count]") as HTMLElement;
    const matches = root.querySelector("[data-record-audit-matches]") as HTMLElement;
    const more = root.querySelector("[data-record-audit-more]") as HTMLButtonElement;
    var visibleLimit = recordAuditBatchSize;
    /** Applies the current filter controls to the visible event batch. */
    function apply(): void {
      const needle = search.value.trim().toLocaleLowerCase();
      const matching = events.filter((event) => {
        if (category.value && auditCategory(event) !== category.value) {
          return false;
        }
        if (action.value && String(event.action || "") !== action.value) {
          return false;
        }
        if (!needle) {
          return true;
        }
        const serialized = JSON.stringify(event).toLocaleLowerCase();
        return serialized.includes(needle);
      });
      const visible = matching.slice(0, visibleLimit);
      count.textContent = visible.length.toLocaleString();
      matches.textContent = matching.length.toLocaleString();
      more.hidden = visible.length >= matching.length;
      const streamMarkup = <AuditStream events={visible} emptyMessage="No recorded events match the local detail filters." />;
      renderTree(streamMarkup, stream);
    }
    /** Resets the visible batch limit and reapplies the filters. */
    function resetAndApply(): void {
      visibleLimit = recordAuditBatchSize;
      apply();
    }
    search.addEventListener("input", resetAndApply);
    category.addEventListener("change", resetAndApply);
    action.addEventListener("change", resetAndApply);
    more.addEventListener("click", () => {
      visibleLimit += recordAuditBatchSize;
      apply();
    });
  });
}
