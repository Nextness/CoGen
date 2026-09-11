// Corpus: articles, authors, references, sources lists.
import {
  app, value, pageSizes, corpusSections, section, PageHeader,
  formatNumber, percent,
  humanLabel as humanLabelState, SourceResultCountSummary, FilterChips, StatusChip, detailLinkFor, columnNamesOf,
} from "../state.tsx";
import { h, Fragment, render as renderTree, cx } from "../jsx/jsx-runtime.ts";
import type { ClassName } from "../jsx/classes.ts";
import { api, tables } from "../api.tsx";
import type { CorpusResponse, IdentityEvidenceResponse, IdentityEvidenceRow, TableInfo, TableRowsResponse, WireRecord } from "../api/types.ts";
import { DataTable, bindTableControls } from "../components/data-table.tsx";
import type { DataTableContext } from "../components/data-table.tsx";
import { Pagination } from "../components/pagination.tsx";
import { articlesColumns, articlesExpandFields } from "../components/article-table.tsx";
import { articleCollectionView } from "./evaluation.tsx";
import { setURL } from "../router.tsx";

/** Typed compound class names used by this module. */
const classNames = {
  uiFadedText: cx("ui", "faded", "text"),
  uiFormRwFilterBar: cx("ui", "form", "rw-filter-bar"),
  uiInfoMessage: cx("ui", "info", "message"),
  uiInput: cx("ui", "input"),
  uiLabel: cx("ui", "label"),
  uiPrimaryButton: cx("ui", "primary", "button"),
  uiSegment: cx("ui", "segment"),
  uiStatistic: cx("ui", "statistic"),
  uiStatisticsRwIdentitySummary: cx("ui", "statistics", "rw-identity-summary"),
  uiTable: cx("ui", "table"),
  uiTopAttachedHeader: cx("ui", "top", "attached", "header"),
};

const authorColumns = ["citation_name", "orcid", "first_name", "last_name", "article_count", "affiliation_count"];

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
function columnNames(table: TableInfo | undefined): string[] {
  if (!table) return [];
  return columnNamesOf(table.columns);
}

/** Renders the column definition used for identity evidence rows. */
function IdentityEvidenceTable(props: { data: IdentityEvidenceResponse; context: DataTableContext & { perPage: number } }): JSX.Element {
  const stats = props.data.stats;
  const rows = props.data.rows;

  var pct = "—";
  if (stats.resolutions > 0) pct = percent(stats.unclear, stats.resolutions);

  const metrics = (
    <div className={classNames.uiStatisticsRwIdentitySummary}>
      <div className={classNames.uiStatistic}>
        <span className="label">Authors searched by name</span>
        <span className="value">{formatNumber(stats.resolutions)}</span>
        <small>Observed author occurrences</small>
      </div>
      <div className={classNames.uiStatistic}>
        <span className="label">Unclear ORCID matches</span>
        <span className="value">{formatNumber(stats.unclear)}</span>
        <small>{pct} of searches</small>
      </div>
      <div className={classNames.uiStatistic}>
        <span className="label">Provider failures</span>
        <span className="value">{formatNumber(stats.provider_failed)}</span>
        <small>Searches with incomplete evidence</small>
      </div>
      <div className={classNames.uiStatistic}>
        <span className="label">Candidate ORCIDs</span>
        <span className="value">{formatNumber(stats.candidates)}</span>
        <small>Never assigned automatically</small>
      </div>
    </div>
  );

  var emptyMessage = "No name-search evidence was recorded for this run.";
  if (value("q")) emptyMessage = "No evidence matches this search.";
  const emptyCell = <td colSpan={4} className="rw-table-empty">{emptyMessage}</td>;
  var body: JSX.Element[] = [<tr>{emptyCell}</tr>];
  if (rows.length) {
    body = rows.map((row: IdentityEvidenceRow) => {
      var errorHtml: JSX.Element | null = null;
      if (row.error_message) errorHtml = <p className={classNames.uiFadedText}>{row.error_message}</p>;
      const authorTarget = detailLinkFor("author", row.author_occurrence_id);
      var authorLink: JSX.Element = <span>{row.queried_citation_name}</span>;
      if (row.evidence_revision_id !== null) authorLink = <a href={authorTarget.href} data-state={JSON.stringify(authorTarget.state)}>{row.queried_citation_name}</a>;
      var articleLink: JSX.Element = <span>{row.article_title || "Not recorded"}</span>;
      if (row.work_revision_id) articleLink = clippedRecordLink("article", row.work_revision_id, row.article_title);
      const stageLabel = humanLabelState(row.evidence_stage);
      var evidenceStage: JSX.Element | null = null;
      if (row.evidence_stage) evidenceStage = <small className={classNames.uiFadedText}>Search captured during {stageLabel}</small>;
      return (
        <tr>
          <td>
            <StatusChip raw={row.status} />
            {errorHtml}
          </td>
          <td>
            {authorLink}
          </td>
          <td>
            {articleLink}
            {evidenceStage}
          </td>
          <td>{row.doi || "Not recorded"}</td>
        </tr>
      );
    });
  }

  const paginationData = props.data.pagination || {};
  const paginationOptions = {
    page: props.context.page,
    perPage: props.context.perPage,
    itemLabel: "article evidence records",
  };
  const paginationMarkup = <Pagination result={paginationData} options={paginationOptions} />;

  return (
    <section data-table-owner={corpusSections.identity_evidence.table}>
      {metrics}
      <div className="table-wrap" aria-label="Author identity evidence table">
        <table className={classNames.uiTable}>
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
    </section>
  );
}

