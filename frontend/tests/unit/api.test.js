// Unit tests for api.js — endpoint builder and fetch helpers.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import './setup.js';
import { endpoint, api, tables } from '../../../src/server/frontend/api.js';
import { state } from '../../../src/server/frontend/state.js';

describe('api.js — endpoint', function() {

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

describe('api.js — api', function() {

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
      });
    };

    const result = await api('/api/test');
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
      });
    };

    const result = await api('/api/test');
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
      });
    };

    await assert.rejects(function() {
      return api('/api/test');
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
      });
    };

    await assert.rejects(function() {
      return api('/api/test');
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
      });
    };

    await assert.rejects(function() {
      return api('/api/test');
    }, /invalid JSON/);

    globalThis.fetch = originalFetch;
  });

  it('aborts when state.controller is aborted', async function() {
    const originalFetch = globalThis.fetch;
    var signal;
    globalThis.fetch = function(url, opts) {
      signal = opts.signal;
      return new Promise(function() {}); // Never resolves
    };

    state.controller = new AbortController();
    const promise = api('/api/test');
    state.controller.abort();

    assert.ok(signal);
    assert.ok(signal.aborted);

    globalThis.fetch = originalFetch;
    state.controller = null;
  });

});

describe('api.js — tables', function() {

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
      });
    };

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