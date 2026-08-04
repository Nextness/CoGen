import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { evaluationView } from '../../../../src/server/frontend/views/evaluation.js';
import { app, state } from '../../../../src/server/frontend/state.js';

/** Sets location. */
function setLocation(values) {
  const url = new URL(location.href);
  ['view', 'run_id', 'page', 'per_page', 'sort', 'order', 'q'].forEach(function(key) {
    url.searchParams.delete(key);
  });
  Object.entries(values).forEach(function(entry) {
    url.searchParams.set(entry[0], entry[1]);
  });
  history.pushState({}, '', url.toString());
}

/** Builds a mock fetch response. */
function response(data) {
  return Promise.resolve({
    ok: true,
    status: 200,
    json: function() { return Promise.resolve({ data: data }); }
  });
}

describe('evaluation.js - evaluationView', function() {
  before(function() {
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('requires a selected run without requesting evaluation data', async function() {
    const originalFetch = globalThis.fetch;
    var requested = false;
    globalThis.fetch = function() {
      requested = true;
      return response([]);
    };
    setLocation({ view: 'evaluation' });

    await evaluationView();

    assert.equal(requested, false);
    assert.ok(app.innerHTML.includes('Select a run attempt'));
    globalThis.fetch = originalFetch;
  });

  it('renders normalized articles with red and green inventory tags', async function() {
    const originalFetch = globalThis.fetch;
    var requested = '';
    globalThis.fetch = function(input) {
      requested = String(input);
      return response({
        columns: ['title', 'doi', 'inventory_status', 'inventoried_at'],
        rows: [
          { work_revision_id: 1, title: 'Available article', doi: '10.1000/available', inventory_status: 'available', inventoried_at: '2026-07-29T12:00:00Z' },
          { work_revision_id: 2, title: 'Missing article', doi: '10.1000/missing', inventory_status: 'not_available', inventoried_at: null }
        ],
        pagination: { page: 1, per_page: 50, total_rows: 2, total_pages: 1 }
      });
    };
    setLocation({ view: 'evaluation', run_id: '7' });

    await evaluationView();

    assert.ok(requested.includes('/api/runs/7/evaluation'));
    assert.ok(app.innerHTML.includes('Inventory Status'));
    assert.ok(app.innerHTML.includes('Inventoried at'));
    assert.equal(app.querySelectorAll('.ui.green.label').length, 1);
    assert.equal(app.querySelectorAll('.ui.red.label').length, 1);
    assert.ok(app.querySelector('.ui.green.label').textContent.includes('Available'));
    assert.ok(app.querySelector('.ui.red.label').textContent.includes('Not Available'));
    globalThis.fetch = originalFetch;
  });
});
