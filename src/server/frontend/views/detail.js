// Immutable article, author-occurrence, and reference-mention detail views.
import {
  app, esc, value, link, pageHeader, emptyState, panel, cell, list,
  breadcrumb, statusChip, formatTime, formatBytes, parseObject, humanLabel, bindCopyButtons
} from '../state.js';
import { api } from '../api.js';
import { pagination } from '../components/pagination.js';
import { bindRecordAuditInvestigation, recordAuditInvestigation } from '../components/audit-events.js';

const collectionState = new Map();

/** Returns a context-preserving link to a related detail record. */
function detailLink(kind, id) {
  const updates = { view: kind, article_id: '', author_id: '', reference_id: '' };
  updates[kind + '_id'] = id;
  const currentKind = value('view');
  const currentID = value(currentKind + '_id');
  if (['article', 'author', 'reference'].includes(currentKind) && currentID) {
    updates.return_view = currentKind;
    updates.return_id = currentID;
  }
  return link(updates);
}

/** Returns the context-preserving corpus return URL for a detail view. */
function backToCorpus(kind) {
  var section = 'articles';
  if (kind === 'author') {
    section = 'authors';
  } else if (kind === 'reference') {
    section = 'references';
  }
  return link({
    view: 'corpus', section: section, article_id: '', author_id: '', reference_id: '', table: ''
  });
}

/** Returns detail-view navigation markup back to the originating corpus section. */
function detailReturn(kind) {
  const returnKind = value('return_view');
  const returnID = value('return_id');
  if (['article', 'author', 'reference'].includes(returnKind) && returnID) {
    const updates = {
      view: returnKind, article_id: '', author_id: '', reference_id: '', return_view: '', return_id: ''
    };
    updates[returnKind + '_id'] = returnID;
    const labels = { article: 'Article revision', author: 'Author occurrence', reference: 'Reference mention' };
    return { href: link(updates), label: labels[returnKind] };
  }
  return { href: backToCorpus(kind), label: 'Corpus' };
}

/** Records ed. */
function recorded(raw, fallback) {
  if (raw === null || raw === undefined || raw === '') {
    return fallback || '<span class="ui faded text">Not recorded</span>';
  }
  return esc(raw);
}

/** Returns definition-list markup for labeled record properties. */
function propertyGrid(entries, classes) {
  const items = entries.map(function(entry) {
    const content = entry.html ? entry.value : recorded(entry.value);
    return '<div><dt>' + esc(entry.label) + '</dt><dd>' + content + '</dd></div>';
  }).join('');
  return '<dl class="property-grid ' + esc(classes || '') + '">' + items + '</dl>';
}

/** Returns compact summary-fact markup for a detail record. */
function summaryStrip(entries) {
  return '<dl class="rw-record-summary">' + entries.map(function(entry) {
    const content = entry.html ? entry.value : recorded(entry.value);
    return '<div><dt>' + esc(entry.label) + '</dt><dd>' + content + '</dd></div>';
  }).join('') + '</dl>';
}

/** Converts a stored mapping representation to a displayable object. */
function mappingValue(raw) {
  if (raw === null || raw === undefined || raw === '') {
    return '<span class="ui faded text">Not recorded</span>';
  }
  if (Array.isArray(raw)) {
    return '<ul class="rw-mapping-values">' + raw.map(function(item) {
      return '<li>' + mappingValue(item) + '</li>';
    }).join('') + '</ul>';
  }
  if (raw && typeof raw === 'object') {
    return '<dl class="rw-mapping-list">' + Object.entries(raw).map(function(entry) {
      return '<div><dt>' + esc(humanLabel(entry[0])) + '</dt><dd>' + mappingValue(entry[1]) + '</dd></div>';
    }).join('') + '</dl>';
  }
  return '<span class="rw-mono">' + esc(raw) + '</span>';
}

/** Returns the parsed extension mapping stored on a work revision. */
function extensionMapping(raw) {
  const parsed = parseObject(raw);
  if (!Object.keys(parsed).length) {
    return recorded(raw);
  }
  return mappingValue(parsed);
}

