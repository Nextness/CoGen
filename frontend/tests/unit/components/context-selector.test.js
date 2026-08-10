// Unit tests for components/context-selector.js — search, revision, plan, run selectors.
import { describe, it, before, beforeEach, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { selects, hydrateSelectors } from '../../../src/components/context-selector.ts';
import { state } from '../../../src/state.ts';

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

  it('supports searchable keyboard-ready selection without implying multiple active contexts', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(url) {
      if (url.includes('/api/searches')) {
        return Promise.resolve({
          ok: true,
          status: 200,
            json: function() { return Promise.resolve({ data: { items: [{ id: 's1', label: 'Systematic review' }, { id: 's2', label: 'Mapping study' }] } }); },
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: [] }); },
      });
    };

    await hydrateSelectors();
    const dropdown = document.querySelector('[data-context-dropdown="search"]');
    const trigger = dropdown.querySelector('.rw-search-dropdown__trigger');
    trigger.click();
    assert.equal(trigger.getAttribute('aria-expanded'), 'true');
    const query = dropdown.querySelector('.rw-search-dropdown__query');
    query.value = 'mapping';
    query.dispatchEvent(new Event('input', { bubbles: true }));
    const options = dropdown.querySelectorAll('[role="option"]');
    assert.equal(options.length, 1);
    assert.equal(options[0].textContent, 'Mapping study');
    options[0].click();
    assert.equal(selects.search.value, 's2');
    assert.ok(trigger.textContent.includes('Mapping study'));
    assert.equal(trigger.querySelector('.ui.label'), null);

    globalThis.fetch = originalFetch;
    state.searches = [];
  });

});
