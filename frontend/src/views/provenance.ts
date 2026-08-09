// Read-only execution evidence: audit events, artifacts, cache uses, stages, and run details.
import {
  app, esc, value, link, state, provenanceSections, section, pageHeader,
  emptyPanel, panel, subnav, cell, list, selectedRun, pickID, formatTime, formatBytes,
  formatNumber, humanLabel, statusChip, filterChips, pageSizes
} from '../state.ts';
import { api } from '../api.ts';
import { setURL, bindFocusContext } from '../router.ts';
import { dataTable, bindTableControls } from '../components/data-table.ts';
import type { DataTableContext } from '../components/data-table.ts';
import { auditStream as sharedAuditStream } from '../components/audit-events.ts';
import type { AuditEventRecord } from '../components/audit-events.ts';

const auditFilterKeys: Record<string, string> = {
  audit_q: 'Search',
  audit_category: 'Category',
  audit_action: 'Event type',
  audit_actor: 'Source system',
  audit_entity: 'Entity type',
  audit_stage: 'Pipeline stage',
  audit_outcome: 'Outcome'
};

let activeArtifactPreview: any = null;
let activeArtifactRow: HTMLElement | null = null;
let auditEvents: AuditEventRecord[] = [];
let auditCursor = '';
let auditHasMore = false;

/** Returns the selected comma-separated values for an audit facet. */
function selectedValues(raw: any): string[] {
  return String(raw || '').split(',').map(function(item) { return item.trim(); }).filter(Boolean);
}

/** Returns a multi-select control for one audit facet. */
function auditMultiSelect(name: string, label: string, options: any[], selectedRaw: any): string {
  const selected = new Set(selectedValues(selectedRaw));
  const summary = selected.size ? selected.size + ' selected' : 'Any';
  const choices = (options || []).map(function(item) {
    return '<label class="rw-check-option"><input type="checkbox" name="' + esc(name) + '" value="' + esc(item) + '"'
      + (selected.has(String(item)) ? ' checked' : '') + '><span>' + esc(humanLabel(item)) + '</span></label>';
  }).join('');
  const empty = choices || '<p class="ui faded text">No recorded values are available for this run.</p>';
  return '<details class="rw-multi-select" data-multi-select><summary><span>' + esc(label) + '</span><strong>' + esc(summary) + '</strong></summary>'
    + '<div class="rw-multi-select__menu"><div class="rw-multi-select__actions"><button type="button" class="ui basic button" data-multi-select-all>Select all</button>'
    + '<button type="button" class="ui basic button" data-multi-select-clear>Clear</button></div>' + empty + '</div></details>';
}

/** Builds API query parameters from the active audit filters. */
function auditQuery(cursor: string): Record<string, any> {
  return {
    run_id: value('run_id'),
    q: value('audit_q'),
    category: value('audit_category'),
    action: value('audit_action'),
    actor: value('audit_actor'),
    entity_type: value('audit_entity'),
    stage: value('audit_stage'),
    outcome: value('audit_outcome'),
    limit: 25,
    cursor: cursor || ''
  };
}

/** Returns markup summarizing active audit filters and their removal links. */
function auditFilterSummary(): string {
  const filters: Record<string, string> = {};
  Object.keys(auditFilterKeys).forEach(function(key) {
    if (value(key)) {
      filters[key] = value(key);
    }
  });
  const clearUpdates: Record<string, string> = {};
  Object.keys(auditFilterKeys).forEach(function(key) { clearUpdates[key] = ''; });
  return filterChips(filters, auditFilterKeys, { clearUpdates: clearUpdates });
}

/** Returns the complete audit filter form. */
function auditFilters(facets: any): string {
  const actors = list(facets, ['actors']);
  const actions = list(facets, ['actions']);
  const entityTypes = list(facets, ['entity_types']);
  return '<aside class="ui segment rw-audit-filters">'
    + '<div class="ui top attached header"><div><h3>Filter audit evidence</h3>'
    + '<p>Filters are applied by the server and remain visible in the URL.</p></div></div>'
    + '<div class="content"><form id="audit-filter-form" class="ui form">'
    + '<label class="rw-filter-search">Search events'
    + '<input name="audit_q" type="search" value="' + esc(value('audit_q')) + '" placeholder="Action, entity, source, or metadata"></label>'
    + '<div class="rw-filter-field-grid">'
    + auditMultiSelect('audit_category', 'Category', ['pipeline', 'enrichment', 'validation', 'pdf'], value('audit_category'))
    + auditMultiSelect('audit_action', 'Event type', actions, value('audit_action'))
    + auditMultiSelect('audit_actor', 'Source system', actors, value('audit_actor'))
    + auditMultiSelect('audit_entity', 'Entity type', entityTypes, value('audit_entity'))
    + '</div>'
    + '<details class="rw-filter-disclosure"><summary>Stage and outcome filters</summary>'
    + '<div class="rw-filter-field-grid rw-filter-field-grid--stacked">'
    + '<label>Pipeline stage<input name="audit_stage" value="' + esc(value('audit_stage')) + '" placeholder="For example, normalize"></label>'
    + '<label>Outcome<input name="audit_outcome" value="' + esc(value('audit_outcome')) + '" placeholder="For example, completed"></label>'
    + '</div></details>'
    + auditFilterSummary()
    + '<div class="rw-filter-actions"><button type="submit" class="ui primary button">Apply filters</button>'
    + '<a class="ui basic button" href="' + link(Object.fromEntries(Object.keys(auditFilterKeys).map(function(key) { return [key, '']; }))) + '">Reset</a></div>'
    + '</form></div></aside>';
}

