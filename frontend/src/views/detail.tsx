// Immutable article, author-occurrence, and reference-mention detail views.
import {
  app, value, link, emptyState, cell, list,
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
  const updates: Record<string, any> = { view: kind, article_id: '', author_id: '', reference_id: '' };
  updates[kind + '_id'] = id;
  const currentKind = value('view');
  const currentID = value(currentKind + '_id');
  if (['article', 'author', 'reference'].includes(currentKind) && currentID) {
    updates.return_view = currentKind;
    updates.return_id = currentID;
  }
  return link(updates);
}

/** Returns the context-preserving corpus return URL for a detail view. */
function backToCorpus(kind: string): string {
  var section = 'articles';
  if (kind === 'author') {
    section = 'authors';
  } else if (kind === 'reference') {
    section = 'references';
  }
  return link({
    view: 'corpus', section: section, article_id: '', author_id: '', reference_id: '', table: ''
  });
}

/** Renders a recorded value or its unavailable presentation. */
function recorded(raw: any, fallback?: JSX.Element): JSX.Element {
  if (raw === null || raw === undefined || raw === '') {
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
  return (
    <dl className={"property-grid " + (classes || '')}>
      {entries.map(function(entry) {
        const content = entry.html ? raw(entry.value) : recorded(entry.value);
        return <div><dt>{entry.label}</dt><dd>{content}</dd></div>;
      })}
    </dl>
  );
}

/** Renders compact summary-fact markup for a detail record. */
function summaryStrip(entries: DetailEntry[]): JSX.Element {
  return (
    <dl className="rw-record-summary">
      {entries.map(function(entry) {
        const content = entry.html ? raw(entry.value) : recorded(entry.value);
        return <div><dt>{entry.label}</dt><dd>{content}</dd></div>;
      })}
    </dl>
  );
}

/** Converts a stored mapping representation to a displayable object. */
function mappingValue(raw: any): JSX.Element {
  if (raw === null || raw === undefined || raw === '') {
    return <span className="ui faded text">Not recorded</span>;
  }
  if (Array.isArray(raw)) {
    return <ul className="rw-mapping-values">{raw.map(function(item) {
      return <li>{mappingValue(item)}</li>;
    })}</ul>;
  }
  if (raw && typeof raw === 'object') {
    return <dl className="rw-mapping-list">{Object.entries(raw).map(function(entry) {
      return <div><dt>{humanLabel(entry[0])}</dt><dd>{mappingValue(entry[1])}</dd></div>;
    })}</dl>;
  }
  return <span className="rw-mono">{raw}</span>;
}

/** Renders the parsed extension mapping stored on a work revision. */
function extensionMapping(raw: any): JSX.Element {
  const parsed = parseObject(raw);
  if (!Object.keys(parsed).length) {
    return recorded(raw);
  }
  return mappingValue(parsed);
}

/** Returns normalized keyword values from stored array or delimited input. */
function keywordValues(raw: any): string[] {
  if (Array.isArray(raw)) {
    return raw.map(String).filter(Boolean);
  }
  if (raw === null || raw === undefined || raw === '') {
    return [];
  }
  if (typeof raw === 'string') {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return parsed.map(String).filter(Boolean);
      }
    } catch (_) {
      // Stored legacy values may be delimited text rather than JSON.
    }
    const separator = raw.includes(';') ? ';' : ',';
    return raw.split(separator).map(function(item) { return item.trim(); }).filter(Boolean);
  }
  return [];
}

/** Renders label markup for normalized keyword values. */
function keywordMarkup(raw: any): JSX.Element {
  const keywords = keywordValues(raw);
  if (!keywords.length) {
    return <span className="ui faded text">Not recorded</span>;
  }
  const rawText = typeof raw === 'string' ? raw : JSON.stringify(raw);
  return (
    <div className="rw-keywords">
      <div className="rw-keyword-tags">{keywords.map(function(keyword) {
        return <span className="ui label">{keyword}</span>;
      })}</div>
      <details className="rw-keywords__raw"><summary>Show stored keyword value</summary>
        <div><code>{rawText}</code><button type="button" className="ui basic button" data-copy-text={rawText}>Copy</button></div>
      </details>
    </div>
  );
}

