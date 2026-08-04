// Unit tests for views/trash.js — trashed runs view.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { trashView } from '../../../../src/server/frontend/views/trash.js';
import { app, state } from '../../../../src/server/frontend/state.js';

describe('trash.js — trashView', function() {

  before(function() {
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('renders trash view with trashed runs and restore history', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(url) {
      if (url.includes('/api/trash')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() {
            return Promise.resolve({
              data: { runs: [{ id: 'trashed-1', status: 'failed' }] },
            });
          },
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() {
          return Promise.resolve({
            data: { events: [{ action: 'run_restored', occurred_at: '2024-01-15T10:00:00Z' }] },
          });
        },
      });
    };

    await trashView();
    assert.ok(app.innerHTML.includes('Trash'));
    assert.ok(app.innerHTML.includes('Trashed run attempts'));
    assert.ok(app.innerHTML.includes('Restore history'));
    assert.ok(!app.innerHTML.includes('rw-dashboard--equal-height'));

    globalThis.fetch = originalFetch;
  });

});
