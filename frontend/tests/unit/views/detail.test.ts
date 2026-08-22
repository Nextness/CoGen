// Unit tests for views/detail.tsx — article, author, reference detail views.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.ts';
import { detailView } from '../../../src/views/detail.tsx';
import { app, state } from '../../../src/state.tsx';

describe('detail.tsx — detailView', function() {

  before(function() {
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it('shows empty state when no id is set', async function() {
    const url = new URL(location.href);
    url.searchParams.delete('article_id');
    history.pushState({}, '', url.toString());

    await detailView('article');
    assert.ok(app.innerHTML.includes('Open a record'));
  });

  it('renders article detail when article_id is set', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() {
          return Promise.resolve({
            data: {
              article: {
                id: 'a1', work_id: 10, pipeline_run_id: 1, title: 'Test Article', doi: '10.1000/test', year: 2024,
                abstract: 'Line one.\nLine two.',
                keywords: '["process mining","BPMN"]',
                keywords_plus: 'workflow; modelling',
                extension_data: '{"normalized_journal":"Journal of Tests","validation_status":"valid"}'
              },
              authors: [{ id: 'auth1', citation_name: 'Smith, J', author_order: 1 }],
              references: [],
              pdf_status: { status: 'available', byte_size: 2048, content_hash: 'pdf-hash' },
              enriched_fields: [],
              stage_outcomes: [
                { stage_name: 'parse', outcome: 'parsed', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
                { stage_name: 'enrich_metadata', outcome: 'enriched', created_at: '2024-01-01T00:01:00Z', updated_at: '2024-01-01T00:01:00Z' },
                { stage_name: 'validate', outcome: 'valid', created_at: '2024-01-01T00:02:00Z', updated_at: '2024-01-01T00:02:00Z' },
                { stage_name: 'normalize', outcome: 'normalized', created_at: '2024-01-01T00:03:00Z', updated_at: '2024-01-01T00:03:00Z' },
              ],
              audit_events: ([{
                id: 'field-enriched', action: 'field_enriched', actor: 'crossref', correlation_id: 'run:1:work:1', occurred_at: '2024-01-01T00:00:00Z',
                metadata_json: '{"field":"title","provider":"crossref"}'
              }] as any[]).concat(Array.from({ length: 29 }, function(_, index) {
                return { id: index + 1, action: 'validation_changed', actor: 'pipeline', correlation_id: 'run:1:work:1', occurred_at: '2024-01-01T00:00:00Z' };
              })),
            },
          });
        },
      } as unknown as Response);
    } as typeof fetch;

    const url = new URL(location.href);
    url.searchParams.set('article_id', 'a1');
    url.searchParams.set('view', 'article');
    url.searchParams.set('origin', 'view=corpus&section=articles&q=preserved+query&page=3&expanded=a1');
    history.pushState({}, '', url.toString());

    await detailView('article');
    assert.ok(app.innerHTML.includes('Article revision'));
    assert.ok(app.innerHTML.includes('Test Article'));
    assert.ok(app.innerHTML.includes('Bibliographic metadata'));
    assert.ok(app.innerHTML.includes('Provenance summary'));
    assert.ok(app.innerHTML.includes('rw-reading-workspace'));
    assert.ok(app.innerHTML.includes('Download PDF'));
    assert.ok(app.innerHTML.includes('Line one. Line two.'));
    assert.ok(app.innerHTML.includes('process mining'));
    assert.ok(app.innerHTML.includes('Show stored keyword value'));
    assert.ok(app.innerHTML.includes('Normalized Journal'));
    assert.ok(app.innerHTML.includes('Pipeline stage history'));
    assert.ok(app.innerHTML.includes('Parse'));
    assert.ok(app.innerHTML.includes('Enriched'));
    assert.ok(app.innerHTML.includes('Valid'));
    assert.ok(app.innerHTML.includes('Normalized'));
    assert.ok(app.innerHTML.includes('rw-audit-event--validation'));
    assert.ok(app.innerHTML.includes('rw-audit-event--enrichment'));
    assert.ok(app.innerHTML.includes('Correlation ID'));
    assert.equal(document.querySelectorAll('.rw-audit-event').length, 25);
    const moreEvents = document.querySelector('[data-record-audit-more]') as HTMLButtonElement;
    assert.equal(moreEvents.hidden, false);
    moreEvents.click();
    assert.equal(document.querySelectorAll('.rw-audit-event').length, 30);
    assert.equal(moreEvents.hidden, true);
    const breadcrumb = document.querySelector('#workspace-breadcrumb') as HTMLElement;
    assert.match(breadcrumb.textContent, /Home.*Deepdive.*Corpus.*10.1000\/test/);
    const corpusCrumb = Array.from(breadcrumb.querySelectorAll('a')).find(function(anchor) { return anchor.textContent === 'Corpus'; }) as HTMLAnchorElement;
    assert.ok(corpusCrumb.href.includes('q=preserved+query'));
    assert.ok(corpusCrumb.href.includes('page=3'));
    assert.ok(corpusCrumb.href.includes('expanded=a1'));
    const authorLink = Array.from(document.querySelectorAll('a')).find(function(anchor) { return anchor.textContent.includes('Smith, J'); }) as HTMLAnchorElement;
    assert.ok(authorLink.href.includes('origin='));
    assert.ok(decodeURIComponent(authorLink.href).includes('view=corpus'));

    globalThis.fetch = originalFetch;
    url.searchParams.delete('article_id');
    url.searchParams.delete('origin');
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());
  });

  it('renders discarded validation reasons as individual entries', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() {
          return Promise.resolve({
            data: {
              article: { id: 'a2', title: 'Discarded article' },
              authors: [],
              references: [],
              enriched_fields: [],
              audit_events: [],
              stage_outcomes: [{
                stage_name: 'validate', outcome: 'discarded',
                reason: '["DOI is missing","Publication year is invalid"]',
                created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z'
              }],
            },
          });
        },
      } as unknown as Response);
    } as typeof fetch;

    const url = new URL(location.href);
    url.searchParams.set('article_id', 'a2');
    url.searchParams.set('view', 'article');
    history.pushState({}, '', url.toString());

    await detailView('article');
    assert.ok(app.innerHTML.includes('Pipeline stage history'));
    assert.ok(app.innerHTML.includes('DOI is missing'));
    assert.ok(app.innerHTML.includes('Publication year is invalid'));
    assert.ok(!app.innerHTML.includes('[&quot;DOI is missing&quot;'));

    globalThis.fetch = originalFetch;
    url.searchParams.delete('article_id');
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());
  });

  it('renders author detail when author_id is set', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() {
          return Promise.resolve({
            data: {
              author: { id: 'auth1', citation_name: 'Smith, J' },
              articles: [],
              audit_events: [],
              identity_evidence: [{
                resolution_id: 4, status: 'orcid_is_unclear', provider: 'orcid', resolved_at: '2024-01-01T00:00:00Z',
                candidates: [{ candidate_orcid: '0000-0001-2345-6789', provider_display_name: 'Jane Smith', provider_rank: 1, query_url: 'https://orcid.example/search' }]
              }],
            },
          });
        },
      } as unknown as Response);
    } as typeof fetch;

    const url = new URL(location.href);
    url.searchParams.set('author_id', 'auth1');
    url.searchParams.set('view', 'author');
    history.pushState({}, '', url.toString());

    await detailView('author');
    assert.ok(app.innerHTML.includes('Author occurrence'));
    assert.ok(app.innerHTML.includes('Smith, J'));
    assert.ok(app.innerHTML.includes('ORCID candidate evidence'));
    assert.ok(app.innerHTML.includes('0000-0001-2345-6789'));
    assert.ok(app.innerHTML.includes('orcid_is_unclear'));
    assert.match((document.querySelector('#workspace-breadcrumb') as HTMLElement).textContent, /Home.*Deepdive.*Corpus.*Author/);
    assert.ok(!app.innerHTML.includes('Back to Article revision'));

    globalThis.fetch = originalFetch;
    url.searchParams.delete('author_id');
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());
  });

  it('renders the search term coverage panel', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() {
          return Promise.resolve({
            data: {
              article: { id: 'a3', title: 'Coverage article', abstract: 'BPMN abstract', keywords: '["bpmn"]', keywords_plus: '[]' },
              authors: [],
              references: [],
              enriched_fields: [],
              audit_events: [],
              stage_outcomes: [],
              term_matches: {
                title: ['BPMN'], abstract: ['BPMN'], keywords: ['BPMN'], keywords_plus: [],
                matched_total: 1, term_total: 3, sources: ['scopus'],
                terms_with_sources: { BPMN: ['scopus'], scheduling: ['scopus'], metaheuristic: ['wos'] },
                unmatched: ['scheduling', 'metaheuristic'],
              },
            },
          });
        },
      } as unknown as Response);
    } as typeof fetch;

    const url = new URL(location.href);
    url.searchParams.set('article_id', 'a3');
    url.searchParams.set('view', 'article');
    history.pushState({}, '', url.toString());

    await detailView('article');
    assert.ok(app.innerHTML.includes('Search term coverage'));
    assert.ok(app.innerHTML.includes('1 of 3 search terms matched this article.'));
    assert.ok(app.innerHTML.includes('All search terms'));
    assert.ok(app.innerHTML.includes('Matched terms'));
    assert.ok(app.innerHTML.includes('Unmatched terms'));
    assert.ok(app.innerHTML.includes('metaheuristic'));
    assert.ok(app.innerHTML.includes('(wos)'));
    assert.ok(app.innerHTML.includes('(scopus)'));

    globalThis.fetch = originalFetch;
    url.searchParams.delete('article_id');
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());
  });

  it('renders no search terms recorded when term_matches is null', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() {
          return Promise.resolve({
            data: {
              article: { id: 'a4', title: 'No coverage article' },
              authors: [],
              references: [],
              enriched_fields: [],
              audit_events: [],
              stage_outcomes: [],
              term_matches: null,
            },
          });
        },
      } as unknown as Response);
    } as typeof fetch;

    const url = new URL(location.href);
    url.searchParams.set('article_id', 'a4');
    url.searchParams.set('view', 'article');
    history.pushState({}, '', url.toString());

    await detailView('article');
    assert.ok(app.innerHTML.includes('Search term coverage'));
    assert.ok(app.innerHTML.includes('No search terms recorded for this run.'));

    globalThis.fetch = originalFetch;
    url.searchParams.delete('article_id');
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());
  });

  it('renders reference detail when reference_id is set', async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() {
          return Promise.resolve({
            data: {
              reference: { id: 'ref1', doi: '10.1234/test' },
            },
          });
        },
      } as unknown as Response);
    } as typeof fetch;

    const url = new URL(location.href);
    url.searchParams.set('reference_id', 'ref1');
    url.searchParams.set('view', 'reference');
    history.pushState({}, '', url.toString());

    await detailView('reference');
    assert.ok(app.innerHTML.includes('Reference mention'));
    assert.ok(app.innerHTML.includes('10.1234/test'));

    globalThis.fetch = originalFetch;
    url.searchParams.delete('reference_id');
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());
  });

});
