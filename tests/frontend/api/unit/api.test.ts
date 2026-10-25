// Unit tests for api.tsx — endpoint builder and fetch helpers.
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

import "../../../support/node/setup.ts";
import { APIError, endpoint, api, downloadExport, mutate, pdfStatus, preparePDFUpload, removePDF, tables, uploadPDF } from "../../../../frontend/src/api.tsx";
import type { PDFMutationResponse, PDFStatus, PDFUploadRequest } from "../../../../frontend/src/api/types.ts";
import { state } from "../../../../frontend/src/state.tsx";

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
    var request: RequestInit | undefined;
    globalThis.fetch = function(_url: RequestInfo | URL, options?: RequestInit) {
      request = options;
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: ['a', 'b'] }); },
      } as unknown as Response);
    } as typeof fetch;

    const result = await api('/api/test');
    assert.deepEqual(result, ['a', 'b']);
    assert.equal(request?.method, 'GET');
    assert.equal((request?.headers as Record<string, string> | undefined)?.Accept, 'application/json');

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
      } as unknown as Response);
    } as typeof fetch;

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
      } as unknown as Response);
    } as typeof fetch;

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
      } as unknown as Response);
    } as typeof fetch;

    await assert.rejects(function() {
      return api('/api/test');
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
    void api('/api/test');
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

describe('api.tsx — PDF transport', function() {

  before(function() {
    state.controller = null;
  });

  it('reads current PDF status for a work', async function() {
    const originalFetch = globalThis.fetch;
    var url: RequestInfo | URL | undefined;
    globalThis.fetch = function(input: RequestInfo | URL) {
      url = input;
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ status: 'not_available', mutation_etag: '"pdf-10-1"' }); },
      } as unknown as Response);
    } as typeof fetch;

    const status = await pdfStatus(10);
    assert.equal(url, '/api/works/10/pdf-status');
    assert.equal(status.mutation_etag, '"pdf-10-1"');

    globalThis.fetch = originalFetch;
  });

  it('computes upload size and SHA-256 without transmitting the filename', async function() {
    const file = new File([new TextEncoder().encode('abc')], 'private-name.pdf', { type: 'application/pdf' });
    const status: PDFStatus = { status: 'not_available', mutation_etag: '"pdf-10-1"' };
    const request = await preparePDFUpload(file, status);
    assert.equal(request.file, file);
    assert.equal(request.size, 3);
    assert.equal(request.sha256, 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad');
    assert.equal(request.mutationETag, '"pdf-10-1"');
  });

  it('rejects empty files and status without a mutation ETag before upload', async function() {
    const empty = new File([], 'empty.pdf', { type: 'application/pdf' });
    await assert.rejects(function() {
      return preparePDFUpload(empty, { status: 'not_available', mutation_etag: '"pdf-10-1"' });
    }, /non-empty/);
    const file = new File([new TextEncoder().encode('abc')], 'paper.pdf', { type: 'application/pdf' });
    await assert.rejects(function() {
      return preparePDFUpload(file, { status: 'not_available', mutation_etag: null });
    }, /mutation precondition/);
  });

  it('uploads the binary body with size, digest, and If-Match headers', async function() {
    const originalFetch = globalThis.fetch;
    const file = new File([new TextEncoder().encode('abc')], 'private-name.pdf', { type: 'application/pdf' });
    const request: PDFUploadRequest = {
      file: file,
      size: 3,
      sha256: 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad',
      mutationETag: '"pdf-10-2"',
    };
    const response: PDFMutationResponse = {
      changed: true,
      operation: 'uploaded',
      pdf_status: { status: 'available', mutation_etag: '"pdf-10-3"' },
      event_key: 'event-1',
      audit_delivery: 'pending',
    };
    var url: RequestInfo | URL | undefined;
    var options: RequestInit | undefined;
    globalThis.fetch = function(input: RequestInfo | URL, init?: RequestInit) {
      url = input;
      options = init;
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve(response); } } as unknown as Response);
    } as typeof fetch;
    state.controller = new AbortController();
    state.controller.abort();

    assert.deepEqual(await uploadPDF(7, 11, request), response);
    assert.equal(url, '/api/runs/7/articles/11/pdf');
    assert.equal(options?.method, 'PUT');
    assert.equal((options?.headers as Record<string, string> | undefined)?.['Content-Type'], 'application/pdf');
    assert.equal((options?.headers as Record<string, string> | undefined)?.['If-Match'], '"pdf-10-2"');
    assert.equal((options?.headers as Record<string, string> | undefined)?.['X-PDF-Size'], '3');
    assert.equal((options?.headers as Record<string, string> | undefined)?.['X-PDF-SHA256'], request.sha256);
    assert.equal(options?.body, file);
    assert.equal(options?.signal, undefined);

    globalThis.fetch = originalFetch;
    state.controller = null;
  });

  it('removes a PDF with the conditional ETag and no body', async function() {
    const originalFetch = globalThis.fetch;
    var options: RequestInit | undefined;
    globalThis.fetch = function(_input: RequestInfo | URL, init?: RequestInit) {
      options = init;
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ changed: true, operation: 'removed', pdf_status: { status: 'not_available' }, event_key: 'event-2', audit_delivery: 'pending' }); } } as unknown as Response);
    } as typeof fetch;

    await removePDF(7, 11, '"pdf-10-3"');
    assert.equal(options?.method, 'DELETE');
    assert.equal((options?.headers as Record<string, string> | undefined)?.['If-Match'], '"pdf-10-3"');
    assert.equal(options?.body, undefined);
    assert.equal(options?.signal, undefined);

    globalThis.fetch = originalFetch;
  });

  it('shares structured conflict decoding with PDF uploads', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: false,
        status: 412,
        json: function() {
          return Promise.resolve({
            error: {
              message: 'The PDF selection changed; refresh and retry',
              code: 'pdf_state_conflict',
              details: { expected_change_counter: 2, current_pdf_status: { status: 'available' } },
            },
          });
        },
      } as unknown as Response);
    } as typeof fetch;
    const request: PDFUploadRequest = {
      file: new File([new TextEncoder().encode('abc')], 'paper.pdf', { type: 'application/pdf' }),
      size: 3,
      sha256: 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad',
      mutationETag: '"pdf-10-2"',
    };

    await assert.rejects(function() {
      return uploadPDF(7, 11, request);
    }, function(error: unknown) {
      assert.ok(error instanceof APIError);
      assert.equal(error.status, 412);
      assert.equal(error.code, 'pdf_state_conflict');
      assert.deepEqual(error.details, { expected_change_counter: 2, current_pdf_status: { status: 'available' } });
      return true;
    });

    globalThis.fetch = originalFetch;
  });

});


