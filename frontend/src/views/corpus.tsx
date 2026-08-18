// Corpus: articles, authors, references, sources lists.
import {
  app, value, link, pageSizes, corpusSections, section, PageHeader,
  sourceResultCountSummary, formatNumber, percent, filterChips,
  humanLabel as humanLabelState, SourceResultCountSummary, FilterChips, StatusChip
} from '../state.tsx';
import { h, Fragment, raw, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api, tables } from '../api.tsx';
import { DataTable, bindTableControls } from '../components/data-table.tsx';
import type { DataTableContext } from '../components/data-table.tsx';
import { pagination as renderPagination } from '../components/pagination.tsx';
import { setURL } from '../router.tsx';

// Core columns shown in the articles table; extra fields appear in expandable rows.
const articlesColumns = ['doi', 'title', 'year', 'journal', 'source'];
const authorColumns = ['citation_name', 'orcid', 'first_name', 'last_name', 'article_count', 'affiliation_count'];
const articlesExpandFields = [
  { f: 'title', w: 'full' as const },
  { f: 'authors', w: 'full' as const },
  { f: 'journal', w: 10 },
  { f: 'publisher', w: 10 },
  { f: 'abstract', w: 'full' as const },
  { f: 'term_matches', w: 'full' as const, label: 'Matched search terms', render: termMatchMarkup },
  { f: 'work_id', w: 4 },
  { f: 'year', w: 4 },
  { f: 'source', w: 4 },
  { f: 'doi', w: 4 },
  { f: 'validation_status', w: 4 },
  { f: 'citation_count', w: 5 },
  { f: 'reference_count', w: 5 },
  { f: 'producer_stage', w: 5 },
  { f: 'created_at', w: 5 },
];

const referenceColumns = ['mention_order', 'title', 'author', 'year', 'doi', 'citing_title'];
const referenceExpandFields = [
  { f: 'id', w: 4 },
  { f: 'work_revision_id', w: 4 },
  { f: 'resolved_work_id', w: 4 },
  { f: 'source', w: 4 },
  { f: 'created_at', w: 4 },
];

const sourceColumns = ['source_name', 'source_type', 'record_index', 'parse_status', 'reject_reason', 'created_at'];
const sourceExpandFields = [
  { f: 'id', w: 3 },
  { f: 'run_source_id', w: 4 },
  { f: 'content_hash', w: 13 },
];

const columnLabels: Record<string, string> = {
  id: 'ID',
  work_id: 'Work',
  work_revision_id: 'Article revision',
  citation_name: 'Observed author',
  first_name: 'First name',
  last_name: 'Last name',
  orcid: 'ORCID',
  person_id: 'Person',
  article_count: 'Articles',
  affiliation_count: 'Affiliations',
  mention_order: 'Order',
  resolved_work_id: 'Resolved work',
  citing_title: 'Citing article',
  source_name: 'Provider',
  source_type: 'Format',
  record_index: 'Record',
  parse_status: 'Parse outcome',
  reject_reason: 'Reason',
  content_hash: 'Content hash',
  created_at: 'Captured',
};

const scopedSortFields: Record<string, string[]> = {
  articles: ['id', 'title', 'year', 'journal', 'publisher', 'source', 'doi', 'validation_status', 'citation_count', 'reference_count', 'created_at'],
  authors: ['id', 'citation_name', 'first_name', 'last_name', 'orcid', 'article_count', 'affiliation_count', 'created_at'],
  references: ['id', 'work_revision_id', 'mention_order', 'doi', 'title', 'author', 'year', 'source', 'resolved_work_id', 'created_at'],
  sources: ['id', 'run_source_id', 'source_name', 'source_type', 'record_index', 'parse_status', 'reject_reason', 'content_hash', 'created_at'],
  identity_evidence: ['id', 'status', 'citation_name', 'article_title', 'doi', 'candidate_count', 'resolved_at'],
};

/** Returns the ordered union of column names present in result rows. */
function columnNames(table: any): string[] {
  if (!table) {
    return [];
  }
  return (table.columns || []).map(function(column: any) {
    if (typeof column === 'string') {
      return column;
    }
    return column.name;
  }).filter(Boolean);
}

