// Shared audit-event presentation for Provenance and immutable record details.
import { currentDetailOrigin, formatDate, formatTime, humanLabel, link, list, parseObject, StatusChip, value } from "../state.tsx";
import { h, Fragment, render as renderTree, cx } from "../jsx/jsx-runtime.ts";
import type { ClassName } from "../jsx/classes.ts";
import { api, errorMessage } from "../api.tsx";
import type { AuditEventRecord, AuditRecordedData, AuditResponse, DetailCollectionPage, WireRecord } from "../api/types.ts";

/** Typed compound class names used by this module. */
const classNames = {
  rwFilterBarRwRecordAuditControls: cx("rw-filter-bar", "rw-record-audit__controls"),
  uiButton: cx("ui", "button"),
  uiErrorMessage: cx("ui", "error", "message"),
  uiFadedText: cx("ui", "faded", "text"),
  uiLabel: cx("ui", "label"),
  uiNeutralLabel: cx("ui", "neutral", "label"),
  uiWarningMessage: cx("ui", "warning", "message"),
};

const recordAuditBatchSize = 25;

/** One audit event record returned by the server. */
export type { AuditEventRecord } from "../api/types.ts";

/** Classifies an audit event into its presentation category. */
export type AuditCategory = "review" | "pdf" | "enrichment" | "validation" | "pipeline";

/** Defined audit-event modifier for each presentation category. */
const auditCategoryClasses: Record<AuditCategory, ClassName> = {
  review: "rw-audit-event--review",
  pdf: "rw-audit-event--pdf",
  enrichment: "rw-audit-event--enrichment",
  validation: "rw-audit-event--validation",
  pipeline: "rw-audit-event--pipeline",
};

