// Unit tests for views/relationships.js — relationships view.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { relationshipsView } from '../../../../src/server/frontend/views/relationships.js';
import { app, state } from '../../../../src/server/frontend/state.js';

describe('relationships.js — relationshipsView', function() {

  before(function() {
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('shows empty state when no run_id is set', async function() {
    const url = new URL(location.href);
    url.searchParams.delete('run_id');
    history.pushState({}, '', url.toString());

    await relationshipsView();
    assert.ok(app.innerHTML.includes('Select a run attempt'));
  });

  it('renders relationships view when run_id is set', async function() {
    const originalFetch = globalThis.fetch;
    var requested = '';
    globalThis.fetch = function(input) {
      requested = String(input);
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() {
          return Promise.resolve({
            data: {
              nodes: [],
              edges: [],
              counts: { article_matches: 0, nodes_rendered: 0, edges_rendered: 0 },
            },
          });
        },
      });
    };

    const url = new URL(location.href);
    url.searchParams.set('run_id', 'run-1');
    history.pushState({}, '', url.toString());

    await relationshipsView();
    assert.ok(app.innerHTML.includes('Relationships'));
    assert.ok(app.innerHTML.includes('Explore a bounded network'));
    assert.ok(app.innerHTML.includes('rw-segmented-control'));
    assert.ok(app.innerHTML.includes('research_network'));
    assert.ok(app.innerHTML.includes('Advanced filters'));
    assert.ok(app.innerHTML.includes('Connected clusters'));
    assert.ok(app.innerHTML.includes('No relationships match'));
    assert.ok(app.innerHTML.includes('ui primary button'));
    assert.ok(requested.includes('article_limit=2000'));
    assert.equal(app.querySelector('#graph-form [name="graph_mode"]'), null);

    globalThis.fetch = originalFetch;
    url.searchParams.delete('run_id');
    history.pushState({}, '', url.toString());
  });

});
