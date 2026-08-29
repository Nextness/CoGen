// Data table rendering, pagination, sort controls, and cell rendering.
import { asJSON, list, value, Cell, humanLabel } from "../state.tsx";
import { setURL } from "../router.tsx";
import { h, Fragment, cx, classToggle, classHas } from "../jsx/jsx-runtime.ts";
import type { ClassNames } from "../jsx/jsx-runtime.ts";
import { Pagination } from "./pagination.tsx";
import type { ColumnInfo, DataTableContext, WireRecord } from "../api/types.ts";

/** Typed compound class names used by this module. */
const classNames = {
  uiFadedText: cx("ui", "faded", "text"),
};

/** Returns whether a row contains the case-insensitive filter text. */
export function rowFilter(rows: WireRecord[], query: string): WireRecord[] {
  if (!query) return rows;
  const needle = query.toLocaleLowerCase();
  return rows.filter((row) => {
    return Object.values(row).some((item) => {
      return asJSON(item).toLocaleLowerCase().includes(needle);
    });
  });
}

/** Moves focus and scroll position to the table region when available. */
function scrollTableIntoView(root: HTMLElement): void {
  const wrap = root.querySelector<HTMLElement>(".table-wrap");
  if (wrap?.scrollIntoView) wrap.scrollIntoView({ behavior: "smooth", block: "nearest" });
}

/** One data table option set. */
export type { DataTableContext } from "../api/types.ts";