/** Returns summary cards for the filtered audit result. */
function auditSummary(data: any): string {
  const summary = data.summary || {};
  const actions = list(summary, ['actions']);
  const scope = value('run_id')
    ? 'Run ' + esc(value('run_id')) + (selectedValues(value('audit_category')).includes('pdf') ? ' plus global PDF evidence' : '')
    : 'All recorded runs';
  return '<dl class="rw-summary-strip">'
    + '<div><dt>Matching events</dt><dd>' + formatNumber(summary.total_events || auditEvents.length) + '</dd></div>'
    + '<div><dt>Events loaded</dt><dd data-audit-loaded-count>' + formatNumber(auditEvents.length) + '</dd></div>'
    + '<div><dt>Event types</dt><dd>' + formatNumber(actions.length) + '</dd></div>'
    + '<div><dt>Scope</dt><dd>' + scope + '</dd></div>'
    + '</dl>';
}

/** Returns the audit timeline and pagination markup. */
function auditView(data: any): string {
  auditEvents = list(data, ['events', 'items']);
  auditCursor = data.next_cursor || '';
  auditHasMore = Boolean(data.has_more);
  return '<div class="rw-audit-layout">' + auditFilters(data.facets || {})
    + '<section class="ui segment rw-audit-results">'
    + '<div class="ui top attached header"><div><h3>Audit timeline</h3>'
    + '<p>Newest evidence appears first. Open Recorded data only when identifiers or payload changes are needed.</p></div></div>'
    + '<div class="content">' + auditSummary(data)
    + '<div id="audit-event-stream" aria-busy="false">' + sharedAuditStream(auditEvents) + '</div>'
    + '<div class="rw-load-more"' + (auditHasMore ? '' : ' hidden') + '>'
    + '<p class="ui faded text" role="status" aria-live="polite" data-audit-page-status>' + formatNumber(auditEvents.length) + ' events loaded.</p>'
    + '<button type="button" class="ui button" data-audit-load-more>Load 25 older events</button></div>'
    + '<p class="rw-audit-end ui success message" data-audit-end' + (auditHasMore ? ' hidden' : '') + '>The beginning of the recorded history has been reached.</p>'
    + '<div class="ui error message" data-audit-page-error role="alert" hidden></div>'
    + '</div></section></div>';
}

/** Returns the research-context fields displayed for an artifact. */
function artifactContext(context: any): string {
  const entries = [
    ['Search', context.search_id || 'Not recorded'],
    ['Revision', context.search_revision_label || context.search_revision_id || 'Not recorded'],
    ['Execution plan', context.execution_fingerprint ? String(context.execution_fingerprint).slice(0, 16) : context.execution_plan_id || 'Not recorded'],
    ['Run attempt', context.run_id ? 'Run ' + context.run_id + ', attempt ' + context.attempt_number : 'Not recorded']
  ];
  return '<dl class="rw-context-strip">' + entries.map(function(entry) {
    return '<div><dt>' + esc(entry[0]) + '</dt><dd>' + esc(entry[1]) + '</dd></div>';
  }).join('') + '</dl>';
}

/** Returns safe inspect and download actions for an artifact. */
function artifactActions(row: any): string {
  if (!row.has_blob) {
    return '<span class="ui faded text">Payload not stored</span>';
  }
  const inspect = row.preview_available
    ? '<button type="button" class="ui button" data-inspect-artifact="' + esc(row.id) + '">Inspect preview</button>'
    : '<span class="ui label">Download only</span>';
  return '<div class="artifact-actions">' + inspect
    + '<a class="ui basic button" href="/api/artifacts/' + encodeURIComponent(row.id) + '/content" download>Download</a></div>';
}

