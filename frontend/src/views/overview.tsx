// Overview: retention funnel, metrics, coverage, breakdowns.
import {
  app, value, link, formatNumber, formatTime, formatDuration, metricEntries,
  MetricCard, Table, selectedRun, PageHeader, EmptyState, Panel, RetentionFlow,
  Breakdown, SourceResultCountSummary, SourceSearchQueries, list, bindCopyButtons,
  humanLabel, StatusChip
} from '../state.tsx';
import { h, Fragment, render as renderTree } from '../jsx/jsx-runtime.ts';
import { api } from '../api.tsx';
import { bindFocusContext } from '../router.tsx';

/** Renders the unavailable-value presentation shared by metric helpers. */
function unavailableMarkup(): JSX.Element {
  return <span className="ui faded text">Not recorded</span>;
}

/** Renders a normalization metric value or its unavailable presentation. */
function normalizationValue(metric: any): JSX.Element {
  if (metric?.available === false) {
    return unavailableMarkup();
  }
  if (metric?.denominator != null) {
    const pct = (metric.percentage ?? 0).toFixed(2);
    return <small>{formatNumber(metric.value)} of {formatNumber(metric.denominator)} ({pct}%)</small>;
  }
  return <>{formatNumber(metric?.value)}</>;
}

const executionMetricStages = [
  {
    id: "ingest",
    label: "Input capture",
    description: "Raw records accepted from configured source exports.",
    matches: (name: string) => {
      return name === "input_records";
    },
  },
  {
    id: "parse",
    label: "Parsing",
    description: "Source records converted into article metadata.",
    matches: (name: string) => {
      return name.startsWith("parsed_");
    },
  },
  {
    id: "deduplicate",
    label: "Deduplication",
    description: "Unique articles retained and duplicate records identified.",
    matches: (name: string) => {
      return name.startsWith("deduplicated_") || name.startsWith("duplicate_");
    },
  },
  {
    id: "enrich",
    label: "Enrichment",
    description: "Candidate articles and fields updated from provider evidence.",
    matches: (name: string) => {
      return name.startsWith("enrichment_") || name.startsWith("enriched_");
    },
  },
  {
    id: "validate",
    label: "Validation",
    description: "Articles accepted into or discarded from the analysis-ready corpus.",
    matches: (name: string) => {
      return name === "valid_articles" || name === "discarded_articles";
    },
  },
  {
    id: "normalize",
    label: "Normalization",
    description: "Valid article fields assessed and converted to canonical forms.",
    matches: (name: string) => {
      return name.startsWith("normalized_") || name.startsWith("normalization_");
    },
  },
  {
    id: "cache",
    label: "Cache and network",
    description: "Recorded provider-cache decisions and network fetches.",
    matches: (name: string) => {
      return name.startsWith("cache_");
    },
  },
];

/** Renders the numeric value of a captured metric or its unavailable presentation. */
function capturedMetricValue(item: any): JSX.Element {
  if (item.available === false) {
    return unavailableMarkup();
  }
  return <>{formatNumber(item.value)}</>;
}

/** Groups captured metrics by pipeline stage. */
function capturedMetricsByStage(metrics: any[]) {
  const groups = executionMetricStages.map((stage) => {
    return {
      id: stage.id,
      label: stage.label,
      description: stage.description,
      matches: stage.matches,
      metrics: [] as any[],
    };
  });
  const other = {
    id: "other",
    label: "Other run evidence",
    description: "Recorded metrics without a recognized execution-stage prefix.",
    matches: () => {
      return true;
    },
    metrics: [] as any[],
  };
  metrics.forEach((metric) => {
    const name = String(metric.metric || "");
    const group = groups.find((stage) => {
      return stage.matches(name);
    }) || other;
    group.metrics.push(metric);
  });
  if (other.metrics.length) groups.push(other);
  return groups.filter((group) => {
    return group.metrics.length > 0;
  });
}

/** Renders table markup for captured pipeline metrics. */
function CapturedMetricsMarkup(props: { metrics: any[] }): JSX.Element {
  const stageGroups = capturedMetricsByStage(props.metrics);
  const stageSections = stageGroups.map((group) => {
    const rows = group.metrics.map((metric) => {
      var source: JSX.Element = <span className="ui faded text">Run total</span>;
      if (metric.source) source = <>{humanLabel(metric.source)}</>;
      return (
        <tr>
          <td>{humanLabel(metric.metric)}</td>
          <td>{capturedMetricValue(metric)}</td>
          <td>{source}</td>
        </tr>
      );
    });
    return (
      <section className="rw-captured-stage">
        <div className="rw-captured-stage__heading">
          <div>
            <h4>{group.label}</h4>
            <p>{group.description}</p>
          </div>
          <span className="ui label">{formatNumber(group.metrics.length)} metrics</span>
        </div>
        <div className="table-wrap">
          <table className="ui table">
            <thead>
              <tr>
                <th>Metric</th>
                <th>Value</th>
                <th>Scope</th>
              </tr>
            </thead>
            <tbody>{rows}</tbody>
          </table>
        </div>
      </section>
    );
  });
  return (
    <div className="rw-captured-stages">
      {stageSections}
    </div>
  );
}