describe('api.tsx — binary exports', function() {

  it('prepares a bounded ZIP download, reads headers, and revokes its object URL', async function() {
    const originalFetch = globalThis.fetch;
    const originalCreate = URL.createObjectURL;
    const originalRevoke = URL.revokeObjectURL;
    const originalClick = HTMLAnchorElement.prototype.click;
    var created = 0;
    var revoked = 0;
    var clicked = 0;
    var request: RequestInit | undefined;
    URL.createObjectURL = function() {
      created += 1;
      return "blob:export";
    };
    URL.revokeObjectURL = function() {
      revoked += 1;
    };
    HTMLAnchorElement.prototype.click = function() {
      clicked += 1;
    };
    globalThis.fetch = function(_input: RequestInfo | URL, init?: RequestInit) {
      request = init;
      return Promise.resolve({
        ok: true,
        status: 200,
        headers: new Headers({
          "Content-Type": "application/zip",
          "Content-Disposition": "attachment; filename*=utf-8''papers%20%CE%B1.zip",
          "Content-Length": "3",
          "X-PDF-Export-Included": "2",
          "X-PDF-Export-Skipped": "1",
          "X-PDF-Export-Correlation-Id": "corr-1",
        }),
        body: null,
        arrayBuffer: function() { return Promise.resolve(new TextEncoder().encode("zip").buffer); },
      } as unknown as Response);
    } as typeof fetch;

    const result = await downloadExport("/api/runs/7/pdf-exports", { articles: [] }, { contentType: "application/zip", maxBytes: 10 });
    assert.equal(request?.method, "POST");
    assert.equal(request?.body, JSON.stringify({ articles: [] }));
    assert.equal(result.filename, "papers α.zip");
    assert.equal(result.bytes, 3);
    assert.equal(result.included, 2);
    assert.equal(result.skipped, 1);
    assert.equal(result.correlationID, "corr-1");
    assert.equal(created, 1);
    assert.equal(clicked, 1);
    await new Promise((resolve) => setTimeout(resolve, 5));
    assert.equal(revoked, 1);

    globalThis.fetch = originalFetch;
    URL.createObjectURL = originalCreate;
    URL.revokeObjectURL = originalRevoke;
    HTMLAnchorElement.prototype.click = originalClick;
  });

  it('shares structured JSON failure decoding and rejects oversized responses', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: false,
        status: 409,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: function() {
          return Promise.resolve({ error: { message: "No selected PDF could be exported", code: "pdf_export_empty", details: { skipped: 2 } } });
        },
      } as unknown as Response);
    } as typeof fetch;
    await assert.rejects(function() {
      return downloadExport("/api/runs/7/pdf-exports", {}, { contentType: "application/zip", maxBytes: 10 });
    }, function(error: unknown) {
      assert.ok(error instanceof APIError);
      assert.equal(error.status, 409);
      assert.equal(error.code, "pdf_export_empty");
      return true;
    });

    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        headers: new Headers({ "Content-Type": "application/pdf", "Content-Length": "100" }),
        body: null,
        arrayBuffer: function() { return Promise.resolve(new ArrayBuffer(100)); },
      } as unknown as Response);
    } as typeof fetch;
    await assert.rejects(function() {
      return downloadExport("/api/runs/7/articles/11/pdf-export", {}, { contentType: "application/pdf", maxBytes: 10 });
    }, /exceeds the 10-byte download limit/);

    globalThis.fetch = originalFetch;
  });

  it('rejects a response that ends before its declared content length', async function() {
    const originalFetch = globalThis.fetch;
    const encoder = new TextEncoder();
    globalThis.fetch = function() {
      var read = false;
      return Promise.resolve({
        ok: true,
        status: 200,
        headers: new Headers({ "Content-Type": "application/pdf", "Content-Length": "5" }),
        body: {
          getReader: function() {
            return {
              read: function() {
                if (read) return Promise.resolve({ done: true, value: undefined });
                read = true;
                return Promise.resolve({ done: false, value: encoder.encode("abc") });
              },
              cancel: function() { return Promise.resolve(); },
            };
          },
        },
      } as unknown as Response);
    } as typeof fetch;
    await assert.rejects(function() {
      return downloadExport("/api/runs/7/articles/11/pdf-export", {}, { contentType: "application/pdf", maxBytes: 10 });
    }, /ended before the complete file/);
    globalThis.fetch = originalFetch;
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