/** Returns normalized keyword values from stored array or delimited input. */
function keywordValues(raw) {
  if (Array.isArray(raw)) {
    return raw.map(String).filter(Boolean);
  }
  if (raw === null || raw === undefined || raw === '') {
    return [];
  }
  if (typeof raw === 'string') {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return parsed.map(String).filter(Boolean);
      }
    } catch (_) {
      // Stored legacy values may be delimited text rather than JSON.
    }
    const separator = raw.includes(';') ? ';' : ',';
    return raw.split(separator).map(function(item) { return item.trim(); }).filter(Boolean);
  }
  return [];
}

/** Returns label markup for normalized keyword values. */
function keywordMarkup(raw) {
  const keywords = keywordValues(raw);
  if (!keywords.length) {
    return '<span class="ui faded text">Not recorded</span>';
  }
  const rawText = typeof raw === 'string' ? raw : JSON.stringify(raw);
  return '<div class="rw-keywords"><div class="rw-keyword-tags">' + keywords.map(function(keyword) {
    return '<span class="ui label">' + esc(keyword) + '</span>';
  }).join('') + '</div><details class="rw-keywords__raw"><summary>Show stored keyword value</summary>'
    + '<div><code>' + esc(rawText) + '</code><button type="button" class="ui basic button" data-copy-text="' + esc(rawText) + '">Copy</button></div></details></div>';
}

/** Returns expandable JSON markup for a raw record. */
function rawRecord(record, excluded) {
  const rows = Object.entries(record).filter(function([key]) {
    return !excluded.includes(key);
  });
  if (!rows.length) {
    return '';
  }
  return '<details class="rw-disclosure"><summary>Advanced raw record data</summary>'
    + '<div class="rw-disclosure__content">'
    + propertyGrid(rows.map(function([key, item]) {
      if (key === 'extension_data') {
        return { label: 'Extension data', value: extensionMapping(item), html: true };
      }
      return { label: humanLabel(key), value: cell(item, key), html: true };
    }), 'property-grid--compact')
    + '</div></details>';
}

/** Returns expandable markup for a related-record collection. */
function collectionMarkup(key, title, description, columns, rows, page) {
  const pageSize = 25;
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));
  const current = Math.min(Math.max(1, page), totalPages);
  const visible = rows.slice((current - 1) * pageSize, current * pageSize);
  var body;
  if (visible.length) {
    body = visible.map(function(row) {
      return '<tr>' + columns.map(function(column) {
        return '<td>' + column.render(row) + '</td>';
      }).join('') + '</tr>';
    }).join('');
  } else {
    body = '<tr><td colspan="' + Math.max(1, columns.length) + '" class="empty">No records.</td></tr>';
  }
  const table = '<div class="table-wrap" aria-label="' + esc(title) + ' table">'
    + '<table class="ui table"><thead><tr>' + columns.map(function(column) {
      return '<th scope="col">' + esc(column.label) + '</th>';
    }).join('') + '</tr></thead><tbody>' + body + '</tbody></table></div>';
  const pages = rows.length > pageSize ? pagination({
    page: current, per_page: pageSize, total_rows: rows.length, total_pages: totalPages
  }, {
    itemLabel: title.toLocaleLowerCase(), pageAttribute: 'data-detail-page', pageClass: ' detail-page', visibleCount: 5
  }).replaceAll('data-detail-page="', 'data-detail-page="' + esc(key) + ':') : '';
  return '<section class="ui segment rw-detail-collection" data-detail-collection="' + esc(key) + '">'
    + '<div class="ui top attached header"><div><h3>' + esc(title) + '</h3><p>' + esc(description) + '</p></div>'
    + '<span class="ui label">' + rows.length.toLocaleString() + '</span></div>'
    + '<div class="content">' + table + pages + '</div></section>';
}

/** Mounts collection. */
function mountCollection(key, title, description, columns, rows) {
  collectionState.set(key, { title: title, description: description, columns: columns, rows: rows, page: 1 });
  renderCollection(key);
}

/** Renders collection. */
function renderCollection(key) {
  const state = collectionState.get(key);
  const container = document.querySelector('[data-detail-collection-host="' + key + '"]');
  if (!state || !container) {
    return;
  }
  container.innerHTML = collectionMarkup(key, state.title, state.description, state.columns, state.rows, state.page);
  container.querySelectorAll('[data-detail-page]').forEach(function(button) {
    button.addEventListener('click', function() {
      const parts = button.dataset.detailPage.split(':');
      state.page = Number(parts[1]) || 1;
      renderCollection(key);
      container.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    });
  });
}