/** Renders the clipped label text for a record title. */
function clippedLabel(title: unknown): JSX.Element {
  return <span>{String(title || "Not recorded")}</span>;
}

/** Renders a context-preserving record link with a clipped label. */
function clippedRecordLink(kind: string, id: unknown, title: unknown): JSX.Element {
  const target = detailLinkFor(kind as "article" | "author" | "reference", id);
  return <a className="rw-table-title" href={target.href} data-state={JSON.stringify(target.state)} title={String(title || "Not recorded")}>{clippedLabel(title)}</a>;
}

/** Renders escaped record text clipped to the requested length. */
function clippedRecordText(title: unknown): JSX.Element {
  return <span className="rw-table-title" title={String(title || "Not recorded")}>{clippedLabel(title)}</span>;
}

/** Returns section-specific labels and renderers for corpus columns. */
function corpusColumnConfig(current: string): NonNullable<DataTableContext["columnConfig"]> {
  const config: NonNullable<DataTableContext["columnConfig"]> = {};
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
      render: (row: WireRecord) => {
        return clippedRecordLink("article", row.id, row.title);
      },
    };
  }
  if (current === "references") {
    config.mention_order = {
      label: "Order",
      className: "col-reference-order",
    };
    config.title = {
      label: "Referenced title",
      className: "col-reference-title",
      render: (row: WireRecord) => {
        return clippedRecordLink("reference", row.id, row.title);
      },
    };
    config.author = {
      label: "Referenced author",
      className: "col-reference-author",
      render: (row: WireRecord) => {
        return clippedRecordText(row.author);
      },
    };
    config.citing_title = {
      label: "Citing article",
      className: "col-citing-title",
      render: (row: WireRecord) => {
        if (row.work_revision_id) {
          return clippedRecordLink("article", row.work_revision_id, row.citing_title);
        }
        return clippedRecordText(row.citing_title);
      },
    };
  }
  if (current === "authors") {
    config.citation_name = {
      label: "Observed author",
      render: (row: WireRecord) => {
        return clippedRecordLink("author", row.id, row.citation_name);
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

  const heading = (
    <Fragment>
      <PageHeader kicker="Articles and research evidence" title="Corpus" description="Browse articles, review PDFs and decisions, and inspect their authors, references, and source evidence." />
      {collectionChooser}
    </Fragment>
  );
  if (current === "articles" && value("run_id")) {
    await articleCollectionView(heading, corpusView);
    bindCorpusCollection();
    return;
  }

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

  var data: CorpusResponse | IdentityEvidenceResponse | TableRowsResponse | null = null;
  if (scoped) {
    if (current === "identity_evidence") {
      data = await api<IdentityEvidenceResponse>(`/api/runs/${runID}/identity-evidence`, {
        page: page,
        per_page: perPage,
        sort: sort,
        order: order,
        q: query,
      });
    } else {
      data = await api<CorpusResponse>(`/api/runs/${runID}/corpus/${current}`, {
        page: page,
        per_page: perPage,
        sort: sort,
        order: order,
        q: query,
      });
    }
  } else if (knownTable) {
    data = await api<TableRowsResponse>(`/api/tables/${encodeURIComponent(definition.table)}`, {
      page: page,
      per_page: perPage,
      sort: sort,
      order: order,
    });
  }

  var searchLabel = "Find in displayed page";
  if (scoped) searchLabel = "Search selected run";
  const clearDisabled = !query;

  const pageSizeOptions = pageSizes.map((size) => {
    return <option value={size} selected={size === perPage}>{size}</option>;
  });
  const controls = (
    <form className={classNames.uiFormRwFilterBar} data-table-search>
      <label className="rw-filter-bar__search">
        <span>{searchLabel}</span>
        <span className={classNames.uiInput}>
          <input id="corpus-query" type="search" value={query} placeholder="Title, DOI, person, source\u2026" />
          <button type="button" className="clear" data-clear-query disabled={clearDisabled} aria-label="Clear search">{"\u00D7"}</button>
        </span>
      </label>
      <label>
        Rows per page
        <select id="per-page">{pageSizeOptions}</select>
      </label>
      <button type="button" data-search-query className={classNames.uiPrimaryButton}>Search</button>
    </form>
  );

  var explanation: JSX.Element = <p className={classNames.uiInfoMessage}>Select a run to make this list run-scoped. Without one, Advanced-style workspace records remain bounded and paginated.</p>;
  if (scoped) {
    if (current === "articles") {
      explanation = <p className={classNames.uiInfoMessage}>This analysis-ready corpus contains only valid normalized work revisions. Discarded works remain available through validation stage outcomes and provenance.</p>;
    } else if (current === "identity_evidence") {
      explanation = <p className={classNames.uiInfoMessage}>Searches describe the author occurrence captured at the recorded pipeline stage; article links open the normalized revision when available. An ORCID returned by a name search is not assigned to this author or a person record. Review candidates and raw provider payloads before any future confirmation. A provider failure means the name search stopped before all configured queries completed.</p>;
    } else {
      explanation = <p className={classNames.uiInfoMessage}>This bounded, paginated list contains only records attached to the selected historical run.</p>;
    }
  }

  var sortFields: string[] | undefined = undefined;
  if (scoped) sortFields = allowedSortFields;
  var itemLabel = humanLabelState(current).toLocaleLowerCase();
  if (current === "articles") itemLabel = "articles";
  const tableClasses: ClassName[] = ["rw-corpus-table"];
  if (current === "references") tableClasses.push("rw-corpus-table--references");
  if (current === "sources") tableClasses.push("rw-corpus-table--sources");
  const context: DataTableContext & { perPage: number } = {
    page: page,
    perPage: perPage,
    query: query,
    sortFields: sortFields,
    columnConfig: corpusColumnConfig(current),
    itemLabel: itemLabel,
    tableClasses: tableClasses,
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
    sourceCounts = <SourceResultCountSummary items={(data as CorpusResponse).source_result_counts || []} classes={["rw-grid-span-all"]} />;
  }

  var body: JSX.Element = <p className={classNames.uiFadedText}>This database does not contain the expected table.</p>;
  if (current === "identity_evidence" && data) {
    body = <IdentityEvidenceTable data={data as IdentityEvidenceResponse} context={context} />;
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
      {heading}
      {sourceCounts}
      <section className={classNames.uiSegment}>
        <div className={classNames.uiTopAttachedHeader}>
          <div>
            <h3>{definition.title}</h3>
            <p>{definition.description}</p>
          </div>
        </div>
        <div className="content">
          <div data-table-scope={definition.table}>
            {controls}
            {filterSummary}
            {explanation}
            {body}
          </div>
        </div>
      </section>
    </Fragment>
  );
  renderTree(pageMarkup, app);

  bindCorpusCollection();
  bindTableControls(definition.table);
}

/** Binds the collection selector and clears filters owned by the previous collection. */
function bindCorpusCollection(): void {
  const sectionSelect = document.querySelector("#corpus-section-select")!;
  sectionSelect.addEventListener("change", (event) => {
    setURL({
      section: (event.target as HTMLSelectElement).value,
      page: 1,
      q: "",
      sort: "",
      order: "",
      expanded: "",
      pdf_status: "",
      review_status: "",
      review_source: "",
      qualifier: "",
      source: "",
      reviewed: "",
    }, false);
  });
}
