// Overview: retention funnel, metrics, coverage, breakdowns.
import { app, esc, value, link, formatNumber, formatTime, formatDuration, statusChip, metricEntries, metricCard, pageHeader, emptyState, panel, table, retentionFlow, breakdown, sourceResultCountSummary, sourceSearchQueries, selectedRun, list, bindCopyButtons, humanLabel } from '../state.ts';
import { api } from '../api.ts';
import { bindFocusContext } from '../router.ts';

/** Returns a normalization metric value or its unavailable presentation. */
function normalizationValue(metric: any): string {
  if (metric?.available === false) {
    return '<span class="ui faded text">Not recorded</span>';
  }
  if (metric?.denominator != null) {
    const pct = (metric.percentage ?? 0).toFixed(2);
    return '<small>' + formatNumber(metric.value) + ' of '
      + formatNumber(metric.denominator) + ' (' + pct + '%)</small>';
  }
  return formatNumber(metric?.value);
}

const executionMetricStages = [
  { id: 'ingest', label: 'Input capture', description: 'Raw records accepted from configured source exports.', matches: function(name: string) { return name === 'input_records'; } },
  { id: 'parse', label: 'Parsing', description: 'Source records converted into article metadata.', matches: function(name: string) { return name.startsWith('parsed_'); } },
  { id: 'deduplicate', label: 'Deduplication', description: 'Unique articles retained and duplicate records identified.', matches: function(name: string) { return name.startsWith('deduplicated_') || name.startsWith('duplicate_'); } },
  { id: 'enrich', label: 'Enrichment', description: 'Candidate articles and fields updated from provider evidence.', matches: function(name: string) { return name.startsWith('enrichment_') || name.startsWith('enriched_'); } },
  { id: 'validate', label: 'Validation', description: 'Articles accepted into or discarded from the analysis-ready corpus.', matches: function(name: string) { return name === 'valid_articles' || name === 'discarded_articles'; } },
  { id: 'normalize', label: 'Normalization', description: 'Valid article fields assessed and converted to canonical forms.', matches: function(name: string) { return name.startsWith('normalized_') || name.startsWith('normalization_'); } },
  { id: 'cache', label: 'Cache and network', description: 'Recorded provider-cache decisions and network fetches.', matches: function(name: string) { return name.startsWith('cache_'); } },
];

/** Returns the numeric value of a captured metric, or null when unavailable. */
function capturedMetricValue(item: any): string {
  if (item.available === false) {
    return '<span class="ui faded text">Not recorded</span>';
  }
  return formatNumber(item.value);
}

/** Groups captured metrics by pipeline stage. */
function capturedMetricsByStage(metrics: any[]) {
  const groups = executionMetricStages.map(function(stage) {
    return { id: stage.id, label: stage.label, description: stage.description, matches: stage.matches, metrics: [] as any[] };
  });
  const other = { id: 'other', label: 'Other run evidence', description: 'Recorded metrics without a recognized execution-stage prefix.', matches: function() { return true; }, metrics: [] as any[] };
  metrics.forEach(function(metric) {
    const name = String(metric.metric || '');
    const group = groups.find(function(stage) { return stage.matches(name); }) || other;
    group.metrics.push(metric);
  });
  if (other.metrics.length) {
    groups.push(other);
  }
  return groups.filter(function(group) { return group.metrics.length > 0; });
}

/** Returns table markup for captured pipeline metrics. */
function capturedMetricsMarkup(metrics: any[]): string {
  return '<div class="rw-captured-stages">' + capturedMetricsByStage(metrics).map(function(group) {
    const rows = group.metrics.map(function(metric) {
      return '<tr><td>' + esc(humanLabel(metric.metric)) + '</td><td>' + capturedMetricValue(metric) + '</td>'
        + '<td>' + (metric.source ? esc(humanLabel(metric.source)) : '<span class="ui faded text">Run total</span>') + '</td></tr>';
    }).join('');
    return '<section class="rw-captured-stage"><div class="rw-captured-stage__heading"><div><h4>' + esc(group.label) + '</h4><p>' + esc(group.description) + '</p></div>'
      + '<span class="ui label">' + formatNumber(group.metrics.length) + ' metrics</span></div>'
      + '<div class="table-wrap"><table class="ui table"><thead><tr><th>Metric</th><th>Value</th><th>Scope</th></tr></thead><tbody>' + rows + '</tbody></table></div></section>';
  }).join('') + '</div>';
}

/** Returns a metric copy with a percentage derived from its value and denominator. */
function fixedPercentageMetric(metric: any): any {
  if (!metric || metric.available === false || metric.denominator == null) {
    return metric;
  }
  const percentage = Number(metric.percentage ?? (Number(metric.value || 0) * 100 / Number(metric.denominator || 1)));
  return { ...metric, percentage: percentage.toFixed(2) + '%' };
}

