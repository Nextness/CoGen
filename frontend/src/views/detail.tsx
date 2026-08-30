// Immutable article, author-occurrence, and reference-mention detail views.
import {
  app, value, link, linkState, list,
  setBreadcrumb, formatTime, formatBytes, parseObject, humanLabel, bindCopyButtons,
  PageHeader, EmptyState, Panel, StatusChip, Cell, currentDetailOrigin, detailOrigin, routeOwnedKeys,
} from "../state.tsx";
import { h, Fragment, render as renderTree, cx, classAdd, classRemove } from "../jsx/jsx-runtime.ts";
import type { ClassName } from "../jsx/classes.ts";
import { api, errorMessage } from "../api.tsx";
import type {
  ArticleDetailResponse,
  ArticleRecord,
  APIQuery,
  AuthorRecord,
  AuthorDetailResponse,
  DetailCollectionPage,
  EvaluationNavigation,
  EvaluationResponse,
  Identifier,
  IdentityCandidate,
  IdentityCandidatesResponse,
  IdentityResolution,
  PDFStatus,
  ReferenceDetailResponse,
  ReferenceRecord,
  TermMatchSummary,
  WireRecord,
} from "../api/types.ts";
import { bindRecordAuditInvestigation, RecordAuditInvestigation } from "../components/audit-events.tsx";
import type { AuditEventRecord } from "../components/audit-events.tsx";
import { mountArticleReview } from "../components/review-panel.tsx";
import { mountPDFFullscreen } from "../components/pdf-fullscreen.tsx";
import type { PDFFullscreenController } from "../components/pdf-fullscreen.tsx";
import { replaceState } from "../router.tsx";

/** Typed compound class names used by this module. */
const classNames = {
  uiBasicButton: cx("ui", "basic", "button"),
  uiBasicLabel: cx("ui", "basic", "label"),
  uiErrorMessage: cx("ui", "error", "message"),
  uiFadedText: cx("ui", "faded", "text"),
  uiInfoMessage: cx("ui", "info", "message"),
  uiLabel: cx("ui", "label"),
  uiPrimaryButton: cx("ui", "primary", "button"),
  uiSegment: cx("ui", "segment"),
  uiTable: cx("ui", "table"),
  uiTopAttachedHeader: cx("ui", "top", "attached", "header"),
  uiWarningMessage: cx("ui", "warning", "message"),
};

/** One mounted related-record collection. */
interface CollectionState {
  title: string;
  description: string;
  columns: Array<{ label: string; render: (row: WireRecord) => JSX.Element }>;
  rows: WireRecord[];
  total: number;
  hasMore: boolean;
  nextCursor: string;
  currentCursor: string;
  previousCursors: string[];
  endpoint: string;
  cursorKey: string;
  error: string;
  loading: boolean;
  request: number;
}

const collectionState = new Map<string, CollectionState>();
let activeArticleReview: Awaited<ReturnType<typeof mountArticleReview>> | null = null;
let activePDFFullscreen: PDFFullscreenController | null = null;

/** Releases the article review and PDF lifecycle before another SPA view renders. */
export async function destroyActiveArticleReview(): Promise<void> {
  if (activePDFFullscreen) {
    const fullscreen = activePDFFullscreen;
    activePDFFullscreen = null;
    fullscreen.destroy();
  }
  if (activeArticleReview) {
    const review = activeArticleReview;
    activeArticleReview = null;
    await review.destroy();
  }
}

/** One context-preserving detail link target with its carried state. */
interface LinkTarget {
  href: string;
  state: Record<string, string>;
}

/** Returns a context-preserving link to a related detail record. */
function detailLink(kind: string, id: unknown): LinkTarget {
  const updates: Record<string, unknown> = {
    view: kind,
    article_id: "",
    author_id: "",
    reference_id: "",
    origin: currentDetailOrigin(),
  };
  updates[`${kind}_id`] = String(id ?? "");
  return { href: link(updates), state: linkState(updates) };
}

/** Returns the context-preserving corpus return target for a detail view. */
function backToCorpus(kind: string): LinkTarget {
  var section = "articles";
  if (kind === "author") section = "authors";
  else if (kind === "reference") section = "references";

  const updates = {
    view: "corpus",
    section: section,
    article_id: "",
    author_id: "",
    reference_id: "",
    table: "",
  };
  return { href: link(updates), state: linkState(updates) };
}

/** Renders a recorded value or its unavailable presentation. */
function recorded(raw: unknown, fallback?: JSX.Element): JSX.Element {
  if (raw === null || raw === undefined || raw === "") {
    return fallback || <span className={classNames.uiFadedText}>Not recorded</span>;
  }
  if (raw instanceof Node) return raw;
  return <>{String(raw)}</>;
}

/** One labeled detail property. */
interface DetailEntry {
  label: string;
  value: unknown;
}

/** The fields shared by the three detail presentations after endpoint dispatch. */
type RenderableDetailRecord = WireRecord & Partial<ArticleRecord & AuthorRecord & ReferenceRecord> & { id: number };

/** Renders definition-list markup for labeled record properties. */
function propertyGrid(entries: DetailEntry[], classes?: readonly ClassName[]): JSX.Element {
  const rowItems = entries.map((entry) => {
    const content = recorded(entry.value);
    return (
      <div>
        <dt>{entry.label}</dt>
        <dd>{content}</dd>
      </div>
    );
  });

  const gridClass = cx("rw-property-grid", ...(classes || []));
  return (
    <dl className={gridClass}>
      {rowItems}
    </dl>
  );
}

/** Renders compact summary-fact markup for a detail record. */
function summaryStrip(entries: DetailEntry[]): JSX.Element {
  const rowItems = entries.map((entry) => {
    const content = recorded(entry.value);
    return (
      <div>
        <dt>{entry.label}</dt>
        <dd>{content}</dd>
      </div>
    );
  });

  return (
    <dl className="rw-record-summary">
      {rowItems}
    </dl>
  );
}

