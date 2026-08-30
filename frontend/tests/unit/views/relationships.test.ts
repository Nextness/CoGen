// Unit tests for views/relationships.tsx — relationships view.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.ts';
import { seedViewerState } from '../seed.ts';
import { relationshipsView } from '../../../src/views/relationships.tsx';
import { app, state } from '../../../src/state.tsx';

describe('relationships.tsx — relationshipsView', function() {

  before(function() {
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('shows empty state when no run_id is set', async function() {
    seedViewerState({ view: 'relationships' });

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
      } as unknown as Response);
    } as typeof fetch;

    seedViewerState({ view: 'relationships', run_id: 'run-1' });

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
  });

});
