// Unit tests for views/overview.js — overview view.
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { overviewView } from '../../../src/views/overview.js';
import { app, state, value } from '../../../src/state.js';

describe('overview.js — overviewView', function() {

  before(function() {
    state.runs = [];
    state.searches = [];
    state.plans = [];
  });

  it('shows empty state when no run_id is set', async function() {
    const url = new URL(location.href);
    url.searchParams.delete('run_id');
    history.pushState({}, '', url.toString());

    await overviewView();
    assert.ok(app.innerHTML.includes('Select a search'));
  });

  it('renders overview content when run_id is set', async function() {
    const originalFetch = globalThis.fetch;
    var fetchCount = 0;
    globalThis.fetch = function(url) {
      fetchCount += 1;
      if (url.includes('/api/overview')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: function() {
            return Promise.resolve({
              data: {
                retention_funnel: {
                  input_records: 100,
                  parsed_articles: 90,
                  deduplicated_articles: 85,
                  valid_articles: 80,
                  discarded_articles: 5,
                },
                relationship_totals: { work_revisions: 80, authorships: 200, reference_mentions: 500, internal_citations: 50 },
                captured_metrics: [
                  { metric: 'parsed_articles', source: '', available: true, value: 90 },
                  { metric: 'deduplicated_articles', source: '', available: true, value: 85 },
                  { metric: 'enriched_article_updates', source: '', available: true, value: 20 },
                  { metric: 'valid_articles', source: '', available: true, value: 80 },
                ],
                current_coverage: {},
                normalization_breakdown: {
                  normalization_fields_changed: { value: 1, denominator: 4, percentage: 25 }
                },
                normalization_field_breakdown: {},
                enrichment_breakdown: {},
                enrichment_field_breakdown: {},
                enrichment_provider_breakdown: {},
                validation_breakdown: {},
                source_breakdown: {},
                cache_breakdown: {},
                source_result_counts: [],
              },
            });
          },
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: function() { return Promise.resolve({ data: [] }); },
      });
    };

    const url = new URL(location.href);
    url.searchParams.set('run_id', 'run-1');
    history.pushState({}, '', url.toString());
    state.runs = [{
      id: 'run-1', attempt_number: 3, execution_plan_id: 9, status: 'completed', visibility_state: 'active',
      started_at: '2024-01-01T00:00:00Z', finished_at: '2024-01-01T00:05:30Z'
    }];

    await overviewView();
    assert.ok(app.innerHTML.includes('Overview'));
    assert.ok(app.innerHTML.includes('metric rows grouped by stage'));
    assert.ok(app.innerHTML.includes('Parsing'));
    assert.ok(app.innerHTML.includes('Deduplication'));
    assert.ok(app.innerHTML.includes('25.00%'));
    assert.ok(app.innerHTML.includes('rw-run-identity-strip'));
    assert.ok(app.innerHTML.includes('5m 30s'));
    assert.ok(!app.innerHTML.includes('Selected historical run'));
    assert.deepEqual(Array.from(document.querySelectorAll('.rw-retention__phase-header h4'), function(heading) { return heading.textContent; }), [
      'Source selection', 'Pipeline processing', 'Corpus enrichment'
    ]);
    assert.ok(document.querySelector('[data-flow-stage="parsed_articles"] a').href.includes('stage_q=parse'));
    assert.equal(document.querySelector('.dashboard-grid').lastElementChild.classList.contains('rw-overview-evidence'), true);
    assert.ok(fetchCount >= 2);

    globalThis.fetch = originalFetch;
    state.runs = [];
    url.searchParams.delete('run_id');
    history.pushState({}, '', url.toString());
  });

});
