// Corpus: articles, authors, references, sources lists.
import {
  app, value, link, pageSizes, corpusSections, section, PageHeader,
  formatNumber, percent,
  humanLabel as humanLabelState, SourceResultCountSummary, FilterChips, StatusChip
} from '../state.tsx';
import { h, Fragment, raw, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api, tables } from '../api.tsx';
import { DataTable, bindTableControls } from '../components/data-table.tsx';
import type { DataTableContext } from '../components/data-table.tsx';
import { pagination as renderPagination } from '../components/pagination.tsx';
import { setURL } from '../router.tsx';

// Core columns shown in the articles table; extra fields appear in expandable rows.
const articlesColumns = ["doi", "title", "year", "journal", "source"];
const authorColumns = ["citation_name", "orcid", "first_name", "last_name", "article_count", "affiliation_count"];
const articlesExpandFields = [
  {
    f: "title",
    w: "full" as const,
  },
  {
    f: "authors",
    w: "full" as const,
  },
  {
    f: "journal",
    w: 10,
  },
  {
    f: "publisher",
    w: 10,
  },
  {
    f: "abstract",
    w: "full" as const,
  },
  {
    f: "term_matches",
    w: "full" as const,
    label: "Matched search terms",
    render: termMatchMarkup,
  },
  {
    f: "work_id",
    w: 4,
  },
  {
    f: "year",
    w: 4,
  },
  {
    f: "source",
    w: 4,
  },
  {
    f: "doi",
    w: 4,
  },
  {
    f: "validation_status",
    w: 4,
  },
  {
    f: "citation_count",
    w: 5,
  },
  {
    f: "reference_count",
    w: 5,
  },
  {
    f: "producer_stage",
    w: 5,
  },
  {
    f: "created_at",
    w: 5,
  },
];

const referenceColumns = ["mention_order", "title", "author", "year", "doi", "citing_title"];
const referenceExpandFields = [
  {
    f: "id",
    w: 4,
  },
  {
    f: "work_revision_id",
    w: 4,
  },
  {
    f: "resolved_work_id",
    w: 4,
  },
  {
    f: "source",
    w: 4,
  },
  {
    f: "created_at",
    w: 4,
  },
];

const sourceColumns = ["source_name", "source_type", "record_index", "parse_status", "reject_reason", "created_at"];
const sourceExpandFields = [
  {
    f: "id",
    w: 3,
  },
  {
    f: "run_source_id",
    w: 4,
  },
  {
    f: "content_hash",
    w: 13,
  },
];

const columnLabels: Record<string, string> = {
  id: "ID",
  work_id: "Work",
  work_revision_id: "Article revision",
  citation_name: "Observed author",
  first_name: "First name",
  last_name: "Last name",
  orcid: "ORCID",
  person_id: "Person",
  article_count: "Articles",
  affiliation_count: "Affiliations",
  mention_order: "Order",
  resolved_work_id: "Resolved work",
  citing_title: "Citing article",
  source_name: "Provider",
  source_type: "Format",
  record_index: "Record",
  parse_status: "Parse outcome",
  reject_reason: "Reason",
  content_hash: "Content hash",
  created_at: "Captured",
};

const scopedSortFields: Record<string, string[]> = {
  articles: ["id", "title", "year", "journal", "publisher", "source", "doi", "validation_status", "citation_count", "reference_count", "created_at"],
  authors: ["id", "citation_name", "first_name", "last_name", "orcid", "article_count", "affiliation_count", "created_at"],
  references: ["id", "work_revision_id", "mention_order", "doi", "title", "author", "year", "source", "resolved_work_id", "created_at"],
  sources: ["id", "run_source_id", "source_name", "source_type", "record_index", "parse_status", "reject_reason", "content_hash", "created_at"],
  identity_evidence: ["id", "status", "citation_name", "article_title", "doi", "candidate_count", "resolved_at"],
};

/** Returns the ordered union of column names present in result rows. */
function columnNames(table: any): string[] {
  if (!table) return [];
  const names = (table.columns || []).map((column: any) => {
    if (typeof column === "string") return column;
    return column.name;
  });
  return names.filter(Boolean);
}

