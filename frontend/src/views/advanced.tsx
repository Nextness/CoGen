// Advanced: table browser, pagination, sort.
import { app, value, PageHeader, EmptyState } from "../state.tsx";
import { h, Fragment, render as renderTree, cx } from "../jsx/jsx-runtime.ts";
import { api, tables, errorMessage } from "../api.tsx";
import type { TableRowsResponse } from "../api/types.ts";
import { DataTable, bindTableControls } from "../components/data-table.tsx";
import type { DataTableContext } from "../components/data-table.tsx";
import { setURL, replaceState } from "../router.tsx";

/** Typed compound class names used by this module. */
const classNames = {
  uiErrorMessage: cx("ui", "error", "message"),
  uiInfoMessage: cx("ui", "info", "message"),
  uiSegment: cx("ui", "segment"),
  uiTopAttachedHeader: cx("ui", "top", "attached", "header"),
};

const advancedPageSizes = [20, 50, 100];

/** Asynchronously implements advanced view for the viewer. */
export async function advancedView(): Promise<void> {
  const allTables = await tables();
  var current = value("table");
  const requestedTable = current;
  const selectedTable = allTables.find((item) => {
    return item.name === current;
  });
  if (!selectedTable && allTables[0]) {
    current = allTables[0].name;
    replaceState({ table: current, page: 1, sort: "", order: "" });
  }

  const page = Math.max(1, Number(value("page") || 1));
  const requestedPerPage = Number(value("per_page"));
  var perPage = 50;
  if (advancedPageSizes.includes(requestedPerPage)) perPage = requestedPerPage;

  if (!current) {
    const emptyStateMarkup = <EmptyState title="Advanced database inspection" detail="No browseable SQLite tables are available." />;
    renderTree(emptyStateMarkup, app);
    return;
  }

  const currentTable = allTables.find((item) => {
    return item.name === current;
  });

  const rawColumns = currentTable?.columns || [];
  const columnNames = rawColumns.map((column) => {
    return column.name;
  });
  const columns = columnNames.filter(Boolean);

  const requestedSort = value("sort");
  var sort = "";
  if (columns.includes(requestedSort)) sort = requestedSort;

  var order = "asc";
  if (value("order").toLowerCase() === "desc") order = "desc";

  var data: TableRowsResponse | null = null;
  var tableError = "";
  try {
    data = await api<TableRowsResponse>(`/api/tables/${encodeURIComponent(current)}`, {
      page: page,
      per_page: perPage,
      sort: sort,
      order: order,
    }, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
  } catch (failure) {
    tableError = errorMessage(failure, "The selected table could not be loaded.");
  }

  if (data && Number(data.pagination?.page) !== page) {
    replaceState({ page: data.pagination.page });
  }

  const tableOptions = allTables.map((item) => {
    return <option value={item.name} selected={item.name === current}>{item.name}</option>;
  });

  const sizeOptions = advancedPageSizes.map((size) => {
    return <option value={size} selected={size === perPage}>{size}</option>;
  });

  const controls = (
    <div className="rw-filter-bar">
      <label className="rw-filter-bar__search">
        Discovered table
        <select id="table-select">{tableOptions}</select>
      </label>
      <label>
        Rows per page
        <select id="per-page">{sizeOptions}</select>
      </label>
    </div>
  );

  const visibleColumns = columns.slice(0, 7);
  const expandableFields = columns.map((column: string) => {
    return {
      f: column,
      w: "full" as const,
    };
  });

  const context: DataTableContext = {
    page: page,
    perPage: perPage,
    itemLabel: "database rows",
    columnsWhitelist: visibleColumns,
    expandableFields: expandableFields,
    expandLongCells: false,
  };

  var tableBody: JSX.Element = <p className={classNames.uiErrorMessage} role="alert">{tableError}</p>;
  if (data) {
    tableBody = <DataTable tableName={current} result={data} context={context} />;
  }
  var canonicalNotice: JSX.Element | null = null;
  if (requestedTable && requestedTable !== current) {
    canonicalNotice = <p className={classNames.uiInfoMessage}>The requested table was not found. The first available table is shown.</p>;
  }

  const pageMarkup = (
    <Fragment>
      <PageHeader kicker="Implementation-level transparency" title="Advanced database inspection" description="Inspect discovered SQLite tables and stored values. This view is for transparency and debugging, not record editing." />
      <section className={classNames.uiSegment}>
        <div className={classNames.uiTopAttachedHeader}>
          <div>
            <h3>{current}</h3>
            <p>Discovered SQLite values are bounded and paginated. No record can be changed from this view.</p>
          </div>
        </div>
        <div className="content">
          <div data-table-scope={current}>
            {canonicalNotice}
            {controls}
            {tableBody}
          </div>
        </div>
      </section>
    </Fragment>
  );
  renderTree(pageMarkup, app);

  const tableSelect = document.querySelector("#table-select")!;
  tableSelect.addEventListener("change", (event) => {
    setURL({
      table: (event.target as HTMLSelectElement).value,
      page: 1,
      sort: "",
      order: "",
    }, false);
  });

  if (data) bindTableControls(current, Number(data.pagination?.page || page));
}
