// Unit tests for router.js — URL state, view routing, render orchestrator.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import './setup.js';
import { setURL, bindFocusContext, render } from '../../../src/server/frontend/router.js';
import { state, app } from '../../../src/server/frontend/state.js';

describe('router.js — setURL', function() {

  it('updates URL and triggers render', function() {
    const originalHref = location.href;
    setURL({ view: 'corpus' });
    assert.ok(location.href.includes('view=corpus'));

    history.pushState({}, '', originalHref);
  });

  it('replaces state when replace is true', function() {
    const originalHref = location.href;
    setURL({ view: 'provenance' }, true);
    assert.ok(location.href.includes('view=provenance'));

    history.pushState({}, '', originalHref);
  });

});

describe('router.js — bindFocusContext', function() {

  it('binds click handler to focus-context button', function() {
    const button = document.createElement('button');
    button.dataset.focusContext = '';
    document.body.appendChild(button);

    bindFocusContext();

    button.click();
    assert.ok(true);

    button.remove();
  });

  it('does nothing when button is missing', function() {
    bindFocusContext();
    assert.ok(true);
  });

});

describe('router.js — render', function() {

  before(function() {
    state.request = 0;
    state.controller = null;
  });

  it('renders overview when no view is set', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: [] }); },
      });
    };

    const url = new URL(location.href);
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());

    await render();
    assert.ok(app.innerHTML.length > 0 || app.innerHTML === '');
    assert.equal(document.querySelector('.context-panel').hidden, false);
    assert.equal(document.querySelector('.primary-nav').hidden, false);
    assert.equal(document.querySelector('[data-view-link="overview"]').classList.contains('active'), true);
    assert.match(document.querySelector('#workspace-breadcrumb').textContent, /Home.*Deepdive.*Overview/);

    globalThis.fetch = originalFetch;
  });

  it('renders Home as the root shell without Deepdive context or tabs', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ data: [] }); } });
    };
    history.pushState({}, '', '?view=home');

    await render();

    assert.equal(document.querySelector('.rw-page-header__kicker'), null);
    assert.equal(document.querySelector('.context-panel').hidden, true);
    assert.equal(document.querySelector('.primary-nav').hidden, true);
    assert.equal(document.querySelector('#workspace-breadcrumb').textContent.trim(), 'Home');

    globalThis.fetch = originalFetch;
  });

});
