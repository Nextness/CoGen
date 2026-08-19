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
  return rows.filter((row) => {
    return Object.values(row).some((item) => {
      return asJSON(item).toLocaleLowerCase().includes(needle);
    });
  });
}

/** Moves focus and scroll position to the table region when available. */
function scrollTableIntoView(): void {
  const wrap = document.querySelector<HTMLElement>(".table-wrap");
  if (wrap) wrap.scrollIntoView({ behavior: "smooth", block: "nearest" });
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
  const columnNames = columns.map((column) => {
    if (typeof column === "string") {
      return column;
    }
    return column.name;
  });
  columns = columnNames.filter(Boolean);

  if (context.columnsWhitelist && context.columnsWhitelist.length) {
    const availableColumns = new Set(columns);
    columns = context.columnsWhitelist.filter((column) => {
      return availableColumns.has(column);
    });
  }

  const keys = {
    page: context.pageKey || "page",
    perPage: context.perPageKey || "per_page",
    sort: context.sortKey || "sort",
    order: context.orderKey || "order",
    query: context.queryKey || "q",
    expanded: context.expandedKey || "expanded",
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

  var emptyMessage = "No records on this page.";
  if (context.query) emptyMessage = "No displayed records match this search.";
  var rowsHtml: JSX.Element[] = [<tr><td colspan={Math.max(1, colCount)} className="empty">{emptyMessage}</td></tr>];
  if (rows.length) {
    rowsHtml = rows.map((row, idx) => {
      const key = String(row[rowKey] ?? idx);
      const initiallyExpanded = expandedRows.has(key);
      const detailID = `table-row-detail-${String(props.tableName).toLowerCase().replace(/[^a-z0-9]+/g, "-")}-${idx}`;
      var toggleCell: JSX.Element | null = null;
      if (hasExpand) {
        var toggleTitle = "Show row details";
        if (initiallyExpanded) toggleTitle = "Hide row details";
        var toggleGlyph = "\u25B6";
        if (initiallyExpanded) toggleGlyph = "\u25BC";
        toggleCell = (
          <td className="toggle-cell">
            <button type="button" className="expand-toggle" aria-expanded={String(initiallyExpanded)} aria-controls={detailID} aria-label={toggleTitle} data-expand-row={idx} data-row-key={key} title={toggleTitle}>
              {toggleGlyph}
            </button>
          </td>
        );
      }

      const cells = columns.map((column) => {
        const config = columnConfig[column] || {};
        var content: JSX.Element = raw(cell(row[column], column, props.tableName, { expandLong: context.expandLongCells !== false }));
        if (config.render) {
          content = config.render(row, row[column]);
        }
        return <td className={config.className}>{content}</td>;
      });
      var rowClasses = "";
      if (hasExpand) {
        rowClasses = "expandable-row";
        if (initiallyExpanded) rowClasses += " expanded";
      }

      var expandRowHtml: JSX.Element | null = null;
      if (hasExpand) {
        const fieldsHtml = expandFields.map((field) => {
          const val = row[field.f];
          var style = `grid-column:span ${field.w}`;
          if (field.w === "full") style = "grid-column:1/-1";
          var display: JSX.Element = <>{asJSON(val)}</>;
          if (field.render) {
            display = field.render(row);
          } else if (val === null || val === undefined) {
            display = <span className="ui faded text">Not recorded</span>;
          }
          return (
            <div style={style}>
              <dt>{field.label || humanLabel(field.f)}</dt>
              <dd>{display}</dd>
            </div>
          );
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
          <tr className={rowClasses} data-row-key={key}>
            {toggleCell}
            {cells}
          </tr>
          {expandRowHtml}
        </Fragment>
      );
    });
  }

  const expandAttr: Record<string, unknown> = {};
  if (hasExpand) {
    expandAttr["data-expandable-table"] = true;
    expandAttr["data-expanded-param"] = keys.expanded;
  }

  var toggleHeader: JSX.Element | null = null;
  if (hasExpand) {
    toggleHeader = <th className="toggle-cell" aria-hidden="true"></th>;
  }

  const headerCells = columns.map((column) => {
    const config = columnConfig[column] || {};
    const label = config.label || column;
    const className = config.className;
    if (sortableColumns.has(column)) {
      var sortIndicator = "";
      var ariaSort = "none";
      if (value(keys.sort) === column) {
        const descending = value(keys.order) === "desc";
        if (descending) {
          sortIndicator = " \u2193";
          ariaSort = "descending";
        } else {
          sortIndicator = " \u2191";
          ariaSort = "ascending";
        }
      }
      return (
        <th scope="col" aria-sort={ariaSort} className={className}>
          <button type="button" data-sort={column}>
            {label}
            {sortIndicator}
          </button>
        </th>
      );
    }
    return <th scope="col" className={className}>{label}</th>;
  });

  var tableClasses = "ui table data-table";
  if (context.tableClass) {
    tableClasses += ` ${esc(context.tableClass)}`;
  }

  const currentSort = value(keys.sort);
  const currentOrder = value(keys.order);
  var sortDescription = "";
  if (currentSort) {
    var orderLabel = "ascending";
    if (currentOrder === "desc") orderLabel = "descending";
    sortDescription = `Sorted by ${currentSort} (${orderLabel})`;
  }

  return (
    <Fragment>
      <div className="table-wrap" data-table-root {...expandAttr}>
        <table className={tableClasses} aria-label={`${props.tableName} results`}>
          <thead>
            <tr>
              {toggleHeader}
              {headerCells}
            </tr>
          </thead>
          <tbody>{rowsHtml}</tbody>
        </table>
      </div>
      <Pagination result={props.result.pagination || { page: page }} options={{
        page: page,
        perPage: context.perPage,
        itemLabel: context.itemLabel || "records",
        secondary: sortDescription,
      }} />
    </Fragment>
  );
}

/** Renders a data table to an HTML string. */
export function dataTable(tableName: string, result: any, context?: DataTableContext): string {
  const tableMarkup = <DataTable tableName={tableName} result={result} context={context} />;
  return renderToString(tableMarkup);
}

/** Binds DOM behavior for table controls. */
export function bindTableControls(tableName: string, page: number, context?: DataTableContext): void {
  if (!context) context = {};
  const keys = {
    page: context.pageKey || "page",
    perPage: context.perPageKey || "per_page",
    sort: context.sortKey || "sort",
    order: context.orderKey || "order",
    query: context.queryKey || "q",
    expanded: context.expandedKey || "expanded",
  };
  /** Maps context key names to their URL query parameter names. */
  function updates(values: Record<string, any>): Record<string, any> {
    const result: Record<string, any> = {};
    Object.entries(values).forEach(([key, raw]) => {
      result[(keys as Record<string, string>)[key] || key] = raw;
    });
    return result;
  }

  const sortButtons = document.querySelectorAll<HTMLButtonElement>("[data-sort]");
  sortButtons.forEach((button) => {
    button.addEventListener("click", () => {
      const sort = button.dataset.sort as string;
      var order = "asc";
      if (value(keys.sort) === sort && value(keys.order) !== "desc") order = "desc";
      scrollTableIntoView();
      setURL(updates({
        sort: sort,
        order: order,
        page: 1,
        expanded: "",
      }), false);
    });
  });

  const pageButtons = document.querySelectorAll<HTMLButtonElement>("[data-page]");
  pageButtons.forEach((button) => {
    button.addEventListener("click", () => {
      scrollTableIntoView();
      setURL(updates({
        page: button.dataset.page,
        expanded: "",
      }), false);
    });
  });

  const perPage = document.querySelector<HTMLSelectElement>(context.perPageSelector || "#per-page");
  if (perPage) {
    perPage.addEventListener("change", (event) => {
      scrollTableIntoView();
      setURL(updates({
        perPage: (event.target as HTMLSelectElement).value,
        page: 1,
        expanded: "",
      }), false);
    });
  }

  const queryInput = document.querySelector<HTMLInputElement>(context.querySelector || "#corpus-query");
  const queryForm = queryInput?.closest<HTMLFormElement>("form");
  if (queryForm) {
    queryForm.addEventListener("submit", (event) => {
      event.preventDefault();
      setURL(updates({
        query: queryInput!.value,
        page: 1,
        expanded: "",
      }), false);
    });
  }

  const searchButton = document.querySelector<HTMLButtonElement>(context.searchButtonSelector || "[data-search-query]");
  if (searchButton && searchButton.type !== "submit") {
    searchButton.addEventListener("click", () => {
      const input = document.querySelector<HTMLInputElement>(context.querySelector || "#corpus-query");
      var query = "";
      if (input) query = input.value;
      setURL(updates({
        query: query,
        page: 1,
        expanded: "",
      }), false);
    });
  }

  const clearButton = document.querySelector<HTMLButtonElement>(context.clearButtonSelector || "[data-clear-query]");
  if (clearButton) {
    clearButton.addEventListener("click", () => {
      setURL(updates({
        query: "",
        page: 1,
        expanded: "",
      }), false);
    });
  }

  const expandTable = document.querySelector<HTMLElement>("[data-expandable-table]");
  if (expandTable) {
    expandTable.addEventListener("click", handleExpandToggle);
  }
}

/** Handles expand toggle. */
function handleExpandToggle(event: Event): void {
  var toggle = (event.target as HTMLElement).closest<HTMLElement>(".expand-toggle");
  if (!toggle) {
    const selection = window.getSelection?.();
    if (selection && !selection.isCollapsed) return;
    if ((event.target as HTMLElement).closest("a, button, input, select, summary, details")) return;
    const row = (event.target as HTMLElement).closest<HTMLElement>("tr");
    if (!row || row.classList.contains("expansion-row")) return;
    toggle = row.querySelector(".expand-toggle");
    if (!toggle) return;
  }

  const rowIdx = toggle.dataset.expandRow as string;
  const expandRow = document.querySelector<HTMLElement>(`tr.expansion-row[data-expand-row="${rowIdx}"]`);
  if (!expandRow) return;

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
