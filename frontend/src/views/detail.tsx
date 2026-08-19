// Immutable article, author-occurrence, and reference-mention detail views.
import {
  app, value, link, cell, list,
  setBreadcrumb, statusChip, formatTime, formatBytes, parseObject, humanLabel, bindCopyButtons,
  PageHeader, EmptyState, Panel, StatusChip
} from '../state.tsx';
import { h, Fragment, raw, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api } from '../api.tsx';
import { pagination } from '../components/pagination.tsx';
import { bindRecordAuditInvestigation, RecordAuditInvestigation } from '../components/audit-events.tsx';
import type { AuditEventRecord } from '../components/audit-events.tsx';
import { mountArticleReview } from '../components/review-panel.tsx';

/** One mounted related-record collection. */
interface CollectionState {
  title: string;
  description: string;
  columns: Array<{ label: string; render: (row: any) => JSX.Element }>;
  rows: any[];
  page: number;
}

const collectionState = new Map<string, CollectionState>();
let activeArticleReview: any = null;

/** Releases the article review and PDF lifecycle before another SPA view renders. */
export async function destroyActiveArticleReview(): Promise<void> {
  if (activeArticleReview) {
    await activeArticleReview.destroy();
    activeArticleReview = null;
  }
}

/** Returns a context-preserving link to a related detail record. */
function detailLink(kind: string, id: any): string {
  const updates: Record<string, any> = {
    view: kind,
    article_id: "",
    author_id: "",
    reference_id: "",
  };
  updates[`${kind}_id`] = id;

  const currentKind = value("view");
  const currentID = value(`${currentKind}_id`);
  const checkList = ["article", "author", "reference"];
  if (checkList.includes(currentKind) && currentID) {
    updates.return_view = currentKind;
    updates.return_id = currentID;
  }

  return link(updates);
}

/** Returns the context-preserving corpus return URL for a detail view. */
function backToCorpus(kind: string): string {
  var section = "articles";
  if (kind === "author") section = "authors";
  else if (kind === "reference") section = "references";

  return link({
    view: "corpus",
    section: section,
    article_id: "",
    author_id: "",
    reference_id: "",
    table: "",
  });
}

/** Renders a recorded value or its unavailable presentation. */
function recorded(raw: any, fallback?: JSX.Element): JSX.Element {
  if (raw === null || raw === undefined || raw === "") {
    return fallback || <span className="ui faded text">Not recorded</span>;
  }
  return <>{raw}</>;
}

/** One labeled detail property. */
interface DetailEntry {
  label: string;
  value: any;
  html?: boolean;
}

/** Renders definition-list markup for labeled record properties. */
function propertyGrid(entries: DetailEntry[], classes?: string): JSX.Element {
  const rowItems = entries.map((entry) => {
    var content = recorded(entry.value);
    if (entry.html) content = raw(entry.value);
    return (
      <div>
        <dt>{entry.label}</dt>
        <dd>{content}</dd>
      </div>
    );
  });

  const gridClass = `property-grid ${classes || ""}`;
  return (
    <dl className={gridClass}>
      {rowItems}
    </dl>
  );
}