/** Returns escaped validation or failure reason markup for a stage outcome. */
function stageReasonMarkup(raw) {
  if (typeof raw === 'string') {
    try {
      const reasons = JSON.parse(raw);
      if (Array.isArray(reasons) && reasons.length) {
        return '<ul class="rw-mapping-values">' + reasons.map(function(reason) {
          return '<li>' + esc(reason) + '</li>';
        }).join('') + '</ul>';
      }
    } catch (_) {
      // Validation reasons created before normalization may be plain text.
    }
  }
  return recorded(raw);
}

/** Returns the article detail view from its immutable revision payload. */
function articleView(record, data) {
  const extension = parseObject(record.extension_data);
  const validation = extension.validation_status || record.validation_status || 'Not recorded';
  const audits = list(data.audit_events, ['events', 'items']);
  const enriched = list(data.enriched_fields, ['rows', 'items']);
  const pdf = data.pdf_status || { status: 'not_available' };
  const providers = new Set();
  const fields = new Set();
  enriched.forEach(function(item) {
    const metadata = parseObject(item.metadata_json);
    if (metadata.provider) {
      providers.add(metadata.provider);
    }
    if (metadata.field) {
      fields.add(metadata.field);
    }
  });

  const summary = summaryStrip([
    { label: 'DOI', value: record.doi },
    { label: 'Year', value: record.year },
    { label: 'Journal', value: record.journal },
    { label: 'Publisher', value: record.publisher },
    { label: 'Source', value: record.source },
    { label: 'Validation', value: statusChip(validation), html: true },
  ]);

  const provenance = propertyGrid([
    { label: 'Run attempt', value: record.pipeline_run_id },
    { label: 'Work', value: record.work_id },
    { label: 'Revision', value: record.id },
    { label: 'Produced by stage', value: record.producer_stage },
    { label: 'Captured', value: formatTime(record.created_at) },
    { label: 'Enrichment providers', value: Array.from(providers).join(', ') },
    { label: 'Enriched fields', value: Array.from(fields).join(', ') },
    { label: 'Payload hash', value: record.payload_hash },
  ]);

  var abstract = '<p class="ui faded text">No abstract was recorded for this revision.</p>';
  if (record.abstract) {
    abstract = '<p class="rw-abstract-text">' + esc(String(record.abstract).replace(/\s+/g, ' ').trim()) + '</p>';
  }
  const bibliography = panel('Bibliographic metadata', 'Human-readable metadata for this immutable revision.',
    abstract + propertyGrid([
      { label: 'Keywords', value: keywordMarkup(record.keywords), html: true },
      { label: 'Keywords plus', value: keywordMarkup(record.keywords_plus), html: true },
      { label: 'Citation count', value: record.citation_count },
      { label: 'Reference count', value: record.reference_count },
    ], 'property-grid--compact'), 'rw-detail-section');

  const pdfPanel = pdfStatusPanel(record, pdf);

  return summary
    + pdfPanel
    + panel('Provenance summary', 'Where this revision came from and how it was captured.', provenance, 'rw-detail-section')
    + bibliography
    + '<div data-detail-collection-host="article-authors"></div>'
    + '<div data-detail-collection-host="article-references"></div>'
    + '<div data-detail-collection-host="article-stage-outcomes"></div>'
    + panel('Audit events', 'Append-only persisted audit records for this work in the selected run.', recordAuditInvestigation(audits), 'rw-detail-section')
    + rawRecord(record, ['title', 'doi', 'year', 'journal', 'publisher', 'source', 'abstract', 'keywords', 'keywords_plus', 'citation_count', 'reference_count'])
    + '<span data-mount-detail-collections hidden></span>';
}

/** Returns PDF inventory and download-status markup for an article. */
export function pdfStatusPanel(record, pdf) {
  const pdfLabels = {
    available: 'Available',
    unavailable: 'Unavailable without a DOI',
    not_available: 'Not Available'
  };
  var pdfAction = '<span class="ui faded text">No stored PDF is available.</span>';
  if (pdf.status === 'available') {
    pdfAction = '<a class="ui primary button" href="/api/pdf/' + encodeURIComponent(record.work_id)
      + '" target="_blank" rel="noopener">Open PDF</a>';
  }
  return panel('Full-text PDF', 'Read-only state from the companion PDF store.',
    propertyGrid([
      { label: 'Status', value: statusChip(pdfLabels[pdf.status] || humanLabel(pdf.status)), html: true },
      { label: 'Size', value: pdf.byte_size ? formatBytes(pdf.byte_size) : null },
      { label: 'Inventoried at', value: pdf.inventoried_at ? formatTime(pdf.inventoried_at) : null },
      { label: 'Content hash', value: pdf.content_hash },
    ], 'property-grid--compact') + '<div class="rw-filter-actions">' + pdfAction + '</div>', 'rw-detail-section');
}

