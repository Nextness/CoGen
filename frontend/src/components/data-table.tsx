// Data table rendering, pagination, sort controls, and cell rendering.
import { esc, asJSON, list, value, cell, humanLabel } from "../state.tsx";
import { setURL } from "../router.tsx";
import { h, Fragment, renderToString, raw } from "../jsx/jsx-runtime.ts";
import { Pagination } from "./pagination.tsx";

/** Returns whether a row contains the case-insensitive filter text. */
export function rowFilter(rows: any[], query: string): any[] {
  if (!query) {
    return rows;
  }
  const needle = query.toLocaleLowerCase();
  return rows.filter(function(row) {
    return Object.values(row).some(function(item) {
      return asJSON(item).toLocaleLowerCase().includes(needle);
    });
  });
}

/** Moves focus and scroll position to the table region when available. */
function scrollTableIntoView(): void {
  var wrap = document.querySelector<HTMLElement>(".table-wrap");
  if (wrap) {
    wrap.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }
}

/** One data table option set. */
export interface DataTableContext {
  columnsWhitelist?: string[];
  pageKey?: string;
  perPageKey?: string;
  sortKey?: string;
  orderKey?: string;
  queryKey?: string;
  expandedKey?: string;
  query?: string;
  page?: number;
  perPage?: number;
  sortFields?: string[];
  expandableFields?: Array<{ f: string; w: number | string; label?: string; render?: (row: any) => JSX.Element }>;
  rowKey?: string;
  columnConfig?: Record<string, { label?: string; className?: string; render?: (row: any, value: any) => JSX.Element }>;
  expandLongCells?: boolean;
  tableClass?: string;
  itemLabel?: string;
  perPageSelector?: string;
  querySelector?: string;
  searchButtonSelector?: string;
  clearButtonSelector?: string;
}