/** Converts a stored mapping representation to a displayable object. */
function mappingValue(raw: unknown): JSX.Element {
  if (raw === null || raw === undefined || raw === "") return recorded(raw);
  if (Array.isArray(raw)) {
    const listItems = raw.map((item) => {
      const mappingResult = mappingValue(item);
      return <li>{mappingResult}</li>;
    });
    return <ul className="rw-mapping-values">{listItems}</ul>;
  }
  if (raw && typeof raw === "object") {
    const objectEntries = Object.entries(raw);
    const entryRows = objectEntries.map(([key, itemValue]) => {
      const valueMarkup = mappingValue(itemValue);
      return (
        <div>
          <dt>{humanLabel(key)}</dt>
          <dd>{valueMarkup}</dd>
        </div>
      );
    });
    return <dl className="rw-mapping-list">{entryRows}</dl>;
  }
  return <span className="rw-mono">{String(raw)}</span>;
}

/** Renders the parsed extension mapping stored on a work revision. */
function extensionMapping(raw: unknown): JSX.Element {
  const parsed = parseObject(raw);
  if (!Object.keys(parsed).length) return recorded(raw);
  return mappingValue(parsed);
}

/** Returns normalized keyword values from stored array or delimited input. */
function keywordValues(raw: unknown): string[] {
  if (Array.isArray(raw)) {
    const strings = raw.map(String);
    return strings.filter(Boolean);
  }
  if (raw === null || raw === undefined || raw === "") return [];
  if (typeof raw === "string") {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        const strings = parsed.map(String);
        return strings.filter(Boolean);
      }
    } catch (_) {
      // Stored legacy values may be delimited text rather than JSON.
    }

    var separator = ",";
    if (raw.includes(";")) separator = ";";
    const parts = raw.split(separator);
    const trimmed = parts.map((item) => item.trim());
    return trimmed.filter(Boolean);
  }
  return [];
}

/** Renders label markup for normalized keyword values. */
function keywordMarkup(raw: unknown): JSX.Element {
  const keywords = keywordValues(raw);
  if (!keywords.length) return <span className={classNames.uiFadedText}>Not recorded</span>;

  var rawText = JSON.stringify(raw);
  if (typeof raw === "string") rawText = raw;
  const keywordTags = keywords.map((keyword) => {
    return <span className={classNames.uiLabel}>{keyword}</span>;
  });

  return (
    <div className="rw-keywords">
      <div className="rw-keyword-tags">{keywordTags}</div>
      <details className="rw-keywords__raw">
        <summary>Show stored keyword value</summary>
        <div>
          <code>{rawText}</code>
          <button type="button" className={classNames.uiBasicButton} data-copy-text={rawText}>Copy</button>
        </div>
      </details>
    </div>
  );
}

/** Renders expandable JSON markup for a raw record. */
function rawRecord(record: WireRecord, excluded: string[]): JSX.Element | null {
  const entries = Object.entries(record);
  const rows = entries.filter(([key]) => {
    return !excluded.includes(key);
  });
  if (!rows.length) return null;

  const gridEntries = rows.map(([key, item]) => {
    if (key === "extension_data") {
      return {
        label: "Extension data",
        value: extensionMapping(item),
      };
    }
    return {
      label: humanLabel(key),
      value: <Cell item={item} column={key} />,
    };
  });

  const grid = propertyGrid(gridEntries, ["rw-property-grid--compact"]);
  return (
    <details className="rw-disclosure">
      <summary>Advanced raw record data</summary>
      <div className="rw-disclosure__content">
        {grid}
      </div>
    </details>
  );
}

/** Renders expandable markup for a related-record collection. */
function CollectionMarkup(props: { collectionKey: string; state: CollectionState }): JSX.Element {
  const collection = props.state;
  const emptyColumnSpan = Math.max(1, collection.columns.length);
  const emptyCell = <td colSpan={emptyColumnSpan} className="rw-table-empty">No records.</td>;
  var body: JSX.Element[] = [<tr>{emptyCell}</tr>];
  if (collection.rows.length) {
    body = collection.rows.map((row) => {
      const cells = collection.columns.map((column) => {
        const cellContent = column.render(row);
        return <td>{cellContent}</td>;
      });
      return <tr>{cells}</tr>;
    });
  }

  const headerCells = collection.columns.map((column) => {
    return <th scope="col">{column.label}</th>;
  });
  const table = (
    <div className="table-wrap" aria-label={`${collection.title} table`}>
      <table className={classNames.uiTable}>
        <thead>
          <tr>{headerCells}</tr>
        </thead>
        <tbody>{body}</tbody>
      </table>
    </div>
  );

  var previous: JSX.Element | null = null;
  if (collection.previousCursors.length) {
    previous = <button type="button" className={classNames.uiBasicButton} data-detail-previous disabled={collection.loading}>Previous page</button>;
  }
  var next: JSX.Element | null = null;
  if (collection.hasMore) {
    next = <button type="button" className={classNames.uiBasicButton} data-detail-next disabled={collection.loading}>Next page</button>;
  }
  var paging: JSX.Element | null = null;
  if (previous || next) paging = <nav aria-label={`${collection.title} pages`}>{previous}{next}</nav>;
  var error: JSX.Element | null = null;
  if (collection.error) error = <p className={classNames.uiErrorMessage} role="alert">{collection.error}</p>;

  const rowCount = collection.total.toLocaleString();
  return (
    <section className={classNames.uiSegment} data-detail-collection={props.collectionKey}>
      <div className={classNames.uiTopAttachedHeader}>
        <div>
          <h3>{collection.title}</h3>
          <p>{collection.description}</p>
        </div>
        <span className={classNames.uiLabel}>{rowCount}</span>
      </div>
      <div className="content">
        {table}
        {paging}
        {error}
      </div>
    </section>
  );
}

