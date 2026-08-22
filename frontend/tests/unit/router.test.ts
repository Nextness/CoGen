// Unit tests for router.tsx — URL state, view routing, render orchestrator.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import './setup.ts';
import { setURL, bindFocusContext, render } from '../../src/router.tsx';
import { state, app } from '../../src/state.tsx';

describe('router.tsx — setURL', function() {

  it('updates URL and triggers render', function() {
    const originalHref = location.href;
    setURL({ view: 'corpus' }, false);
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

describe('router.tsx — bindFocusContext', function() {

  it('binds click handler to focus-context button', function() {
    document.querySelectorAll('[data-focus-context]').forEach(function(element) { element.remove(); });
    const button = document.createElement('button');
    button.dataset.focusContext = '';
    document.body.appendChild(button);
    const trigger = document.querySelector('[data-context-dropdown="search"] .rw-search-dropdown__trigger') as HTMLButtonElement;
    trigger.disabled = false;

    bindFocusContext();

    button.click();
    assert.equal(document.activeElement, trigger);

    button.remove();
  });

  it('does nothing when button is missing', function() {
    document.querySelectorAll('[data-focus-context]').forEach(function(element) { element.remove(); });
    (document.querySelector('[data-context-dropdown="search"] .rw-search-dropdown__trigger') as HTMLButtonElement).focus();
    const focused = document.activeElement;
    bindFocusContext();
    assert.equal(document.activeElement, focused);
  });

});

describe('router.tsx — render', function() {

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
      } as unknown as Response);
    } as typeof fetch;

    const url = new URL(location.href);
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());

    await render();
    assert.ok(app.querySelector('#page-title'));
    assert.equal((document.querySelector('.rw-context-panel') as HTMLElement).hidden, false);
    assert.equal((document.querySelector('.rw-primary-nav') as HTMLElement).hidden, false);
    assert.equal(document.querySelector('[data-view-link="overview"]')!.classList.contains('active'), true);
    assert.match(document.querySelector('#workspace-breadcrumb')!.textContent, /Home.*Deepdive.*Overview/);

    globalThis.fetch = originalFetch;
  });

  it('renders Home as the root shell without Deepdive context or tabs', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ data: [] }); } } as unknown as Response);
    } as typeof fetch;
    history.pushState({}, '', '?view=home');

    await render();

    assert.equal(document.querySelector('.rw-page-header__kicker'), null);
    assert.equal((document.querySelector('.rw-context-panel') as HTMLElement).hidden, true);
    assert.equal((document.querySelector('.rw-primary-nav') as HTMLElement).hidden, true);
    assert.equal(document.querySelector('#workspace-breadcrumb')!.textContent.trim(), 'Home');

    globalThis.fetch = originalFetch;
  });

  it("claims the next route controller synchronously before teardown yields", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: [] }),
      } as Response);
    }) as typeof fetch;
    history.replaceState({}, "", "?view=home");
    const previous = new AbortController();
    state.controller = previous;

    const pending = render();

    assert.equal(previous.signal.aborted, true);
    assert.notEqual(state.controller, previous);
    await pending;
    globalThis.fetch = originalFetch;
  });

});
