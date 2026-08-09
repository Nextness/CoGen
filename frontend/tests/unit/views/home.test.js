// Unit tests for views/home.js, including workspace history and run lifecycle controls.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { homeView } from '../../../../src/server/frontend/views/home.js';
import { app, state } from '../../../../src/server/frontend/state.js';

/** Returns a JSON response compatible with the frontend API helper. */
function response(data) {
  return Promise.resolve({
    ok: true,
    status: 200,
    json: function() { return Promise.resolve({ data: data }); },
  });
}

describe('home.js — homeView', function() {

  it('renders research hierarchy metrics, elapsed run data, Deepdive links, and reversible lifecycle controls', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(input) {
      const url = String(input);
      if (url.startsWith('/api/searches')) {
        return response({ searches: [{
          id: 1,
          search_id: 'process-mining',
          created_at: '2024-01-01T00:00:00Z',
          revisions: [{ id: 11, label: 'Revision 1', created_at: '2024-01-02T00:00:00Z' }]
        }] });
      }
      if (url.startsWith('/api/plans')) {
        return response({ plans: [{ id: 21, search_revision_id: 11, execution_fingerprint: 'fingerprint' }] });
      }
      if (url.startsWith('/api/runs')) {
        return response({ runs: [{
          id: 31,
          execution_plan_id: 21,
          search_revision_id: 11,
          attempt_number: 3,
          status: 'completed',
          visibility_state: 'active',
          started_at: '2024-01-03T10:00:00Z',
          finished_at: '2024-01-03T10:12:34Z'
        }] });
      }
      return response({});
    };
    history.pushState({}, '', '?view=home');
    state.searches = [];

    await homeView();

    assert.equal(document.querySelector('.rw-page-header__kicker'), null);
    assert.deepEqual(Array.from(document.querySelectorAll('.rw-home-kpis .label'), function(label) { return label.textContent; }), [
      'Search terms', 'Search revisions', 'Execution plans', 'Run attempts'
    ]);
    assert.ok(app.innerHTML.includes('process-mining'));
    assert.ok(app.innerHTML.includes('Revision 1'));
    assert.ok(app.innerHTML.includes('12m 34s'));
    const explore = Array.from(document.querySelectorAll('a')).find(function(anchor) { return anchor.textContent === 'Explore'; });
    assert.ok(explore.href.includes('view=overview'));
    assert.ok(explore.href.includes('search_id=1'));
    assert.ok(explore.href.includes('search_revision_id=11'));
    assert.ok(explore.href.includes('plan_id=21'));
    assert.ok(explore.href.includes('run_id=31'));

    const lifecycle = document.querySelector('[data-run-visibility="trashed"]');
    lifecycle.click();
    const dialog = document.querySelector('[data-run-dialog]');
    assert.equal(dialog.hidden, false);
    assert.match(dialog.textContent, /without deleting immutable evidence/);
    dialog.querySelector('[data-run-dialog-close]').click();
    assert.equal(dialog.hidden, true);

    globalThis.fetch = originalFetch;
  });

});