/** Mounts collection. */
function mountCollection(key: string, title: string, description: string, columns: Array<{ label: string; render: (row: WireRecord) => JSX.Element }>, source: DetailCollectionPage<WireRecord>, endpoint: string, cursorKey: string): void {
  const rows = list<WireRecord>(source, ["rows", "items"]);
  const state: CollectionState = {
    title: title,
    description: description,
    columns: columns,
    rows: rows,
    total: Number(source?.total ?? rows.length),
    hasMore: Boolean(source?.has_more),
    nextCursor: source?.next_cursor || "",
    currentCursor: "",
    previousCursors: [],
    endpoint: endpoint,
    cursorKey: cursorKey,
    error: "",
    loading: false,
    request: 0,
  };
  collectionState.set(key, state);
  renderCollection(key);
  const requestedCursor = value(cursorKey);
  if (requestedCursor) loadCollectionPage(key, requestedCursor, false);
}

/** Loads one cursor page while preserving the prior visible page after a local failure. */
async function loadCollectionPage(key: string, cursor: string, rememberCurrent: boolean): Promise<void> {
  const state = collectionState.get(key);
  if (!state) return;
  const sequence = ++state.request;
  state.loading = true;
  state.error = "";
  renderCollection(key);
  try {
    const data = await api<DetailCollectionPage<WireRecord>>(state.endpoint, { run_id: value("run_id"), limit: 25, cursor: cursor }, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    if (sequence !== state.request) return;
    if (rememberCurrent) state.previousCursors.push(state.currentCursor);
    state.rows = list(data, ["rows", "items"]);
    state.total = Number(data.total ?? state.rows.length);
    state.hasMore = Boolean(data.has_more);
    state.nextCursor = data.next_cursor || "";
    state.currentCursor = cursor;
    replaceState({ [state.cursorKey]: cursor });
  } catch (error) {
    if (sequence !== state.request) return;
    state.error = errorMessage(error, `Unable to load ${state.title.toLocaleLowerCase()}.`);
  } finally {
    if (sequence === state.request) {
      state.loading = false;
      renderCollection(key);
    }
  }
}

/** Renders collection. */
function renderCollection(key: string): void {
  const state = collectionState.get(key);
  const container = document.querySelector<HTMLElement>(`[data-detail-collection-host="${key}"]`);
  if (!state || !container) return;
  var collectionMarkup = <CollectionMarkup collectionKey={key} state={state} />;
  if (key === "author-identity") collectionMarkup = <IdentityCollectionMarkup state={state} />;
  renderTree(collectionMarkup, container);
  container.querySelector<HTMLButtonElement>("[data-detail-next]")?.addEventListener("click", () => {
    loadCollectionPage(key, state.nextCursor, true);
  });
  container.querySelector<HTMLButtonElement>("[data-detail-previous]")?.addEventListener("click", () => {
    const cursor = state.previousCursors.pop() || "";
    loadCollectionPage(key, cursor, false);
  });
  if (key === "author-identity") bindIdentityCandidatePages(value("run_id"));
}

/** Renders escaped validation or failure reason markup for a stage outcome. */
function stageReasonMarkup(raw: unknown): JSX.Element {
  if (typeof raw !== "string") return recorded(raw);

  try {
    const reasons = JSON.parse(raw);
    if (Array.isArray(reasons) && reasons.length) {
      const reasonsMap = reasons.map((reason) => {
        return <li>{reason}</li>;
      });
      return <ul className="rw-mapping-values">{reasonsMap}</ul>;
    }
  } catch (_) {
    // Validation reasons created before normalization may be plain text.
  }
  return recorded(raw);
}

/** Renders the search term coverage panel for an article revision. */
function SearchTermCoveragePanel(props: { matches: TermMatchSummary | null; record: ArticleRecord }): JSX.Element {
  if (props.matches === null || props.matches === undefined) {
    const body = <p className={classNames.uiFadedText}>No search terms recorded for this run.</p>;
    return (
      <Panel
        title="Search term coverage"
        description="Derived from the search queries recorded for this run."
        body={body}
      />
    );
  }
  const matches = props.matches;

  const fields = [
    {
      key: "title",
      label: "Title",
      recorded: Boolean(props.record.title),
    },
    {
      key: "abstract",
      label: "Abstract",
      recorded: Boolean(props.record.abstract),
    },
    {
      key: "keywords",
      label: "Keywords",
      recorded: keywordValues(props.record.keywords).length > 0,
    },
    {
      key: "keywords_plus",
      label: "Keywords plus",
      recorded: keywordValues(props.record.keywords_plus).length > 0,
    },
  ] as const;

  // Distinct term names recorded as matched in any of the article's fields.
  const matchedTermNames = new Set<string>();
  fields.forEach((field) => {
    const fieldMatches = matches[field.key] || [];
    fieldMatches.forEach((term: string) => {
      matchedTermNames.add(term);
    });
  });

  // The recorded term inventory, each [term, sources] entry split into matched and unmatched groups.
  const termEntries = Object.entries(matches.terms_with_sources || {});
  const matchedTerms = termEntries.filter(([term]) => {
    return matchedTermNames.has(term);
  });
  const unmatchedTerms = termEntries.filter(([term]) => {
    return !matchedTermNames.has(term);
  });

  const fieldElements: JSX.Element[] = fields.map((field) => {
    const fieldMatches = matches[field.key] || [];
    var content: JSX.Element = <span className={classNames.uiFadedText}>No matched terms</span>;

    if (!field.recorded) {
      content = <span className={classNames.uiFadedText}>Not recorded</span>;
    } else if (fieldMatches.length) {
      const termTags: JSX.Element[] = fieldMatches.map((term: string) => {
        return <span className={classNames.uiLabel}>{term}</span>;
      });
      content = <span className="rw-keyword-tags">{termTags}</span>;
    }

    return (
      <div className="rw-term-field">
        <span className="rw-term-field__label">{field.label}</span>
        {content}
      </div>
    );
  });

  var disclosureContent: JSX.Element = <p className={classNames.uiFadedText}>No search terms recorded.</p>;
  if (termEntries.length) {
    const matchedTermTags = matchedTerms.map(([term, sources]) => {
      return (
        <span className={classNames.uiLabel}>
          {term}
          <span className="rw-term-sources">({sources.join(", ")})</span>
        </span>
      );
    });
    const unmatchedTermTags = unmatchedTerms.map(([term, sources]) => {
      return (
        <span className={classNames.uiLabel}>
          {term}
          <span className="rw-term-sources">({sources.join(", ")})</span>
        </span>
      );
    });
    const sections: JSX.Element[] = [];
    if (matchedTerms.length) {
      sections.push(
        <Fragment>
          <p className={classNames.uiFadedText}>Matched terms</p>
          <div className="rw-keyword-tags">{matchedTermTags}</div>
        </Fragment>
      );
    }
    if (unmatchedTerms.length) {
      sections.push(
        <Fragment>
          <p className={classNames.uiFadedText}>Unmatched terms</p>
          <div className="rw-keyword-tags">{unmatchedTermTags}</div>
        </Fragment>
      );
    }
    disclosureContent = <Fragment>{sections}</Fragment>;
  }

  const matchedTotal = matches.matched_total;
  const termTotal = matches.term_total;
  const panelBody: JSX.Element = (
    <Fragment>
      <p className={classNames.uiFadedText}>{matchedTotal} of {termTotal} search terms matched this article.</p>
      <div className="rw-term-fields">{fieldElements}</div>
      <details className="rw-disclosure">
        <summary>All search terms</summary>
        <div className="rw-disclosure__content">{disclosureContent}</div>
      </details>
    </Fragment>
  );

  return (
    <Panel
      title="Search term coverage"
      description="Which recorded search terms matched this article's fields. Matching is a local approximation of the database queries."
      body={panelBody}
    />
  );
}

/** Renders the article detail view from its immutable revision payload. */
function ArticleView(props: { record: ArticleRecord; data: ArticleDetailResponse }): JSX.Element {
  const extension = parseObject(props.record.extension_data);
  const validation = extension.validation_status || props.record.validation_status || "Not recorded";
  const audits: AuditEventRecord[] = list(props.data.audit_events, ["events", "items"]);
  const enrichment = props.data.enrichment_summary || {};
  const pdf = props.data.pdf_status || { status: "not_available" };
  const providers = new Set(list(enrichment, ["providers"]));
  const enrichedFieldNames = new Set(list(enrichment, ["fields"]));

  const summary = summaryStrip([
    {
      label: "DOI",
      value: props.record.doi,
    },
    {
      label: "Year",
      value: props.record.year,
    },
    {
      label: "Journal",
      value: props.record.journal,
    },
    {
      label: "Publisher",
      value: props.record.publisher,
    },
    {
      label: "Source",
      value: props.record.source,
    },
    {
      label: "Validation",
      value: <StatusChip raw={validation} />,
    },
  ]);

  const provenance = propertyGrid([
    {
      label: "Run attempt",
      value: props.record.pipeline_run_id,
    },
    {
      label: "Work",
      value: props.record.work_id,
    },
    {
      label: "Revision",
      value: props.record.id,
    },
    {
      label: "Produced by stage",
      value: props.record.producer_stage,
    },
    {
      label: "Captured",
      value: formatTime(props.record.created_at),
    },
    {
      label: "Enrichment providers",
      value: Array.from(providers).join(", "),
    },
    {
      label: "Enriched fields",
      value: Array.from(enrichedFieldNames).join(", "),
    },
    {
      label: "Payload hash",
      value: props.record.payload_hash,
    },
  ]);

  var abstract: JSX.Element = <p className={classNames.uiFadedText}>No abstract was recorded for this revision.</p>;
  if (props.record.abstract) {
    const abstractText = String(props.record.abstract);
    const normalized = abstractText.replace(/\s+/g, " ");
    abstract = <p className="rw-abstract-text">{normalized.trim()}</p>;
  }

  const bibliographyGrid = propertyGrid([
    {
      label: "Keywords",
      value: keywordMarkup(props.record.keywords),
    },
    {
      label: "Keywords plus",
      value: keywordMarkup(props.record.keywords_plus),
    },
    {
      label: "Citation count",
      value: props.record.citation_count,
    },
    {
      label: "Reference count",
      value: props.record.reference_count,
    },
  ], ["rw-property-grid--compact"]);

  const bibliographyBody = (
    <Fragment>
      {abstract}
      {bibliographyGrid}
    </Fragment>
  );
  const bibliography = (
    <Panel title="Bibliographic metadata" description="Human-readable metadata for this immutable revision."
      body={bibliographyBody}
    />
  );

  const pdfPanel = <PDFStatusPanel record={props.record} pdf={pdf} />;
  const rawRecordMarkup = rawRecord(props.record, ["title", "doi", "year", "journal", "publisher", "source", "abstract", "keywords", "keywords_plus", "citation_count", "reference_count"]);

  const auditEventsBody = (
    <div data-article-audit-host>
      <RecordAuditInvestigation events={audits} collection={props.data.audit_events} endpoint={`/api/articles/${encodeURIComponent(props.record.id)}/collections/audit`} cursorKey="detail_audit_cursor" />
    </div>
  );

  return (
    <Fragment>
      {summary}
      {pdfPanel}
      <div className="rw-reading-workspace">
        <div data-pdf-viewer-host></div>
        <div data-review-host></div>
      </div>
      <SearchTermCoveragePanel matches={props.data.term_matches} record={props.record} />
      <Panel title="Provenance summary" description="Where this revision came from and how it was captured." body={provenance} />
      {bibliography}
      <div data-detail-collection-host="article-authors"></div>
      <div data-detail-collection-host="article-references"></div>
      <div data-detail-collection-host="article-stage-outcomes"></div>
      <Panel title="Audit events" description="Append-only persisted audit records for this work in the selected run." body={auditEventsBody} />
      {rawRecordMarkup}
      <span data-mount-detail-collections hidden></span>
    </Fragment>
  );
}

/** Renders PDF inventory and download-status markup for an article. */
export function PDFStatusPanel(props: { record: ArticleRecord; pdf: PDFStatus }): JSX.Element {
  const pdfLabels: Record<string, string> = {
    available: "Available",
    unavailable: "Unavailable without a DOI",
    not_available: "Not Available",
  };
  var pdfAction: JSX.Element = <span className={classNames.uiFadedText}>No stored PDF is available.</span>;
  if (props.pdf.status === "available") {
    pdfAction = <a className={classNames.uiPrimaryButton} href={`/api/pdf/${encodeURIComponent(props.record.work_id)}`} download>Download PDF</a>;
  }

  var sizeValue: string | null = null;
  if (props.pdf.byte_size) sizeValue = formatBytes(props.pdf.byte_size);
  var inventoriedValue: string | null = null;
  if (props.pdf.inventoried_at) inventoriedValue = formatTime(props.pdf.inventoried_at);

  const pdfGrid = propertyGrid([
    {
      label: "Status",
      value: <StatusChip raw={pdfLabels[props.pdf.status] || humanLabel(props.pdf.status)} />,
    },
    {
      label: "Size",
      value: sizeValue,
    },
    {
      label: "Inventoried at",
      value: inventoriedValue,
    },
    {
      label: "Content hash",
      value: props.pdf.content_hash,
    },
  ], ["rw-property-grid--compact"]);

  const pdfStatusBody = (
    <div className="rw-pdf-status-strip">
      {pdfGrid}
      <div className="rw-pdf-status-strip__action">{pdfAction}</div>
    </div>
  );
  return (
    <Panel title="Full-text PDF" description="Read-only state from the companion PDF store."
      body={pdfStatusBody}
    />
  );
}

/** Renders one ranked ORCID candidate list without implying confirmed identity. */
function IdentityCandidateList(props: { candidates: IdentityCandidate[] }): JSX.Element {
  if (!props.candidates.length) return <p className={classNames.uiFadedText}>No provider candidate was returned.</p>;
  const candidateItems = props.candidates.map((candidate) => {
    var links = <a href={candidate.query_url} target="_blank" rel="noreferrer">Provider query</a>;
    if (candidate.payload_artifact_id) {
      links = (
        <Fragment>
          {links}
          <span aria-hidden="true">·</span>
          <a href={link({ view: "provenance", section: "artifacts", artifact_id: candidate.payload_artifact_id })} data-state={JSON.stringify(linkState({ view: "provenance", section: "artifacts", artifact_id: candidate.payload_artifact_id }))}>Raw payload artifact</a>
        </Fragment>
      );
    }
    return (
      <li>
        <div>
          <strong>{candidate.candidate_orcid}</strong>
          <span>{candidate.provider_display_name || "Provider name not recorded"}</span>
        </div>
        <span className={classNames.uiBasicLabel}>Rank {candidate.provider_rank}</span>
        <div className="rw-inline-group">{links}</div>
      </li>
    );
  });
  return <ol className="rw-identity-candidate-list">{candidateItems}</ol>;
}

/** Renders candidate ORCID evidence associated with the selected author occurrence. */
function AuthorIdentityEvidence(props: { evidence: IdentityResolution[] }): JSX.Element {
  const evidence = props.evidence;
  if (!evidence.length) {
    const emptyEvidenceBody = <p className={classNames.uiFadedText}>No ORCID name-search evidence was recorded for this author occurrence.</p>;
    return <Panel title="ORCID candidate evidence" description="Name-search candidates remain uncertain evidence and are never assigned automatically." body={emptyEvidenceBody} />;
  }

  const body = evidence.map((resolution) => {
    const candidates = list<IdentityCandidate>(resolution, ["candidates"]);
    const candidateList = <IdentityCandidateList candidates={candidates} />;
    var moreCandidates: JSX.Element | null = null;
    if (resolution.candidates_truncated) {
      moreCandidates = (
        <button type="button" className={classNames.uiBasicButton} data-load-identity-candidates>
          Browse {resolution.candidate_count} candidates
        </button>
      );
    }
    var providerError: JSX.Element | null = null;
    if (resolution.error_message) {
      providerError = (
        <p className={classNames.uiWarningMessage}>
          <span className="header">Provider response</span>
          {resolution.error_message}
        </p>
      );
    }
    return (
      <section className="rw-identity-resolution" data-identity-resolution={resolution.resolution_id}>
        <header>
          <div>
            <h4>{humanLabel(resolution.provider || "ORCID")} name search</h4>
            <p>Resolved {formatTime(resolution.resolved_at)}</p>
          </div>
          <StatusChip raw={resolution.status} />
        </header>
        {providerError}
        <div data-identity-candidate-list>{candidateList}</div>
        {moreCandidates}
      </section>
    );
  });

  const evidenceBody = <Fragment>{body}</Fragment>;
  return <Panel title="ORCID candidate evidence" description="Review provider candidates here without treating a name-search match as confirmed identity." body={evidenceBody} />;
}

/** Renders one cursor page of author identity resolutions with local continuation status. */
function IdentityCollectionMarkup(props: { state: CollectionState }): JSX.Element {
  const evidence = <AuthorIdentityEvidence evidence={props.state.rows as IdentityResolution[]} />;
  var previous: JSX.Element | null = null;
  if (props.state.previousCursors.length) previous = <button type="button" className={classNames.uiBasicButton} data-detail-previous disabled={props.state.loading}>Previous identity page</button>;
  var next: JSX.Element | null = null;
  if (props.state.hasMore) next = <button type="button" className={classNames.uiBasicButton} data-detail-next disabled={props.state.loading}>Next identity page</button>;
  var controls: JSX.Element | null = null;
  if (previous || next) controls = <nav aria-label="Identity resolution pages">{previous}{next}</nav>;
  var error: JSX.Element | null = null;
  if (props.state.error) error = <p className={classNames.uiErrorMessage} role="alert">{props.state.error}</p>;
  return <section className="rw-content-stack" data-detail-collection="author-identity">{evidence}{controls}{error}</section>;
}

/** Binds on-demand traversal for paged author identity candidates. */
function bindIdentityCandidatePages(runID: string): void {
  app.querySelectorAll<HTMLElement>("[data-identity-resolution]").forEach((section) => {
    const button = section.querySelector<HTMLButtonElement>("[data-load-identity-candidates]");
    if (!button) return;
    var cursor = "";
    button.addEventListener("click", async () => {
      const resolutionID = section.dataset.identityResolution || "";
      const host = section.querySelector<HTMLElement>("[data-identity-candidate-list]")!;
      button.disabled = true;
      classAdd(button, ["loading"]);
      try {
        const page = await api<IdentityCandidatesResponse>(`/api/identity-resolutions/${encodeURIComponent(resolutionID)}/candidates`, {
          run_id: runID,
          limit: 25,
          cursor: cursor,
        }, {
          method: "GET",
          headers: { Accept: "application/json" },
        });
        const candidateMarkup = <IdentityCandidateList candidates={list(page, ["items"])} />;
        renderTree(candidateMarkup, host);
        cursor = page.next_cursor || "";
        if (cursor) {
          button.textContent = "Next candidate page";
          button.disabled = false;
          classRemove(button, "loading");
        } else {
          button.remove();
        }
      } catch (failure) {
        button.textContent = `Retry candidates: ${errorMessage(failure, "Unknown error")}`;
        button.disabled = false;
        classRemove(button, "loading");
      }
    });
  });
}

/** Renders the author occurrence detail view with related articles and audit evidence. */
function AuthorView(props: { record: AuthorRecord; data: AuthorDetailResponse }): JSX.Element {
  const articles = list(props.data.articles, ["rows", "items"]);
  const audits: AuditEventRecord[] = list(props.data.audit_events, ["events", "items"]);
  var identity = "Observed occurrence only";
  if (props.record.person_id) identity = "Linked global person";

  const summary = summaryStrip([
    {
      label: "Citation name",
      value: props.record.citation_name,
    },
    {
      label: "ORCID observed",
      value: props.record.orcid,
    },
    {
      label: "Identity status",
      value: <StatusChip raw={identity} />,
    },
    {
      label: "Articles in response",
      value: props.data.articles?.total ?? articles.length,
    },
  ]);

  const identityGrid = propertyGrid([
    {
      label: "First name",
      value: props.record.first_name,
    },
    {
      label: "Last name",
      value: props.record.last_name,
    },
    {
      label: "Observed ORCID",
      value: props.record.orcid,
    },
    {
      label: "Linked person",
      value: props.record.person_id,
    },
    {
      label: "Person ORCID",
      value: props.record.person_orcid,
    },
    {
      label: "Captured",
      value: formatTime(props.record.created_at),
    },
  ]);

  const rawRecordMarkup = rawRecord(props.record, ["citation_name", "first_name", "last_name", "orcid", "person_id", "person_orcid"]);

  const authorAuditBody = <RecordAuditInvestigation events={audits} collection={props.data.audit_events} endpoint={`/api/authors/${encodeURIComponent(props.record.id)}/collections/audit`} cursorKey="detail_audit_cursor" />;

  return (
    <Fragment>
      <div className={classNames.uiInfoMessage}>
        <span className="header">Observed author occurrence</span>
        This historical record does not establish a global person identity. Matching names remain separate unless explicit identity evidence links them.
      </div>
      {summary}
      <Panel title="Observed identity data" description="The exact name and identity values stored for this occurrence." body={identityGrid} />
      <div data-detail-collection-host="author-identity"></div>
      <div data-detail-collection-host="author-articles"></div>
      <Panel title="Audit events" description="Filter and inspect append-only events directly associated with this author occurrence." body={authorAuditBody} />
      {rawRecordMarkup}
      <span data-mount-author-articles hidden></span>
    </Fragment>
  );
}

/** Renders the reference mention detail view with citation context. */
function ReferenceView(props: { record: ReferenceRecord }): JSX.Element {
  const resolved = Boolean(props.record.resolved_revision_id);
  var resolutionLabel = "Unresolved";
  if (resolved) resolutionLabel = "Resolved internally";

  const summary = summaryStrip([
    {
      label: "Reference order",
      value: props.record.mention_order,
    },
    {
      label: "DOI",
      value: props.record.doi,
    },
    {
      label: "Year",
      value: props.record.year,
    },
    {
      label: "Resolution",
      value: <StatusChip raw={resolutionLabel} />,
    },
  ]);

  const citingGrid = propertyGrid([
    {
      label: "Article title",
      value: props.record.citing_title,
    },
    {
      label: "Article revision",
      value: <a href={detailLink("article", props.record.work_revision_id).href} data-state={JSON.stringify(detailLink("article", props.record.work_revision_id).state)}>{props.record.work_revision_id}</a>,
    },
    {
      label: "Work",
      value: props.record.work_id,
    },
    {
      label: "Run attempt",
      value: props.record.pipeline_run_id,
    },
  ]);

  var resolvedBody: JSX.Element = <p className={classNames.uiFadedText}>This reference mention was not resolved to a work revision in the selected run.</p>;
  if (resolved) {
    resolvedBody = propertyGrid([
      {
        label: "Target title",
        value: props.record.resolved_title,
      },
      {
        label: "Target revision",
        value: <a href={detailLink("article", props.record.resolved_revision_id).href} data-state={JSON.stringify(detailLink("article", props.record.resolved_revision_id).state)}>{props.record.resolved_revision_id}</a>,
      },
      {
        label: "Resolution",
        value: <StatusChip raw="Resolved internally" />,
      },
    ]);
  }

  const citedGrid = propertyGrid([
    {
      label: "Referenced title",
      value: props.record.title,
    },
    {
      label: "Referenced author",
      value: props.record.author,
    },
    {
      label: "Source",
      value: props.record.source,
    },
    {
      label: "Captured",
      value: formatTime(props.record.created_at),
    },
  ]);

  const rawRecordMarkup = rawRecord(props.record, ["title", "author", "doi", "year", "source", "mention_order", "citing_title", "resolved_title"]);

  return (
    <Fragment>
      {summary}
      <div className="rw-record-comparison">
        <Panel title="Citing article" description="The immutable article revision that contains this reference mention." body={citingGrid} />
        <Panel title="Resolved target" description="A target is shown only when the stored relationship resolved inside this run." body={resolvedBody} />
      </div>
      <Panel title="Cited-reference metadata" description="Raw bibliographic values captured for this mention." body={citedGrid} />
      {rawRecordMarkup}
    </Fragment>
  );
}

/** Asynchronously implements detail view for the viewer. */
export async function detailView(kind: string): Promise<void> {
  await destroyActiveArticleReview();
  const labels: Record<string, string> = {
    article: "Article revision",
    author: "Author occurrence",
    reference: "Reference mention",
  };
  const idKey = `${kind}_id`;
  const id = value(idKey);
  if (!id) {
    const emptyStateMarkup = <EmptyState title={labels[kind]} detail="Open a record from Corpus, Relationships, or Advanced database inspection." />;
    renderTree(emptyStateMarkup, app);
    return;
  }

  collectionState.clear();
  const apiOptions: APIQuery = { run_id: value("run_id") };
  let data: ArticleDetailResponse | AuthorDetailResponse | ReferenceDetailResponse;
  let record: RenderableDetailRecord;
  if (kind === "article") {
    const articleData = await api<ArticleDetailResponse>(`/api/articles/${encodeURIComponent(id)}`, apiOptions, { method: "GET", headers: { Accept: "application/json" } });
    data = articleData;
    record = articleData.article;
  } else if (kind === "author") {
    const authorData = await api<AuthorDetailResponse>(`/api/authors/${encodeURIComponent(id)}`, apiOptions, { method: "GET", headers: { Accept: "application/json" } });
    data = authorData;
    record = authorData.author;
  } else {
    const referenceData = await api<ReferenceDetailResponse>(`/api/references/${encodeURIComponent(id)}`, apiOptions, { method: "GET", headers: { Accept: "application/json" } });
    data = referenceData;
    record = referenceData.reference;
  }
  const origin = detailOrigin();
  var evaluationNavigation: EvaluationNavigation | null = null;
  var evaluationNavigationError = "";
  if (kind === "article" && origin?.view === "evaluation") {
    const navigationQuery: APIQuery = { current_revision_id: id };
    routeOwnedKeys.evaluation.forEach((key) => {
      if (key !== "page" && key !== "per_page") navigationQuery[key] = origin.params.get(key) || "";
    });
    navigationQuery.page = 1;
    navigationQuery.per_page = 20;
    try {
      const queue = await api<EvaluationResponse>(`/api/runs/${encodeURIComponent(value("run_id"))}/evaluation`, navigationQuery, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      evaluationNavigation = queue.queue_navigation;
    } catch (error) {
      evaluationNavigationError = errorMessage(error, "Unable to load evaluation navigation.");
    }
  }

  var title = labels[kind];
  if (kind === "article") title = record.title || labels[kind];
  else if (kind === "author") title = record.citation_name || labels[kind];
  else title = record.title || record.doi || labels[kind];

  var body: JSX.Element = <ReferenceView record={record as ReferenceRecord} />;
  if (kind === "article") {
    body = <ArticleView record={record as ArticleRecord} data={data as ArticleDetailResponse} />;
  } else if (kind === "author") {
    body = <AuthorView record={record as AuthorRecord} data={data as AuthorDetailResponse} />;
  }

  const homeHref = link({
    view: "home",
    article_id: "",
    author_id: "",
    reference_id: "",
    origin: "",
  });
  const deepdiveHref = link({
    view: "overview",
    article_id: "",
    author_id: "",
    reference_id: "",
    origin: "",
  });
  const corpusTarget = backToCorpus(kind);
  const articlesTarget = backToCorpus("article");
  const referencesTarget = backToCorpus("reference");
  const crumbs: Array<{ label: string; href?: string; state?: Record<string, string> }> = [
    {
      label: "Home",
      href: homeHref,
    },
    {
      label: "Deepdive",
      href: deepdiveHref,
    },
  ];
  if (origin) {
    crumbs.push({ label: origin.label, href: origin.href, state: origin.state });
    crumbs.push({ label: record.doi || title });
  } else if (kind === "article") {
    crumbs.push({ label: "Corpus", href: corpusTarget.href, state: corpusTarget.state });
    crumbs.push({
      label: "Analysis-ready articles",
      href: articlesTarget.href,
      state: articlesTarget.state,
    });
    crumbs.push({ label: record.doi || title });
  } else if (kind === "author") {
    crumbs.push({ label: "Corpus", href: corpusTarget.href, state: corpusTarget.state });
    crumbs.push({ label: "Author" });
  } else {
    crumbs.push({ label: "Corpus", href: corpusTarget.href, state: corpusTarget.state });
    crumbs.push({
      label: "Reference mentions",
      href: referencesTarget.href,
      state: referencesTarget.state,
    });
    crumbs.push({ label: record.doi || title });
  }
  setBreadcrumb(crumbs);

  var originActions: JSX.Element | null = null;
  if (origin) {
    var previousAction: JSX.Element | null = null;
    var nextAction: JSX.Element | null = null;
    if (evaluationNavigation?.previous_work_revision_id) {
      const updates = { view: "article", article_id: evaluationNavigation.previous_work_revision_id };
      previousAction = <a className={classNames.uiBasicButton} href={link(updates)} data-state={JSON.stringify(linkState(updates))}>Previous unreviewed</a>;
    }
    if (evaluationNavigation?.next_work_revision_id) {
      const updates = { view: "article", article_id: evaluationNavigation.next_work_revision_id };
      nextAction = <a className={classNames.uiPrimaryButton} href={link(updates)} data-state={JSON.stringify(linkState(updates))}>Next unreviewed</a>;
    }
    var navigationError: JSX.Element | null = null;
    if (evaluationNavigationError) navigationError = <span className={classNames.uiWarningMessage}>Queue navigation is unavailable: {evaluationNavigationError}</span>;
    originActions = (
      <nav aria-label="Detail record navigation">
        <a className={classNames.uiBasicButton} href={origin.href} data-state={JSON.stringify(origin.state)}>Return to {origin.label}</a>
        {previousAction}
        {nextAction}
        {navigationError}
      </nav>
    );
  }

  const page = (
    <Fragment>
      <PageHeader kicker={labels[kind]} title={title} description="" />
      {originActions}
      <article className="rw-record-detail">{body}</article>
    </Fragment>
  );
  renderTree(page, app);

  if (kind === "article") {
    const articleData = data as ArticleDetailResponse;
    mountCollection("article-authors", "Ordered authors", "Observed author occurrences in bibliographic order.", [
      {
        label: "Order",
        render: (row) => recorded(row.author_order),
      },
      {
        label: "Author occurrence",
        render: (row) => <a href={detailLink("author", row.id).href} data-state={JSON.stringify(detailLink("author", row.id).state)}>{String(row.citation_name || "Not recorded")}</a>,
      },
      {
        label: "ORCID",
        render: (row) => recorded(row.orcid),
      },
      {
        label: "Affiliation",
        render: (row) => recorded(row.affiliation),
      },
      {
        label: "Identity evidence",
        render: (row) => {
          if (row.person_id) return <StatusChip raw="Linked global person" />;
          return <StatusChip raw="Observed occurrence only" />;
        },
      },
    ], articleData.authors, `/api/articles/${encodeURIComponent(record.id)}/collections/authors`, "detail_authors_cursor");
    mountCollection("article-references", "Reference mentions", "Ordered references captured for this article revision.", [
      {
        label: "Order",
        render: (row) => recorded(row.mention_order),
      },
      {
        label: "Reference mention",
        render: (row) => <a href={detailLink("reference", row.id).href} data-state={JSON.stringify(detailLink("reference", row.id).state)}>{String(row.title || row.doi || `Reference ${row.id}`)}</a>,
      },
      {
        label: "Author",
        render: (row) => recorded(row.author),
      },
      {
        label: "Year",
        render: (row) => recorded(row.year),
      },
      {
        label: "Resolution",
        render: (row) => {
          if (row.resolved_revision_id) {
            return <a href={detailLink("article", row.resolved_revision_id).href} data-state={JSON.stringify(detailLink("article", row.resolved_revision_id).state)}><StatusChip raw="Resolved internally" /></a>;
          }
          return <StatusChip raw="Unresolved" />;
        },
      },
    ], articleData.references, `/api/articles/${encodeURIComponent(record.id)}/collections/references`, "detail_references_cursor");
    mountCollection("article-stage-outcomes", "Pipeline stage history", "Recorded per-work outcomes for the selected run. Audit events below are persisted append-only records.", [
      {
        label: "Stage",
        render: (row) => recorded(humanLabel(row.stage_name)),
      },
      {
        label: "Outcome",
        render: (row) => <StatusChip raw={row.outcome || "Not recorded"} />,
      },
      {
        label: "Reason",
        render: (row) => stageReasonMarkup(row.reason),
      },
      {
        label: "First recorded",
        render: (row) => recorded(formatTime(row.created_at)),
      },
      {
        label: "Last updated",
        render: (row) => recorded(formatTime(row.updated_at)),
      },
    ], articleData.stage_outcomes, `/api/articles/${encodeURIComponent(record.id)}/collections/stages`, "detail_stages_cursor");
    if (Number(record.id) > 0 && Number(record.work_id) > 0 && Number(record.pipeline_run_id) > 0) {
      activeArticleReview = await mountArticleReview(
        document.querySelector("[data-review-host]") as HTMLElement,
        document.querySelector<HTMLElement>("[data-pdf-viewer-host]"),
        record as ArticleRecord,
        articleData,
        async () => {
          const refreshed = await api<ArticleDetailResponse>(`/api/articles/${encodeURIComponent(record.id)}`, { run_id: value("run_id") }, {
            method: "GET",
            headers: {
              Accept: "application/json",
            },
          });
          const events = list(refreshed.audit_events, ["events", "items"]);
          const auditHost = document.querySelector("[data-article-audit-host]") as HTMLElement | null;
          if (!auditHost) return;
          const auditMarkup = <RecordAuditInvestigation events={events} />;
          renderTree(auditMarkup, auditHost);
          bindRecordAuditInvestigation(events);
        }
      );
      activePDFFullscreen = mountPDFFullscreen({
        workspace: document.querySelector(".rw-reading-workspace") as HTMLElement,
        reviewHost: document.querySelector("[data-review-host]") as HTMLElement,
      });
    }
  }
  if (kind === "author") {
    const authorData = data as AuthorDetailResponse;
    mountCollection("author-articles", "Linked article revisions", "Articles that contain this observed author occurrence.", [
      {
        label: "Article revision",
        render: (row) => <a href={detailLink("article", row.work_revision_id).href} data-state={JSON.stringify(detailLink("article", row.work_revision_id).state)}>{String(row.title || "Not recorded")}</a>,
      },
      {
        label: "Year",
        render: (row) => recorded(row.year),
      },
      {
        label: "DOI",
        render: (row) => recorded(row.doi),
      },
      {
        label: "Author order",
        render: (row) => recorded(row.author_order),
      },
      {
        label: "Affiliation",
        render: (row) => recorded(row.affiliation),
      },
    ], authorData.articles, `/api/authors/${encodeURIComponent(record.id)}/collections/articles`, "detail_articles_cursor");
    mountCollection("author-identity", "ORCID candidate evidence", "Name-search candidates remain uncertain evidence and are never assigned automatically.", [], authorData.identity_evidence, `/api/authors/${encodeURIComponent(record.id)}/collections/identity`, "detail_identity_cursor");
  }
  if ("audit_events" in data) bindRecordAuditInvestigation(list(data.audit_events, ["events", "items"]));
  bindCopyButtons();
}