/** Returns the run artifact inventory markup. */
function artifactsView(data: any): string {
  const artifacts = list(data, ['artifacts']);
  activeArtifactPreview = null;
  activeArtifactRow = null;
  var rows;
  if (artifacts.length) {
    rows = artifacts.map(function(row) {
      const role = row.artifact_roles || 'Stage artifact';
      const stages = [row.produced_by_steps ? 'Produced: ' + row.produced_by_steps : '', row.consumed_by_steps ? 'Consumed: ' + row.consumed_by_steps : ''].filter(Boolean).join(' / ');
      return '<tr data-artifact-row="' + esc(row.id) + '">'
        + '<td><strong>' + esc(humanLabel(role)) + '</strong><small class="rw-cell-note">Artifact ' + esc(row.id) + '</small></td>'
        + '<td>' + (stages ? esc(stages) : '<span class="ui faded text">Not linked to a recorded step</span>') + '</td>'
        + '<td><span class="rw-mono">' + esc(row.content_type) + '</span><small class="rw-cell-note">' + esc(formatBytes(row.byte_size)) + '</small></td>'
        + '<td><span class="rw-cell" title="' + esc(row.content_hash) + '">' + esc(row.content_hash) + '</span></td>'
        + '<td>' + esc(formatTime(row.created_at)) + '</td>'
        + '<td>' + artifactActions(row) + '</td></tr>';
    }).join('');
  } else {
    rows = '<tr><td colspan="6" class="empty">No artifacts were recorded for this run.</td></tr>';
  }
  return artifactContext(data.context || {})
    + '<p class="ui info message">Artifact inspection is read-only. Text previews are bounded before they reach the browser; download the original file for complete inspection.</p>'
    + '<section class="ui segment rw-artifact-list"><div class="ui top attached header"><div><h3>Recorded artifacts</h3>'
    + '<p>Content-addressed files linked to this run through configuration roles or execution steps.</p></div>'
    + '<span class="ui label">' + artifacts.length.toLocaleString() + ' artifacts</span></div>'
    + '<div class="content"><div class="table-wrap"><table class="ui table rw-artifact-table">'
    + '<thead><tr><th>Role</th><th>Execution use</th><th>Format and size</th><th>Content hash</th><th>Created</th><th>Actions</th></tr></thead>'
    + '<tbody>' + rows + '</tbody></table></div></div></section>'
    + '<section class="ui segment rw-artifact-inspector" id="artifact-inspector">'
    + '<div class="ui top attached header"><div><h3>Artifact preview</h3><p>Inspect a safe prefix of a text-based artifact without loading the full file into the document.</p></div></div>'
    + '<div class="content">' + emptyPanel('No artifact selected', 'Choose Inspect preview from the artifact list above.') + '</div></section>';
}

/** Returns page-size option markup with the current value selected. */
function pageSizeOptions(current: any): string {
  return pageSizes.map(function(size) {
    return '<option value="' + size + '"' + (Number(current) === size ? ' selected' : '') + '>' + size + '</option>';
  }).join('');
}

/** Returns cache-use evidence and pagination markup. */
function cacheView(data: any): string {
  const rows = list(data, ['rows', 'cache_uses']);
  const columns = list(data, ['columns']);
  const page = Math.max(1, Number(value('cache_page') || data.pagination?.page || 1));
  const perPage = Number(value('cache_per_page') || data.pagination?.per_page || 50);
  const pagination = data.pagination || { page: page, per_page: perPage, total_rows: rows.length, total_pages: Math.max(1, Math.ceil(rows.length / perPage)) };
  const result = { columns: columns.length ? columns : Object.keys(rows[0] || {}), rows: rows, pagination: pagination };
  const filterSummary = value('cache_q') ? filterChips({ cache_q: value('cache_q') }, { cache_q: 'Search' }, {
    removeUpdates: { cache_page: 1 }, clearUpdates: { cache_q: '', cache_page: 1 }
  }) : '';
  const controls = '<form class="rw-table-controls" id="cache-controls"><label class="rw-table-search">Search cache evidence'
    + '<input id="cache-query" type="search" value="' + esc(value('cache_q')) + '" placeholder="Provider, namespace, outcome, or fingerprint"></label>'
    + '<label>Rows per page<select id="cache-per-page">' + pageSizeOptions(perPage) + '</select></label>'
    + '<button type="submit" class="ui primary button" data-cache-search>Search</button>' + filterSummary + '</form>';
  const tableMarkup = dataTable('Cache uses', result, {
    page: page,
    perPage: perPage,
    pageKey: 'cache_page', perPageKey: 'cache_per_page', sortKey: 'cache_sort', orderKey: 'cache_order', queryKey: 'cache_q', expandedKey: 'cache_expanded',
    query: value('cache_q'), itemLabel: 'cache uses',
    columnsWhitelist: ['provider', 'namespace', 'outcome', 'cache_layer', 'response_status', 'used_at', 'expires_at', 'payload_artifact_id', 'request_fingerprint'],
    columnConfig: {
      provider: { label: 'Provider' }, namespace: { label: 'Namespace' },
      outcome: { label: 'Outcome', render: function(row, raw) { return statusChip(raw); } },
      cache_layer: { label: 'Cache layer' }, response_status: { label: 'Response' },
      used_at: { label: 'Used', render: function(row, raw) { return esc(formatTime(raw)); } },
      expires_at: { label: 'Expires', render: function(row, raw) { return esc(formatTime(raw)); } },
      payload_artifact_id: { label: 'Payload artifact', render: function(row, raw) {
        return raw ? '<a href="' + link({ section: 'artifacts' }) + '">Artifact ' + esc(raw) + '</a>' : '<span class="ui faded text">None</span>';
      } },
      request_fingerprint: { label: 'Request fingerprint', className: 'rw-column-fingerprint' }
    }
  });
  return panel('Cache use records', 'Recorded provider-response reuse and negative or stale outcomes for this run.', controls + tableMarkup, 'rw-cache-view');
}