/** Renders the column definition used for identity evidence rows. */
function IdentityEvidenceTable(props: { data: any; context: DataTableContext & { perPage: number } }): JSX.Element {
  const stats = props.data.stats || {};
  const rows = props.data.rows || [];

  var pct = "—";
  if (stats.resolutions > 0) pct = percent(stats.unclear, stats.resolutions);

  const metrics = (
    <div className="ui statistics rw-identity-summary">
      <div className="ui statistic">
        <span className="label">Authors searched by name</span>
        <span className="value">{formatNumber(stats.resolutions)}</span>
        <small>Observed author occurrences</small>
      </div>
      <div className="ui statistic">
        <span className="label">Unclear ORCID matches</span>
        <span className="value">{formatNumber(stats.unclear)}</span>
        <small>{pct} of searches</small>
      </div>
      <div className="ui statistic">
        <span className="label">Provider failures</span>
        <span className="value">{formatNumber(stats.provider_failed)}</span>
        <small>Searches with incomplete evidence</small>
      </div>
      <div className="ui statistic">
        <span className="label">Candidate ORCIDs</span>
        <span className="value">{formatNumber(stats.candidates)}</span>
        <small>Never assigned automatically</small>
      </div>
    </div>
  );

  var emptyMessage = "No name-search evidence was recorded for this run.";
  if (value("q")) emptyMessage = "No evidence matches this search.";
  const emptyCell = <td colspan={4} className="empty">{emptyMessage}</td>;
  var body: JSX.Element[] = [<tr>{emptyCell}</tr>];
  if (rows.length) {
    body = rows.map((row: any) => {
      var errorHtml: JSX.Element | null = null;
      if (row.error_message) errorHtml = <p className="muted">{row.error_message}</p>;
      const authorHref = link({
        view: "author",
        author_id: row.author_occurrence_id,
      });
      return (
        <tr>
          <td>
            <StatusChip raw={row.status} />
            {errorHtml}
          </td>
          <td>
            <a href={authorHref}>{row.queried_citation_name}</a>
          </td>
          <td>{row.article_title || "Not recorded"}</td>
          <td>{row.doi || "Not recorded"}</td>
        </tr>
      );
    });
  }

  const paginationData = props.data.pagination || {};
  const paginationOptions = {
    page: props.context.page,
    perPage: props.context.perPage,
    itemLabel: "author records",
  };
  const paginationMarkup = raw(renderPagination(paginationData, paginationOptions));

  return (
    <Fragment>
      {metrics}
      <div className="table-wrap" aria-label="Author identity evidence table">
        <table className="ui table">
          <thead>
            <tr>
              <th><button type="button" data-sort="status">Status</button></th>
              <th><button type="button" data-sort="citation_name">Observed author</button></th>
              <th><button type="button" data-sort="article_title">Paper</button></th>
              <th><button type="button" data-sort="doi">DOI</button></th>
            </tr>
          </thead>
          <tbody>{body}</tbody>
        </table>
      </div>
      {paginationMarkup}
    </Fragment>
  );
}

/** Renders the clipped label text for a record title. */
function clippedLabel(title: any): JSX.Element {
  return <span>{title || "Not recorded"}</span>;
}

/** Renders a context-preserving record link with a clipped label. */
function clippedRecordLink(kind: string, idKey: string, id: any, title: any): JSX.Element {
  const updates: Record<string, any> = {
    view: kind,
    article_id: "",
    author_id: "",
    reference_id: "",
  };
  updates[idKey] = id;
  const recordHref = link(updates);
  return <a className="rw-table-title" href={recordHref} title={title || "Not recorded"}>{clippedLabel(title)}</a>;
}

/** Renders escaped record text clipped to the requested length. */
function clippedRecordText(title: any): JSX.Element {
  return <span className="rw-table-title" title={title || "Not recorded"}>{clippedLabel(title)}</span>;
}

/** Renders the stored search-term coverage for one article row. */
function termMatchMarkup(row: any): JSX.Element {
  if (row.term_matches === null || row.term_matches === undefined) {
    return <span className="ui faded text">No search terms recorded</span>;
  }
  const fields = [
    {
      key: "title",
      label: "Title",
    },
    {
      key: "abstract",
      label: "Abstract",
    },
    {
      key: "keywords",
      label: "Keywords",
    },
    {
      key: "keywords_plus",
      label: "Keywords plus",
    },
  ];
  const fieldElements: JSX.Element[] = fields.map(({ key, label }) => {
    const terms = row.term_matches[key] || [];
    var content: JSX.Element = <span className="ui faded text">No matched terms</span>;
    if (terms.length) {
      const termTags: JSX.Element[] = terms.map((term: string) => {
        return <span className="ui label">{term}</span>;
      });
      content = <span className="rw-keyword-tags">{termTags}</span>;
    }
    return (
      <div className="rw-term-field">
        <span className="rw-term-field__label">{label}</span>
        {content}
      </div>
    );
  });
  const matchedTotal = row.term_matches.matched_total;
  const termTotal = row.term_matches.term_total;
  return (
    <Fragment>
      <p className="muted">{matchedTotal} of {termTotal} search terms matched</p>
      <div className="rw-term-fields">{fieldElements}</div>
    </Fragment>
  );
}

