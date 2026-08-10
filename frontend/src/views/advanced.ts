// Advanced: table browser, pagination, sort.
import { app, esc, value, pageSizes, pageHeader, emptyState } from '../state.ts';
import { api, tables } from '../api.ts';
import { dataTable, bindTableControls } from '../components/data-table.ts';
import { setURL } from '../router.ts';

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
    app.innerHTML = emptyState('Advanced database inspection', 'No browseable SQLite tables are available.');
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

  var tableOptions = allTables.map(function(item) {
    var selected = '';
    if (item.name === current) {
      selected = ' selected';
    }
    return '<option value="' + esc(item.name) + '"' + selected + '>' + esc(item.name) + ' (' + item.row_count + ')</option>';
  }).join('');

  var sizeOptions = pageSizes.map(function(size) {
    var selected = '';
    if (size === perPage) {
      selected = ' selected';
    }
    return '<option value="' + size + '"' + selected + '>' + size + '</option>';
  }).join('');

  const controls = '<div class="rw-table-controls">'
    + '<label class="rw-table-controls__search">Discovered table<select id="table-select">' + tableOptions + '</select></label>'
    + '<label>Rows per page<select id="per-page">' + sizeOptions + '</select></label>'
    + '</div>';

  const visibleColumns = columns.slice(0, 7);
  const expandableFields = columns.map(function(column: string) {
    return { f: column, w: 'full' as const };
  });

  app.innerHTML = pageHeader('Implementation-level transparency', 'Advanced database inspection', 'Inspect discovered SQLite tables and stored values. This view is for transparency and debugging, not record editing.')
    + '<section class="ui segment rw-data-section"><div class="ui top attached header"><div><h3>' + esc(current) + '</h3>'
    + '<p>Discovered SQLite values are bounded and paginated. No record can be changed from this view.</p></div></div>'
    + '<div class="content">' + controls + dataTable(current, data, {
      page: page,
      perPage: perPage,
      itemLabel: 'database rows',
      columnsWhitelist: visibleColumns,
      expandableFields: expandableFields,
      expandLongCells: false,
      tableClass: 'rw-advanced-table'
    })
    + '</div></section>';

  document.querySelector('#table-select')!.addEventListener('change', function(event) {
    setURL({ table: (event.target as HTMLSelectElement).value, page: 1, sort: '', order: '' }, false);
  });

  bindTableControls(current, page);
}