/** Returns the effective display status for a work-stage record. */
function stageStatus(summary: any, step: any): string {
  const recorded = String(step?.step_status || '').toLocaleLowerCase();
  if (recorded) {
    return recorded;
  }
  const outcomes = summary?.outcomes || {};
  const names = Object.keys(outcomes).map(function(item) { return item.toLocaleLowerCase(); });
  if (names.some(function(item) { return item.includes('fail') || item.includes('error'); })) {
    return 'failed';
  }
  if (names.length && names.every(function(item) { return item.includes('skip'); })) {
    return 'skipped';
  }
  if (names.some(function(item) { return item.includes('discard') || item.includes('warning'); })) {
    return 'warning';
  }
  return summary ? 'completed' : 'not-recorded';
}

/** Returns ordered stage-flow markup for one work. */
function stageFlow(summaries: any[], steps: any[]): string {
  const summaryByName = new Map(summaries.map(function(item) { return [item.stage_name, item]; }));
  const stepByName = new Map(steps.map(function(item) { return [item.step_name, item]; }));
  const standard = ['preflight', 'load', 'parse', 'deduplicate', 'enrich', 'enrich_metadata', 'enrich_identity', 'validate', 'normalize', 'finalize'];
  const available = new Set<string>([...summaryByName.keys(), ...stepByName.keys()]);
  const names = standard.filter(function(name) { return available.delete(name); }).concat(Array.from(available).sort());
  if (!names.length) {
    return '<div class="rw-empty-inline"><strong>No stage progression was recorded.</strong><p>Detailed stage records are also unavailable for this run.</p></div>';
  }
  return '<ol class="rw-stage-flow">' + names.map(function(name, index) {
    const summary = summaryByName.get(name);
    const step = stepByName.get(name);
    const status = stageStatus(summary, step);
    const outcomes = summary?.outcomes || {};
    const outcomeText = Object.entries(outcomes).map(function(entry) { return formatNumber(entry[1]) + ' ' + humanLabel(entry[0]).toLocaleLowerCase(); }).join(', ');
    const artifacts: string[] = [];
    if (step?.input_artifact_id) {
      artifacts.push('<a href="' + link({ section: 'artifacts' }) + '">Input artifact ' + esc(step.input_artifact_id) + '</a>');
    }
    if (step?.output_artifact_id) {
      artifacts.push('<a href="' + link({ section: 'artifacts' }) + '">Output artifact ' + esc(step.output_artifact_id) + '</a>');
    }
    var duration = 'Not recorded';
    if (step?.duration_seconds != null) {
      const seconds = Number(step.duration_seconds);
      if (seconds === 0 && step.started_at && step.finished_at) {
        duration = 'Less than 1 second';
      } else {
        duration = seconds.toLocaleString(undefined, { maximumFractionDigits: 3 }) + (seconds === 1 ? ' second' : ' seconds');
      }
    }
    const records = summary ? formatNumber(summary.total_records) : '<span class="ui faded text">Not applicable</span>';
    return '<li class="rw-stage-step rw-stage-step--' + esc(status) + '">'
      + '<div class="rw-stage-step__marker"><span>' + (index + 1) + '</span></div>'
      + '<div class="rw-stage-step__body"><div class="rw-stage-step__heading"><h4>' + esc(humanLabel(name)) + '</h4>' + statusChip(humanLabel(status)) + '</div>'
      + '<p>' + esc(outcomeText || (step ? 'Execution step recorded.' : 'Stage summary recorded.')) + '</p>'
      + '<dl><div><dt>Work outcome records</dt><dd>' + records + '</dd></div>'
      + '<div><dt>Duration</dt><dd>' + esc(duration) + '</dd></div></dl>'
      + (artifacts.length ? '<div class="rw-stage-step__artifacts">' + artifacts.join('') + '</div>' : '')
      + '</div></li>';
  }).join('') + '</ol>';
}