/** Returns the author occurrence detail view with related articles and audit evidence. */
function authorView(record, data) {
  const articles = list(data.articles, ['rows', 'items']);
  const audits = list(data.audit_events, ['events', 'items']);
  const identity = record.person_id ? 'Linked global person' : 'Observed occurrence only';
  return '<div class="identity-notice">This is one observed author occurrence in a historical run. Name similarity alone does not establish a global person identity.</div>'
    + summaryStrip([
      { label: 'Citation name', value: record.citation_name },
      { label: 'ORCID observed', value: record.orcid },
      { label: 'Identity status', value: statusChip(identity), html: true },
      { label: 'Articles in response', value: articles.length },
    ])
    + panel('Observed identity data', 'The exact name and identity values stored for this occurrence.', propertyGrid([
      { label: 'First name', value: record.first_name },
      { label: 'Last name', value: record.last_name },
      { label: 'Observed ORCID', value: record.orcid },
      { label: 'Linked person', value: record.person_id },
      { label: 'Person ORCID', value: record.person_orcid },
      { label: 'Captured', value: formatTime(record.created_at) },
    ]), 'rw-detail-section')
    + '<div data-detail-collection-host="author-articles"></div>'
    + panel('Audit events', 'Filter and inspect append-only events directly associated with this author occurrence.', recordAuditInvestigation(audits), 'rw-detail-section')
    + rawRecord(record, ['citation_name', 'first_name', 'last_name', 'orcid', 'person_id', 'person_orcid'])
    + '<span data-mount-author-articles hidden></span>';
}

/** Returns the reference mention detail view with citation context. */
function referenceView(record) {
  const resolved = Boolean(record.resolved_revision_id);
  return summaryStrip([
    { label: 'Reference order', value: record.mention_order },
    { label: 'DOI', value: record.doi },
    { label: 'Year', value: record.year },
    { label: 'Resolution', value: statusChip(resolved ? 'Resolved internally' : 'Unresolved'), html: true },
  ])
    + '<div class="rw-record-comparison">'
    + panel('Citing article', 'The immutable article revision that contains this reference mention.', propertyGrid([
      { label: 'Article title', value: record.citing_title },
      { label: 'Article revision', value: '<a href="' + detailLink('article', record.work_revision_id) + '">' + recorded(record.work_revision_id) + '</a>', html: true },
      { label: 'Work', value: record.work_id },
      { label: 'Run attempt', value: record.pipeline_run_id },
    ]), 'rw-detail-section')
    + panel('Resolved target', 'A target is shown only when the stored relationship resolved inside this run.', resolved ? propertyGrid([
      { label: 'Target title', value: record.resolved_title },
      { label: 'Target revision', value: '<a href="' + detailLink('article', record.resolved_revision_id) + '">' + recorded(record.resolved_revision_id) + '</a>', html: true },
      { label: 'Resolution', value: statusChip('Resolved internally'), html: true },
    ]) : '<p class="ui faded text">This reference mention was not resolved to a work revision in the selected run.</p>', 'rw-detail-section')
    + '</div>'
    + panel('Cited-reference metadata', 'Raw bibliographic values captured for this mention.', propertyGrid([
      { label: 'Referenced title', value: record.title },
      { label: 'Referenced author', value: record.author },
      { label: 'Source', value: record.source },
      { label: 'Captured', value: formatTime(record.created_at) },
    ]), 'rw-detail-section')
    + rawRecord(record, ['title', 'author', 'doi', 'year', 'source', 'mention_order', 'citing_title', 'resolved_title']);
}

