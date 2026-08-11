import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.ts';
import { provenanceView } from '../../../src/views/provenance.tsx';
import { app, state } from '../../../src/state.tsx';

/** Sets location. */
function setLocation(values: Record<string, string>) {
  const url = new URL(location.href);
  ['section', 'run_id', 'audit_q', 'audit_category', 'audit_action', 'audit_actor', 'audit_entity', 'audit_stage', 'audit_outcome', 'cache_page', 'stage_page'].forEach(function(key) { url.searchParams.delete(key); });
  Object.entries(values).forEach(function(entry) { url.searchParams.set(entry[0], entry[1]); });
  history.pushState({}, '', url.toString());
}

/** Builds a mock fetch response. */
function response(data: unknown) {
  return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ data: data }); } } as unknown as Response);
}

describe('provenance.js — provenanceView', function() {
  before(function() {
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('shows one contextual empty panel for run-scoped sections without a run', async function() {
    setLocation({ section: 'artifacts' });
    await provenanceView();
    assert.ok(app.innerHTML.includes('Select a run attempt'));
    assert.equal(app.querySelectorAll('#page-title').length, 1);
  });

  it('renders server-filtered audit evidence with visible active filters', async function() {
    const originalFetch = globalThis.fetch;
    var requested = '';
    globalThis.fetch = function(input) {
      requested = String(input);
      return response({
        events: [{ id: 3, action: 'field_enriched', actor: 'crossref', entity_type: 'work_revision', entity_id: '1', occurred_at: '2024-01-01T00:01:00Z', metadata_json: { field: 'title', provider: 'crossref' } }],
        summary: { total_events: 1, actions: [{ action: 'field_enriched', count: 1 }] },
        facets: { actors: ['crossref'], actions: ['field_enriched'], entity_types: ['work_revision'] },
        has_more: false
      });
    } as typeof fetch;
    setLocation({ section: 'audit', run_id: '1', audit_category: 'enrichment' });

    await provenanceView();
    assert.ok(requested.includes('category=enrichment'));
    assert.ok(app.innerHTML.includes('Filter audit evidence'));
    assert.ok(app.innerHTML.includes('Applied filters'));
    assert.ok(app.innerHTML.includes('rw-audit-event--enrichment'));
    assert.ok(app.innerHTML.includes('Title enriched by crossref'));
    assert.ok(!app.innerHTML.includes('data-timeline-filter'));

    globalThis.fetch = originalFetch;
  });

  it('updates URL state when audit filters are submitted', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return response({ events: [], summary: { total_events: 0, actions: [] }, facets: { actors: [], actions: [], entity_types: [] } });
    } as typeof fetch;
    setLocation({ section: 'audit', run_id: '1' });

    await provenanceView();
    const category = Array.from(document.querySelectorAll('[name="audit_category"]')).find(function(input) {
      return (input as HTMLInputElement).value === 'validation';
    }) as HTMLInputElement;
    category.checked = true;
    document.querySelector('#audit-filter-form')!.dispatchEvent(new window.Event('submit', { bubbles: true, cancelable: true }));
    assert.equal(new URL(location.href).searchParams.get('audit_category'), 'validation');

    globalThis.fetch = originalFetch;
  });

  it('appends older audit events without duplicates and preserves open details', async function() {
    const originalFetch = globalThis.fetch;
    var calls = 0;
    globalThis.fetch = function() {
      calls += 1;
      if (calls === 1) {
        return response({
          events: [{ id: 3, action: 'validation_changed', actor: 'pipeline', entity_type: 'work_revision', entity_id: '1', pipeline_run_id: 1, occurred_at: '2024-01-02T00:01:00Z', metadata_json: { reason: 'Recorded reason', detail: 'visible' } }],
          summary: { total_events: 2, actions: [{ action: 'validation_changed', count: 2 }] },
          facets: { actors: ['pipeline'], actions: ['validation_changed'], entity_types: ['work_revision'] },
          next_cursor: 'older-page',
          has_more: true
        });
      }
      return response({
        events: [
          { id: 3, action: 'validation_changed', actor: 'pipeline', entity_type: 'work_revision', entity_id: '1', pipeline_run_id: 1, occurred_at: '2024-01-02T00:01:00Z' },
          { id: 2, action: 'pipeline_completed', actor: 'pipeline', entity_type: 'pipeline_run', entity_id: '1', pipeline_run_id: 1, occurred_at: '2024-01-01T00:01:00Z' }
        ],
        next_cursor: '',
        has_more: false
      });
    } as typeof fetch;
    setLocation({ section: 'audit', run_id: '1' });

    await provenanceView();
    const details = document.querySelector('.rw-event-details') as HTMLDetailsElement;
    details.open = true;
    (document.querySelector('[data-audit-load-more]') as HTMLButtonElement).click();
    await new Promise(function(resolve) { setTimeout(resolve, 0); });

    assert.equal(document.querySelectorAll('[data-audit-event-id]').length, 2);
    assert.equal((document.querySelector('[data-audit-event-id="3"] .rw-event-details') as HTMLDetailsElement).open, true);
    assert.equal((document.querySelector('[data-audit-loaded-count]') as HTMLElement).textContent, '2');
    assert.equal((document.querySelector('[data-audit-end]') as HTMLElement).hidden, false);
    assert.equal((document.querySelector('.rw-load-more') as HTMLElement).hidden, true);

    globalThis.fetch = originalFetch;
  });

  it('renders bounded artifact preview metadata and truncation guidance', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(input) {
      if (String(input).includes('/inspect')) {
        return response({ artifact_id: 9, content_type: 'text/plain', byte_size: 90000, stored_byte_size: 90000, preview_byte_size: 65536, truncated: true, format: 'text', content: 'safe prefix' });
      }
      return response({
        context: { run_id: 1, attempt_number: 1 },
        artifacts: [{ id: 9, artifact_roles: 'input_manifest', content_hash: 'hash', byte_size: 90000, content_type: 'text/plain', created_at: '2024-01-01T00:00:00Z', has_blob: 1, preview_available: true }]
      });
    } as typeof fetch;
    setLocation({ section: 'artifacts', run_id: '1' });

    await provenanceView();
    (document.querySelector('[data-inspect-artifact]') as HTMLButtonElement).click();
    await new Promise(function(resolve) { setTimeout(resolve, 0); });
    assert.ok(app.innerHTML.includes('Preview truncated'));
    assert.ok(app.innerHTML.includes('Bytes shown'));
    assert.ok(app.innerHTML.includes('Download original'));
    assert.ok(app.innerHTML.includes('safe prefix'));

    globalThis.fetch = originalFetch;
  });

  it('renders cache search and complete pagination controls', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return response({
        columns: ['id', 'provider', 'outcome'],
        rows: [{ id: 1, provider: 'crossref', outcome: 'hit' }],
        pagination: { page: 2, per_page: 20, total_rows: 41, total_pages: 3 }
      });
    } as typeof fetch;
    setLocation({ section: 'cache', run_id: '1', cache_page: '2' });

    await provenanceView();
    assert.ok(app.innerHTML.includes('Cache use records'));
    assert.ok(app.innerHTML.includes('Search cache evidence'));
    assert.ok(app.innerHTML.includes('First'));
    assert.ok(app.innerHTML.includes('Last'));
    assert.ok(app.innerHTML.includes('Page 2 of 3'));

    globalThis.fetch = originalFetch;
  });

  it('renders stage progression before paginated details', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return response({
        columns: ['id', 'work_id', 'stage_name', 'outcome'],
        rows: [{ id: 1, work_id: 2, stage_name: 'validate', outcome: 'valid' }],
        pagination: { page: 1, per_page: 20, total_rows: 1, total_pages: 1 },
        stage_summaries: [{ stage_name: 'validate', total_records: 1, outcomes: { valid: 1 } }],
        run_steps: [{ step_name: 'preflight', step_status: 'completed', duration_seconds: 5, input_artifact_id: 1, output_artifact_id: 2 }]
      });
    } as typeof fetch;
    setLocation({ section: 'stages', run_id: '1' });

    await provenanceView();
    assert.ok(app.innerHTML.includes('Stage outcomes and progression'));
    assert.ok(app.innerHTML.includes('rw-stage-flow'));
    assert.ok(app.innerHTML.includes('Preflight'));
    assert.ok(app.innerHTML.includes('5 seconds'));
    assert.ok(app.innerHTML.indexOf('Stage outcomes and progression') < app.innerHTML.indexOf('Detailed stage outcomes'));

    globalThis.fetch = originalFetch;
  });

  it('renders run identity and configuration snapshots', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() { return response({ artifacts: [] }); } as typeof fetch;
    setLocation({ section: 'run', run_id: '1' });
    state.runs = [{ id: '1', attempt_number: 1, status: 'completed' }];

    await provenanceView();
    assert.ok(app.innerHTML.includes('Stored run attempt'));
    assert.ok(app.innerHTML.includes('Configuration snapshots'));

    globalThis.fetch = originalFetch;
    state.runs = [];
  });
});