/** Returns work-stage evidence and pagination markup. */
function stagesView(data: any): string {
  const rows = list(data, ['rows']);
  const summaries = list(data, ['stage_summaries']);
  const steps = list(data, ['run_steps']);
  const page = Math.max(1, Number(value('stage_page') || data.pagination?.page || 1));
  const perPage = Number(value('stage_per_page') || data.pagination?.per_page || 50);
  const result = { columns: list(data, ['columns']), rows: rows, pagination: data.pagination || { page: page, per_page: perPage, total_rows: rows.length } };
  const controls = '<form class="rw-table-controls" id="stage-controls"><label class="rw-table-search">Search detailed outcomes'
    + '<input id="stage-query" type="search" value="' + esc(value('stage_q')) + '" placeholder="Stage, outcome, reason, or work ID"></label>'
    + '<label>Rows per page<select id="stage-per-page">' + pageSizeOptions(perPage) + '</select></label>'
    + '<button type="submit" class="ui primary button" data-stage-search>Search</button>'
    + (value('stage_q') ? filterChips({ stage_q: value('stage_q') }, { stage_q: 'Search' }, {
      removeUpdates: { stage_page: 1 }, clearUpdates: { stage_q: '', stage_page: 1 }
    }) : '') + '</form>';
  const details = dataTable('Detailed stage outcomes', result, {
    page: page, perPage: perPage, query: value('stage_q'), itemLabel: 'stage outcomes',
    pageKey: 'stage_page', perPageKey: 'stage_per_page', sortKey: 'stage_sort', orderKey: 'stage_order', queryKey: 'stage_q', expandedKey: 'stage_expanded',
    columnsWhitelist: ['work_id', 'stage_name', 'outcome', 'reason', 'created_at', 'updated_at'],
    columnConfig: {
      work_id: { label: 'Work' }, stage_name: { label: 'Stage' },
      outcome: { label: 'Outcome', render: function(row, raw) { return statusChip(raw); } },
      reason: { label: 'Reason' },
      created_at: { label: 'Recorded', render: function(row, raw) { return esc(formatTime(raw)); } },
      updated_at: { label: 'Updated', render: function(row, raw) { return esc(formatTime(raw)); } }
    }
  });
  return '<section class="ui segment rw-stage-overview"><div class="ui top attached header"><div><h3>Stage outcomes and progression</h3>'
    + '<p>This view explains what happened during the run. It does not provide pipeline controls.</p></div></div>'
    + '<p class="ui info message">Counts describe stored per-work outcome rows, not raw input records or field updates. A stage without per-work evidence is marked not applicable. Durations use recorded step timestamps; steps inside one timestamp tick are shown as less than one second.</p>'
    + '<div class="content">' + stageFlow(summaries, steps) + '</div></section>'
    + panel('Detailed stage outcomes', 'Per-work evidence remains available below the run-level progression.', controls + details, 'rw-stage-details');
}

/** Returns stored run details and exact configuration links. */
function runView(artifactData: any): string {
  const run = selectedRun();
  const plan = state.plans.find(function(item) {
    return String(pickID(item)) === String(run?.execution_plan_id || value('plan_id'));
  });
  const snapshots = list(artifactData, ['artifacts']).filter(function(artifact) { return artifact.artifact_roles; });
  const started = run?.started_at ? new Date(run.started_at) : null;
  const finished = run?.finished_at ? new Date(run.finished_at) : null;
  const duration = started && finished && !Number.isNaN(started.getTime()) && !Number.isNaN(finished.getTime())
    ? Math.max(0, (finished.getTime() - started.getTime()) / 1000).toLocaleString() + ' seconds' : 'Not recorded';
  const summary = '<dl class="rw-summary-strip">'
    + '<div><dt>Run attempt</dt><dd>' + esc(run?.attempt_number || value('run_id')) + '</dd></div>'
    + '<div><dt>Status</dt><dd>' + statusChip(run?.status) + '</dd></div>'
    + '<div><dt>Started</dt><dd>' + esc(formatTime(run?.started_at)) + '</dd></div>'
    + '<div><dt>Duration</dt><dd>' + esc(duration) + '</dd></div></dl>';
  const fields: Record<string, any> = {
    run_id: run?.id || value('run_id'), execution_plan_id: run?.execution_plan_id || value('plan_id'),
    attempt_number: run?.attempt_number, status: run?.status, visibility: run?.visibility_state,
    started_at: run?.started_at, finished_at: run?.finished_at, summary: run?.summary,
    execution_fingerprint: plan?.execution_fingerprint, resolved_manifest_hash: plan?.resolved_manifest_hash,
    input_manifest_hash: plan?.input_manifest_hash, enrichment_enabled: plan?.enrichment_enabled
  };
  const properties = '<dl class="property-grid property-grid--compact">' + Object.entries(fields).map(function(entry) {
    return '<div><dt>' + esc(humanLabel(entry[0])) + '</dt><dd>' + cell(entry[1], entry[0]) + '</dd></div>';
  }).join('') + '</dl>';
  var snapshotRows;
  if (snapshots.length) {
    snapshotRows = '<ul class="rw-file-list">' + snapshots.map(function(artifact) {
      const action = artifact.has_blob
        ? '<a class="ui basic button" href="/api/artifacts/' + encodeURIComponent(artifact.id) + '/content" download>Download</a>'
        : '<span class="ui label">Payload unavailable</span>';
      return '<li><div><strong>' + esc(humanLabel(artifact.artifact_roles)) + '</strong>'
        + '<span>' + esc(formatBytes(artifact.byte_size)) + ' / ' + esc(artifact.content_type) + '</span></div>' + action + '</li>';
    }).join('') + '</ul>';
  } else {
    snapshotRows = '<p class="ui faded text">No configuration snapshots were recorded for this legacy run.</p>';
  }
  return summary
    + panel('Stored run attempt', 'Immutable execution identity, lifecycle, and plan fingerprints.', properties, 'rw-run-record')
    + panel('Configuration snapshots', 'Exact workspace configuration, resolved manifest, and input manifest captured for this attempt.', snapshotRows, 'rw-run-snapshots');
}

