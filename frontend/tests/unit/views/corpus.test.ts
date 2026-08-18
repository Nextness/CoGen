// Unit tests for views/corpus.js — corpus view, columnNames, identityEvidenceTable.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.ts';
import { corpusView } from '../../../src/views/corpus.tsx';
import { app, state, value } from '../../../src/state.tsx';

describe('corpus.js — corpusView', function() {

  before(function() {
    state.tables = [];
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('renders corpus view with articles section', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(url) {
      if (String(url).includes('/api/tables')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() { return Promise.resolve({ data: { tables: [{ name: 'work_revisions', columns: ['id', 'work_id', 'doi', 'title', 'year', 'journal', 'source'] }] } }); },
        } as unknown as Response);
      }
      if (String(url).includes('/api/runs')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() {
            return Promise.resolve({
              data: {
                columns: ['id', 'work_id', 'doi', 'title', 'year', 'journal', 'source'],
                rows: [{ id: 1, work_id: 2, doi: '10.1000/test', title: 'Test Article', year: 2024, journal: 'Journal', source: 'scopus' }],
                pagination: { page: 1, total_pages: 1, total_rows: 1 },
              },
            });
          },
        } as unknown as Response);
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: [] }); },
      } as unknown as Response);
    } as typeof fetch;

    const url = new URL(location.href);
    url.searchParams.set('view', 'corpus');
    url.searchParams.set('section', 'articles');
    url.searchParams.set('run_id', 'run-1');
    history.pushState({}, '', url.toString());

    await corpusView();
    assert.ok(app.innerHTML.includes('Corpus'));
    assert.ok(document.querySelector('#corpus-section-select'));
    assert.equal(document.querySelector('.rw-data-section .ui.tabular.menu'), null);
    assert.deepEqual(Array.from(document.querySelectorAll('.rw-corpus-table thead th'), function(cell) { return cell.textContent.trim(); }).filter(Boolean), [
      'DOI', 'Title', 'Year', 'Journal', 'Source'
    ]);
    assert.ok(!app.innerHTML.includes('<th scope="col" class="col-id"'));

    globalThis.fetch = originalFetch;
    state.tables = [];
    url.searchParams.delete('run_id');
    url.searchParams.delete('section');
    url.searchParams.delete('view');
    history.pushState({}, '', url.toString());
  });

  it('renders matched search terms in the article row expansion', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(url) {
      if (String(url).includes('/api/tables')) {
        return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ data: { tables: [{ name: 'work_revisions', columns: ['id', 'work_id', 'doi', 'title', 'year', 'journal', 'source'] }] } }); } } as unknown as Response);
      }
      if (String(url).includes('/api/runs')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() {
            return Promise.resolve({
              data: {
                columns: ['id', 'work_id', 'doi', 'title', 'year', 'journal', 'source'],
                rows: [{
                  id: 1, work_id: 2, doi: '10.1000/test', title: 'Test Article', year: 2024, journal: 'Journal', source: 'scopus',
                  term_matches: { title: ['BPMN'], abstract: [], keywords: ['scheduling'], keywords_plus: [], matched_total: 2, term_total: 5, sources: ['scopus'] },
                }],
                pagination: { page: 1, total_pages: 1, total_rows: 1 },
              },
            });
          },
        } as unknown as Response);
      }
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ data: [] }); } } as unknown as Response);
    } as typeof fetch;
    state.tables = [];
    history.pushState({}, '', '?view=corpus&section=articles&run_id=1');

    await corpusView();
    assert.ok(app.innerHTML.includes('Matched search terms'));
    assert.ok(app.innerHTML.includes('2 of 5 search terms matched'));
    assert.ok(app.innerHTML.includes('BPMN'));
    assert.ok(app.innerHTML.includes('scheduling'));

    globalThis.fetch = originalFetch;
    state.tables = [];
    history.pushState({}, '', '?view=home');
  });

  it('renders no search terms recorded when term_matches is absent', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(url) {
      if (String(url).includes('/api/tables')) {
        return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ data: { tables: [{ name: 'work_revisions', columns: ['id', 'work_id', 'doi', 'title', 'year', 'journal', 'source'] }] } }); } } as unknown as Response);
      }
      if (String(url).includes('/api/runs')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() {
            return Promise.resolve({
              data: {
                columns: ['id', 'work_id', 'doi', 'title', 'year', 'journal', 'source'],
                rows: [{ id: 1, work_id: 2, doi: '10.1000/test', title: 'Test Article', year: 2024, journal: 'Journal', source: 'scopus' }],
                pagination: { page: 1, total_pages: 1, total_rows: 1 },
              },
            });
          },
        } as unknown as Response);
      }
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ data: [] }); } } as unknown as Response);
    } as typeof fetch;
    state.tables = [];
    history.pushState({}, '', '?view=corpus&section=articles&run_id=1');

    await corpusView();
    assert.ok(app.innerHTML.includes('Matched search terms'));
    assert.ok(app.innerHTML.includes('No search terms recorded'));

    globalThis.fetch = originalFetch;
    state.tables = [];
    history.pushState({}, '', '?view=home');
  });

  it('keeps identity candidate details out of the table and distinguishes no-candidate from unclear statuses', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function(url) {
      if (String(url).includes('/api/tables')) {
        return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ data: { tables: [{ name: 'author_identity_resolutions', columns: ['id', 'status'] }] } }); } } as unknown as Response);
      }
      if (String(url).includes('/identity-evidence')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() {
            return Promise.resolve({ data: {
              stats: { resolutions: 2, unclear: 1, provider_failed: 0, candidates: 1 },
              rows: [
                { id: 1, author_occurrence_id: 5, status: 'no_orcid_candidate', queried_citation_name: 'No Candidate', article_title: 'Paper A', doi: '10.1/a', candidates: [{ candidate_orcid: 'must-not-render' }] },
                { id: 2, author_occurrence_id: 6, status: 'orcid_is_unclear', queried_citation_name: 'Unclear', article_title: 'Paper B', doi: '10.1/b' }
              ],
              pagination: { page: 1, total_pages: 1, total_rows: 2 }
            } });
          },
        } as unknown as Response);
      }
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve({ data: [] }); } } as unknown as Response);
    } as typeof fetch;
    state.tables = [];
    history.pushState({}, '', '?view=corpus&section=identity_evidence&run_id=1');

    await corpusView();

    assert.deepEqual(Array.from(document.querySelectorAll('.rw-data-section table thead th'), function(cell) { return cell.textContent.trim(); }), [
      'Status', 'Observed author', 'Paper', 'DOI'
    ]);
    assert.equal((document.querySelector('.ui.grey.label') as HTMLElement).textContent, 'no_orcid_candidate');
    assert.equal((document.querySelector('.ui.orange.label') as HTMLElement).textContent, 'orcid_is_unclear');
    assert.ok(!app.innerHTML.includes('must-not-render'));
    assert.ok(!app.innerHTML.includes('Candidate evidence'));

    globalThis.fetch = originalFetch;
    state.tables = [];
    history.pushState({}, '', '?view=home');
  });

});
