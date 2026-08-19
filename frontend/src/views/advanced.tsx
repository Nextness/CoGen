// Advanced: table browser, pagination, sort.
import { app, value, pageSizes, PageHeader, EmptyState } from '../state.tsx';
import { h, Fragment, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api, tables } from '../api.tsx';
import { DataTable, bindTableControls } from '../components/data-table.tsx';
import type { DataTableContext } from '../components/data-table.tsx';
import { setURL } from '../router.tsx';

/** Asynchronously implements advanced view for the viewer. */
export async function advancedView(): Promise<void> {
  const allTables = await tables();
  var current = value("table");
  if (!current && allTables[0]) current = allTables[0].name;

  const page = Math.max(1, Number(value("page") || 1));
  const requestedPerPage = Number(value("per_page"));
  var perPage = 50;
  if (pageSizes.includes(requestedPerPage)) perPage = requestedPerPage;

  if (!current) {
    const emptyStateMarkup = <EmptyState title="Advanced database inspection" detail="No browseable SQLite tables are available." />;
    renderTree(emptyStateMarkup, app);
    return;
  }

  const selectedTable = allTables.find((item) => {
    return item.name === current;
  });

  const rawColumns = selectedTable?.columns || [];
  const columnNames = rawColumns.map((column: any) => {
    if (typeof column === "string") return column;
    return column.name;
  });
  const columns = columnNames.filter(Boolean);

  const requestedSort = value("sort");
  var sort = "";
  if (columns.includes(requestedSort)) sort = requestedSort;

  var order = "asc";
  if (value("order").toLowerCase() === "desc") order = "desc";

  const data = await api(`/api/tables/${encodeURIComponent(current)}`, {
    page: page,
    per_page: perPage,
    sort: sort,
    order: order,
  }, {
    method: "GET",
    headers: { Accept: "application/json" },
  });

  const tableOptions = allTables.map((item) => {
    return <option value={item.name} selected={item.name === current}>{item.name} ({item.row_count})</option>;
  });

  const sizeOptions = pageSizes.map((size) => {
    return <option value={size} selected={size === perPage}>{size}</option>;
  });

  const controls = (
    <div className="rw-table-controls">
      <label className="rw-table-controls__search">
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
    tableClass: "rw-advanced-table",
  };

  const pageMarkup = (
    <Fragment>
      <PageHeader kicker="Implementation-level transparency" title="Advanced database inspection" description="Inspect discovered SQLite tables and stored values. This view is for transparency and debugging, not record editing." />
      <section className="ui segment rw-data-section">
        <div className="ui top attached header">
          <div>
            <h3>{current}</h3>
            <p>Discovered SQLite values are bounded and paginated. No record can be changed from this view.</p>
          </div>
        </div>
        <div className="content">
          {controls}
          <DataTable tableName={current} result={data} context={context} />
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

  bindTableControls(current, page);
}