/** Asynchronously implements provenance view for the viewer. */
export async function provenanceView(): Promise<void> {
  var current = section('section', 'audit');
  if (!provenanceSections[current]) {
    current = 'audit';
  }
  const title = provenanceSections[current][0];
  if (!value('run_id') && current !== 'audit') {
    app.innerHTML = pageHeader('Execution evidence', 'Provenance', 'Inspect append-only evidence without changing the workspace.')
      + subnav(Object.entries(provenanceSections).map(function(entry) { return [entry[0], entry[1][0]]; }), current, 'section')
      + emptyPanel(title, 'Select a run attempt to inspect this provenance view.', '<button type="button" class="ui button" data-focus-context>Focus context selector</button>');
    bindFocusContext();
    return;
  }

  var content;
  if (current === 'audit') {
    content = auditView(await api('/api/audit', auditQuery('')));
  } else if (current === 'artifacts') {
    content = artifactsView(await api('/api/runs/' + encodeURIComponent(value('run_id')) + '/artifacts'));
  } else if (current === 'cache') {
    content = cacheView(await api('/api/runs/' + encodeURIComponent(value('run_id')) + '/cache-uses', {
      page: value('cache_page') || 1, per_page: value('cache_per_page') || 50,
      sort: value('cache_sort') || 'id', order: value('cache_order') || 'asc', q: value('cache_q')
    }));
  } else if (current === 'stages') {
    content = stagesView(await api('/api/runs/' + encodeURIComponent(value('run_id')) + '/stages', {
      page: value('stage_page') || 1, per_page: value('stage_per_page') || 50,
      sort: value('stage_sort') || 'id', order: value('stage_order') || 'asc', q: value('stage_q')
    }));
  } else {
    content = runView(await api('/api/runs/' + encodeURIComponent(value('run_id')) + '/artifacts'));
  }

  app.innerHTML = pageHeader('Execution evidence', 'Provenance', 'Inspect append-only audit events, artifacts, cache decisions, and stage outcomes without changing the selected run.')
    + subnav(Object.entries(provenanceSections).map(function(entry) { return [entry[0], entry[1][0]]; }), current, 'section')
    + content;

  if (current === 'audit') {
    bindAuditControls();
  } else if (current === 'artifacts') {
    bindArtifactInspection();
  } else if (current === 'cache') {
    bindTableControls('cache', Number(value('cache_page') || 1), {
      pageKey: 'cache_page', perPageKey: 'cache_per_page', sortKey: 'cache_sort', orderKey: 'cache_order', queryKey: 'cache_q', expandedKey: 'cache_expanded',
      querySelector: '#cache-query', perPageSelector: '#cache-per-page', searchButtonSelector: '[data-cache-search]'
    });
  } else if (current === 'stages') {
    bindTableControls('stages', Number(value('stage_page') || 1), {
      pageKey: 'stage_page', perPageKey: 'stage_per_page', sortKey: 'stage_sort', orderKey: 'stage_order', queryKey: 'stage_q', expandedKey: 'stage_expanded',
      querySelector: '#stage-query', perPageSelector: '#stage-per-page', searchButtonSelector: '[data-stage-search]'
    });
  }
}