/** Renders the column definition used for identity evidence rows. */
function IdentityEvidenceTable(props: { data: any; context: DataTableContext & { perPage: number } }): JSX.Element {
  const data = props.data;
  const stats = data.stats || {};
  const rows = data.rows || [];
  const page = props.context.page;

  var pct;
  if (stats.resolutions > 0) {
    pct = percent(stats.unclear, stats.resolutions);
  } else {
    pct = '—';
  }

  const metrics = (
    <div className="ui statistics rw-identity-summary">
      <div className="ui statistic"><span className="label">Authors searched by name</span><span className="value">{formatNumber(stats.resolutions)}</span><small>Observed author occurrences</small></div>
      <div className="ui statistic"><span className="label">Unclear ORCID matches</span><span className="value">{formatNumber(stats.unclear)}</span><small>{pct} of searches</small></div>
      <div className="ui statistic"><span className="label">Provider failures</span><span className="value">{formatNumber(stats.provider_failed)}</span><small>Searches with incomplete evidence</small></div>
      <div className="ui statistic"><span className="label">Candidate ORCIDs</span><span className="value">{formatNumber(stats.candidates)}</span><small>Never assigned automatically</small></div>
    </div>
  );

  var body: JSX.Element[];
  if (rows.length) {
    body = rows.map(function(row: any) {
      var errorHtml: JSX.Element | null = null;
      if (row.error_message) {
        errorHtml = <p className="muted">{row.error_message}</p>;
      }
      return <tr>
        <td><StatusChip raw={row.status} />{errorHtml}</td>
        <td><a href={link({ view: 'author', author_id: row.author_occurrence_id })}>{row.queried_citation_name}</a></td>
        <td>{row.article_title || 'Not recorded'}</td>
        <td>{row.doi || 'Not recorded'}</td>
      </tr>;
    });
  } else {
    var emptyMessage;
    if (value('q')) {
      emptyMessage = 'No evidence matches this search.';
    } else {
      emptyMessage = 'No name-search evidence was recorded for this run.';
    }
    body = [<tr><td colspan={4} className="empty">{emptyMessage}</td></tr>];
  }

  const paginationData = data.pagination || {};

  return (
    <Fragment>
      {metrics}
      <div className="table-wrap" aria-label="Author identity evidence table">
        <table className="ui table">
          <thead><tr>
            <th><button type="button" data-sort="status">Status</button></th>
            <th><button type="button" data-sort="citation_name">Observed author</button></th>
            <th><button type="button" data-sort="article_title">Paper</button></th>
            <th><button type="button" data-sort="doi">DOI</button></th>
          </tr></thead>
          <tbody>{body}</tbody>
        </table>
      </div>
      {raw(renderPagination(paginationData, { page: page, perPage: props.context.perPage, itemLabel: 'author records' }))}
    </Fragment>
  );
}

/** Renders a context-preserving record link with a clipped label. */
function clippedRecordLink(kind: string, idKey: string, id: any, title: any): JSX.Element {
  const updates: Record<string, any> = { view: kind, article_id: '', author_id: '', reference_id: '' };
  updates[idKey] = id;
  return <a className="rw-table-title" href={link(updates)} title={title || 'Not recorded'}><span>{title || 'Not recorded'}</span></a>;
}

/** Renders escaped record text clipped to the requested length. */
function clippedRecordText(title: any): JSX.Element {
  return <span className="rw-table-title" title={title || 'Not recorded'}><span>{title || 'Not recorded'}</span></span>;
}

/** Renders the stored search-term coverage for one article row. */
function termMatchMarkup(row: any): JSX.Element {
  const matches = row.term_matches;
  if (matches === null || matches === undefined) {
    return <span className="ui faded text">No search terms recorded</span>;
  }
  const fields = [
    { key: 'title', label: 'Title' },
    { key: 'abstract', label: 'Abstract' },
    { key: 'keywords', label: 'Keywords' },
    { key: 'keywords_plus', label: 'Keywords plus' },
  ];
  return (
    <Fragment>
      <p className="muted">{matches.matched_total} of {matches.term_total} search terms matched</p>
      <div className="rw-term-fields">
        {fields.map(function(field) {
          const terms = matches[field.key] || [];
          return (
            <div className="rw-term-field">
              <span className="rw-term-field__label">{field.label}</span>
              {terms.length
                ? <span className="rw-keyword-tags">{terms.map(function(term: string) {
                  return <span className="ui label">{term}</span>;
                })}</span>
                : <span className="ui faded text">No matched terms</span>}
            </div>
          );
        })}
      </div>
    </Fragment>
  );
}