/** Renders and binds a filterable, sortable, paginated in-memory data table. */
export function DataTable(props: { tableName: string; result: any; context?: DataTableContext }): JSX.Element {
  const context = props.context || {};

  var columns = list(props.result, ["columns", "schema"]);
  if (!columns.length) {
    columns = list(props.result.table, ["columns", "schema"]);
  }
  columns = columns.map(function(column) {
    if (typeof column === "string") {
      return column;
    }
    return column.name;
  }).filter(Boolean);

  if (context.columnsWhitelist && context.columnsWhitelist.length) {
    const availableColumns = new Set(columns);
    columns = context.columnsWhitelist.filter(function(column) {
      return availableColumns.has(column);
    });
  }

  const keys = {
    page: context.pageKey || "page",
    perPage: context.perPageKey || "per_page",
    sort: context.sortKey || "sort",
    order: context.orderKey || "order",
    query: context.queryKey || "q",
    expanded: context.expandedKey || "expanded"
  };
  const rows = rowFilter(list(props.result, ["rows", "items"]), context.query || "");
  const page = context.page;
  const sortableColumns = new Set(context.sortFields || columns);
  const expandFields = context.expandableFields || [];
  const hasExpand = expandFields.length > 0;
  const colCount = columns.length + (hasExpand ? 1 : 0);
  const rowKey = context.rowKey || "id";
  const expandedRows = new Set(String(value(keys.expanded) || "").split(",").filter(Boolean));
  const columnConfig = context.columnConfig || {};

  let rowsHtml: JSX.Element[];
  if (rows.length) {
    rowsHtml = rows.map(function(row, idx) {
      const key = String(row[rowKey] ?? idx);
      const initiallyExpanded = expandedRows.has(key);
      const detailID = "table-row-detail-" + String(props.tableName).toLowerCase().replace(/[^a-z0-9]+/g, "-") + "-" + idx;
      let toggleCell: JSX.Element | null = null;
      if (hasExpand) {
        const toggleTitle = initiallyExpanded ? "Hide row details" : "Show row details";
        toggleCell = (
          <td className="toggle-cell">
            <button type="button" className="expand-toggle" aria-expanded={String(initiallyExpanded)} aria-controls={detailID} aria-label={toggleTitle} data-expand-row={idx} data-row-key={key} title={toggleTitle}>
              {initiallyExpanded ? "\u25BC" : "\u25B6"}
            </button>
          </td>
        );
      }

      const cells = columns.map(function(column) {
        const config = columnConfig[column] || {};
        const content: JSX.Element = config.render
          ? config.render(row, row[column])
          : raw(cell(row[column], column, props.tableName, { expandLong: context.expandLongCells !== false }));
        return <td className={config.className}>{content}</td>;
      });
      const rowClasses = hasExpand ? "expandable-row" + (initiallyExpanded ? " expanded" : "") : "";

      let expandRowHtml: JSX.Element | null = null;
      if (hasExpand) {
        const fieldsHtml = expandFields.map(function(ef) {
          const val = row[ef.f];
          const style = ef.w === "full" ? "grid-column:1/-1" : "grid-column:span " + ef.w;
          let display: JSX.Element;
          if (ef.render) {
            display = ef.render(row);
          } else if (val === null || val === undefined) {
            display = <span className="ui faded text">Not recorded</span>;
          } else {
            display = <>{asJSON(val)}</>;
          }
          return <div style={style}><dt>{ef.label || humanLabel(ef.f)}</dt><dd>{display}</dd></div>;
        });
        expandRowHtml = (
          <tr id={detailID} className="expansion-row" data-expand-row={idx} data-row-key={key} hidden={!initiallyExpanded}>
            <td colspan={colCount}>
              <dl className="property-grid">{fieldsHtml}</dl>
            </td>
          </tr>
        );
      }

      return (
        <Fragment>
          <tr className={rowClasses} data-row-key={key}>{toggleCell}{cells}</tr>
          {expandRowHtml}
        </Fragment>
      );
    });
  } else {
    const emptyMessage = context.query ? "No displayed records match this search." : "No records on this page.";
    rowsHtml = [<tr><td colspan={Math.max(1, colCount)} className="empty">{emptyMessage}</td></tr>];
  }

  const expandAttr: Record<string, unknown> = {};
  if (hasExpand) {
    expandAttr["data-expandable-table"] = true;
    expandAttr["data-expanded-param"] = keys.expanded;
  }

  const toggleHeader = hasExpand ? <th className="toggle-cell" aria-hidden="true"></th> : null;

  const headerCells = columns.map(function(column) {
    const config = columnConfig[column] || {};
    const label = config.label || column;
    const className = config.className;
    if (sortableColumns.has(column)) {
      var sortIndicator = "";
      if (value(keys.sort) === column) {
        sortIndicator = value(keys.order) === "desc" ? " \u2193" : " \u2191";
      }
      var ariaSort = "none";
      if (value(keys.sort) === column) {
        ariaSort = value(keys.order) === "desc" ? "descending" : "ascending";
      }
      return <th scope="col" aria-sort={ariaSort} className={className}><button type="button" data-sort={column}>{label}{sortIndicator}</button></th>;
    }
    return <th scope="col" className={className}>{label}</th>;
  });

  var tableClasses = "ui table data-table";
  if (context.tableClass) {
    tableClasses = tableClasses + " " + esc(context.tableClass);
  }

  const currentSort = value(keys.sort);
  const currentOrder = value(keys.order);
  var sortDescription = "";
  if (currentSort) {
    sortDescription = "Sorted by " + currentSort + " (" + (currentOrder === "desc" ? "descending" : "ascending") + ")";
  }

  return (
    <Fragment>
      <div className="table-wrap" data-table-root {...expandAttr}>
        <table className={tableClasses} aria-label={props.tableName + " results"}>
          <thead><tr>{toggleHeader}{headerCells}</tr></thead>
          <tbody>{rowsHtml}</tbody>
        </table>
      </div>
      <Pagination result={props.result.pagination || { page: page }} options={{
        page: page,
        perPage: context.perPage,
        itemLabel: context.itemLabel || "records",
        secondary: sortDescription
      }} />
    </Fragment>
  );
}

/** Renders and binds a filterable, sortable, paginated in-memory data table. */
export function dataTable(tableName: string, result: any, context?: DataTableContext): string {
  return renderToString(<DataTable tableName={tableName} result={result} context={context} />);
}

