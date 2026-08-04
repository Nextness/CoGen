// Unit tests for views/corpus.js — corpus view, columnNames, identityEvidenceTable.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { corpusView } from '../../../../src/server/frontend/views/corpus.js';
import { app, state, value } from '../../../../src/server/frontend/state.js';

describe('corpus.js — corpusView', function() {

  before(function() {
    state.tables = [];
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('renders corpus view with articles section', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(url) {
      if (url.includes('/api/tables')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() { return Promise.resolve({ data: { tables: [{ name: 'work_revisions', columns: ['id', 'title'] }] } }); },
        });
      }
      if (url.includes('/api/runs')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() {
            return Promise.resolve({
              data: {
                columns: ['id', 'title'],
                rows: [{ id: 1, title: 'Test Article' }],
                pagination: { page: 1, total_pages: 1, total_rows: 1 },
              },
            });
          },
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: [] }); },
      });
    };

    const url = new URL(location.href);
    url.searchParams.set('view', 'corpus');
    url.searchParams.set('section', 'articles');
    url.searchParams.set('run_id', 'run-1');
    history.pushState({}, '', url.toString());

    await corpusView();
    assert.ok(app.innerHTML.includes('Corpus'));

    globalThis.fetch = originalFetch;
    state.tables = [];
    url.searchParams.delete('run_id');
    url.searchParams.delete('section');
    url.searchParams.delete('view');
    history.pushState({}, '', url.toString());
  });

});