/** Renders expandable JSON markup for a raw record. */
function rawRecord(record: Record<string, any>, excluded: string[]): JSX.Element | null {
  const rows = Object.entries(record).filter(function([key]) {
    return !excluded.includes(key);
  });
  if (!rows.length) {
    return null;
  }
  return (
    <details className="rw-disclosure">
      <summary>Advanced raw record data</summary>
      <div className="rw-disclosure__content">
        {propertyGrid(rows.map(function([key, item]) {
          if (key === 'extension_data') {
            return { label: 'Extension data', value: extensionMapping(item) };
          }
          return { label: humanLabel(key), value: cell(item, key), html: true as const };
        }), 'property-grid--compact')}
      </div>
    </details>
  );
}

/** Renders expandable markup for a related-record collection. */
function CollectionMarkup(props: { collectionKey: string; title: string; description: string; columns: Array<{ label: string; render: (row: any) => JSX.Element }>; rows: any[]; page: number }): JSX.Element {
  const pageSize = 25;
  const totalPages = Math.max(1, Math.ceil(props.rows.length / pageSize));
  const current = Math.min(Math.max(1, props.page), totalPages);
  const visible = props.rows.slice((current - 1) * pageSize, current * pageSize);
  var body: JSX.Element[];
  if (visible.length) {
    body = visible.map(function(row) {
      return <tr>{props.columns.map(function(column) {
        return <td>{column.render(row)}</td>;
      })}</tr>;
    });
  } else {
    body = [<tr><td colspan={Math.max(1, props.columns.length)} className="empty">No records.</td></tr>];
  }
  const table = (
    <div className="table-wrap" aria-label={props.title + " table"}>
      <table className="ui table">
        <thead><tr>{props.columns.map(function(column) {
          return <th scope="col">{column.label}</th>;
        })}</tr></thead>
        <tbody>{body}</tbody>
      </table>
    </div>
  );
  const pages = props.rows.length > pageSize ? raw(pagination({
    page: current, per_page: pageSize, total_rows: props.rows.length, total_pages: totalPages
  }, {
    itemLabel: props.title.toLocaleLowerCase(), pageAttribute: 'data-detail-page', pageClass: ' detail-page', visibleCount: 5
  }).replaceAll('data-detail-page="', 'data-detail-page="' + props.collectionKey + ':')) : null;
  return (
    <section className="ui segment rw-detail-collection" data-detail-collection={props.collectionKey}>
      <div className="ui top attached header"><div><h3>{props.title}</h3><p>{props.description}</p></div><span className="ui label">{props.rows.length.toLocaleString()}</span></div>
      <div className="content">{table}{pages}</div>
    </section>
  );
}

/** Mounts collection. */
function mountCollection(key: string, title: string, description: string, columns: Array<{ label: string; render: (row: any) => JSX.Element }>, rows: any[]): void {
  collectionState.set(key, { title: title, description: description, columns: columns, rows: rows, page: 1 });
  renderCollection(key);
}

/** Renders collection. */
function renderCollection(key: string): void {
  const state = collectionState.get(key);
  const container = document.querySelector<HTMLElement>('[data-detail-collection-host="' + key + '"]');
  if (!state || !container) {
    return;
  }
  renderTree(<CollectionMarkup collectionKey={key} title={state.title} description={state.description} columns={state.columns} rows={state.rows} page={state.page} />, container);
  container.querySelectorAll<HTMLButtonElement>('[data-detail-page]').forEach(function(button) {
    button.addEventListener('click', function() {
      const parts = button.dataset.detailPage!.split(':');
      state.page = Number(parts[1]) || 1;
      renderCollection(key);
      container.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    });
  });
}

/** Renders escaped validation or failure reason markup for a stage outcome. */
function stageReasonMarkup(raw: any): JSX.Element {
  if (typeof raw === 'string') {
    try {
      const reasons = JSON.parse(raw);
      if (Array.isArray(reasons) && reasons.length) {
        return <ul className="rw-mapping-values">{reasons.map(function(reason) {
          return <li>{reason}</li>;
        })}</ul>;
      }
    } catch (_) {
      // Validation reasons created before normalization may be plain text.
    }
  }
  return recorded(raw);
}