/** Returns section-specific labels and renderers for corpus columns. */
function corpusColumnConfig(current: string): Record<string, any> {
  const config: Record<string, any> = {};
  Object.entries(columnLabels).forEach(([key, label]) => {
    config[key] = { label: label };
  });
  config.id = {
    label: "ID",
    className: "col-id",
  };
  config.work_id = {
    label: "Work",
    className: "col-id",
  };
  config.year = {
    label: "Year",
    className: "col-year",
  };
  config.source = {
    label: "Source",
    className: "col-source",
  };
  config.doi = {
    label: "DOI",
    className: "col-doi",
  };
  config.journal = {
    label: "Journal",
    className: "col-journal",
  };

  if (current === "articles") {
    config.title = {
      label: "Title",
      className: "col-title",
      render: (row: any) => {
        return clippedRecordLink("article", "article_id", row.id, row.title);
      },
    };
  }
  if (current === "references") {
    config.title = {
      label: "Referenced title",
      className: "col-title",
      render: (row: any) => {
        return clippedRecordLink("reference", "reference_id", row.id, row.title);
      },
    };
    config.citing_title = {
      label: "Citing article",
      className: "col-title",
      render: (row: any) => {
        if (row.work_revision_id) {
          return clippedRecordLink("article", "article_id", row.work_revision_id, row.citing_title);
        }
        return clippedRecordText(row.citing_title);
      },
    };
  }
  if (current === "authors") {
    config.citation_name = {
      label: "Observed author",
      className: "col-person",
      render: (row: any) => {
        return clippedRecordLink("author", "author_id", row.id, row.citation_name);
      },
    };
  }
  if (current === "sources") {
    config.source_name = {
      label: "Provider",
      className: "col-provider",
    };
    config.source_type = {
      label: "Format",
      className: "col-format",
    };
    config.record_index = {
      label: "Record",
      className: "col-record-index",
    };
    config.parse_status = {
      label: "Parse outcome",
      className: "col-parse-status",
    };
    config.reject_reason = {
      label: "Reason",
      className: "col-reject-reason",
    };
    config.created_at = {
      label: "Captured",
      className: "col-captured-at",
    };
  }
  return config;
}