/** Classifies an audit event into its presentation category. */
export function auditCategory(event: AuditEventRecord): AuditCategory {
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
function eventMetadata(event: AuditEventRecord): WireRecord {
  return parseObject(event.metadata_json);
}

/** Derives the display outcome from recorded metadata and action semantics. */
function auditOutcome(event: AuditEventRecord, metadata: WireRecord, after: WireRecord): string {
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
      origin: currentDetailOrigin(),
    });
  } else if (id && type === "author_occurrence") {
    href = link({
      view: "author",
      author_id: id,
      origin: currentDetailOrigin(),
    });
  } else if (id && type === "reference_mention") {
    href = link({
      view: "reference",
      reference_id: id,
      origin: currentDetailOrigin(),
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
function eventSummary(event: AuditEventRecord, metadata: WireRecord, before: WireRecord, after: WireRecord): string {
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
function ReviewDecisionState(props: { label: string; state: WireRecord }): JSX.Element {
  var substatuses: unknown[] = [];
  if (Array.isArray(props.state.sub_statuses)) substatuses = props.state.sub_statuses;
  var substatusMarkup: JSX.Element = <span className={classNames.uiFadedText}>None</span>;
  if (substatuses.length) {
    const substatusLabels = substatuses.map((substatus) => {
      return <span className={classNames.uiNeutralLabel}>{humanLabel(substatus)}</span>;
    });
    substatusMarkup = <div className="rw-review-audit-substatuses">{substatusLabels}</div>;
  }
  var reasonMarkup: JSX.Element = <span className={classNames.uiFadedText}>Not recorded</span>;
  if (props.state.reason) reasonMarkup = <>{String(props.state.reason)}</>;
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
function ReviewDecisionChange(props: { event: AuditEventRecord; before: WireRecord; after: WireRecord }): JSX.Element | null {
  if (String(props.event.action || "") !== "work_review_version_created" || !props.before.status || !props.after.status) return null;
  return (
    <div className="rw-review-audit-change" aria-label="Review decision change">
      <ReviewDecisionState label="Previous decision" state={props.before} />
      <ReviewDecisionState label="New decision" state={props.after} />
    </div>
  );
}

/** Renders expandable facts and JSON payloads for an audit event. */
function EventDetails(props: { event: AuditEventRecord; metadata: WireRecord; before: WireRecord; after: WireRecord }): JSX.Element | null {
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
  }) as Array<[string, unknown]>;
  if (!facts.length && !props.event.id) {
    return null;
  }
  var factsMarkup: JSX.Element | null = null;
  if (facts.length) {
    const factRows = facts.map(([label, value]) => {
      var shown: JSX.Element = <>{String(value)}</>;
      if ((label === "Input artifact" || label === "Output artifact") && value) {
        shown = <a href={link({ section: "artifacts" })}>Artifact {String(value)}</a>;
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
  return (
    <details className="rw-event-details" data-audit-recorded-details={props.event.id}>
      <summary>Recorded data</summary>
      <div className="rw-event-details__body">
        {factsMarkup}
        <div data-audit-recorded-host><p className={classNames.uiFadedText}>Open this disclosure to load privacy-scrubbed recorded JSON.</p></div>
      </div>
    </details>
  );
}

/** Renders one lazy audit recorded-data response. */
function RecordedData(props: { data: AuditRecordedData }): JSX.Element {
  const candidates: Array<[string, WireRecord | null | undefined]> = [["Metadata", props.data.metadata], ["Before", props.data.before], ["After", props.data.after]];
  const sections = candidates.filter(([, item]) => {
    return item && Object.keys(item).length > 0;
  }).map(([label, item]) => {
    return <div><h5>{label}</h5><pre>{JSON.stringify(item, null, 2)}</pre></div>;
  });
  var truncation: JSX.Element | null = null;
  if (props.data.truncated_fields?.length) {
    truncation = <p className={classNames.uiWarningMessage}>The {props.data.truncated_fields.join(", ")} payload exceeded the {props.data.byte_limit.toLocaleString()} byte inspection budget and was not loaded.</p>;
  }
  if (!sections.length && !truncation) return <p className={classNames.uiFadedText}>No recorded JSON fields are available for this event.</p>;
  return <Fragment>{truncation}{sections}</Fragment>;
}

/** Binds one-shot, run-scoped loading for every visible Recorded data disclosure. */
export function bindAuditRecordedData(root: ParentNode = document): void {
  root.querySelectorAll<HTMLDetailsElement>("[data-audit-recorded-details]").forEach((details) => {
    if (details.dataset.auditRecordedBound) return;
    details.dataset.auditRecordedBound = "true";
    details.addEventListener("toggle", async () => {
      if (!details.open || details.dataset.auditRecordedLoaded) return;
      const host = details.querySelector<HTMLElement>("[data-audit-recorded-host]")!;
      const eventID = details.dataset.auditRecordedDetails || "";
      host.textContent = "Loading recorded data…";
      try {
        const data = await api<AuditRecordedData>(`/api/audit/${encodeURIComponent(eventID)}/recorded-data`, { run_id: value("run_id") }, {
          method: "GET",
          headers: { Accept: "application/json" },
        });
        const recordedDataMarkup = <RecordedData data={data} />;
        renderTree(recordedDataMarkup, host);
        details.dataset.auditRecordedLoaded = "true";
      } catch (error) {
        const errorMarkup = <p className={classNames.uiErrorMessage}>{errorMessage(error, "Unable to load recorded data.")}</p>;
        renderTree(errorMarkup, host);
      }
    });
  });
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
  const eventClass = cx("rw-audit-event", auditCategoryClasses[category]);
  var stageMarkup: JSX.Element | null = null;
  if (stage) {
    stageMarkup = <span>Stage: <strong>{String(stage)}</strong></span>;
  }
  return (
    <article className={eventClass} data-audit-event-id={eventID}>
      <time dateTime={timestamp || ""}><span>{formatTime(timestamp)}</span></time>
      <div className="rw-audit-event__main">
        <div className="rw-audit-event__heading">
          <h5>{humanLabel(props.event.action || "event")}</h5>
          <span className={classNames.uiLabel}>{humanLabel(category)}</span>
          <StatusChip raw={outcome} />
        </div>
        <p>{eventSummary(props.event, metadata, before, after)}</p>
        <ReviewDecisionChange event={props.event} before={before} after={after} />
        <div className="rw-audit-event__context">
          <span>Source: <strong>{String(source)}</strong></span>
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
    if (parsed && !Number.isNaN(parsed.getTime())) key = formatDate(parsed);
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key)!.push(event);
  });
  const daySections = Array.from(groups.entries()).map(([date, events]) => {
    const firstEventID = String(events[0]?.id || "unrecorded").replace(/[^a-zA-Z0-9_-]/g, "-");
    const headingID = `audit-day-${firstEventID}`;
    const eventItems = events.map((event) => {
      return <li><AuditEventMarkup event={event} /></li>;
    });
    return (
      <section className="rw-audit-day" aria-labelledby={headingID} data-audit-date={date}>
        <h4 id={headingID}>{date}</h4>
        <ol className="rw-audit-events">{eventItems}</ol>
      </section>
    );
  });
  return <Fragment>{daySections}</Fragment>;
}

/** Renders the record audit investigation controls and initial event batch. */
export function RecordAuditInvestigation(props: { events: AuditEventRecord[]; collection?: DetailCollectionPage<AuditEventRecord>; endpoint?: string; cursorKey?: string }): JSX.Element {
  const actionNames = props.events.map((event) => {
    return String(event.action || "event");
  });
  const actionSet = new Set(actionNames);
  const actions = Array.from(actionSet).sort();
  const initialEvents = props.events.slice(0, recordAuditBatchSize);
  const hasMore = Boolean(props.collection?.has_more);
  const nextCursor = props.collection?.next_cursor || "";
  const total = Number(props.collection?.total ?? props.events.length);
  const actionOptionElements = actions.map((action) => {
    return <option value={action}>{humanLabel(action)}</option>;
  });
  const actionOptions = [
    <option value="">All event types</option>,
    ...actionOptionElements,
  ];
  return (
    <div data-record-audit data-record-audit-endpoint={props.endpoint || ""} data-record-audit-cursor-key={props.cursorKey || ""} data-record-audit-next-cursor={nextCursor}>
      <div className={classNames.rwFilterBarRwRecordAuditControls}>
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
            <option value="review">Review</option>
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
        <span data-record-audit-matches>{total.toLocaleString()}</span> recorded events available
      </div>
      <div data-record-audit-stream><AuditStream events={initialEvents} emptyMessage="No recorded events match the local detail filters." /></div>
      <div className="rw-record-audit__actions">
        <button type="button" className={classNames.uiButton} data-record-audit-more hidden={initialEvents.length >= props.events.length && !hasMore}>Load {recordAuditBatchSize} more events</button>
        <span className={classNames.uiFadedText} data-record-audit-page-status role="status"></span>
      </div>
    </div>
  );
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
    const pageStatus = root.querySelector("[data-record-audit-page-status]") as HTMLElement;
    var visibleLimit = recordAuditBatchSize;
    var nextCursor = (root as HTMLElement).dataset.recordAuditNextCursor || "";
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
      var availableCount = matching.length;
      if (!search.value && !category.value && !action.value) {
        availableCount = Number(matches.textContent?.replace(/[^0-9]/g, "")) || matching.length;
      }
      matches.textContent = availableCount.toLocaleString();
      more.hidden = visible.length >= matching.length && !nextCursor;
      const streamMarkup = <AuditStream events={visible} emptyMessage="No recorded events match the local detail filters." />;
      renderTree(streamMarkup, stream);
      bindAuditRecordedData(stream);
    }
    /** Resets the visible batch limit and reapplies the filters. */
    function resetAndApply(): void {
      visibleLimit = recordAuditBatchSize;
      apply();
    }
    search.addEventListener("input", resetAndApply);
    category.addEventListener("change", resetAndApply);
    action.addEventListener("change", resetAndApply);
    more.addEventListener("click", async () => {
      if (visibleLimit < events.length) {
        visibleLimit += recordAuditBatchSize;
        apply();
        return;
      }
      const endpoint = (root as HTMLElement).dataset.recordAuditEndpoint || "";
      if (!endpoint || !nextCursor) return;
      more.disabled = true;
      pageStatus.textContent = "Loading older audit events…";
      try {
        const cursor = nextCursor;
        const data = await api<AuditResponse | DetailCollectionPage<AuditEventRecord>>(endpoint, { run_id: value("run_id"), limit: recordAuditBatchSize, cursor: cursor }, {
          method: "GET",
          headers: { Accept: "application/json" },
        });
        const known = new Set(events.map((event) => String(event.id)));
        list(data, ["events", "items"]).forEach((event) => {
          if (!known.has(String(event.id))) {
            known.add(String(event.id));
            events.push(event);
          }
        });
        nextCursor = String(data.next_cursor || "");
        const cursorKey = (root as HTMLElement).dataset.recordAuditCursorKey || "";
        if (cursorKey) history.replaceState({}, "", link({ [cursorKey]: cursor }));
        visibleLimit += recordAuditBatchSize;
        pageStatus.textContent = `${events.length.toLocaleString()} audit events loaded.`;
        apply();
      } catch (error) {
        pageStatus.textContent = errorMessage(error, "Unable to load older audit events.");
      } finally {
        more.disabled = false;
      }
    });
  });
  bindAuditRecordedData();
}