/** Returns section-specific labels and renderers for corpus columns. */
function corpusColumnConfig(current: string): Record<string, any> {
  const config: Record<string, any> = {};
  Object.entries(columnLabels).forEach(function([key, label]) {
    config[key] = { label: label };
  });
  config.id = { label: 'ID', className: 'col-id' };
  config.work_id = { label: 'Work', className: 'col-id' };
  config.year = { label: 'Year', className: 'col-year' };
  config.source = { label: 'Source', className: 'col-source' };
  config.doi = { label: 'DOI', className: 'col-doi' };
  config.journal = { label: 'Journal', className: 'col-journal' };

  if (current === 'articles') {
    config.title = {
      label: 'Title',
      className: 'col-title',
      render: function(row: any) { return clippedRecordLink('article', 'article_id', row.id, row.title); }
    };
  }
  if (current === 'references') {
    config.title = {
      label: 'Referenced title',
      className: 'col-title',
      render: function(row: any) { return clippedRecordLink('reference', 'reference_id', row.id, row.title); }
    };
    config.citing_title = {
      label: 'Citing article',
      className: 'col-title',
      render: function(row: any) {
        if (row.work_revision_id) {
          return clippedRecordLink('article', 'article_id', row.work_revision_id, row.citing_title);
        }
        return clippedRecordText(row.citing_title);
      }
    };
  }
  if (current === 'authors') {
    config.citation_name = {
      label: 'Observed author',
      className: 'col-person',
      render: function(row: any) { return clippedRecordLink('author', 'author_id', row.id, row.citation_name); }
    };
  }
  if (current === 'sources') {
    config.source_name = { label: 'Provider', className: 'col-provider' };
    config.source_type = { label: 'Format', className: 'col-format' };
    config.record_index = { label: 'Record', className: 'col-record-index' };
    config.parse_status = { label: 'Parse outcome', className: 'col-parse-status' };
    config.reject_reason = { label: 'Reason', className: 'col-reject-reason' };
    config.created_at = { label: 'Captured', className: 'col-captured-at' };
  }
  return config;
}

