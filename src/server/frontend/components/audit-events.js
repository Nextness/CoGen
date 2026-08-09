// Shared audit-event presentation for Provenance and immutable record details.
import { esc, formatTime, humanLabel, link, parseObject, statusChip } from '../state.js';

const recordAuditBatchSize = 25;

/** Classifies an audit event into its presentation category. */
export function auditCategory(event) {
  const action = String(event.action || '');
  if (action.startsWith('review_') || action.startsWith('work_review_') || action.startsWith('note_') || action.startsWith('anchor_')) {
    return 'review';
  }
  if (action.startsWith('pdf_')) {
    return 'pdf';
  }
  if (action === 'field_enriched' || action.startsWith('cache_') || action === 'network_fetch') {
    return 'enrichment';
  }
  if (action.startsWith('validation_')) {
    return 'validation';
  }
  return 'pipeline';
}

/** Parses an audit event's stored metadata object. */
function eventMetadata(event) {
  return parseObject(event.metadata_json);
}

/** Derives the display outcome from recorded metadata and action semantics. */
function auditOutcome(event, metadata, after) {
  const recorded = metadata.outcome || metadata.status || metadata.cache_outcome || after.status;
  if (recorded) {
    return String(recorded);
  }
  const action = String(event.action || '').toLocaleLowerCase();
  if (action.includes('failed') || action.includes('error')) {
    return 'failed';
  }
  if (action.includes('discarded') || action.includes('trashed')) {
    return 'warning';
  }
  if (action.includes('skipped')) {
    return 'skipped';
  }
  return 'recorded';
}

/** Returns a context-preserving link or label for the affected audit entity. */
function auditEntity(event) {
  const type = String(event.entity_type || 'record');
  const id = event.entity_id;
  var href = '';
  if (id && type === 'work_revision') {
    href = link({ view: 'article', article_id: id });
  } else if (id && type === 'author_occurrence') {
    href = link({ view: 'author', author_id: id });
  } else if (id && type === 'reference_mention') {
    href = link({ view: 'reference', reference_id: id });
  } else if (type === 'pipeline_run') {
    href = link({ view: 'provenance', section: 'run' });
  }
  const label = humanLabel(type) + (id ? ' #' + id : '');
  if (href) {
    return '<a href="' + href + '">' + esc(label) + '</a>';
  }
  return '<span>' + esc(label) + '</span>';
}

/** Returns a concise human-readable summary of an audit event. */
function eventSummary(event, metadata) {
  const action = String(event.action || '').toLocaleLowerCase();
  if (metadata.field && metadata.provider) {
    return humanLabel(metadata.field) + ' enriched by ' + metadata.provider + '.';
  }
  if (metadata.reasons) {
    const reasons = Array.isArray(metadata.reasons) ? metadata.reasons : [metadata.reasons];
    return reasons.join('; ');
  }
  if (metadata.error) {
    return String(metadata.error);
  }
  if (metadata.reason) {
    return String(metadata.reason);
  }
  if (metadata.search_id) {
    return 'Search ' + metadata.search_id + (metadata.revision ? ', revision ' + metadata.revision + '.' : '.');
  }
  if (action.startsWith('pdf_inventory')) {
    return 'The PDF inventory state was recorded for this work.';
  }
  if (action.startsWith('pdf_document')) {
    return 'A validated PDF document was recorded in the companion store.';
  }
  if (action.startsWith('pipeline_')) {
    return 'The selected pipeline run changed lifecycle state.';
  }
  if (action.startsWith('cache_')) {
    return 'A provider cache decision was recorded with its request evidence.';
  }
  if (action.startsWith('review_') || action.startsWith('work_review_')) {
    return 'An immutable local review version was recorded.';
  }
  return 'Recorded append-only audit event.';
}

/** Returns metadata fields not already represented in the primary event presentation. */
function additionalMetadata(metadata) {
  const displayed = new Set([
    'stage', 'stage_name', 'outcome', 'status', 'provider', 'field', 'reasons',
    'error', 'reason', 'search_id', 'revision', 'duration_seconds', 'duration',
    'input_artifact_id', 'output_artifact_id',
    'note_body', 'body', 'selected_text', 'reviewer_email', 'email'
  ]);
  return safeAuditPayload(Object.fromEntries(Object.entries(metadata).filter(function(entry) {
    return !displayed.has(entry[0]);
  })));
}