/** Binds DOM behavior for table controls. */
export function bindTableControls(tableName: string, page: number, context?: DataTableContext): void {
  if (!context) {
    context = {};
  }
  const keys = {
    page: context.pageKey || "page",
    perPage: context.perPageKey || "per_page",
    sort: context.sortKey || "sort",
    order: context.orderKey || "order",
    query: context.queryKey || "q",
    expanded: context.expandedKey || "expanded"
  };
  /** Updates s. */
  function updates(values: Record<string, any>): Record<string, any> {
    const result: Record<string, any> = {};
    Object.entries(values).forEach(function([key, raw]) {
      result[(keys as Record<string, string>)[key] || key] = raw;
    });
    return result;
  }

  document.querySelectorAll<HTMLButtonElement>("[data-sort]").forEach(function(button) {
    button.addEventListener("click", function() {
      const sort = button.dataset.sort as string;
      var order;
      if (value(keys.sort) === sort && value(keys.order) !== "desc") {
        order = "desc";
      } else {
        order = "asc";
      }
      scrollTableIntoView();
      setURL(updates({ sort: sort, order: order, page: 1, expanded: "" }), false);
    });
  });

  document.querySelectorAll<HTMLButtonElement>("[data-page]").forEach(function(button) {
    button.addEventListener("click", function() {
      scrollTableIntoView();
      setURL(updates({ page: button.dataset.page, expanded: "" }), false);
    });
  });

  const perPage = document.querySelector<HTMLSelectElement>(context.perPageSelector || "#per-page");
  if (perPage) {
    perPage.addEventListener("change", function(event) {
      scrollTableIntoView();
      setURL(updates({ perPage: (event.target as HTMLSelectElement).value, page: 1, expanded: "" }), false);
    });
  }

  const queryInput = document.querySelector<HTMLInputElement>(context.querySelector || "#corpus-query");
  const queryForm = queryInput?.closest<HTMLFormElement>("form");
  if (queryForm) {
    queryForm.addEventListener("submit", function(event) {
      event.preventDefault();
      setURL(updates({ query: queryInput!.value, page: 1, expanded: "" }), false);
    });
  }

  const searchButton = document.querySelector<HTMLButtonElement>(context.searchButtonSelector || "[data-search-query]");
  if (searchButton && searchButton.type !== "submit") {
    searchButton.addEventListener("click", function() {
      const input = document.querySelector<HTMLInputElement>(context.querySelector || "#corpus-query");
      var query = "";
      if (input) {
        query = input.value;
      }
      setURL(updates({ query: query, page: 1, expanded: "" }), false);
    });
  }

  const clearButton = document.querySelector<HTMLButtonElement>(context.clearButtonSelector || "[data-clear-query]");
  if (clearButton) {
    clearButton.addEventListener("click", function() {
      setURL(updates({ query: "", page: 1, expanded: "" }), false);
    });
  }

  const expandTable = document.querySelector<HTMLElement>("[data-expandable-table]");
  if (expandTable) {
    expandTable.addEventListener("click", (event: Event) => handleExpandToggle(event));
  }
}

/** Handles expand toggle. */
function handleExpandToggle(event: Event): void {
  var toggle = (event.target as HTMLElement).closest<HTMLElement>(".expand-toggle");
  if (!toggle) {
    const selection = window.getSelection?.();
    if (selection && !selection.isCollapsed) {
      return;
    }
    if ((event.target as HTMLElement).closest("a, button, input, select, summary, details")) {
      return;
    }
    const row = (event.target as HTMLElement).closest<HTMLElement>("tr");
    if (!row || row.classList.contains("expansion-row")) {
      return;
    }
    toggle = row.querySelector(".expand-toggle");
    if (!toggle) {
      return;
    }
  }

  const rowIdx = toggle.dataset.expandRow as string;
  const expandRow = document.querySelector<HTMLElement>(`tr.expansion-row[data-expand-row="` + rowIdx + `"]`);
  if (!expandRow) {
    return;
  }

  const expanded = !expandRow.hidden;
  expandRow.hidden = expanded;
  toggle.setAttribute("aria-expanded", String(!expanded));
  const sourceRow = toggle.closest("tr");
  if (sourceRow) {
    sourceRow.classList.toggle("expanded", !expanded);
  }
  if (expanded) {
    toggle.textContent = "\u25B6";
    toggle.title = "Show row details";
    toggle.setAttribute("aria-label", "Show row details");
  } else {
    toggle.textContent = "\u25BC";
    toggle.title = "Hide row details";
    toggle.setAttribute("aria-label", "Hide row details");
  }

  const rowKey = toggle.dataset.rowKey;
  if (rowKey) {
    const table = toggle.closest<HTMLElement>("[data-expandable-table]");
    const expandedParam = table?.dataset.expandedParam || "expanded";
    const expandedKeys = new Set(String(value(expandedParam) || "").split(",").filter(Boolean));
    if (expanded) {
      expandedKeys.delete(rowKey);
    } else {
      expandedKeys.add(rowKey);
    }
    const url = new URL(location.href);
    if (expandedKeys.size) {
      url.searchParams.set(expandedParam, Array.from(expandedKeys).join(","));
    } else {
      url.searchParams.delete(expandedParam);
    }
    history.replaceState({}, "", url.toString());
  }
}