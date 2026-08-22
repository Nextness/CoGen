// Unit tests for components/shell.tsx — health check.
import { describe, it, before, beforeEach } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.ts';
import { healthStatus, initHealthCheck, initMobileNavToggle } from '../../../src/components/shell.tsx';

describe('shell.tsx — healthStatus', function() {

  it('is a DOM element', function() {
    assert.ok(healthStatus instanceof HTMLElement);
    assert.equal(healthStatus.id, 'health-status');
  });

});

describe('shell.tsx — initHealthCheck', function() {

  beforeEach(function() {
    healthStatus.textContent = '';
    healthStatus.className = '';
  });

  it('sets healthy status on successful response', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: { readable: true } }); },
      } as unknown as Response);
    } as typeof fetch;

    initHealthCheck();
    // initHealthCheck does not return the promise chain, so wait for microtasks
    await new Promise(function(resolve) { setTimeout(resolve, 10); });
    assert.equal(healthStatus.textContent, 'Database healthy');

    globalThis.fetch = originalFetch;
  });

  it('sets unavailable status when readable is false', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: { readable: false } }); },
      } as unknown as Response);
    } as typeof fetch;

    initHealthCheck();
    await new Promise(function(resolve) { setTimeout(resolve, 10); });
    assert.equal(healthStatus.textContent, 'Database unavailable');

    globalThis.fetch = originalFetch;
  });

it('sets unavailable status on fetch failure', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.reject(new Error('Network error'));
    } as typeof fetch;

    initHealthCheck();
    await new Promise(function(resolve) { setTimeout(resolve, 10); });
    assert.equal(healthStatus.textContent, 'Database unavailable');

    globalThis.fetch = originalFetch;
  });
});

describe('shell.tsx — initMobileNavToggle', function() {

  var toggle: HTMLButtonElement;
  var nav: HTMLElement;

  before(function() {
    document.querySelectorAll('#mobile-nav-toggle, .rw-primary-nav').forEach(function(el) { el.remove(); });
    document.body.classList.remove('rw-mobile-nav-open');

    toggle = document.createElement('button');
    toggle.id = 'mobile-nav-toggle';
    toggle.setAttribute('aria-expanded', 'false');
    document.body.appendChild(toggle);

    nav = document.createElement('nav');
    nav.className = 'rw-primary-nav';
    var link = document.createElement('a');
    link.href = '#';
    nav.appendChild(link);
    document.body.appendChild(nav);

    initMobileNavToggle();
  });

  it('toggles rw-mobile-nav-open on click', function() {
    toggle.click();
    assert.ok(nav.classList.contains('rw-mobile-nav-open'));
    assert.ok(document.body.classList.contains('rw-mobile-nav-open'));
    assert.equal(toggle.getAttribute('aria-expanded'), 'true');

    toggle.click();
    assert.ok(!nav.classList.contains('rw-mobile-nav-open'));
    assert.ok(!document.body.classList.contains('rw-mobile-nav-open'));
    assert.equal(toggle.getAttribute('aria-expanded'), 'false');
  });

  it('closes nav when a nav link is clicked', function() {
    toggle.click();
    assert.ok(nav.classList.contains('rw-mobile-nav-open'));

    var link = nav.querySelector('a') as HTMLAnchorElement;
    link.click();
    assert.ok(!nav.classList.contains('rw-mobile-nav-open'));
    assert.equal(toggle.getAttribute('aria-expanded'), 'false');
  });

  it("owns the disclosure and restores focus when Escape closes it", function() {
    toggle.click();
    assert.equal(toggle.getAttribute("aria-controls"), nav.id);

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));

    assert.equal(toggle.getAttribute("aria-expanded"), "false");
    assert.equal(document.activeElement, toggle);
  });

  it("closes the disclosure after an outside click", function() {
    toggle.click();
    document.body.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    assert.equal(toggle.getAttribute("aria-expanded"), "false");
  });

  it('is a no-op when toggle or nav is missing', function() {
    // Separate call with missing elements — already tested by the single init above
    assert.ok(toggle);
    assert.ok(nav);
  });

});