/** Binds DOM behavior for audit controls. */
function bindAuditControls(): void {
  const form = document.querySelector<HTMLFormElement>('#audit-filter-form');
  if (form) {
    form.addEventListener('submit', function(event) {
      event.preventDefault();
      const values = new FormData(form);
      const updates: Record<string, any> = {};
      const multiValueKeys = new Set(['audit_category', 'audit_action', 'audit_actor', 'audit_entity']);
      Object.keys(auditFilterKeys).forEach(function(key) {
        updates[key] = multiValueKeys.has(key) ? values.getAll(key).join(',') : values.get(key) || '';
      });
      setURL(updates);
    });
    form.querySelectorAll('[data-multi-select]').forEach(function(control) {
      const checkboxes = Array.from(control.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
      const refresh = function() {
        const count = checkboxes.filter(function(input) { return input.checked; }).length;
        (control.querySelector('summary strong') as HTMLElement).textContent = count ? count + ' selected' : 'Any';
      };
      (control.querySelector('[data-multi-select-all]') as HTMLButtonElement).addEventListener('click', function() {
        checkboxes.forEach(function(input) { input.checked = true; });
        refresh();
      });
      (control.querySelector('[data-multi-select-clear]') as HTMLButtonElement).addEventListener('click', function() {
        checkboxes.forEach(function(input) { input.checked = false; });
        refresh();
      });
      checkboxes.forEach(function(input) { input.addEventListener('change', refresh); });
      control.addEventListener('toggle', function() {
        if ((control as HTMLDetailsElement).open) {
          form.querySelectorAll<HTMLDetailsElement>('[data-multi-select][open]').forEach(function(other) {
            if (other !== control) {
              other.open = false;
            }
          });
        }
      });
    });
  }
  const button = document.querySelector<HTMLButtonElement>('[data-audit-load-more]');
  if (button) {
    button.addEventListener('click', async function() {
      const stream = document.querySelector<HTMLElement>('#audit-event-stream')!;
      const pageStatus = document.querySelector<HTMLElement>('[data-audit-page-status]')!;
      const pageError = document.querySelector<HTMLElement>('[data-audit-page-error]')!;
      const openEventIDs = new Set(Array.from(stream.querySelectorAll('.rw-event-details[open]')).map(function(details) {
        return (details.closest('.rw-audit-event') as HTMLElement | null)?.dataset.auditEventId;
      }));
      pageError.hidden = true;
      pageError.textContent = '';
      stream.setAttribute('aria-busy', 'true');
      button.disabled = true;
      button.classList.add('loading');
      try {
        const data = await api('/api/audit', auditQuery(auditCursor));
        const knownIDs = new Set(auditEvents.map(function(event) { return String(event.id); }));
        list(data, ['events', 'items']).forEach(function(event) {
          if (!knownIDs.has(String(event.id))) {
            knownIDs.add(String(event.id));
            auditEvents.push(event as AuditEventRecord);
          }
        });
        auditCursor = data.next_cursor || '';
        auditHasMore = Boolean(data.has_more);
        stream.innerHTML = sharedAuditStream(auditEvents);
        openEventIDs.forEach(function(id) {
          const event = Array.from(stream.querySelectorAll('.rw-audit-event[data-audit-event-id]')).find(function(item) {
            return (item as HTMLElement).dataset.auditEventId === id;
          });
          const details = event?.querySelector<HTMLDetailsElement>('.rw-event-details');
          if (details) details.open = true;
        });
        (document.querySelector('[data-audit-loaded-count]') as HTMLElement).textContent = formatNumber(auditEvents.length);
        pageStatus.textContent = formatNumber(auditEvents.length) + ' events loaded.';
        (document.querySelector('.rw-load-more') as HTMLElement).hidden = !auditHasMore;
        (document.querySelector('[data-audit-end]') as HTMLElement).hidden = auditHasMore;
      } catch (error: any) {
        pageError.textContent = error.message || 'Unable to load older audit events.';
        pageError.hidden = false;
        pageStatus.textContent = 'Older events were not loaded. The current timeline is unchanged.';
      } finally {
        stream.setAttribute('aria-busy', 'false');
        button.disabled = false;
        button.classList.remove('loading');
      }
    });
  }
}

/** Binds DOM behavior for artifact inspection. */
function bindArtifactInspection(): void {
  document.querySelectorAll<HTMLButtonElement>('[data-inspect-artifact]').forEach(function(button) {
    button.addEventListener('click', async function() {
      const id = button.dataset.inspectArtifact as string;
      activeArtifactRow = button.closest('tr');
      document.querySelectorAll<HTMLElement>('[data-artifact-row]').forEach(function(row) { row.classList.toggle('selected', row === activeArtifactRow); });
      const previous = button.textContent;
      button.disabled = true;
      button.classList.add('loading');
      try {
        const payload = await api('/api/artifacts/' + encodeURIComponent(id) + '/inspect', { preview_bytes: 65536 });
        const raw = payload.content || '';
        var formatted = raw;
        var formatError = false;
        if (payload.format === 'json') {
          try {
            formatted = JSON.stringify(JSON.parse(raw), null, 2);
          } catch (_) {
            formatted = raw;
            formatError = true;
          }
        }
        activeArtifactPreview = { ...payload, raw: raw, formatted: formatted, formatError: formatError, mode: payload.format === 'json' && !formatError ? 'formatted' : 'raw', wrap: false };
        renderArtifactInspector();
      } catch (error: any) {
        (document.querySelector('#artifact-inspector .content') as HTMLElement).innerHTML = '<div class="ui error message"><div class="header">Preview unavailable</div>'
          + '<p>' + esc(error.message || 'Unable to inspect this artifact.') + '</p></div>'
          + '<a class="ui button" href="/api/artifacts/' + encodeURIComponent(id) + '/content" download>Download original</a>';
      } finally {
        button.disabled = false;
        button.classList.remove('loading');
        button.textContent = previous;
      }
    });
  });
}

/** Renders artifact inspector. */
function renderArtifactInspector(): void {
  const preview = activeArtifactPreview;
  if (!preview) {
    return;
  }
  const shown = preview.mode === 'formatted' ? preview.formatted : preview.raw;
  const canFormat = preview.format === 'json' && preview.formatted !== preview.raw;
  const id = preview.artifact_id;
  const truncation = preview.truncated
    ? '<div class="ui warning message"><div class="header">Preview truncated</div><p>This page shows the first '
      + esc(formatBytes(preview.preview_byte_size)) + ' of ' + esc(formatBytes(preview.stored_byte_size || preview.byte_size))
      + '. Download the original file to inspect its complete contents.</p></div>'
    : '<p class="ui success message">The complete stored text fits within the safe preview limit.</p>';
  const modeButtons = canFormat
    ? '<button type="button" class="ui basic button" data-artifact-preview-mode="formatted"' + (preview.mode === 'formatted' ? ' disabled' : '') + '>Formatted JSON</button>'
      + '<button type="button" class="ui basic button" data-artifact-preview-mode="raw"' + (preview.mode === 'raw' ? ' disabled' : '') + '>Raw text</button>' : '';
  var formatNotice = '';
  if (preview.formatError && preview.truncated) {
    formatNotice = '<p class="ui info message">Formatted JSON is unavailable because this bounded prefix does not contain the complete document. Raw text is shown.</p>';
  } else if (preview.formatError) {
    formatNotice = '<p class="ui warning message">The stored content is not valid JSON. Raw text is shown; download the original file for independent inspection.</p>';
  }
  (document.querySelector('#artifact-inspector .content') as HTMLElement).innerHTML = truncation
    + formatNotice
    + '<dl class="rw-summary-strip"><div><dt>Original size</dt><dd>' + esc(formatBytes(preview.byte_size)) + '</dd></div>'
    + '<div><dt>Stored size</dt><dd>' + esc(formatBytes(preview.stored_byte_size)) + '</dd></div>'
    + '<div><dt>Bytes shown</dt><dd>' + esc(formatBytes(preview.preview_byte_size)) + '</dd></div>'
    + '<div><dt>Content type</dt><dd>' + esc(preview.content_type) + '</dd></div></dl>'
    + '<div class="artifact-inspector-toolbar">' + modeButtons
    + '<button type="button" class="ui basic button" data-toggle-artifact-wrap>' + (preview.wrap ? 'Disable line wrapping' : 'Wrap long lines') + '</button>'
    + '<button type="button" class="ui basic button" data-copy-artifact-preview>Copy displayed text</button>'
    + '<a class="ui primary button" href="/api/artifacts/' + encodeURIComponent(id) + '/content" download>Download original</a></div>'
    + '<pre class="rw-artifact-preview' + (preview.wrap ? ' rw-artifact-preview--wrap' : '') + '">' + esc(shown) + '</pre>'
    + '<p class="ui faded text" data-artifact-copy-status></p>';
  document.querySelectorAll<HTMLButtonElement>('[data-artifact-preview-mode]').forEach(function(button) {
    button.addEventListener('click', function() { activeArtifactPreview.mode = button.dataset.artifactPreviewMode; renderArtifactInspector(); });
  });
  document.querySelector('[data-toggle-artifact-wrap]')!.addEventListener('click', function() {
    activeArtifactPreview.wrap = !activeArtifactPreview.wrap;
    renderArtifactInspector();
  });
  document.querySelector('[data-copy-artifact-preview]')!.addEventListener('click', async function() {
    const status = document.querySelector('[data-artifact-copy-status]') as HTMLElement;
    try {
      await copyArtifactText(shown);
      status.textContent = 'Displayed preview copied to the clipboard.';
    } catch (_) {
      status.textContent = 'Copy failed. Select the displayed text manually.';
    }
  });
}

/** Asynchronously copies artifact text. */
async function copyArtifactText(text: string): Promise<void> {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    return navigator.clipboard.writeText(text);
  }
  const area = document.createElement('textarea');
  area.value = text;
  area.setAttribute('readonly', '');
  area.style.position = 'fixed';
  area.style.opacity = '0';
  document.body.append(area);
  area.select();
  const copied = document.execCommand('copy');
  area.remove();
  if (!copied) {
    throw new Error('Clipboard unavailable');
  }
}