/** Renders the search term coverage panel for an article revision. */
function SearchTermCoveragePanel(props: { matches: any; record: any }): JSX.Element {
  const matches = props.matches;
  const record = props.record;
  if (matches === null || matches === undefined) {
    return (
      <Panel title="Search term coverage" description="Derived from the search queries recorded for this run."
        body={<p className="ui faded text">No search terms recorded for this run.</p>} classes="rw-detail-section" />
    );
  }
  const fields = [
    { key: 'title', label: 'Title', recorded: Boolean(record.title) },
    { key: 'abstract', label: 'Abstract', recorded: Boolean(record.abstract) },
    { key: 'keywords', label: 'Keywords', recorded: keywordValues(record.keywords).length > 0 },
    { key: 'keywords_plus', label: 'Keywords plus', recorded: keywordValues(record.keywords_plus).length > 0 },
  ];
  const termsWithSources = matches.terms_with_sources || {};
  const matchedSet = new Set<string>();
  fields.forEach(function(field) {
    (matches[field.key] || []).forEach(function(term: string) {
      matchedSet.add(term);
    });
  });
  const allTerms = Object.entries(termsWithSources) as Array<[string, string[]]>;
  const matchedTerms = allTerms.filter(function(entry) { return matchedSet.has(entry[0]); });
  const unmatchedTerms = allTerms.filter(function(entry) { return !matchedSet.has(entry[0]); });

  return (
    <Panel title="Search term coverage" description="Which recorded search terms matched this article's fields. Matching is a local approximation of the database queries."
      body={
        <Fragment>
          <p className="muted">{matches.matched_total} of {matches.term_total} search terms matched this article.</p>
          <div className="rw-term-fields">
            {fields.map(function(field) {
              const terms = matches[field.key] || [];
              var content: JSX.Element;
              if (!field.recorded) {
                content = <span className="ui faded text">Not recorded</span>;
              } else if (terms.length) {
                content = <span className="rw-keyword-tags">{terms.map(function(term: string) {
                  return <span className="ui label">{term}</span>;
                })}</span>;
              } else {
                content = <span className="ui faded text">No matched terms</span>;
              }
              return <div className="rw-term-field"><span className="rw-term-field__label">{field.label}</span>{content}</div>;
            })}
          </div>
          <details className="rw-disclosure">
            <summary>All search terms</summary>
            <div className="rw-disclosure__content">
              {allTerms.length
                ? <Fragment>
                  {matchedTerms.length ? <Fragment><p className="muted">Matched terms</p><div className="rw-keyword-tags">{matchedTerms.map(function(entry) {
                    return <span className="ui label">{entry[0]}<span className="rw-term-sources">({entry[1].join(', ')})</span></span>;
                  })}</div></Fragment> : null}
                  {unmatchedTerms.length ? <Fragment><p className="muted">Unmatched terms</p><div className="rw-keyword-tags">{unmatchedTerms.map(function(entry) {
                    return <span className="ui label">{entry[0]}<span className="rw-term-sources">({entry[1].join(', ')})</span></span>;
                  })}</div></Fragment> : null}
                </Fragment>
                : <p className="ui faded text">No search terms recorded.</p>}
            </div>
          </details>
        </Fragment>
      } classes="rw-detail-section" />
  );
}