/** Returns a metric copy with a percentage derived from its value and denominator. */
function fixedPercentageMetric(metric: any): any {
  if (!metric || metric.available === false || metric.denominator == null) {
    return metric;
  }
  const percentage = Number(metric.percentage ?? (Number(metric.value || 0) * 100 / Number(metric.denominator || 1)));
  return {
    ...metric,
    percentage: `${percentage.toFixed(2)}%`,
  };
}

/** Asynchronously implements overview view for the viewer. */
export async function overviewView(): Promise<void> {
  if (!value("run_id")) {
    const emptyStateMarkup = (
      <EmptyState title="Overview" detail="Select a search, revision, plan, and run attempt to inspect what the pipeline captured." action={<button type="button" data-focus-context>Focus context selector</button>} />
    );
    renderTree(emptyStateMarkup, app);
    bindFocusContext();
    return;
  }

  const runID = value("run_id");
  const [overview, cache] = await Promise.all([
    api("/api/overview", { run_id: runID }, {
      method: "GET",
      headers: { Accept: "application/json" },
    }),
    api(`/api/runs/${encodeURIComponent(runID)}/cache-uses`, {}, {
      method: "GET",
      headers: { Accept: "application/json" },
    }),
  ]);

  const run = selectedRun() || {};
  const captured = overview.captured_metrics || [];
  const relationship = overview.relationship_totals || {};
  const normalization = overview.normalization_breakdown || {};
  const normalizationFields = overview.normalization_field_breakdown || {};

  const corpusCards = [
    ["Analysis-ready articles", relationship.analysis_ready_articles, link({
      view: "corpus",
      section: "articles",
    })],
    ["All immutable work revisions", relationship.work_revisions, link({
      view: "advanced",
      table: "work_revisions",
      page: 1,
    })],
    ["Analysis-ready authorships", relationship.authorships, link({
      view: "corpus",
      section: "authors",
    })],
    ["Analysis-ready reference mentions", relationship.reference_mentions, link({
      view: "corpus",
      section: "references",
    })],
    ["Internal citations", relationship.internal_citations, link({
      view: "relationships",
      mode: "citation",
    })],
  ];

  const runIdentity = (
    <dl className="rw-run-identity" aria-label="Run identity">
      <div>
        <dt>Run attempt</dt>
        <dd>{run.attempt_number || run.id || runID}</dd>
      </div>
      <div>
        <dt>Started</dt>
        <dd>{formatTime(run.started_at)}</dd>
      </div>
      <div>
        <dt>Finished</dt>
        <dd>{formatTime(run.finished_at)}</dd>
      </div>
      <div>
        <dt>Duration</dt>
        <dd>{formatDuration(run.started_at, run.finished_at)}</dd>
      </div>
      <div>
        <dt>Execution plan</dt>
        <dd>{run.execution_plan_id || value("plan_id") || "—"}</dd>
      </div>
      <div>
        <dt>Outcome</dt>
        <dd><StatusChip raw={run.status} /></dd>
      </div>
      <div>
        <dt>Visibility</dt>
        <dd><StatusChip raw={run.visibility_state || "active"} /></dd>
      </div>
    </dl>
  );

  const coverage = metricEntries(overview.current_coverage || {});

  const normalizationCards = [
    ["Valid articles processed", normalization.normalized_articles_processed],
    ["Fields assessed", normalization.normalization_fields_processed],
    ["Canonical form changed", normalization.normalization_fields_changed],
    ["Already canonical", normalization.normalization_fields_already_canonical],
    ["No input value", normalization.normalization_fields_unavailable],
  ];

  const normalizationFieldLabels: Record<string, string> = {
    publisher: "Publisher",
    journal: "Journal (revision metadata)",
    author_name: "Author name",
    affiliation: "Affiliation",
  };

  const fieldEntries = Object.entries(normalizationFieldLabels);
  const normalizationRows = fieldEntries.map(([field, label]) => {
    return {
      field: label,
      ...(normalizationFields[field] || {}),
    };
  });

  var cacheUses = 0;
  if (cache) {
    cacheUses = cache.pagination?.total_rows ?? list(cache, ["cache_uses"]).length;
  }

  const capturedMetrics = (
    <details className="rw-disclosure rw-overview-evidence span-all">
      <summary>
        <span>All recorded execution metrics</span>
        <small>{formatNumber(captured.length)} metric rows grouped by stage</small>
      </summary>
      <div className="rw-disclosure__content">
        <div className="rw-captured-evidence-intro">
          <h3>Captured during execution</h3>
          <p>Values recorded while each pipeline stage ran. Missing evidence is not treated as zero.</p>
        </div>
        <CapturedMetricsMarkup metrics={captured} />
      </div>
    </details>
  );

  const corpusSummaryCards = corpusCards.map(([name, metric, href]) => {
    return <MetricCard name={name} metric={metric} href={href} />;
  });
  var coverageCards: JSX.Element[] = [<p className="ui faded text">Not recorded for this run.</p>];
  if (coverage.length) {
    coverageCards = coverage.map(([name, metric]) => {
      return <MetricCard name={name} metric={metric} />;
    });
  }
  const normalizationCardsMarkup = normalizationCards.map(([name, metric]) => {
    return <MetricCard name={name} metric={fixedPercentageMetric(metric)} />;
  });
  const normalizationColumns = [
    {
      label: "Field",
      render: (row: any) => {
        return <>{row.field}</>;
      },
    },
    {
      label: "Assessed",
      render: (row: any) => {
        return normalizationValue(row.processed);
      },
    },
    {
      label: "Changed",
      render: (row: any) => {
        return normalizationValue(row.changed);
      },
    },
    {
      label: "Already canonical",
      render: (row: any) => {
        return normalizationValue(row.already_canonical);
      },
    },
    {
      label: "Unavailable",
      render: (row: any) => {
        return normalizationValue(row.unavailable);
      },
    },
  ];

  const corpusSummaryBody = <div className="ui statistics">{corpusSummaryCards}</div>;
  const coverageBody = <div className="ui statistics">{coverageCards}</div>;
  const normalizationBody = (
    <Fragment>
      <div className="ui statistics">{normalizationCardsMarkup}</div>
      <Table title="Normalization field outcomes" description={"Changed, already-canonical, and unavailable counts use each field\u2019s assessed count as their denominator."} columns={normalizationColumns} rows={normalizationRows} classes="rw-normalization-outcomes" />
    </Fragment>
  );
  const cacheBody = (
    <Fragment>
      <div className="metric-grid"><MetricCard name="Recorded cache uses" metric={{ value: cacheUses }} /></div>
      <p className="ui info message">Reuse does not mean a work revision was copied without evidence. Each cache use remains linked to this historical run.</p>
    </Fragment>
  );

  var sourceFilterDiagnostics: JSX.Element | null = null;
  if (overview.source_filter_diagnostics?.length) {
    const diagnosticItems = overview.source_filter_diagnostics.map((diagnostic: any) => {
      return <li><strong>{diagnostic.source || "Unknown source"}:</strong> {diagnostic.message || "Stored source-filter evidence is invalid."}</li>;
    });
    sourceFilterDiagnostics = (
      <div className="ui negative message span-all" role="alert">
        <h3>Source-filter evidence needs attention</h3>
        <p>The retention flow excludes malformed stored counts instead of treating them as zero.</p>
        <ul>{diagnosticItems}</ul>
      </div>
    );
  }

  const pageMarkup = (
    <Fragment>
      <PageHeader kicker="" title="Overview" description="Recorded execution evidence and current coverage are shown separately to preserve their meaning." />
      <div className="ui grid dashboard-grid">
        <section className="rw-run-identity-strip span-all">{runIdentity}</section>
        <RetentionFlow overview={overview} />
        {sourceFilterDiagnostics}
        <SourceResultCountSummary items={overview.source_result_counts} classes="span-all" />
        <SourceSearchQueries items={overview.source_result_counts} classes="span-all" />
        <Panel title="Corpus summary" description="Immutable records available for this selected run." body={corpusSummaryBody} />
        <Panel title="Current data coverage" description="Derived from stored run data, not necessarily captured when the run completed." body={coverageBody} />
        <Breakdown title="Enrichment activity" source={overview.enrichment_breakdown} />
        <Breakdown title="Enriched fields" source={overview.enrichment_field_breakdown} />
        <Breakdown title="Enrichment by provider" source={overview.enrichment_provider_breakdown} />
        <Breakdown title="Validation activity" source={overview.validation_breakdown} />
        <Panel title="Normalization activity" description="Every valid article is processed. Field checks are mutually exclusive: changed, already canonical, or unavailable. Journal canonical forms are stored in revision metadata." body={normalizationBody} />
        <Breakdown title="Source distribution" source={overview.source_breakdown} valueLabel="Result" useTotal={true} />
        <Breakdown title="Cache activity" source={overview.cache_breakdown} />
        <Panel title="Cache-use explanation" description="A cache hit means a recorded provider response or completed computation was reused with provenance." body={cacheBody} />
        {capturedMetrics}
      </div>
    </Fragment>
  );
  renderTree(pageMarkup, app);

  bindCopyButtons();
}
