// Unit tests for views/advanced.js — advanced database inspection view.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { advancedView } from '../../../../src/server/frontend/views/advanced.js';
import { app, state } from '../../../../src/server/frontend/state.js';

describe('advanced.js — advancedView', function() {

  before(function() {
    state.tables = [];
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('renders advanced view with table browser', async function() {
    const originalFetch = globalThis.fetch;
    var fetchCount = 0;
    globalThis.fetch = function(url) {
      fetchCount += 1;
      if (url.includes('/api/tables') && !url.includes('/api/tables/')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() {
            return Promise.resolve({
              data: { tables: [{ name: 'works', columns: ['id', 'title'], row_count: 100 }] },
            });
          },
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() {
          return Promise.resolve({
            data: {
              columns: ['id', 'title'],
              rows: [{ id: 1, title: 'Test' }],
              pagination: { page: 1, total_pages: 1, total_rows: 1 },
            },
          });
        },
      });
    };

    const url = new URL(location.href);
    url.searchParams.set('view', 'advanced');
    history.pushState({}, '', url.toString());

    await advancedView();
    assert.ok(app.innerHTML.includes('Advanced database inspection'));
    assert.ok(app.querySelector('th.toggle-cell'));
    assert.equal(app.querySelector('th.toggle-cell').textContent, '');
    assert.ok(app.querySelector('td.toggle-cell .expand-toggle'));
    assert.ok(fetchCount >= 2);

    globalThis.fetch = originalFetch;
    state.tables = [];
    url.searchParams.delete('view');
    history.pushState({}, '', url.toString());
  });

});