/** Renders the article detail view from its immutable revision payload. */
function ArticleView(props: { record: any; data: any }): JSX.Element {
  const record = props.record;
  const data = props.data;
  const extension = parseObject(record.extension_data);
  const validation = extension.validation_status || record.validation_status || 'Not recorded';
  const audits: AuditEventRecord[] = list(data.audit_events, ['events', 'items']);
  const enriched = list(data.enriched_fields, ['rows', 'items']);
  const pdf = data.pdf_status || { status: 'not_available' };
  const providers = new Set();
  const fields = new Set();
  enriched.forEach(function(item) {
    const metadata = parseObject(item.metadata_json);
    if (metadata.provider) {
      providers.add(metadata.provider);
    }
    if (metadata.field) {
      fields.add(metadata.field);
    }
  });

  const summary = summaryStrip([
    { label: 'DOI', value: record.doi },
    { label: 'Year', value: record.year },
    { label: 'Journal', value: record.journal },
    { label: 'Publisher', value: record.publisher },
    { label: 'Source', value: record.source },
    { label: 'Validation', value: statusChip(validation), html: true as const },
  ]);

  const provenance = propertyGrid([
    { label: 'Run attempt', value: record.pipeline_run_id },
    { label: 'Work', value: record.work_id },
    { label: 'Revision', value: record.id },
    { label: 'Produced by stage', value: record.producer_stage },
    { label: 'Captured', value: formatTime(record.created_at) },
    { label: 'Enrichment providers', value: Array.from(providers).join(', ') },
    { label: 'Enriched fields', value: Array.from(fields).join(', ') },
    { label: 'Payload hash', value: record.payload_hash },
  ]);

  var abstract: JSX.Element = <p className="ui faded text">No abstract was recorded for this revision.</p>;
  if (record.abstract) {
    abstract = <p className="rw-abstract-text">{String(record.abstract).replace(/\s+/g, ' ').trim()}</p>;
  }
  const bibliography = (
    <Panel title="Bibliographic metadata" description="Human-readable metadata for this immutable revision."
      body={<Fragment>{abstract}{propertyGrid([
        { label: 'Keywords', value: keywordMarkup(record.keywords) },
        { label: 'Keywords plus', value: keywordMarkup(record.keywords_plus) },
        { label: 'Citation count', value: record.citation_count },
        { label: 'Reference count', value: record.reference_count },
      ], 'property-grid--compact')}</Fragment>} classes="rw-detail-section" />
  );

  const pdfPanel = <PDFStatusPanel record={record} pdf={pdf} />;

  return (
    <Fragment>
      {summary}
      {pdfPanel}
      <div className="rw-reading-workspace"><div data-pdf-viewer-host></div><div data-review-host></div></div>
      <Panel title="Provenance summary" description="Where this revision came from and how it was captured." body={provenance} classes="rw-detail-section" />
      {bibliography}
      <SearchTermCoveragePanel matches={data.term_matches} record={record} />
      <div data-detail-collection-host="article-authors"></div>
      <div data-detail-collection-host="article-references"></div>
      <div data-detail-collection-host="article-stage-outcomes"></div>
      <Panel title="Audit events" description="Append-only persisted audit records for this work in the selected run." body={<div data-article-audit-host><RecordAuditInvestigation events={audits} /></div>} classes="rw-detail-section rw-article-audit-panel" />
      {rawRecord(record, ['title', 'doi', 'year', 'journal', 'publisher', 'source', 'abstract', 'keywords', 'keywords_plus', 'citation_count', 'reference_count'])}
      <span data-mount-detail-collections hidden></span>
    </Fragment>
  );
}

/** Renders PDF inventory and download-status markup for an article. */
export function PDFStatusPanel(props: { record: any; pdf: any }): JSX.Element {
  const record = props.record;
  const pdf = props.pdf;
  const pdfLabels: Record<string, string> = {
    available: 'Available',
    unavailable: 'Unavailable without a DOI',
    not_available: 'Not Available'
  };
  var pdfAction: JSX.Element = <span className="ui faded text">No stored PDF is available.</span>;
  if (pdf.status === 'available') {
    pdfAction = <a className="ui primary button" href={'/api/pdf/' + encodeURIComponent(record.work_id)} download>Download PDF</a>;
  }
  return (
    <Panel title="Full-text PDF" description="Read-only state from the companion PDF store."
      body={<div className="rw-pdf-status-strip">{propertyGrid([
        { label: 'Status', value: statusChip(pdfLabels[pdf.status] || humanLabel(pdf.status)), html: true as const },
        { label: 'Size', value: pdf.byte_size ? formatBytes(pdf.byte_size) : null },
        { label: 'Inventoried at', value: pdf.inventoried_at ? formatTime(pdf.inventoried_at) : null },
        { label: 'Content hash', value: pdf.content_hash },
      ], 'property-grid--compact')}<div className="rw-pdf-status-strip__action">{pdfAction}</div></div>} classes="rw-detail-section rw-full-text-panel" />
  );
}

