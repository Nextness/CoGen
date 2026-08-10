// Unit tests for components/data-table.js — row filter, data table rendering, controls.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { rowFilter, dataTable, bindTableControls } from '../../../src/components/data-table.ts';
import { state, value } from '../../../src/state.ts';

describe('data-table.js — rowFilter', function() {

  const rows = [
    { name: 'Alice', email: 'alice@example.com' },
    { name: 'Bob', email: 'bob@test.org' },
    { name: 'Charlie', email: 'charlie@example.com' },
  ];

  it('returns all rows when query is empty', function() {
    assert.equal(rowFilter(rows, '').length, 3);
    assert.equal(rowFilter(rows, null).length, 3);
    assert.equal(rowFilter(rows, undefined).length, 3);
  });

  it('filters rows by matching any field', function() {
    const result = rowFilter(rows, 'alice');
    assert.equal(result.length, 1);
    assert.equal(result[0].name, 'Alice');
  });

  it('is case-insensitive', function() {
    const result = rowFilter(rows, 'ALICE');
    assert.equal(result.length, 1);
  });

  it('matches across multiple fields', function() {
    const result = rowFilter(rows, 'example');
    assert.equal(result.length, 2);
  });

  it('returns empty array when no match', function() {
    assert.equal(rowFilter(rows, 'zzzzz').length, 0);
  });

});

describe('data-table.js — dataTable', function() {

  it('renders a table with header and rows', function() {
    const result = {
      columns: ['name', 'age'],
      rows: [
        { name: 'Alice', age: 30 },
        { name: 'Bob', age: 25 },
      ],
      pagination: { page: 1, total_pages: 1, total_rows: 2 },
    };
    const html = dataTable('test', result, { page: 1 });
    assert.ok(html.includes('Alice'));
    assert.ok(html.includes('Bob'));
    assert.ok(html.includes('name'));
    assert.ok(html.includes('age'));
    assert.ok(html.includes('table-wrap'));
    assert.ok(html.includes('pagination'));
  });

  it('renders empty state when no rows', function() {
    const result = {
      columns: ['name'],
      rows: [],
      pagination: { page: 1 },
    };
    const html = dataTable('test', result, { page: 1 });
    assert.ok(html.includes('No records'));
  });

  it('renders empty state with query message when query is set', function() {
    const result = {
      columns: ['name'],
      rows: [],
      pagination: { page: 1 },
    };
    const html = dataTable('test', result, { page: 1, query: 'search' });
    assert.ok(html.includes('No displayed records match'));
  });

  it('renders expandable rows when expandableFields provided', function() {
    const result = {
      columns: ['id', 'title'],
      rows: [{ id: 1, title: 'Test', authors: 'Alice' }],
      pagination: { page: 1 },
    };
    const html = dataTable('test', result, {
      page: 1,
      expandableFields: [{ f: 'authors', w: 'full' }],
    });
    assert.ok(html.includes('expand-toggle'));
    assert.ok(html.includes('aria-label="Show row details"'));
    assert.ok(html.includes('aria-controls="table-row-detail-test-0"'));
    assert.ok(html.includes('title="Show row details"'));
    assert.ok(html.includes('data-expand-row'));
    assert.ok(html.includes('expansion-row'));
    assert.ok(html.includes('data-expandable-table'));
    assert.ok(html.includes('<dt>Authors</dt>'));
  });

  it('renders sort buttons for sortable columns', function() {
    const result = {
      columns: ['name', 'age'],
      rows: [{ name: 'A', age: 1 }],
      pagination: { page: 1 },
    };
    const html = dataTable('test', result, { page: 1, sortFields: ['name', 'age'] });
    assert.ok(html.includes('data-sort'));
    assert.ok(html.includes('button'));
    assert.ok(html.includes('aria-sort="none"'));
  });

  it('disables previous button on page 1', function() {
    const result = {
      columns: ['name'],
      rows: [{ name: 'A' }],
      pagination: { page: 1, total_pages: 3 },
    };
    const html = dataTable('test', result, { page: 1 });
    assert.ok(html.includes('disabled'));
  });

  it('disables next button when has_next is false', function() {
    const result = {
      columns: ['name'],
      rows: [{ name: 'A' }],
      pagination: { page: 3, total_pages: 3, has_next: false },
    };
    const html = dataTable('test', result, { page: 3 });
    assert.ok(html.includes('disabled'));
  });

  it('handles column objects with name property', function() {
    const result = {
      columns: [{ name: 'title' }, { name: 'year' }],
      rows: [{ title: 'Test', year: 2024 }],
      pagination: { page: 1 },
    };
    const html = dataTable('test', result, { page: 1 });
    assert.ok(html.includes('title'));
    assert.ok(html.includes('year'));
    assert.ok(html.includes('Test'));
  });

  it('filters and orders columns by whitelist', function() {
    const result = {
      columns: ['id', 'title', 'year', 'hidden'],
      rows: [{ id: 1, title: 'T', year: 2024, hidden: 'x' }],
      pagination: { page: 1 },
    };
    const html = dataTable('test', result, {
      page: 1,
      columnsWhitelist: ['title', 'id'],
    });
    assert.ok(html.includes('id'));
    assert.ok(html.includes('title'));
    assert.ok(!html.includes('hidden'));
    const host = document.createElement('div');
    host.innerHTML = html;
    assert.deepEqual(Array.from(host.querySelectorAll('thead th'), function(cell) { return cell.textContent; }), ['title', 'id']);
  });

});

describe('data-table.js — bindTableControls', function() {

  it('binds click handlers to sort buttons', function() {
    document.body.innerHTML = '<div id="app"><table><thead><tr>'
      + '<th scope="col"><button type="button" data-sort="name">name</button></th>'
      + '</tr></thead></table></div>';

    bindTableControls('test', 1);

    const button = document.querySelector('[data-sort]');
    assert.ok(button);

    button.click();
    assert.ok(true);
  });

  it('binds click handlers to page buttons', function() {
    document.body.innerHTML = '<div id="app">'
      + '<button type="button" data-page="2">Next</button>'
      + '</div>';

    bindTableControls('test', 1);

    const button = document.querySelector('[data-page]');
    assert.ok(button);
    button.click();
    assert.ok(true);
  });

});
