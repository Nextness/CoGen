// Advanced: table browser, pagination, sort.
import { app, value, pageSizes, PageHeader, EmptyState } from '../state.tsx';
import { h, Fragment, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api, tables } from '../api.tsx';
import { DataTable, bindTableControls } from '../components/data-table.tsx';
import { setURL } from '../router.tsx';

/** Asynchronously implements advanced view for the viewer. */
export async function advancedView(): Promise<void> {
  const allTables = await tables();
  var current = value('table');
  if (!current && allTables[0]) {
    current = allTables[0].name;
  }

  const page = Math.max(1, Number(value('page') || 1));
  var perPage = Number(value('per_page'));
  if (!pageSizes.includes(perPage)) {
    perPage = 50;
  }

  if (!current) {
    renderTree(<EmptyState title="Advanced database inspection" detail="No browseable SQLite tables are available." />, app);
    return;
  }

  const selectedTable = allTables.find(function(item) {
    return item.name === current;
  });

  const columns = (selectedTable?.columns || []).map(function(column: any) {
    if (typeof column === 'string') {
      return column;
    }
    return column.name;
  }).filter(Boolean);

  var sort = value('sort');
  if (!columns.includes(sort)) {
    sort = '';
  }

  var order;
  if (value('order').toLowerCase() === 'desc') {
    order = 'desc';
  } else {
    order = 'asc';
  }

  const data = await api('/api/tables/' + encodeURIComponent(current), {
    page: page,
    per_page: perPage,
    sort: sort,
    order: order
  }, { method: 'GET', headers: { Accept: 'application/json' } });

  const tableOptions = allTables.map(function(item) {
    return <option value={item.name} selected={item.name === current}>{item.name} ({item.row_count})</option>;
  });

  const sizeOptions = pageSizes.map(function(size) {
    return <option value={size} selected={size === perPage}>{size}</option>;
  });

  const controls = (
    <div className="rw-table-controls">
      <label className="rw-table-controls__search">Discovered table<select id="table-select">{tableOptions}</select></label>
      <label>Rows per page<select id="per-page">{sizeOptions}</select></label>
    </div>
  );

  const visibleColumns = columns.slice(0, 7);
  const expandableFields = columns.map(function(column: string) {
    return { f: column, w: 'full' as const };
  });

  renderTree(
    <Fragment>
      <PageHeader kicker="Implementation-level transparency" title="Advanced database inspection" description="Inspect discovered SQLite tables and stored values. This view is for transparency and debugging, not record editing." />
      <section className="ui segment rw-data-section">
        <div className="ui top attached header"><div><h3>{current}</h3><p>Discovered SQLite values are bounded and paginated. No record can be changed from this view.</p></div></div>
        <div className="content">{controls}
          <DataTable tableName={current} result={data} context={{
            page: page,
            perPage: perPage,
            itemLabel: 'database rows',
            columnsWhitelist: visibleColumns,
            expandableFields: expandableFields,
            expandLongCells: false,
            tableClass: 'rw-advanced-table'
          }} />
        </div>
      </section>
    </Fragment>,
    app
  );

  document.querySelector('#table-select')!.addEventListener('change', function(event) {
    setURL({ table: (event.target as HTMLSelectElement).value, page: 1, sort: '', order: '' }, false);
  });

  bindTableControls(current, page);
}