/** Renders candidate ORCID evidence associated with the selected author occurrence. */
function AuthorIdentityEvidence(props: { data: any }): JSX.Element {
  const evidence = list(props.data.identity_evidence, ['rows', 'items']);
  if (!evidence.length) {
    return <Panel title="ORCID candidate evidence" description="Name-search candidates remain uncertain evidence and are never assigned automatically." body={<p className="ui faded text">No ORCID name-search evidence was recorded for this author occurrence.</p>} classes="rw-detail-section" />;
  }
  const body = evidence.map(function(resolution) {
    const candidates = list(resolution, ['candidates']);
    const candidateList = candidates.length
      ? <ol className="rw-identity-candidate-list">{candidates.map(function(candidate) {
        var links = <a href={candidate.query_url} target="_blank" rel="noreferrer">Provider query</a>;
        if (candidate.payload_artifact_id) links = <Fragment>{links} <span aria-hidden="true">·</span> <a href={'/api/artifacts/' + encodeURIComponent(candidate.payload_artifact_id) + '/content'}>Raw payload</a></Fragment>;
        return <li><div><strong>{candidate.candidate_orcid}</strong><span>{candidate.provider_display_name || 'Provider name not recorded'}</span></div><span className="ui basic label">Rank {candidate.provider_rank}</span><div className="rw-inline-group">{links}</div></li>;
      })}</ol>
      : <p className="ui faded text">No provider candidate was returned.</p>;
    const providerError = resolution.error_message ? <p className="ui warning message"><span className="header">Provider response</span>{resolution.error_message}</p> : null;
    return <section className="rw-identity-resolution"><header><div><h4>{humanLabel(resolution.provider || 'ORCID')} name search</h4><p>Resolved {formatTime(resolution.resolved_at)}</p></div><StatusChip raw={resolution.status} /></header>{providerError}{candidateList}</section>;
  });
  return <Panel title="ORCID candidate evidence" description="Review provider candidates here without treating a name-search match as confirmed identity." body={<Fragment>{body}</Fragment>} classes="rw-detail-section" />;
}

/** Renders the author occurrence detail view with related articles and audit evidence. */
function AuthorView(props: { record: any; data: any }): JSX.Element {
  const record = props.record;
  const data = props.data;
  const articles = list(data.articles, ['rows', 'items']);
  const audits: AuditEventRecord[] = list(data.audit_events, ['events', 'items']);
  const identity = record.person_id ? 'Linked global person' : 'Observed occurrence only';
  return (
    <Fragment>
      <div className="ui info message"><span className="header">Observed author occurrence</span>This historical record does not establish a global person identity. Matching names remain separate unless explicit identity evidence links them.</div>
      {summaryStrip([
        { label: 'Citation name', value: record.citation_name },
        { label: 'ORCID observed', value: record.orcid },
        { label: 'Identity status', value: statusChip(identity), html: true as const },
        { label: 'Articles in response', value: articles.length },
      ])}
      <Panel title="Observed identity data" description="The exact name and identity values stored for this occurrence." body={propertyGrid([
        { label: 'First name', value: record.first_name },
        { label: 'Last name', value: record.last_name },
        { label: 'Observed ORCID', value: record.orcid },
        { label: 'Linked person', value: record.person_id },
        { label: 'Person ORCID', value: record.person_orcid },
        { label: 'Captured', value: formatTime(record.created_at) },
      ])} classes="rw-detail-section" />
      <AuthorIdentityEvidence data={data} />
      <div data-detail-collection-host="author-articles"></div>
      <Panel title="Audit events" description="Filter and inspect append-only events directly associated with this author occurrence." body={<RecordAuditInvestigation events={audits} />} classes="rw-detail-section" />
      {rawRecord(record, ['citation_name', 'first_name', 'last_name', 'orcid', 'person_id', 'person_orcid'])}
      <span data-mount-author-articles hidden></span>
    </Fragment>
  );
}