/** Renders compact summary-fact markup for a detail record. */
function summaryStrip(entries: DetailEntry[]): JSX.Element {
  const rowItems = entries.map((entry) => {
    var content = recorded(entry.value);
    if (entry.html) content = raw(entry.value);
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
function mappingValue(raw: any): JSX.Element {
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
  return <span className="rw-mono">{raw}</span>;
}

/** Renders the parsed extension mapping stored on a work revision. */
function extensionMapping(raw: any): JSX.Element {
  const parsed = parseObject(raw);
  if (!Object.keys(parsed).length) return recorded(raw);
  return mappingValue(parsed);
}

/** Returns normalized keyword values from stored array or delimited input. */
function keywordValues(raw: any): string[] {
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
function keywordMarkup(raw: any): JSX.Element {
  const keywords = keywordValues(raw);
  if (!keywords.length) return <span className="ui faded text">Not recorded</span>;

  var rawText = JSON.stringify(raw);
  if (typeof raw === "string") rawText = raw;
  const keywordTags = keywords.map((keyword) => {
    return <span className="ui label">{keyword}</span>;
  });

  return (
    <div className="rw-keywords">
      <div className="rw-keyword-tags">{keywordTags}</div>
      <details className="rw-keywords__raw">
        <summary>Show stored keyword value</summary>
        <div>
          <code>{rawText}</code>
          <button type="button" className="ui basic button" data-copy-text={rawText}>Copy</button>
        </div>
      </details>
    </div>
  );
}

/** Renders expandable JSON markup for a raw record. */
function rawRecord(record: Record<string, any>, excluded: string[]): JSX.Element | null {
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
      value: cell(item, key),
      html: true as const,
    };
  });

  const grid = propertyGrid(gridEntries, "property-grid--compact");
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
function CollectionMarkup(props: { collectionKey: string; title: string; description: string; columns: Array<{ label: string; render: (row: any) => JSX.Element }>; rows: any[]; page: number }): JSX.Element {
  const pageSize = 25;
  const totalPages = Math.max(1, Math.ceil(props.rows.length / pageSize));
  const currentPage = Math.min(Math.max(1, props.page), totalPages);
  const visible = props.rows.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const emptyColumnSpan = Math.max(1, props.columns.length);
  const emptyCell = <td colspan={emptyColumnSpan} className="empty">No records.</td>;
  var body: JSX.Element[] = [<tr>{emptyCell}</tr>];
  if (visible.length) {
    body = visible.map((row) => {
      const cells = props.columns.map((column) => {
        const cellContent = column.render(row);
        return <td>{cellContent}</td>;
      });
      return <tr>{cells}</tr>;
    });
  }

  const headerCells = props.columns.map((column) => {
    return <th scope="col">{column.label}</th>;
  });
  const table = (
    <div className="table-wrap" aria-label={`${props.title} table`}>
      <table className="ui table">
        <thead>
          <tr>{headerCells}</tr>
        </thead>
        <tbody>{body}</tbody>
      </table>
    </div>
  );

  const paginationConfig = {
    page: currentPage,
    per_page: pageSize,
    total_rows: props.rows.length,
    total_pages: totalPages,
  };

  const paginationOptions = {
    itemLabel: props.title.toLocaleLowerCase(),
    pageAttribute: "data-detail-page",
    pageClass: " detail-page",
    visibleCount: 5,
  };

  var pages: JSX.Element | null = null;
  if (props.rows.length > pageSize) {
    const paginationMarkup = pagination(paginationConfig, paginationOptions);
    const scopedPageLinks = paginationMarkup.replaceAll("data-detail-page=\"", `data-detail-page="${props.collectionKey}:`);
    pages = raw(scopedPageLinks);
  }

  const rowCount = props.rows.length.toLocaleString();
  return (
    <section className="ui segment rw-detail-collection" data-detail-collection={props.collectionKey}>
      <div className="ui top attached header">
        <div>
          <h3>{props.title}</h3>
          <p>{props.description}</p>
        </div>
        <span className="ui label">{rowCount}</span>
      </div>
      <div className="content">
        {table}
        {pages}
      </div>
    </section>
  );
}

/** Mounts collection. */
function mountCollection(key: string, title: string, description: string, columns: Array<{ label: string; render: (row: any) => JSX.Element }>, rows: any[]): void {
  collectionState.set(key, {
    title: title,
    description: description,
    columns: columns,
    rows: rows,
    page: 1,
  });

  renderCollection(key);
}

/** Renders collection. */
function renderCollection(key: string): void {
  const state = collectionState.get(key);
  const container = document.querySelector<HTMLElement>(`[data-detail-collection-host="${key}"]`);
  if (!state || !container) return;
  const collectionMarkup = (
    <CollectionMarkup
      collectionKey={key}
      title={state.title}
      description={state.description}
      columns={state.columns}
      rows={state.rows}
      page={state.page}
    />
  );
  renderTree(collectionMarkup, container);
  const pageButtons = container.querySelectorAll<HTMLButtonElement>("[data-detail-page]");
  pageButtons.forEach((button) => {
    button.addEventListener("click", () => {
      const parts = button.dataset.detailPage!.split(":");
      state.page = Number(parts[1]) || 1;
      renderCollection(key);
      container.scrollIntoView({
        behavior: "smooth",
        block: "nearest",
      });
    });
  });
}

/** Renders escaped validation or failure reason markup for a stage outcome. */
function stageReasonMarkup(raw: any): JSX.Element {
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
function SearchTermCoveragePanel(props: { matches: any; record: any }): JSX.Element {
  if (props.matches === null || props.matches === undefined) {
    const body = <p className="ui faded text">No search terms recorded for this run.</p>;
    return (
      <Panel
        title="Search term coverage"
        description="Derived from the search queries recorded for this run."
        body={body}
        classes="rw-detail-section"
      />
    );
  }

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
  ];

  // Distinct term names recorded as matched in any of the article's fields.
  const matchedTermNames = new Set<string>();
  fields.forEach((field) => {
    const fieldMatches = props.matches[field.key] || [];
    fieldMatches.forEach((term: string) => {
      matchedTermNames.add(term);
    });
  });

  // The recorded term inventory, each [term, sources] entry split into matched and unmatched groups.
  const termEntries = Object.entries(props.matches.terms_with_sources || {}) as Array<[string, string[]]>;
  const matchedTerms = termEntries.filter(([term]) => {
    return matchedTermNames.has(term);
  });
  const unmatchedTerms = termEntries.filter(([term]) => {
    return !matchedTermNames.has(term);
  });

  const fieldElements: JSX.Element[] = fields.map((field) => {
    const fieldMatches = props.matches[field.key] || [];
    var content: JSX.Element = <span className="ui faded text">No matched terms</span>;

    if (!field.recorded) {
      content = <span className="ui faded text">Not recorded</span>;
    } else if (fieldMatches.length) {
      const termTags: JSX.Element[] = fieldMatches.map((term: string) => {
        return <span className="ui label">{term}</span>;
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

  var disclosureContent: JSX.Element = <p className="ui faded text">No search terms recorded.</p>;
  if (termEntries.length) {
    const matchedTermTags = matchedTerms.map(([term, sources]) => {
      return (
        <span className="ui label">
          {term}
          <span className="rw-term-sources">({sources.join(", ")})</span>
        </span>
      );
    });
    const unmatchedTermTags = unmatchedTerms.map(([term, sources]) => {
      return (
        <span className="ui label">
          {term}
          <span className="rw-term-sources">({sources.join(", ")})</span>
        </span>
      );
    });
    const sections: JSX.Element[] = [];
    if (matchedTerms.length) {
      sections.push(
        <Fragment>
          <p className="muted">Matched terms</p>
          <div className="rw-keyword-tags">{matchedTermTags}</div>
        </Fragment>
      );
    }
    if (unmatchedTerms.length) {
      sections.push(
        <Fragment>
          <p className="muted">Unmatched terms</p>
          <div className="rw-keyword-tags">{unmatchedTermTags}</div>
        </Fragment>
      );
    }
    disclosureContent = <Fragment>{sections}</Fragment>;
  }

  const matchedTotal = props.matches.matched_total;
  const termTotal = props.matches.term_total;
  const panelBody: JSX.Element = (
    <Fragment>
      <p className="muted">{matchedTotal} of {termTotal} search terms matched this article.</p>
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
function ArticleView(props: { record: any; data: any }): JSX.Element {
  const extension = parseObject(props.record.extension_data);
  const validation = extension.validation_status || props.record.validation_status || "Not recorded";
  const audits: AuditEventRecord[] = list(props.data.audit_events, ["events", "items"]);
  const enriched = list(props.data.enriched_fields, ["rows", "items"]);
  const pdf = props.data.pdf_status || { status: "not_available" };
  const providers = new Set();
  const enrichedFieldNames = new Set();
  enriched.forEach((item) => {
    const metadata = parseObject(item.metadata_json);
    if (metadata.provider) {
      providers.add(metadata.provider);
    }
    if (metadata.field) {
      enrichedFieldNames.add(metadata.field);
    }
  });

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
      value: statusChip(validation),
      html: true as const,
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

  var abstract: JSX.Element = <p className="ui faded text">No abstract was recorded for this revision.</p>;
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
  ], "property-grid--compact");

  const bibliographyBody = (
    <Fragment>
      {abstract}
      {bibliographyGrid}
    </Fragment>
  );
  const bibliography = (
    <Panel title="Bibliographic metadata" description="Human-readable metadata for this immutable revision."
      body={bibliographyBody}
      classes="rw-detail-section"
    />
  );

  const pdfPanel = <PDFStatusPanel record={props.record} pdf={pdf} />;
  const rawRecordMarkup = rawRecord(props.record, ["title", "doi", "year", "journal", "publisher", "source", "abstract", "keywords", "keywords_plus", "citation_count", "reference_count"]);

  const auditEventsBody = (
    <div data-article-audit-host>
      <RecordAuditInvestigation events={audits} />
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
      <Panel title="Provenance summary" description="Where this revision came from and how it was captured." body={provenance} classes="rw-detail-section" />
      {bibliography}
      <div data-detail-collection-host="article-authors"></div>
      <div data-detail-collection-host="article-references"></div>
      <div data-detail-collection-host="article-stage-outcomes"></div>
      <Panel title="Audit events" description="Append-only persisted audit records for this work in the selected run." body={auditEventsBody} classes="rw-detail-section rw-article-audit-panel" />
      {rawRecordMarkup}
      <span data-mount-detail-collections hidden></span>
    </Fragment>
  );
}

/** Renders PDF inventory and download-status markup for an article. */
export function PDFStatusPanel(props: { record: any; pdf: any }): JSX.Element {
  const pdfLabels: Record<string, string> = {
    available: "Available",
    unavailable: "Unavailable without a DOI",
    not_available: "Not Available",
  };
  var pdfAction: JSX.Element = <span className="ui faded text">No stored PDF is available.</span>;
  if (props.pdf.status === "available") {
    pdfAction = <a className="ui primary button" href={`/api/pdf/${encodeURIComponent(props.record.work_id)}`} download>Download PDF</a>;
  }

  var sizeValue: string | null = null;
  if (props.pdf.byte_size) sizeValue = formatBytes(props.pdf.byte_size);
  var inventoriedValue: string | null = null;
  if (props.pdf.inventoried_at) inventoriedValue = formatTime(props.pdf.inventoried_at);

  const pdfGrid = propertyGrid([
    {
      label: "Status",
      value: statusChip(pdfLabels[props.pdf.status] || humanLabel(props.pdf.status)),
      html: true as const,
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
  ], "property-grid--compact");

  const pdfStatusBody = (
    <div className="rw-pdf-status-strip">
      {pdfGrid}
      <div className="rw-pdf-status-strip__action">{pdfAction}</div>
    </div>
  );
  return (
    <Panel title="Full-text PDF" description="Read-only state from the companion PDF store."
      body={pdfStatusBody}
      classes="rw-detail-section rw-full-text-panel"
    />
  );
}

/** Renders candidate ORCID evidence associated with the selected author occurrence. */
function AuthorIdentityEvidence(props: { data: any }): JSX.Element {
  const evidence = list(props.data.identity_evidence, ["rows", "items"]);
  if (!evidence.length) {
    const emptyEvidenceBody = <p className="ui faded text">No ORCID name-search evidence was recorded for this author occurrence.</p>;
    return <Panel title="ORCID candidate evidence" description="Name-search candidates remain uncertain evidence and are never assigned automatically." body={emptyEvidenceBody} classes="rw-detail-section" />;
  }

  const body = evidence.map((resolution) => {
    const candidates = list(resolution, ["candidates"]);
    var candidateList: JSX.Element = <p className="ui faded text">No provider candidate was returned.</p>;
    if (candidates.length) {
      const candidateItems = candidates.map((candidate) => {
        var links = <a href={candidate.query_url} target="_blank" rel="noreferrer">Provider query</a>;
        if (candidate.payload_artifact_id) {
          links = (
            <Fragment>
              {links}
              <span aria-hidden="true">·</span>
              <a href={`/api/artifacts/${encodeURIComponent(candidate.payload_artifact_id)}/content`}>Raw payload</a>
            </Fragment>
          );
        }
        return (
          <li>
            <div>
              <strong>{candidate.candidate_orcid}</strong>
              <span>{candidate.provider_display_name || "Provider name not recorded"}</span>
            </div>
            <span className="ui basic label">Rank {candidate.provider_rank}</span>
            <div className="rw-inline-group">{links}</div>
          </li>
        );
      });
      candidateList = <ol className="rw-identity-candidate-list">{candidateItems}</ol>;
    }
    var providerError: JSX.Element | null = null;
    if (resolution.error_message) {
      providerError = (
        <p className="ui warning message">
          <span className="header">Provider response</span>
          {resolution.error_message}
        </p>
      );
    }
    return (
      <section className="rw-identity-resolution">
        <header>
          <div>
            <h4>{humanLabel(resolution.provider || "ORCID")} name search</h4>
            <p>Resolved {formatTime(resolution.resolved_at)}</p>
          </div>
          <StatusChip raw={resolution.status} />
        </header>
        {providerError}
        {candidateList}
      </section>
    );
  });

  const evidenceBody = <Fragment>{body}</Fragment>;
  return <Panel title="ORCID candidate evidence" description="Review provider candidates here without treating a name-search match as confirmed identity." body={evidenceBody} classes="rw-detail-section" />;
}

/** Renders the author occurrence detail view with related articles and audit evidence. */
function AuthorView(props: { record: any; data: any }): JSX.Element {
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
      value: statusChip(identity),
      html: true as const,
    },
    {
      label: "Articles in response",
      value: articles.length,
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

  const authorAuditBody = <RecordAuditInvestigation events={audits} />;

  return (
    <Fragment>
      <div className="ui info message">
        <span className="header">Observed author occurrence</span>
        This historical record does not establish a global person identity. Matching names remain separate unless explicit identity evidence links them.
      </div>
      {summary}
      <Panel title="Observed identity data" description="The exact name and identity values stored for this occurrence." body={identityGrid} classes="rw-detail-section" />
      <AuthorIdentityEvidence data={props.data} />
      <div data-detail-collection-host="author-articles"></div>
      <Panel title="Audit events" description="Filter and inspect append-only events directly associated with this author occurrence." body={authorAuditBody} classes="rw-detail-section" />
      {rawRecordMarkup}
      <span data-mount-author-articles hidden></span>
    </Fragment>
  );
}

/** Renders the reference mention detail view with citation context. */
function ReferenceView(props: { record: any }): JSX.Element {
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
      value: statusChip(resolutionLabel),
      html: true as const,
    },
  ]);

  const citingGrid = propertyGrid([
    {
      label: "Article title",
      value: props.record.citing_title,
    },
    {
      label: "Article revision",
      value: <a href={detailLink("article", props.record.work_revision_id)}>{props.record.work_revision_id}</a>,
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

  var resolvedBody: JSX.Element = <p className="ui faded text">This reference mention was not resolved to a work revision in the selected run.</p>;
  if (resolved) {
    resolvedBody = propertyGrid([
      {
        label: "Target title",
        value: props.record.resolved_title,
      },
      {
        label: "Target revision",
        value: <a href={detailLink("article", props.record.resolved_revision_id)}>{props.record.resolved_revision_id}</a>,
      },
      {
        label: "Resolution",
        value: statusChip("Resolved internally"),
        html: true as const,
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
        <Panel title="Citing article" description="The immutable article revision that contains this reference mention." body={citingGrid} classes="rw-detail-section" />
        <Panel title="Resolved target" description="A target is shown only when the stored relationship resolved inside this run." body={resolvedBody} classes="rw-detail-section" />
      </div>
      <Panel title="Cited-reference metadata" description="Raw bibliographic values captured for this mention." body={citedGrid} classes="rw-detail-section" />
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
  var apiOptions: Record<string, any> = {};
  if (kind === "author") apiOptions = { run_id: value("run_id") };

  const data = await api(`/api/${kind}s/${encodeURIComponent(id)}`, apiOptions, {
    method: "GET",
    headers: {
      Accept: "application/json",
    },
  });
  const record = data.article || data.author || data.reference || data;

  var title = labels[kind];
  if (kind === "article") title = record.title || labels[kind];
  else if (kind === "author") title = record.citation_name || labels[kind];
  else title = record.title || record.doi || labels[kind];

  var body: JSX.Element = <ReferenceView record={record} />;
  if (kind === "article") {
    body = <ArticleView record={record} data={data} />;
  } else if (kind === "author") {
    body = <AuthorView record={record} data={data} />;
  }

  const homeHref = link({
    view: "home",
    article_id: "",
    author_id: "",
    reference_id: "",
    return_view: "",
    return_id: "",
  });
  const deepdiveHref = link({
    view: "overview",
    article_id: "",
    author_id: "",
    reference_id: "",
    return_view: "",
    return_id: "",
  });
  const corpusHref = backToCorpus(kind);
  const crumbs: Array<{ label: string; href?: string }> = [
    {
      label: "Home",
      href: homeHref,
    },
    {
      label: "Deepdive",
      href: deepdiveHref,
    },
    {
      label: "Corpus",
      href: corpusHref,
    },
  ];
  if (kind === "article") {
    crumbs.push({
      label: "Analysis-ready articles",
      href: backToCorpus("article"),
    });
    crumbs.push({ label: record.doi || title });
  } else if (kind === "author") {
    crumbs.push({ label: "Author" });
  } else {
    crumbs.push({
      label: "Reference mentions",
      href: backToCorpus("reference"),
    });
    crumbs.push({ label: record.doi || title });
  }
  setBreadcrumb(crumbs);

  const page = (
    <Fragment>
      <PageHeader kicker={labels[kind]} title={title} description="" />
      <article className="rw-record-detail">{body}</article>
    </Fragment>
  );
  renderTree(page, app);

  if (kind === "article") {
    mountCollection("article-authors", "Ordered authors", "Observed author occurrences in bibliographic order.", [
      {
        label: "Order",
        render: (row) => recorded(row.author_order),
      },
      {
        label: "Author occurrence",
        render: (row) => <a href={detailLink("author", row.id)}>{row.citation_name}</a>,
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
    ], list(data.authors, ["rows", "items"]));
    mountCollection("article-references", "Reference mentions", "Ordered references captured for this article revision.", [
      {
        label: "Order",
        render: (row) => recorded(row.mention_order),
      },
      {
        label: "Reference mention",
        render: (row) => <a href={detailLink("reference", row.id)}>{row.title || row.doi || `Reference ${row.id}`}</a>,
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
            return <a href={detailLink("article", row.resolved_revision_id)}><StatusChip raw="Resolved internally" /></a>;
          }
          return <StatusChip raw="Unresolved" />;
        },
      },
    ], list(data.references, ["rows", "items"]));
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
    ], list(data.stage_outcomes, ["rows", "items"]));
    if (Number(record.id) > 0 && Number(record.work_id) > 0 && Number(record.pipeline_run_id) > 0) {
      activeArticleReview = await mountArticleReview(
        document.querySelector("[data-review-host]") as HTMLElement,
        document.querySelector<HTMLElement>("[data-pdf-viewer-host]"),
        record,
        data,
        async () => {
          const refreshed = await api(`/api/articles/${encodeURIComponent(record.id)}`, {}, {
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
    }
  }
  if (kind === "author") {
    mountCollection("author-articles", "Linked article revisions", "Articles that contain this observed author occurrence.", [
      {
        label: "Article revision",
        render: (row) => <a href={detailLink("article", row.work_revision_id)}>{row.title}</a>,
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
    ], list(data.articles, ["rows", "items"]));
  }
  bindRecordAuditInvestigation(list(data.audit_events, ["events", "items"]));
  bindCopyButtons();
}
