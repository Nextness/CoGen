// Unit tests for api.tsx — endpoint builder and fetch helpers.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import './setup.ts';
import { endpoint, api, mutate, tables } from '../../src/api.tsx';
import { state } from '../../src/state.tsx';

describe('api.tsx — endpoint', function() {

  it('builds a path with query parameters', function() {
    const result = endpoint('/api/test', { a: '1', b: '2' });
    assert.equal(result, '/api/test?a=1&b=2');
  });

  it('omits empty and null parameters', function() {
    const result = endpoint('/api/test', { a: '', b: null, c: undefined, d: 'keep' });
    assert.ok(!result.includes('a='));
    assert.ok(!result.includes('b='));
    assert.ok(!result.includes('c='));
    assert.ok(result.includes('d=keep'));
  });

  it('handles no query', function() {
    assert.equal(endpoint('/api/test'), '/api/test');
  });

});

describe('api.tsx — api', function() {

  before(function() {
    state.controller = null;
  });

  it('fetches and returns data from a successful response', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: ['a', 'b'] }); },
      } as unknown as Response);
    } as typeof fetch;

    const result = await api('/api/test', {}, { method: 'GET', headers: { Accept: 'application/json' } });
    assert.deepEqual(result, ['a', 'b']);

    globalThis.fetch = originalFetch;
  });

  it('returns the full body when no data key', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ message: 'ok' }); },
      } as unknown as Response);
    } as typeof fetch;

    const result = await api('/api/test', {}, { method: 'GET', headers: { Accept: 'application/json' } });
    assert.deepEqual(result, { message: 'ok' });

    globalThis.fetch = originalFetch;
  });

  it('throws on non-ok response with error message', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: false,
        status: 404,
        json: function() { return Promise.resolve({ error: { message: 'Not found' } }); },
      } as unknown as Response);
    } as typeof fetch;

    await assert.rejects(function() {
      return api('/api/test', {}, { method: 'GET', headers: { Accept: 'application/json' } });
    }, /Not found/);

    globalThis.fetch = originalFetch;
  });

  it('throws on non-ok response without error message', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: false,
        status: 500,
        json: function() { return Promise.resolve({}); },
      } as unknown as Response);
    } as typeof fetch;

    await assert.rejects(function() {
      return api('/api/test', {}, { method: 'GET', headers: { Accept: 'application/json' } });
    }, /Request failed \(500\)/);

    globalThis.fetch = originalFetch;
  });

  it('throws on invalid JSON response', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.reject(new Error('parse error')); },
      } as unknown as Response);
    } as typeof fetch;

    await assert.rejects(function() {
      return api('/api/test', {}, { method: 'GET', headers: { Accept: 'application/json' } });
    }, /invalid JSON/);

    globalThis.fetch = originalFetch;
  });

  it('aborts when state.controller is aborted', async function() {
    const originalFetch = globalThis.fetch;
    var signal: AbortSignal | undefined;
    globalThis.fetch = function(_url: RequestInfo | URL, opts?: RequestInit) {
      signal = opts?.signal ?? undefined;
      return new Promise(function() {}); // Never resolves
    } as typeof fetch;

    state.controller = new AbortController();
    const promise = api('/api/test', {}, { method: 'GET', headers: { Accept: 'application/json' } });
    state.controller.abort();

    assert.ok(signal);
    assert.ok(signal.aborted);

    globalThis.fetch = originalFetch;
    state.controller = null;
  });

  it('sends same-origin JSON mutations', async function() {
    const originalFetch = globalThis.fetch;
    var request: RequestInit | undefined;
    globalThis.fetch = function(_url: RequestInfo | URL, options?: RequestInit) {
      request = options;
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ saved: true }); } } as unknown as Response);
    } as typeof fetch;
    state.controller = new AbortController();
    assert.deepEqual(await mutate('/api/review', 'PUT', { status: 'approved' }), { saved: true });
    assert.equal(request?.method, 'PUT');
    assert.equal((request?.headers as Record<string, string> | undefined)?.['Content-Type'], 'application/json');
    assert.equal(request?.body, '{"status":"approved"}');
    assert.equal(request?.signal, undefined);
    state.controller.abort();
    assert.equal(request?.signal, undefined);
    globalThis.fetch = originalFetch;
    state.controller = null;
  });

});

describe('api.tsx — tables', function() {

  before(function() {
    state.tables = [];
  });

  it('fetches tables on first call and caches them', async function() {
    const originalFetch = globalThis.fetch;
    var callCount = 0;
    globalThis.fetch = function() {
      callCount += 1;
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: { tables: [{ name: 'works' }] } }); },
      } as unknown as Response);
    } as typeof fetch;

    const result1 = await tables();
    assert.equal(callCount, 1);
    assert.equal(result1.length, 1);

    const result2 = await tables();
    assert.equal(callCount, 1); // Should not fetch again
    assert.equal(result2.length, 1);

    globalThis.fetch = originalFetch;
    state.tables = [];
  });

});