/** Renders the reference mention detail view with citation context. */
function ReferenceView(props: { record: any }): JSX.Element {
  const record = props.record;
  const resolved = Boolean(record.resolved_revision_id);
  return (
    <Fragment>
      {summaryStrip([
        { label: 'Reference order', value: record.mention_order },
        { label: 'DOI', value: record.doi },
        { label: 'Year', value: record.year },
        { label: 'Resolution', value: statusChip(resolved ? 'Resolved internally' : 'Unresolved'), html: true as const },
      ])}
      <div className="rw-record-comparison">
        <Panel title="Citing article" description="The immutable article revision that contains this reference mention." body={propertyGrid([
          { label: 'Article title', value: record.citing_title },
          { label: 'Article revision', value: <a href={detailLink('article', record.work_revision_id)}>{record.work_revision_id}</a> },
          { label: 'Work', value: record.work_id },
          { label: 'Run attempt', value: record.pipeline_run_id },
        ])} classes="rw-detail-section" />
        <Panel title="Resolved target" description="A target is shown only when the stored relationship resolved inside this run." body={resolved ? propertyGrid([
          { label: 'Target title', value: record.resolved_title },
          { label: 'Target revision', value: <a href={detailLink('article', record.resolved_revision_id)}>{record.resolved_revision_id}</a> },
          { label: 'Resolution', value: statusChip('Resolved internally'), html: true as const },
        ]) : <p className="ui faded text">This reference mention was not resolved to a work revision in the selected run.</p>} classes="rw-detail-section" />
      </div>
      <Panel title="Cited-reference metadata" description="Raw bibliographic values captured for this mention." body={propertyGrid([
        { label: 'Referenced title', value: record.title },
        { label: 'Referenced author', value: record.author },
        { label: 'Source', value: record.source },
        { label: 'Captured', value: formatTime(record.created_at) },
      ])} classes="rw-detail-section" />
      {rawRecord(record, ['title', 'author', 'doi', 'year', 'source', 'mention_order', 'citing_title', 'resolved_title'])}
    </Fragment>
  );
}