/** Asynchronously implements corpus view for the viewer. */
export async function corpusView(): Promise<void> {
  var current: string;
  if (corpusSections[section('section', 'articles')]) {
    current = section('section', 'articles');
  } else {
    current = 'articles';
  }

  const definition = corpusSections[current];
  const allTables = await tables();
  const known = allTables.find(function(item) {
    return item.name === definition.table;
  });

  const page = Math.max(1, Number(value('page') || 1));
  var perPage = Number(value('per_page'));
  if (!pageSizes.includes(perPage)) {
    perPage = 50;
  }

  const scoped = Boolean(value('run_id'));
  const allowedSortFields = scoped ? scopedSortFields[current] : columnNames(known);
  var sort = value('sort');
  if (!allowedSortFields.includes(sort)) {
    sort = '';
  }
  var order;
  if (value('order').toLowerCase() === 'desc') {
    order = 'desc';
  } else {
    order = 'asc';
  }

  var data;
  if (scoped) {
    if (current === 'identity_evidence') {
      data = await api('/api/runs/' + encodeURIComponent(value('run_id')) + '/identity-evidence', {
        page: page,
        per_page: perPage,
        sort: sort,
        order: order,
        q: value('q')
      }, { method: 'GET', headers: { Accept: 'application/json' } });
    } else {
      data = await api('/api/runs/' + encodeURIComponent(value('run_id')) + '/corpus/' + current, {
        page: page,
        per_page: perPage,
        sort: sort,
        order: order,
        q: value('q')
      }, { method: 'GET', headers: { Accept: 'application/json' } });
    }
  } else if (known) {
    data = await api('/api/tables/' + encodeURIComponent(definition.table), {
      page: page,
      per_page: perPage,
      sort: sort,
      order: order
    }, { method: 'GET', headers: { Accept: 'application/json' } });
  } else {
    data = null;
  }

  var searchLabel;
  if (scoped) {
    searchLabel = 'Search selected run';
  } else {
    searchLabel = 'Find in displayed page';
  }

  var clearDisabled = !value('q');

  const controls = (
    <form className="ui form rw-table-controls" data-table-search>
      <label className="rw-table-controls__search"><span>{searchLabel}</span><span className="ui input">
        <input id="corpus-query" type="search" value={value('q')} placeholder="Title, DOI, person, source\u2026" />
        <button type="button" className="clear" data-clear-query disabled={clearDisabled} aria-label="Clear search">{"\u00D7"}</button>
      </span></label>
      <label className="rw-table-controls__size">Rows per page<select id="per-page">
        {pageSizes.map(function(size) {
          return <option value={size} selected={size === perPage}>{size}</option>;
        })}
      </select></label>
      <button type="button" data-search-query className="ui primary button">Search</button>
    </form>
  );

  const collectionOptions = Object.entries(corpusSections).map(function([id, item]) {
    const label = id === 'sources' ? 'Source records' : item.title;
    return <option value={id} selected={id === current}>{label}</option>;
  });
  const collectionChooser = (
    <div className="rw-corpus-collection">
      <label htmlFor="corpus-section-select"><span>Corpus collection</span><select id="corpus-section-select">{collectionOptions}</select></label>
      <p>Choose the evidence collection displayed below.</p>
    </div>
  );

  var explanation;
  if (scoped) {
    if (current === 'articles') {
      explanation = <p className="ui info message">This analysis-ready corpus contains only valid normalized work revisions. Discarded works remain available through validation stage outcomes and provenance.</p>;
    } else if (current === 'identity_evidence') {
      explanation = <p className="ui info message">An ORCID returned by a name search is not assigned to this author or a person record. Review candidates and raw provider payloads before any future confirmation. A provider failure means the name search stopped before all configured queries completed.</p>;
    } else {
      explanation = <p className="ui info message">This bounded, paginated list contains only records attached to the selected historical run.</p>;
    }
  } else {
    explanation = <p className="ui info message">Select a run to make this list run-scoped. Without one, Advanced-style workspace records remain bounded and paginated.</p>;
  }

  const context: DataTableContext & { perPage: number } = {
    page: page,
    perPage: perPage,
    query: value('q'),
    sortFields: scoped ? allowedSortFields : undefined,
    columnConfig: corpusColumnConfig(current),
    itemLabel: current === 'articles' ? 'articles' : humanLabelState(current).toLocaleLowerCase(),
    tableClass: 'rw-corpus-table rw-corpus-table--' + current
  };

  if (current === 'articles') {
    context.columnsWhitelist = articlesColumns;
    context.expandableFields = articlesExpandFields;
  } else if (current === 'authors') {
    context.columnsWhitelist = authorColumns;
  } else if (current === 'references') {
    context.columnsWhitelist = referenceColumns;
    context.expandableFields = referenceExpandFields;
  } else if (current === 'sources') {
    context.columnsWhitelist = sourceColumns;
    context.expandableFields = sourceExpandFields;
  }

  var sourceCounts: JSX.Element | null = null;
  if (current === 'sources' && scoped && data) {
    sourceCounts = <SourceResultCountSummary items={data.source_result_counts} classes="span-all" />;
  }

  var body: JSX.Element;
  if (current === 'identity_evidence' && data) {
    body = <IdentityEvidenceTable data={data} context={context} />;
  } else if (data) {
    body = <DataTable tableName={definition.table} result={data} context={context} />;
  } else {
    body = <p className="empty">This database does not contain the expected table.</p>;
  }

  const filterSummary = value('q') ? <FilterChips filters={{ q: value('q') }} labels={{ q: 'Search' }} options={{ clearUpdates: { q: '', page: 1 } }} /> : null;

  renderTree(
    <Fragment>
      <PageHeader kicker="Immutable research corpus" title="Corpus" description="Browse immutable revisions, observed authors, reference mentions, and captured source records." />
      {collectionChooser}
      {sourceCounts}
      <section className="ui segment rw-data-section">
        <div className="ui top attached header"><div><h3>{definition.title}</h3><p>{definition.description}</p></div></div>
        <div className="content">{controls}{filterSummary}{explanation}{body}</div>
      </section>
    </Fragment>,
    app
  );

  document.querySelector('#corpus-section-select')!.addEventListener('change', function(event) {
    setURL({ section: (event.target as HTMLSelectElement).value, page: 1, q: '', sort: '', order: '', expanded: '' }, false);
  });
  bindTableControls(definition.table, page);
}