/** Asynchronously implements overview view for the viewer. */
export async function overviewView(): Promise<void> {
  if (!value('run_id')) {
    app.innerHTML = emptyState('Overview', 'Select a search, revision, plan, and run attempt to inspect what the pipeline captured.', '<button type="button" data-focus-context>Focus context selector</button>');
    bindFocusContext();
    return;
  }

  const [overview, cache] = await Promise.all([
    api('/api/overview', { run_id: value('run_id') }),
    api('/api/runs/' + encodeURIComponent(value('run_id')) + '/cache-uses')
  ]);

  const run = selectedRun() || {};
  const captured = overview.captured_metrics || [];
  const relationship = overview.relationship_totals || {};
  const normalization = overview.normalization_breakdown || {};
  const normalizationFields = overview.normalization_field_breakdown || {};

  const corpusCards = [
    ['Work revisions', relationship.work_revisions, link({ view: 'corpus', section: 'articles' })],
    ['Authorships', relationship.authorships, link({ view: 'corpus', section: 'authors' })],
    ['Reference mentions', relationship.reference_mentions, link({ view: 'corpus', section: 'references' })],
    ['Internal citations', relationship.internal_citations, link({ view: 'relationships', mode: 'citation' })],
  ];

  const runIdentity = '<dl class="rw-run-identity" aria-label="Run identity">'
    + '<div><dt>Run attempt</dt><dd>' + esc(run.attempt_number || run.id || value('run_id')) + '</dd></div>'
    + '<div><dt>Started</dt><dd>' + esc(formatTime(run.started_at)) + '</dd></div>'
    + '<div><dt>Finished</dt><dd>' + esc(formatTime(run.finished_at)) + '</dd></div>'
    + '<div><dt>Duration</dt><dd>' + esc(formatDuration(run.started_at, run.finished_at)) + '</dd></div>'
    + '<div><dt>Execution plan</dt><dd>' + esc(run.execution_plan_id || value('plan_id') || '—') + '</dd></div>'
    + '<div><dt>Outcome</dt><dd>' + statusChip(run.status) + '</dd></div>'
    + '<div><dt>Visibility</dt><dd>' + statusChip(run.visibility_state || 'active') + '</dd></div>'
    + '</dl>';

  const coverage = metricEntries(overview.current_coverage || {});

  const normalizationCards = [
    ['Valid articles processed', normalization.normalized_articles_processed],
    ['Fields assessed', normalization.normalization_fields_processed],
    ['Canonical form changed', normalization.normalization_fields_changed],
    ['Already canonical', normalization.normalization_fields_already_canonical],
    ['No input value', normalization.normalization_fields_unavailable],
  ];

  const normalizationFieldLabels: Record<string, string> = {
    publisher: 'Publisher',
    journal: 'Journal (revision metadata)',
    author_name: 'Author name',
    affiliation: 'Affiliation'
  };

  const normalizationRows = Object.entries(normalizationFieldLabels).map(function([field, label]) {
    return { field: label, ...(normalizationFields[field] || {}) };
  });

  var cacheUses;
  if (cache) {
    cacheUses = cache.pagination?.total_rows ?? list(cache, ['cache_uses']).length;
  } else {
    cacheUses = 0;
  }

  const capturedMetrics = '<details class="rw-disclosure rw-overview-evidence span-all"><summary><span>All recorded execution metrics</span>'
    + '<small>' + formatNumber(captured.length) + ' metric rows grouped by stage</small></summary><div class="rw-disclosure__content">'
    + '<div class="rw-captured-evidence-intro"><h3>Captured during execution</h3><p>Values recorded while each pipeline stage ran. Missing evidence is not treated as zero.</p></div>'
    + capturedMetricsMarkup(captured)
    + '</div></details>';

  app.innerHTML = pageHeader('', 'Overview', 'Recorded execution evidence and current coverage are shown separately to preserve their meaning.')
    + '<div class="ui grid dashboard-grid">'
    + '<section class="rw-run-identity-strip span-all">' + runIdentity + '</section>'
    + retentionFlow(overview)
    + sourceResultCountSummary(overview.source_result_counts, 'span-all')
    + sourceSearchQueries(overview.source_result_counts, 'span-all')
    + panel('Corpus summary', 'Immutable records available for this selected run.',
        '<div class="ui statistics">' + corpusCards.map(function([name, metric, href]) {
          return metricCard(name, metric, href);
        }).join('') + '</div>')
    + panel('Current data coverage', 'Derived from stored run data, not necessarily captured when the run completed.',
        '<div class="ui statistics">' + (coverage.map(function([name, metric]) {
          return metricCard(name, metric);
        }).join('') || '<p class="ui faded text">Not recorded for this run.</p>') + '</div>')
    + breakdown('Enrichment activity', overview.enrichment_breakdown)
    + breakdown('Enriched fields', overview.enrichment_field_breakdown)
    + breakdown('Enrichment by provider', overview.enrichment_provider_breakdown)
    + breakdown('Validation activity', overview.validation_breakdown)
    + panel('Normalization activity', 'Every valid article is processed. Field checks are mutually exclusive: changed, already canonical, or unavailable. Journal canonical forms are stored in revision metadata.',
        '<div class="ui statistics">' + normalizationCards.map(function([name, metric]) {
          return metricCard(name, fixedPercentageMetric(metric));
        }).join('') + '</div>'
        + table('Normalization field outcomes', 'Changed, already-canonical, and unavailable counts use each field\u2019s assessed count as their denominator.', [
          { label: 'Field', render: function(row: any) { return esc(row.field); } },
          { label: 'Assessed', render: function(row: any) { return normalizationValue(row.processed); } },
          { label: 'Changed', render: function(row: any) { return normalizationValue(row.changed); } },
          { label: 'Already canonical', render: function(row: any) { return normalizationValue(row.already_canonical); } },
          { label: 'Unavailable', render: function(row: any) { return normalizationValue(row.unavailable); } },
        ], normalizationRows, 'rw-normalization-outcomes'))
    + breakdown('Source distribution', overview.source_breakdown, 'Result', true)
    + breakdown('Cache activity', overview.cache_breakdown)
    + panel('Cache-use explanation', 'A cache hit means a recorded provider response or completed computation was reused with provenance.',
        '<div class="metric-grid">' + metricCard('Recorded cache uses', { value: cacheUses }) + '</div>'
        + '<p class="ui info message">Reuse does not mean a work revision was copied without evidence. Each cache use remains linked to this historical run.</p>')
    + capturedMetrics
    + '</div>';

  bindCopyButtons();
}