/** Removes review prose and reviewer contact fields from generic audit payload inspection. */
function safeAuditPayload(raw) {
  const privateKeys = new Set(['note_body', 'body', 'selected_text', 'reviewer_email', 'email']);
  if (Array.isArray(raw)) return raw.map(safeAuditPayload);
  if (!raw || typeof raw !== 'object') return raw;
  return Object.fromEntries(Object.entries(raw).filter(function(entry) {
    return !privateKeys.has(String(entry[0]).toLocaleLowerCase());
  }).map(function(entry) {
    return [entry[0], safeAuditPayload(entry[1])];
  }));
}

/** Returns expandable facts and JSON payloads for an audit event. */
function eventDetails(event, metadata, before, after) {
  const facts = [
    ['Event ID', event.id],
    ['Correlation ID', event.correlation_id],
    ['Duration', metadata.duration_seconds != null ? metadata.duration_seconds + ' seconds' : metadata.duration],
    ['Input artifact', metadata.input_artifact_id],
    ['Output artifact', metadata.output_artifact_id]
  ].filter(function(entry) { return entry[1] !== null && entry[1] !== undefined && entry[1] !== ''; });
  const payloads = [
    ['Metadata', additionalMetadata(metadata)],
    ['Before', safeAuditPayload(before)],
    ['After', safeAuditPayload(after)]
  ].filter(function(entry) { return Object.keys(entry[1]).length > 0; });
  if (!facts.length && !payloads.length) {
    return '';
  }
  const factMarkup = facts.length ? '<dl class="rw-event-facts">' + facts.map(function(entry) {
    var shown = esc(entry[1]);
    if ((entry[0] === 'Input artifact' || entry[0] === 'Output artifact') && entry[1]) {
      shown = '<a href="' + link({ section: 'artifacts' }) + '">Artifact ' + esc(entry[1]) + '</a>';
    }
    return '<div><dt>' + esc(entry[0]) + '</dt><dd>' + shown + '</dd></div>';
  }).join('') + '</dl>' : '';
  const payloadMarkup = payloads.map(function(entry) {
    return '<div><h5>' + esc(entry[0]) + '</h5><pre>' + esc(JSON.stringify(entry[1], null, 2)) + '</pre></div>';
  }).join('');
  return '<details class="rw-event-details"><summary>Recorded data</summary>'
    + '<div class="rw-event-details__body">' + factMarkup + payloadMarkup + '</div></details>';
}

/** Returns the complete escaped markup for one audit event. */
export function auditEventMarkup(event) {
  const metadata = eventMetadata(event);
  const before = parseObject(event.before_json);
  const after = parseObject(event.after_json);
  const category = auditCategory(event);
  const outcome = auditOutcome(event, metadata, after);
  const timestamp = event.occurred_at || event.created_at;
  const source = event.actor || metadata.provider || 'Not recorded';
  const stage = metadata.stage || metadata.stage_name;
  const eventID = String(event.id || 'unrecorded');
  const runContext = event.pipeline_run_id ? 'Run ' + event.pipeline_run_id : (category === 'pdf' ? 'Global PDF evidence' : 'Run not recorded');
  return '<article class="rw-audit-event rw-audit-event--' + esc(category) + '" data-audit-event-id="' + esc(eventID) + '">'
    + '<time datetime="' + esc(timestamp || '') + '"><span>' + esc(formatTime(timestamp)) + '</span></time>'
    + '<div class="rw-audit-event__main">'
    + '<div class="rw-audit-event__heading"><h5>' + esc(humanLabel(event.action || 'event')) + '</h5>'
    + '<span class="ui label">' + esc(humanLabel(category)) + '</span>' + statusChip(outcome) + '</div>'
    + '<p>' + esc(eventSummary(event, metadata)) + '</p>'
    + '<div class="rw-audit-event__context"><span>Source: <strong>' + esc(source) + '</strong></span>'
    + '<span>Scope: <strong>' + esc(runContext) + '</strong></span>'
    + (stage ? '<span>Stage: <strong>' + esc(stage) + '</strong></span>' : '') + '</div>'
    + eventDetails(event, metadata, before, after) + '</div>'
    + '<div class="rw-audit-event__entity"><small>Affected record</small>' + auditEntity(event) + '</div>'
    + '</article>';
}