/** Asynchronously implements corpus view for the viewer. */
export async function corpusView(): Promise<void> {
  const requestedSection = section("section", "articles");
  var current = "articles";
  if (corpusSections[requestedSection]) current = requestedSection;

  const definition = corpusSections[current];
  const allTables = await tables();
  const knownTable = allTables.find((item) => {
    return item.name === definition.table;
  });

  const page = Math.max(1, Number(value("page") || 1));
  const requestedPerPage = Number(value("per_page"));
  var perPage = 50;
  if (pageSizes.includes(requestedPerPage)) perPage = requestedPerPage;

  const runID = value("run_id");
  const scoped = Boolean(runID);
  const query = value("q");
  var allowedSortFields: string[] = columnNames(knownTable);
  if (scoped) allowedSortFields = scopedSortFields[current];
  const requestedSort = value("sort");
  var sort = "";
  if (allowedSortFields.includes(requestedSort)) sort = requestedSort;
  var order = "asc";
  if (value("order").toLowerCase() === "desc") order = "desc";

  var data: any = null;
  if (scoped) {
    if (current === "identity_evidence") {
      data = await api(`/api/runs/${runID}/identity-evidence`, {
        page: page,
        per_page: perPage,
        sort: sort,
        order: order,
        q: query,
      }, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
    } else {
      data = await api(`/api/runs/${runID}/corpus/${current}`, {
        page: page,
        per_page: perPage,
        sort: sort,
        order: order,
        q: query,
      }, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
    }
  } else if (knownTable) {
    data = await api(`/api/tables/${encodeURIComponent(definition.table)}`, {
      page: page,
      per_page: perPage,
      sort: sort,
      order: order,
    }, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
  }

  var searchLabel = "Find in displayed page";
  if (scoped) searchLabel = "Search selected run";
  const clearDisabled = !query;

  const pageSizeOptions = pageSizes.map((size) => {
    return <option value={size} selected={size === perPage}>{size}</option>;
  });
  const controls = (
    <form className="ui form rw-table-controls" data-table-search>
      <label className="rw-table-controls__search">
        <span>{searchLabel}</span>
        <span className="ui input">
          <input id="corpus-query" type="search" value={query} placeholder="Title, DOI, person, source\u2026" />
          <button type="button" className="clear" data-clear-query disabled={clearDisabled} aria-label="Clear search">{"\u00D7"}</button>
        </span>
      </label>
      <label className="rw-table-controls__size">
        Rows per page
        <select id="per-page">{pageSizeOptions}</select>
      </label>
      <button type="button" data-search-query className="ui primary button">Search</button>
    </form>
  );

  const collectionOptions = Object.entries(corpusSections).map(([id, item]) => {
    var label = item.title;
    if (id === "sources") label = "Source records";
    return <option value={id} selected={id === current}>{label}</option>;
  });
  const collectionChooser = (
    <div className="rw-corpus-collection">
      <label htmlFor="corpus-section-select">
        <span>Corpus collection</span>
        <select id="corpus-section-select">{collectionOptions}</select>
      </label>
      <p>Choose the evidence collection displayed below.</p>
    </div>
  );

  var explanation: JSX.Element = <p className="ui info message">Select a run to make this list run-scoped. Without one, Advanced-style workspace records remain bounded and paginated.</p>;
  if (scoped) {
    if (current === "articles") {
      explanation = <p className="ui info message">This analysis-ready corpus contains only valid normalized work revisions. Discarded works remain available through validation stage outcomes and provenance.</p>;
    } else if (current === "identity_evidence") {
      explanation = <p className="ui info message">An ORCID returned by a name search is not assigned to this author or a person record. Review candidates and raw provider payloads before any future confirmation. A provider failure means the name search stopped before all configured queries completed.</p>;
    } else {
      explanation = <p className="ui info message">This bounded, paginated list contains only records attached to the selected historical run.</p>;
    }
  }

  var sortFields: string[] | undefined = undefined;
  if (scoped) sortFields = allowedSortFields;
  var itemLabel = humanLabelState(current).toLocaleLowerCase();
  if (current === "articles") itemLabel = "articles";
  const context: DataTableContext & { perPage: number } = {
    page: page,
    perPage: perPage,
    query: query,
    sortFields: sortFields,
    columnConfig: corpusColumnConfig(current),
    itemLabel: itemLabel,
    tableClass: `rw-corpus-table rw-corpus-table--${current}`,
  };

  if (current === "articles") {
    context.columnsWhitelist = articlesColumns;
    context.expandableFields = articlesExpandFields;
  } else if (current === "authors") {
    context.columnsWhitelist = authorColumns;
  } else if (current === "references") {
    context.columnsWhitelist = referenceColumns;
    context.expandableFields = referenceExpandFields;
  } else if (current === "sources") {
    context.columnsWhitelist = sourceColumns;
    context.expandableFields = sourceExpandFields;
  }

  var sourceCounts: JSX.Element | null = null;
  if (current === "sources" && scoped && data) {
    sourceCounts = <SourceResultCountSummary items={data.source_result_counts} classes="span-all" />;
  }

  var body: JSX.Element = <p className="empty">This database does not contain the expected table.</p>;
  if (current === "identity_evidence" && data) {
    body = <IdentityEvidenceTable data={data} context={context} />;
  } else if (data) {
    body = <DataTable tableName={definition.table} result={data} context={context} />;
  }

  const clearUpdates = {
    q: "",
    page: 1,
  };
  var filterSummary: JSX.Element | null = null;
  if (query) {
    filterSummary = <FilterChips filters={{ q: query }} labels={{ q: "Search" }} options={{ clearUpdates: clearUpdates }} />;
  }

  const pageMarkup = (
    <Fragment>
      <PageHeader kicker="Immutable research corpus" title="Corpus" description="Browse immutable revisions, observed authors, reference mentions, and captured source records." />
      {collectionChooser}
      {sourceCounts}
      <section className="ui segment rw-data-section">
        <div className="ui top attached header">
          <div>
            <h3>{definition.title}</h3>
            <p>{definition.description}</p>
          </div>
        </div>
        <div className="content">
          {controls}
          {filterSummary}
          {explanation}
          {body}
        </div>
      </section>
    </Fragment>
  );
  renderTree(pageMarkup, app);

  const sectionSelect = document.querySelector("#corpus-section-select")!;
  sectionSelect.addEventListener("change", (event) => {
    setURL({
      section: (event.target as HTMLSelectElement).value,
      page: 1,
      q: "",
      sort: "",
      order: "",
      expanded: "",
    }, false);
  });
  bindTableControls(definition.table, page);
}