/** Asynchronously implements detail view for the viewer. */
export async function detailView(kind: string): Promise<void> {
  await destroyActiveArticleReview();
  const labels: Record<string, string> = { article: 'Article revision', author: 'Author occurrence', reference: 'Reference mention' };
  var idKey = kind + '_id';
  const id = value(idKey);
  if (!id) {
    renderTree(<EmptyState title={labels[kind]} detail="Open a record from Corpus, Relationships, or Advanced database inspection." />, app);
    return;
  }

  collectionState.clear();
  const data = await api('/api/' + kind + 's/' + encodeURIComponent(id), kind === 'author' ? { run_id: value('run_id') } : {}, { method: 'GET', headers: { Accept: 'application/json' } });
  const record = data.article || data.author || data.reference || data;
  var title = labels[kind];
  if (kind === 'article') {
    title = record.title || labels[kind];
  } else if (kind === 'author') {
    title = record.citation_name || labels[kind];
  } else {
    title = record.title || record.doi || labels[kind];
  }

  var body: JSX.Element;
  if (kind === 'article') {
    body = <ArticleView record={record} data={data} />;
  } else if (kind === 'author') {
    body = <AuthorView record={record} data={data} />;
  } else {
    body = <ReferenceView record={record} />;
  }

  const homeHref = link({ view: 'home', article_id: '', author_id: '', reference_id: '', return_view: '', return_id: '' });
  const deepdiveHref = link({ view: 'overview', article_id: '', author_id: '', reference_id: '', return_view: '', return_id: '' });
  const corpusHref = backToCorpus(kind);
  const crumbs: Array<{ label: string; href?: string }> = [
    { label: 'Home', href: homeHref },
    { label: 'Deepdive', href: deepdiveHref },
    { label: 'Corpus', href: corpusHref }
  ];
  if (kind === 'article') {
    crumbs.push({ label: 'Analysis-ready articles', href: backToCorpus('article') });
    crumbs.push({ label: record.doi || title });
  } else if (kind === 'author') {
    crumbs.push({ label: 'Author' });
  } else {
    crumbs.push({ label: 'Reference mentions', href: backToCorpus('reference') });
    crumbs.push({ label: record.doi || title });
  }
  setBreadcrumb(crumbs);

  renderTree(
    <Fragment>
      <PageHeader kicker={labels[kind]} title={title} description="" />
      <article className="rw-record-detail">{body}</article>
    </Fragment>,
    app
  );

  if (kind === 'article') {
    mountCollection('article-authors', 'Ordered authors', 'Observed author occurrences in bibliographic order.', [
      { label: 'Order', render: function(row) { return recorded(row.author_order); } },
      { label: 'Author occurrence', render: function(row) { return <a href={detailLink('author', row.id)}>{row.citation_name}</a>; } },
      { label: 'ORCID', render: function(row) { return recorded(row.orcid); } },
      { label: 'Affiliation', render: function(row) { return recorded(row.affiliation); } },
      { label: 'Identity evidence', render: function(row) { return row.person_id ? <StatusChip raw="Linked global person" /> : <StatusChip raw="Observed occurrence only" />; } },
    ], list(data.authors, ['rows', 'items']));
    mountCollection('article-references', 'Reference mentions', 'Ordered references captured for this article revision.', [
      { label: 'Order', render: function(row) { return recorded(row.mention_order); } },
      { label: 'Reference mention', render: function(row) { return <a href={detailLink('reference', row.id)}>{row.title || row.doi || ('Reference ' + row.id)}</a>; } },
      { label: 'Author', render: function(row) { return recorded(row.author); } },
      { label: 'Year', render: function(row) { return recorded(row.year); } },
      { label: 'Resolution', render: function(row) { return row.resolved_revision_id ? <a href={detailLink('article', row.resolved_revision_id)}><StatusChip raw="Resolved internally" /></a> : <StatusChip raw="Unresolved" />; } },
    ], list(data.references, ['rows', 'items']));
    mountCollection('article-stage-outcomes', 'Pipeline stage history', 'Recorded per-work outcomes for the selected run. Audit events below are persisted append-only records.', [
      { label: 'Stage', render: function(row) { return recorded(humanLabel(row.stage_name)); } },
      { label: 'Outcome', render: function(row) { return <StatusChip raw={row.outcome || 'Not recorded'} />; } },
      { label: 'Reason', render: function(row) { return stageReasonMarkup(row.reason); } },
      { label: 'First recorded', render: function(row) { return recorded(formatTime(row.created_at)); } },
      { label: 'Last updated', render: function(row) { return recorded(formatTime(row.updated_at)); } },
    ], list(data.stage_outcomes, ['rows', 'items']));
    if (Number(record.id) > 0 && Number(record.work_id) > 0 && Number(record.pipeline_run_id) > 0) {
      activeArticleReview = await mountArticleReview(
        document.querySelector('[data-review-host]') as HTMLElement,
        document.querySelector<HTMLElement>('[data-pdf-viewer-host]'),
        record,
        data,
        async function() {
          const refreshed = await api('/api/articles/' + encodeURIComponent(record.id), {}, { method: 'GET', headers: { Accept: 'application/json' } });
          const events = list(refreshed.audit_events, ['events', 'items']);
          const auditHost = document.querySelector('[data-article-audit-host]') as HTMLElement | null;
          if (!auditHost) return;
          renderTree(<RecordAuditInvestigation events={events} />, auditHost);
          bindRecordAuditInvestigation(events);
        }
      );
    }
  }
  if (kind === 'author') {
    mountCollection('author-articles', 'Linked article revisions', 'Articles that contain this observed author occurrence.', [
      { label: 'Article revision', render: function(row) { return <a href={detailLink('article', row.work_revision_id)}>{row.title}</a>; } },
      { label: 'Year', render: function(row) { return recorded(row.year); } },
      { label: 'DOI', render: function(row) { return recorded(row.doi); } },
      { label: 'Author order', render: function(row) { return recorded(row.author_order); } },
      { label: 'Affiliation', render: function(row) { return recorded(row.affiliation); } },
    ], list(data.articles, ['rows', 'items']));
  }
  bindRecordAuditInvestigation(list(data.audit_events, ['events', 'items']));
  bindCopyButtons();
}