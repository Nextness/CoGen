// Unit tests for components/context-selector.js — search, revision, plan, run selectors.
import { describe, it, before, beforeEach, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { selects, clearContext, hydrateSelectors } from '../../../../src/server/frontend/components/context-selector.js';
import { state } from '../../../../src/server/frontend/state.js';

describe('context-selector.js — selects', function() {

  it('selects.search is a DOM element', function() {
    assert.ok(selects.search instanceof HTMLElement);
    assert.equal(selects.search.id, 'search-select');
  });

  it('selects.revision is a DOM element', function() {
    assert.ok(selects.revision instanceof HTMLElement);
    assert.equal(selects.revision.id, 'revision-select');
  });

  it('selects.plan is a DOM element', function() {
    assert.ok(selects.plan instanceof HTMLElement);
    assert.equal(selects.plan.id, 'plan-select');
  });

  it('selects.run is a DOM element', function() {
    assert.ok(selects.run instanceof HTMLElement);
    assert.equal(selects.run.id, 'run-select');
  });

  it('clearContext is a DOM element', function() {
    assert.ok(clearContext instanceof HTMLElement);
    assert.equal(clearContext.id, 'clear-context');
  });

});

describe('context-selector.js — hydrateSelectors', function() {

  before(function() {
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('fetches searches and populates the search selector', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(url) {
      if (url.includes('/api/searches')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() { return Promise.resolve({ data: { items: [{ id: 's1', label: 'Search 1' }] } }); },
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: [] }); },
      });
    };

    await hydrateSelectors();
    assert.equal(state.searches.length, 1);
    assert.equal(selects.search.options.length, 2); // placeholder + 1 item

    globalThis.fetch = originalFetch;
    state.searches = [];
  });

  it('shows selection summary when no revision is selected', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(url) {
      if (url.includes('/api/searches')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() { return Promise.resolve({ data: { items: [{ id: 's1', label: 'S1' }] } }); },
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: [] }); },
      });
    };

    await hydrateSelectors();
    const summary = document.querySelector('#selection-summary');
    assert.ok(summary.textContent.includes('Choose a search'));

    globalThis.fetch = originalFetch;
    state.searches = [];
  });

});