/** Groups audit events by local date and returns timeline markup. */
export function auditStream(events, emptyMessage) {
  if (!events.length) {
    return '<div class="rw-empty-inline"><strong>No audit events match these filters.</strong>'
      + '<p>' + esc(emptyMessage || 'Broaden the filters or reset them. The selected run context has not changed.') + '</p></div>';
  }
  const groups = new Map();
  events.forEach(function(event) {
    const timestamp = event.occurred_at || event.created_at;
    const parsed = timestamp ? new Date(timestamp) : null;
    const key = parsed && !Number.isNaN(parsed.getTime()) ? parsed.toLocaleDateString() : 'Date not recorded';
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key).push(event);
  });
  return Array.from(groups.entries()).map(function(entry, index) {
    const headingID = 'audit-day-' + index;
    return '<section class="rw-audit-day" aria-labelledby="' + headingID + '"><h4 id="' + headingID + '">' + esc(entry[0]) + '</h4>'
      + '<ol class="rw-audit-events">' + entry[1].map(function(event) { return '<li>' + auditEventMarkup(event) + '</li>'; }).join('') + '</ol></section>';
  }).join('');
}

/** Records audit investigation. */
export function recordAuditInvestigation(events) {
  const actions = Array.from(new Set(events.map(function(event) { return String(event.action || 'event'); }))).sort();
  const initialEvents = events.slice(0, recordAuditBatchSize);
  const actionOptions = ['<option value="">All event types</option>'].concat(actions.map(function(action) {
    return '<option value="' + esc(action) + '">' + esc(humanLabel(action)) + '</option>';
  })).join('');
  return '<div class="rw-record-audit" data-record-audit>'
    + '<div class="rw-record-audit__controls"><label>Search events<input type="search" data-record-audit-search placeholder="Action, source, identifier, or metadata"></label>'
    + '<label>Category<select data-record-audit-category><option value="">All categories</option><option value="pipeline">Pipeline</option><option value="enrichment">Enrichment</option><option value="validation">Validation</option><option value="pdf">PDF</option></select></label>'
    + '<label>Event type<select data-record-audit-action>' + actionOptions + '</select></label></div>'
    + '<div class="rw-record-audit__count"><strong data-record-audit-count>' + initialEvents.length.toLocaleString() + '</strong> of '
    + '<span data-record-audit-matches>' + events.length.toLocaleString() + '</span> events shown</div>'
    + '<div data-record-audit-stream>' + auditStream(initialEvents, 'No recorded events match the local detail filters.') + '</div>'
    + '<div class="rw-record-audit__actions"><button type="button" class="ui button" data-record-audit-more'
    + (initialEvents.length >= events.length ? ' hidden' : '') + '>Load ' + recordAuditBatchSize + ' more events</button></div></div>';
}

/** Binds DOM behavior for record audit investigation. */
export function bindRecordAuditInvestigation(events) {
  document.querySelectorAll('[data-record-audit]').forEach(function(root) {
    const search = root.querySelector('[data-record-audit-search]');
    const category = root.querySelector('[data-record-audit-category]');
    const action = root.querySelector('[data-record-audit-action]');
    const stream = root.querySelector('[data-record-audit-stream]');
    const count = root.querySelector('[data-record-audit-count]');
    const matches = root.querySelector('[data-record-audit-matches]');
    const more = root.querySelector('[data-record-audit-more]');
    var visibleLimit = recordAuditBatchSize;
    /** Applies the associated state. */
    function apply() {
      const needle = search.value.trim().toLocaleLowerCase();
      const matching = events.filter(function(event) {
        if (category.value && auditCategory(event) !== category.value) {
          return false;
        }
        if (action.value && String(event.action || '') !== action.value) {
          return false;
        }
        if (!needle) {
          return true;
        }
        return JSON.stringify(event).toLocaleLowerCase().includes(needle);
      });
      const visible = matching.slice(0, visibleLimit);
      count.textContent = visible.length.toLocaleString();
      matches.textContent = matching.length.toLocaleString();
      more.hidden = visible.length >= matching.length;
      stream.innerHTML = auditStream(visible, 'No recorded events match the local detail filters.');
    }
    /** Resets and apply. */
    function resetAndApply() {
      visibleLimit = recordAuditBatchSize;
      apply();
    }
    search.addEventListener('input', resetAndApply);
    category.addEventListener('change', resetAndApply);
    action.addEventListener('change', resetAndApply);
    more.addEventListener('click', function() {
      visibleLimit += recordAuditBatchSize;
      apply();
    });
  });
}