/** Renders and binds a filterable, sortable, paginated in-memory data table. */
export function DataTable(props: { tableName: string; result: unknown; context?: DataTableContext }): JSX.Element {
  const context = props.context || {};
  const response = props.result as { columns?: Array<string | ColumnInfo>; schema?: Array<string | ColumnInfo>; rows?: WireRecord[]; items?: WireRecord[]; table?: { columns?: Array<string | ColumnInfo>; schema?: Array<string | ColumnInfo> }; pagination?: WireRecord };

  var rawColumns = list<string | ColumnInfo>(response, ["columns", "schema"]);
  if (!rawColumns.length) {
    rawColumns = list<string | ColumnInfo>(response.table, ["columns", "schema"]);
  }
  const columnNames = rawColumns.map((column) => {
    if (typeof column === "string") return column;
    return column.name;
  });
  var columns = columnNames.filter(Boolean);

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
  const rows = rowFilter(list<WireRecord>(response, ["rows", "items"]), context.query || "");
  const sortableColumns = new Set(context.sortFields || columns);
  const expandFields = context.expandableFields || [];
  const hasExpand = expandFields.length > 0;
  var colCount = columns.length;
  if (hasExpand) colCount += 1;
  const rowKey = context.rowKey || "id";
  const expandedRows = new Set(String(value(keys.expanded) || "").split(",").filter(Boolean));
  const columnConfig = context.columnConfig || {};

  var emptyMessage = "No records on this page.";
  if (context.query) emptyMessage = "No displayed records match this search.";
  const emptyColspan = Math.max(1, colCount);
  var rowsHtml: JSX.Element[] = [<tr><td colSpan={emptyColspan} className="rw-table-empty">{emptyMessage}</td></tr>];
  if (rows.length) {
    rowsHtml = rows.map((row, idx) => {
      const key = String(row[rowKey] ?? idx);
      const initiallyExpanded = expandedRows.has(key);
      const detailID = `table-row-detail-${String(props.tableName).toLowerCase().replace(/[^a-z0-9]+/g, "-")}-${idx}`;
      const expandedValue = String(initiallyExpanded);
      var toggleCell: JSX.Element | null = null;
      if (hasExpand) {
        var toggleTitle = "Show row details";
        if (initiallyExpanded) toggleTitle = "Hide row details";
        var toggleGlyph = "\u25B6";
        if (initiallyExpanded) toggleGlyph = "\u25BC";
        toggleCell = (
          <td className="toggle-cell">
            <button type="button" className="expand-toggle" aria-expanded={expandedValue} aria-controls={detailID} aria-label={toggleTitle} data-expand-row={idx} data-row-key={key} title={toggleTitle}>
              {toggleGlyph}
            </button>
          </td>
        );
      }

      const cells = columns.map((column) => {
        const config = columnConfig[column] || {};
        var content: JSX.Element = <Cell item={row[column]} column={column} tableName={props.tableName} options={{ expandLong: context.expandLongCells !== false }} />;
        if (config.render) content = config.render(row, row[column]);
        return <td className={config.className}>{content}</td>;
      });
      var rowClasses: ClassNames | undefined;
      if (hasExpand) rowClasses = cx("expandable-row", initiallyExpanded && "expanded");

      var expandRowHtml: JSX.Element | null = null;
      if (hasExpand) {
        const fieldsHtml = expandFields.map((field) => {
          const val = row[field.f];
          var style = `grid-column:span ${field.w}`;
          if (field.w === "full") style = "grid-column:1/-1";
          var display: JSX.Element = <>{asJSON(val)}</>;
          if (field.render) display = field.render(row);
          else if (val === null || val === undefined) display = <span className={classNames.uiFadedText}>Not recorded</span>;
          const labelText = field.label || humanLabel(field.f);
          return (
            <div style={style}>
              <dt>{labelText}</dt>
              <dd>{display}</dd>
            </div>
          );
        });
        expandRowHtml = (
          <tr id={detailID} className="expansion-row" data-expand-row={idx} data-row-key={key} hidden={!initiallyExpanded}>
            <td colSpan={colCount}>
              <dl className="rw-property-grid">{fieldsHtml}</dl>
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

  const tableClasses = cx("ui", "table", ...(context.tableClasses || []));

  const currentSort = value(keys.sort);
  const currentOrder = value(keys.order);
  var sortDescription = "";
  if (currentSort) {
    var orderLabel = "ascending";
    if (currentOrder === "desc") orderLabel = "descending";
    sortDescription = `Sorted by ${currentSort} (${orderLabel})`;
  }

  return (
    <section data-table-owner={props.tableName}>
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
      <Pagination result={response.pagination || { page: context.page }} options={{
        page: context.page,
        perPage: context.perPage,
        itemLabel: context.itemLabel || "records",
        secondary: sortDescription,
      }} />
    </section>
  );
}

/** Binds DOM behavior for table controls. */
export function bindTableControls(tableName: string, page: number, context?: DataTableContext): void {
  if (!context) context = {};
  const root = Array.from(document.querySelectorAll<HTMLElement>("[data-table-owner]")).find((candidate) => {
    return candidate.dataset.tableOwner === tableName;
  });
  if (!root) return;
  const controlScope = root.closest<HTMLElement>("[data-table-scope]") || root;
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

  const sortButtons = root.querySelectorAll<HTMLButtonElement>("[data-sort]");
  sortButtons.forEach((button) => {
    button.addEventListener("click", () => {
      const sort = button.dataset.sort as string;
      var order = "asc";
      if (value(keys.sort) === sort && value(keys.order) !== "desc") order = "desc";
      scrollTableIntoView(root);
      setURL(updates({
        sort: sort,
        order: order,
        page: 1,
        expanded: "",
      }), false);
    });
  });

  const pageButtons = root.querySelectorAll<HTMLButtonElement>("[data-page]");
  pageButtons.forEach((button) => {
    button.addEventListener("click", () => {
      scrollTableIntoView(root);
      setURL(updates({
        page: button.dataset.page,
        expanded: "",
      }), false);
    });
  });

  const perPage = controlScope.querySelector<HTMLSelectElement>(context.perPageSelector || "#per-page");
  if (perPage) {
    perPage.addEventListener("change", (event) => {
      scrollTableIntoView(root);
      setURL(updates({
        perPage: (event.target as HTMLSelectElement).value,
        page: 1,
        expanded: "",
      }), false);
    });
  }

  const queryInput = controlScope.querySelector<HTMLInputElement>(context.querySelector || "#corpus-query");
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

  const searchButton = controlScope.querySelector<HTMLButtonElement>(context.searchButtonSelector || "[data-search-query]");
  if (searchButton && searchButton.type !== "submit") {
    searchButton.addEventListener("click", () => {
      const input = controlScope.querySelector<HTMLInputElement>(context.querySelector || "#corpus-query");
      var query = "";
      if (input) query = input.value;
      setURL(updates({
        query: query,
        page: 1,
        expanded: "",
      }), false);
    });
  }

  const clearButton = controlScope.querySelector<HTMLButtonElement>(context.clearButtonSelector || "[data-clear-query]");
  if (clearButton) {
    clearButton.addEventListener("click", () => {
      setURL(updates({
        query: "",
        page: 1,
        expanded: "",
      }), false);
    });
  }

  const expandTable = root.querySelector<HTMLElement>("[data-expandable-table]");
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
    if (!row || classHas(row, "expansion-row")) return;
    toggle = row.querySelector(".expand-toggle");
    if (!toggle) return;
  }

  const rowIdx = toggle.dataset.expandRow as string;
  const table = toggle.closest<HTMLElement>("[data-expandable-table]");
  const expandRow = table?.querySelector<HTMLElement>(`tr.expansion-row[data-expand-row="${rowIdx}"]`);
  if (!expandRow) return;

  const expanded = !expandRow.hidden;
  expandRow.hidden = expanded;
  toggle.setAttribute("aria-expanded", String(!expanded));
  const sourceRow = toggle.closest("tr");
  if (sourceRow) {
    classToggle(sourceRow, "expanded", !expanded);
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
    const expandedParam = table?.dataset.expandedParam || "expanded";
    const expandedKeys = new Set(String(value(expandedParam) || "").split(",").filter(Boolean));
    if (expanded) expandedKeys.delete(rowKey);
    else expandedKeys.add(rowKey);
    const url = new URL(location.href);
    if (expandedKeys.size) {
      url.searchParams.set(expandedParam, Array.from(expandedKeys).join(","));
    } else {
      url.searchParams.delete(expandedParam);
    }
    history.replaceState({}, "", url.toString());
  }
}