/** Asynchronously implements detail view for the viewer. */
export async function detailView(kind) {
  const labels = { article: 'Article revision', author: 'Author occurrence', reference: 'Reference mention' };
  var idKey = kind + '_id';
  const id = value(idKey);
  if (!id) {
    app.innerHTML = emptyState(labels[kind], 'Open a record from Corpus, Relationships, or Advanced database inspection.');
    return;
  }

  collectionState.clear();
  const data = await api('/api/' + kind + 's/' + encodeURIComponent(id));
  const record = data.article || data.author || data.reference || data;
  const parent = detailReturn(kind);
  const backHref = parent.href;
  var title = labels[kind];
  if (kind === 'article') {
    title = record.title || labels[kind];
  } else if (kind === 'author') {
    title = record.citation_name || labels[kind];
  } else {
    title = record.title || record.doi || labels[kind];
  }

  var body;
  if (kind === 'article') {
    body = articleView(record, data);
  } else if (kind === 'author') {
    body = authorView(record, data);
  } else {
    body = referenceView(record);
  }

  app.innerHTML = breadcrumb({ parentLabel: parent.label, parentHref: backHref, current: labels[kind] })
    + pageHeader(labels[kind], title, 'Immutable evidence captured in the selected historical run.',
      '<a class="ui basic button" href="' + backHref + '">Back to ' + esc(parent.label) + '</a>')
    + '<article class="rw-record-detail">' + body + '</article>';

  if (kind === 'article') {
    mountCollection('article-authors', 'Ordered authors', 'Observed author occurrences in bibliographic order.', [
      { label: 'Order', render: function(row) { return recorded(row.author_order); } },
      { label: 'Author occurrence', render: function(row) { return '<a href="' + detailLink('author', row.id) + '">' + recorded(row.citation_name) + '</a>'; } },
      { label: 'ORCID', render: function(row) { return recorded(row.orcid); } },
      { label: 'Affiliation', render: function(row) { return recorded(row.affiliation); } },
      { label: 'Identity evidence', render: function(row) { return row.person_id ? statusChip('Linked global person') : statusChip('Observed occurrence only'); } },
    ], list(data.authors, ['rows', 'items']));
    mountCollection('article-references', 'Reference mentions', 'Ordered references captured for this article revision.', [
      { label: 'Order', render: function(row) { return recorded(row.mention_order); } },
      { label: 'Reference mention', render: function(row) { return '<a href="' + detailLink('reference', row.id) + '">' + recorded(row.title || row.doi || ('Reference ' + row.id)) + '</a>'; } },
      { label: 'Author', render: function(row) { return recorded(row.author); } },
      { label: 'Year', render: function(row) { return recorded(row.year); } },
      { label: 'Resolution', render: function(row) { return row.resolved_revision_id ? '<a href="' + detailLink('article', row.resolved_revision_id) + '">' + statusChip('Resolved internally') + '</a>' : statusChip('Unresolved'); } },
    ], list(data.references, ['rows', 'items']));
    mountCollection('article-stage-outcomes', 'Pipeline stage history', 'Recorded per-work outcomes for the selected run. Audit events below are persisted append-only records.', [
      { label: 'Stage', render: function(row) { return recorded(humanLabel(row.stage_name)); } },
      { label: 'Outcome', render: function(row) { return statusChip(row.outcome || 'Not recorded'); } },
      { label: 'Reason', render: function(row) { return stageReasonMarkup(row.reason); } },
      { label: 'First recorded', render: function(row) { return recorded(formatTime(row.created_at)); } },
      { label: 'Last updated', render: function(row) { return recorded(formatTime(row.updated_at)); } },
    ], list(data.stage_outcomes, ['rows', 'items']));
  }
  if (kind === 'author') {
    mountCollection('author-articles', 'Linked article revisions', 'Articles that contain this observed author occurrence.', [
      { label: 'Article revision', render: function(row) { return '<a href="' + detailLink('article', row.work_revision_id) + '">' + recorded(row.title) + '</a>'; } },
      { label: 'Year', render: function(row) { return recorded(row.year); } },
      { label: 'DOI', render: function(row) { return recorded(row.doi); } },
      { label: 'Author order', render: function(row) { return recorded(row.author_order); } },
      { label: 'Affiliation', render: function(row) { return recorded(row.affiliation); } },
    ], list(data.articles, ['rows', 'items']));
  }
  bindRecordAuditInvestigation(list(data.audit_events, ['events', 'items']));
  bindCopyButtons();
}
