// Shared state, DOM references, and utility functions.
// This module is imported by every other module. It avoids circular dependencies
// by not importing any project module.

export const app = document.querySelector('#app');
export const notice = document.querySelector('#notice');
export const loading = document.querySelector('#loading');

export const state = {
  searches: [],
  plans: [],
  runs: [],
  tables: [],
  request: 0,
  controller: null
};

export const pageSizes = [20, 50, 100, 200, 500];
export const corpusSections = {
  articles: {
    table: 'work_revisions',
    title: 'Analysis-ready articles',
    description: 'Valid normalized work revisions captured by the selected run.'
  },
  authors: {
    table: 'author_occurrences',
    title: 'Author occurrences',
    description: 'Observed author records. Matching names do not imply the same person.'
  },
  references: {
    table: 'reference_mentions',
    title: 'Reference mentions',
    description: 'Ordered citation mentions retained for each article revision.'
  },
  identity_evidence: {
    table: 'author_identity_resolutions',
    title: 'Author identity / ORCID evidence',
    description: 'Name-search results are review candidates, never confirmed author identities.'
  },
  sources: {
    table: 'source_records',
    title: 'Source records',
    description: 'Captured input records and parse outcomes.'
  },
};
export const provenanceSections = {
  audit: ['Audit timeline', 'Recorded actions for the selected historical run.'],
  artifacts: ['Artifacts', 'Inspect and download locally stored artifacts for the selected run.'],
  cache: ['Cache uses', 'Provider response and computation reuse recorded for this run.'],
  stages: ['Stage outcomes', 'Per-work outcomes recorded for parse, deduplication, validation, enrichment, and normalization.'],
  run: ['Run details', 'The stored execution attempt and its read-only plan context.'],
};
export const graphFilters = ['mode', 'q', 'author', 'orcid', 'reference', 'source', 'year_min', 'year_max', 'citation_min', 'citation_max', 'reference_min', 'reference_max', 'article_limit'];

/** Returns the current URL search parameters. */
export function params() {
  return new URLSearchParams(location.search);
}

/** Returns a named URL parameter or an empty string. */
export function value(name) {
  return params().get(name) || '';
}

/** Returns the selected viewer view. */
export function view() {
  return value('view') || 'overview';
}

/** Returns a named section parameter or its fallback. */
export function section(name, fallback) {
  return value(name) || fallback;
}

/** Escapes a value for safe HTML text insertion. */
export function esc(raw) {
  const element = document.createElement('span');
  element.textContent = raw == null ? '' : String(raw);
  return element.innerHTML;
}

/** Formats a value for JSON-oriented display. */
export function asJSON(item) {
  if (typeof item === 'string') {
    return item;
  }
  return JSON.stringify(item, null, 2);
}

/** Returns the first matching array in an API response. */
export function list(data, keys) {
  if (keys) {
    for (const key of keys) {
      if (Array.isArray(data?.[key])) {
        return data[key];
      }
    }
  }
  if (Array.isArray(data)) {
    return data;
  }
  return [];
}

/** Returns the first supported identifier present on an item. */
export function pickID(item) {
  if (item?.id) {
    return item.id;
  }
  if (item?.search_id) {
    return item.search_id;
  }
  if (item?.run_id) {
    return item.run_id;
  }
  if (item?.plan_id) {
    return item.plan_id;
  }
  return '';
}

/** Returns the first non-empty display field on an item. */
export function text(item, fields) {
  for (const field of fields) {
    if (item?.[field] !== undefined && item?.[field] !== null && item[field] !== '') {
      return String(item[field]);
    }
  }
  return 'Unnamed';
}

/** Converts a value to a finite number or zero. */
export function number(raw) {
  const parsed = Number(raw?.value ?? raw);
  if (Number.isFinite(parsed)) {
    return parsed;
  }
  return 0;
}

/** Formats number. */
export function formatNumber(raw) {
  return number(raw).toLocaleString();
}

/** Formats a count as a percentage of its denominator. */
export function percent(raw, denominator) {
  const count = number(raw);
  const base = number(denominator);
  if (base > 0) {
    return `${(count * 100 / base).toFixed(1)}%`;
  }
  return '—';
}

/** Formats time. */
export function formatTime(raw) {
  if (!raw) {
    return '—';
  }
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return String(raw);
  }
  return date.toLocaleString();
}

/** Formats bytes. */
export function formatBytes(raw) {
  const bytes = Math.max(0, number(raw));
  if (bytes < 1024) {
    return bytes.toLocaleString() + ' B';
  }
  const units = ['KB', 'MB', 'GB', 'TB'];
  var value = bytes;
  var unit = -1;
  do {
    value /= 1024;
    unit += 1;
  } while (value >= 1024 && unit < units.length - 1);
  return value.toLocaleString(undefined, { maximumFractionDigits: value >= 10 ? 1 : 2 }) + ' ' + units[unit];
}

/** Converts a machine-oriented identifier to a title-cased display label. */
export function humanLabel(raw) {
  return String(raw || '')
    .replace(/_/g, ' ')
    .replace(/\b\w/g, function(character) { return character.toUpperCase(); });
}

/** Parses object. */
export function parseObject(raw) {
  if (raw && typeof raw === 'object') {
    return raw;
  }
  if (!raw || typeof raw !== 'string') {
    return {};
  }
  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === 'object') {
      return parsed;
    }
  } catch (_) {
    return {};
  }
  return {};
}

/** Maps a recorded status to its semantic color class. */
export function statusClass(raw) {
  const status = String(raw || '').toLowerCase();
  if (status === 'not_available' || status === 'not available' || status === 'unavailable') {
    return 'red';
  }
  if (status === 'available') {
    return 'green';
  }
  if (/(fail|discard|error|trash|purge|reject|invalid)/.test(status)) {
    return 'red';
  }
  if (/(warning|skip|stale|negative|unresolved|unclear|disabled|incomplete|below|above|unmatched|no[_ -].*(candidate|match))/.test(status)) {
    return 'orange';
  }
  if (/(complete|valid|success|hit|ready|resolve|enrich|normaliz|linked|match)/.test(status)) {
    return 'green';
  }
  return 'blue';
}

/** Returns escaped label markup for a recorded status. */
export function statusChip(raw) {
  const text = esc(raw || 'Not recorded');
  const color = statusClass(raw);
  return `<span class="ui ${color} label">${text}</span>`;
}

/** Normalizes array- or object-backed metrics to display-name and value pairs. */
export function metricEntries(group) {
  if (Array.isArray(group)) {
    return group.map(function(item) {
      const suffix = item.source ? ` (${item.source})` : '';
      return [`${item.metric || 'Metric'}${suffix}`, item];
    });
  }
  return Object.entries(group || {});
}

/** Returns the pipeline run selected by the current URL context. */
export function selectedRun() {
  const runId = value('run_id');
  return state.runs.find(function(run) {
    return String(pickID(run)) === runId;
  });
}

/** Shows error. */
export function showError(error) {
  notice.textContent = error?.message || String(error);
  notice.hidden = false;
}

/** Clears error. */
export function clearError() {
  notice.hidden = true;
  notice.textContent = '';
}

/** Shows or hides the global loading indicator. */
export function busy(isBusy) {
  loading.hidden = !isBusy;
}

/** Builds an internal URL that preserves current research context and applies supplied updates. */
export function link(updates) {
  if (!updates) {
    updates = {};
  }
  const next = params();
  Object.entries(updates).forEach(function([key, raw]) {
    if (raw === '' || raw === null || raw === undefined) {
      next.delete(key);
    } else {
      next.set(key, String(raw));
    }
  });
  if (!next.get('view')) {
    next.set('view', 'overview');
  }
  return `?${next.toString()}`;
}

/** Returns the standard page header with escaped copy and optional actions. */
export function pageHeader(kicker, title, description, extra) {
  if (!extra) {
    extra = '';
  }
  const kickerHtml = kicker ? `<p class="rw-page-header__kicker">${esc(kicker)}</p>` : '';
  const actions = extra ? `<div class="rw-page-header__actions">${extra}</div>` : '';
  return `<header class="rw-page-header"><div class="rw-page-header__main">${kickerHtml}<h2 id="page-title">${esc(title)}</h2><p class="rw-page-header__description">${esc(description)}</p></div>${actions}</header>`;
}

/** Returns research-context breadcrumb markup for the current or supplied parent record. */
export function breadcrumb(options) {
  if (!options) {
    options = {};
  }
  const parts = [];
  const contextualParent = options.parentLabel && options.parentHref;
  if (!contextualParent && value('search_id')) {
    const search = state.searches.find(function(s) {
      return String(pickID(s)) === value('search_id');
    }) || {};
    parts.push(text(search, ['search_id', 'id']));
  }
  if (!contextualParent && value('search_revision_id')) {
    var revisionLabel = value('search_revision_id');
    for (const search of state.searches) {
      const revision = list(search, ['revisions', 'search_revisions']).find(function(item) {
        return String(pickID(item)) === value('search_revision_id');
      });
      if (revision) {
        revisionLabel = text(revision, ['revision_label', 'label', 'id']);
        break;
      }
    }
    parts.push(`Revision ${revisionLabel}`);
  }
  if (!contextualParent && value('plan_id')) {
    const plan = state.plans.find(function(item) {
      return String(pickID(item)) === value('plan_id');
    }) || {};
    const fingerprint = text(plan, ['execution_fingerprint']);
    if (fingerprint === 'Unnamed') {
      parts.push(`Plan ${value('plan_id')}`);
    } else {
      parts.push(`Plan ${fingerprint.slice(0, 12)}`);
    }
  }
  if (!contextualParent && value('run_id')) {
    parts.push(`Run ${value('run_id')}`);
  }
  var crumbItems = parts.map(function(p) {
    return `<span class="section">${esc(p)}</span>`;
  });
  if (contextualParent) {
    crumbItems.push('<a class="section" href="' + options.parentHref + '">' + esc(options.parentLabel) + '</a>');
  }
  if (options.current) {
    crumbItems.push('<span class="section current">' + esc(options.current) + '</span>');
  }
  if (crumbItems.length) {
    return `<nav class="ui breadcrumb" aria-label="Research context path">${crumbItems.join('<span class="divider">/</span>')}</nav>`;
  }
  return '';
}

/** Returns a complete empty-view state with the standard page header. */
export function emptyState(title, detail, action) {
  if (!action) {
    action = '';
  }
  return `${pageHeader('Read-only workspace', title, detail)}<section class="ui basic segment"><p>${esc(detail)}</p>${action}</section>`;
}

/** Returns compact empty-state panel markup. */
export function emptyPanel(title, detail, action) {
  if (!action) {
    action = '';
  }
  return '<section class="ui segment rw-empty-state"><h3>' + esc(title) + '</h3><p>' + esc(detail) + '</p>' + action + '</section>';
}

/** Returns the standard titled content-panel markup. */
export function panel(title, description, body, classes) {
  if (!classes) {
    classes = '';
  }
  var descHtml = '';
  if (description) {
    descHtml = `<p>${esc(description)}</p>`;
  }
  return `<section class="ui segment ${classes}"><div class="ui top attached header"><div><h3>${esc(title)}</h3>${descHtml}</div></div><div class="content">${body}</div></section>`;
}

/** Returns an escaped data table inside the standard panel wrapper. */
export function table(title, description, columns, rows, classes) {
  if (!classes) {
    classes = '';
  }
  const header = columns.map(function(column) {
    return `<th scope="col">${esc(column.label || column)}</th>`;
  }).join('');

  var body;
  if (rows.length) {
    body = rows.map(function(row) {
      const cells = columns.map(function(column) {
        var content;
        if (typeof column === 'string') {
          content = cell(row[column], column);
        } else {
          content = column.render(row);
        }
        return '<td>' + content + '</td>';
      }).join('');
      return '<tr>' + cells + '</tr>';
    }).join('');
  } else {
    body = `<tr><td colspan="${Math.max(columns.length, 1)}" class="empty">No records.</td></tr>`;
  }

  return panel(title, description, `<div class="table-wrap" aria-label="${esc(title)} table"><table class="ui table"><thead><tr>${header}</tr></thead><tbody>${body}</tbody></table></div>`, classes);
}

/** Returns context-preserving tab navigation for a keyed section. */
export function subnav(items, current, key) {
  const links = items.map(function([id, label]) {
    const href = link({ [key]: id });
    const aria = id === current ? ' aria-current="page"' : '';
    const active = id === current ? ' active' : '';
    return `<a href="${href}" class="item${active}"${aria}>${esc(label)}</a>`;
  }).join('');
  return `<nav class="ui tabular menu" aria-label="Section navigation">${links}</nav>`;
}

/** Filters chips. */
export function filterChips(filters, labels, options) {
  if (!options) {
    options = {};
  }
  const entries = Object.entries(filters || {}).filter(function([, raw]) {
    return raw !== '' && raw !== null && raw !== undefined;
  });
  if (!entries.length) {
    return '<div class="rw-filter-summary"><span class="ui faded text">No filters applied.</span></div>';
  }
  const chips = entries.map(function([key, raw]) {
    const updates = { ...(options.removeUpdates || { page: 1 }), [key]: '' };
    const href = link(updates);
    return '<a class="rw-filter-chip" href="' + href + '" title="Remove filter">'
      + '<span>' + esc(labels?.[key] || humanLabel(key)) + ':</span> ' + esc(raw) + ' <b aria-hidden="true">\u00D7</b></a>';
  }).join('');
  var clear = '';
  if (options.clearUpdates) {
    clear = '<a class="rw-filter-clear" href="' + link(options.clearUpdates) + '">Clear all</a>';
  }
  return '<div class="rw-filter-summary"><strong>Applied filters</strong><div class="rw-filter-chips">' + chips + clear + '</div></div>';
}

/** Returns a metric card with availability, denominator, and optional navigation. */
export function metricCard(name, metric, href) {
  if (!href) {
    href = '';
  }
  const unavailable = metric?.available === false;

  var content;
  if (unavailable) {
    content = '<span class="label">' + esc(name) + '</span>'
      + '<span class="value">Not recorded</span>'
      + '<small>Not captured for this run</small>';
  } else {
    const value = esc(formatNumber(metric?.value ?? metric));
    var detail;
    if (metric?.denominator != null) {
      const pct = esc(metric.percentage ?? percent(metric.value, metric.denominator));
      detail = '<small>' + formatNumber(metric.value) + ' of '
        + formatNumber(metric.denominator) + ' (' + pct + ')</small>';
    } else {
      detail = '<small>Recorded value</small>';
    }
    content = '<span class="label">' + esc(name) + '</span>'
      + '<span class="value">' + value + '</span>'
      + detail;
  }

  if (href) {
    return '<div class="ui statistic"><a href="' + href + '">' + content + '</a></div>';
  }
  return '<div class="ui statistic">' + content + '</div>';
}

/** Returns one retention-flow stage with counts, percentages, and optional links. */
export function flowStage(label, raw, base, previous, extraClass, stageKey, options) {
  if (!extraClass) {
    extraClass = '';
  }
  options = options || {};
  var stageClass = 'rw-flow__step';
  if (extraClass) {
    var extraModifiers = extraClass.split(' ');
    extraModifiers.forEach(function(mod) {
      if (mod === 'has-branch') {
        stageClass = stageClass + ' rw-flow__step--has-branch';
      } else if (mod === 'branch') {
        stageClass = stageClass + ' rw-flow__step--branch';
      } else if (mod === 'last') {
        stageClass = stageClass + ' rw-flow__step--last';
      } else if (mod === 'discarded') {
        stageClass = stageClass + ' rw-flow__step--discarded';
      } else if (mod === 'source') {
        stageClass = stageClass + ' rw-flow__step--source';
      }
    });
  }

  var dataAttr = '';
  if (stageKey) {
    dataAttr = ' data-flow-stage="' + esc(stageKey) + '"';
  }

  if (raw?.available === false || raw == null) {
    return '<div class="' + stageClass + '"' + dataAttr + '>'
      + (options.eyebrow ? '<span class="rw-flow__eyebrow">' + esc(options.eyebrow) + '</span>' : '')
      + '<h4>' + esc(label) + '</h4>'
      + '<strong>Not recorded</strong>'
      + '<small>This stage was not captured.</small>'
      + '</div>';
  }

  const count = number(raw);
  const baseCount = number(base);
  const percentage = baseCount > 0 ? count * 100 / baseCount : null;
  const percentageText = percentage == null ? '\u2014' : percentage.toFixed(2) + '%';
  const progressWidth = percentage == null ? 0 : Math.max(0, Math.min(100, percentage));
  const denominatorText = options.denominatorLabel || 'input records';
  var change;
  if (previous == null) {
    change = options.baselineLabel || 'Input baseline';
  } else {
    const diff = previous - count;
    if (diff === 0) {
      change = 'No change from prior';
    } else {
      const sign = diff > 0 ? '\u2212' : '+';
      change = sign + formatNumber(Math.abs(diff)) + ' from prior';
    }
  }

  return '<div class="' + stageClass + '"' + dataAttr + '>'
    + (options.eyebrow ? '<span class="rw-flow__eyebrow">' + esc(options.eyebrow) + '</span>' : '')
    + '<h4>' + esc(label) + '</h4>'
    + (options.description ? '<p class="rw-flow__description" title="' + esc(options.description) + '">' + esc(options.description) + '</p>' : '')
    + '<div class="rw-flow__value"><strong>' + formatNumber(count) + '</strong>'
    + '<span class="rw-flow__percentage">' + percentageText + '</span></div>'
    + '<span class="rw-flow__progress" role="img" aria-label="' + esc(label + ': ' + formatNumber(count) + ' of ' + formatNumber(baseCount) + ' ' + denominatorText + ' (' + percentageText + ')') + '">'
    + '<span style="width:' + progressWidth.toFixed(2) + '%"></span></span>'
    + '<small class="rw-flow__basis">of ' + formatNumber(baseCount) + ' ' + esc(denominatorText) + '</small>'
    + '<small class="rw-flow__delta">' + esc(change) + '</small>'
    + '</div>';
}

var filterPresentations = {
  'NO_FILTER': {
    groupLabel: 'Unfiltered Source Data',
    label: 'Initial raw results',
    description: 'Unfiltered results reported by the configured sources.'
  },
  'RANGE_10_YEARS': {
    groupLabel: 'Publication Range',
    label: 'Publication window',
    description: 'Results retained within the declared 10-year range.'
  },
  'ARTICLE_ONLY': {
    groupLabel: 'Document Type',
    label: 'Article records',
    description: 'Results retained after applying the article-only filter.'
  },
  'ENGLISH_ONLY': {
    groupLabel: 'Language',
    label: 'English-language records',
    description: 'Results retained after the language filter.'
  }
};

/** Combines cumulative source filter counts into ordered cross-source stages. */
function sourceFilterStageSummary(items) {
  var bySource = new Map();
  (items || []).forEach(function(item) {
    if (!Array.isArray(item?.filters) || item.filters.length === 0) {
      return;
    }
    const sourceName = String(item.source || 'source');
    if (!bySource.has(sourceName)) {
      bySource.set(sourceName, []);
    }
    bySource.get(sourceName).push({
      count: Math.max(0, number(item.count)),
      filters: item.filters.map(String)
    });
  });

  const sources = Array.from(bySource.values()).filter(function(stages) { return stages.length > 0; });
  const stageCount = sources.reduce(function(maximum, stages) {
    return Math.max(maximum, stages.length);
  }, 0);
  var stages = [];
  for (var index = 0; index < stageCount; index += 1) {
    var count = 0;
    var appliedFilters = new Set();
    sources.forEach(function(sourceStages) {
      const stage = sourceStages[Math.min(index, sourceStages.length - 1)];
      count += stage.count;
      if (index < sourceStages.length) {
        const currentFilter = stage.filters[stage.filters.length - 1];
        if (currentFilter) {
          appliedFilters.add(currentFilter);
        }
      }
    });
    const filters = Array.from(appliedFilters);
    const presentation = filters.length === 1 ? filterPresentations[filters[0]] : null;
    stages.push({
      count: count,
      groupLabel: presentation?.groupLabel || (index === 0 ? 'Unfiltered Source Data' : 'Source Filter ' + (index + 1)),
      label: presentation?.label || (index === 0 ? 'Initial raw results' : 'Source filter stage ' + (index + 1)),
      description: presentation?.description || filters.map(function(name) { return humanLabel(name); }).join(', ')
    });
  }
  return { stages: stages, sourceCount: sources.length };
}

/** Returns one titled phase in the retention-flow presentation. */
function retentionPhase(title, description, summary, content, className) {
  return '<section class="rw-retention__phase ' + esc(className || '') + '">'
    + '<header class="rw-retention__phase-header"><div><h4>' + esc(title) + '</h4><p>' + esc(description) + '</p></div>'
    + (summary ? '<span class="ui label">' + esc(summary) + '</span>' : '') + '</header>'
    + content + '</section>';
}

/** Returns a labeled group of retention-flow cards. */
function flowGroup(title, cards, className) {
  return '<div class="rw-flow__group ' + esc(className || '') + '">'
    + '<h5 class="rw-flow__group-label">' + esc(title) + '</h5>'
    + '<div class="rw-flow__group-cards">' + cards + '</div></div>';
}

/** Returns the source-selection and pipeline-retention flow for an overview payload. */
export function retentionFlow(overview) {
  const source = overview.retention_funnel || {};
  const input = source.input_records;
  const filterSummary = sourceFilterStageSummary(overview.source_filter_counts);
  const filterStages = filterSummary.stages;
  const hasFilterStages = filterStages.length > 0;
  const inputCount = number(input);
  const initialCount = hasFilterStages ? filterStages[0].count : inputCount;
  const denominatorLabel = hasFilterStages ? 'initial raw results' : 'input records';
  var phases = '';

  if (hasFilterStages) {
    var previousFilterCount = null;
    const sourceStages = filterStages.map(function(stage, index) {
      const markup = flowStage(stage.label, stage.count, initialCount, previousFilterCount, 'source', '', {
        description: stage.description,
        denominatorLabel: denominatorLabel,
        baselineLabel: 'Initial raw-data baseline'
      });
      previousFilterCount = stage.count;
      return flowGroup(stage.groupLabel, markup, 'rw-flow__group--source');
    }).join('');
    const sourceSummary = formatNumber(initialCount) + ' initial raw results'
      + (filterSummary.sourceCount > 1 ? ' across ' + formatNumber(filterSummary.sourceCount) + ' sources' : '');
    phases += retentionPhase('Source selection', 'Declared cumulative result counts before the source files were exported.', sourceSummary,
      '<div class="rw-flow rw-flow--source">' + sourceStages + '</div>', 'rw-retention__phase--source');
  }

  if (!input || input.available === false) {
    if (!hasFilterStages) {
      return panel('Retention flow', 'The recorded path through parsing, deduplication, and validation.', '<p class="ui faded text">Not recorded for this run.</p>', 'span-all rw-panel--no-separator');
    }
    return panel('Retention flow', 'Source selection evidence is available, but pipeline retention was not captured for this run.',
      '<div class="rw-retention">' + phases + '</div>', 'span-all rw-panel--no-separator');
  }

  const parsed = source.parsed_articles;
  const deduped = source.deduplicated_articles;
  const parsedCount = number(parsed);
  const dedupedCount = number(deduped);
  const validCount = number(source.valid_articles);
  const enrichmentCandidates = overview.enrichment_breakdown?.enrichment_candidates;
  const normalizedArticles = overview.normalization_breakdown?.normalized_articles_processed;
  const enrichmentCount = enrichmentCandidates == null || enrichmentCandidates?.available === false
    ? dedupedCount
    : number(enrichmentCandidates);
  const pipelinePrevious = hasFilterStages ? filterStages[filterStages.length - 1].count : null;
  const stageOptions = function(description, eyebrow) {
    return { eyebrow: eyebrow || '', description: description, denominatorLabel: denominatorLabel, baselineLabel: 'Input baseline' };
  };
  const preparationStages = flowGroup('Filtered Raw Data',
    flowStage('Input records', input, initialCount, pipelinePrevious, '', 'input_records',
      stageOptions('Records read from the exported source files.')))
    + flowGroup('Properly Loaded Data',
      flowStage('Parsed articles', parsed, initialCount, inputCount, '', 'parsed_articles',
        stageOptions('Source records converted into article metadata.')))
    + flowGroup('Deduplicating Entries',
      flowStage('Deduplicated articles', deduped, initialCount, parsedCount, '', 'deduplicated_articles',
        stageOptions('Unique articles retained after source merging.')));
  const validationCards = flowStage('Valid articles', source.valid_articles, initialCount, enrichmentCount, 'last', 'valid_articles',
    stageOptions('Articles retained in the analysis-ready corpus.', 'Accepted'))
    + flowStage('Discarded articles', source.discarded_articles, initialCount, enrichmentCount, 'branch discarded', 'discarded_articles',
      stageOptions('Articles rejected by validation rules.', 'Discarded'));
  const researchStages = flowGroup('Enrichment',
    flowStage('Candidate articles', enrichmentCandidates, initialCount, dedupedCount, '', 'enrichment_candidates',
      stageOptions('Deduplicated articles considered for provider enrichment.')))
    + flowGroup('Validation Outcomes', validationCards, 'rw-flow__group--split rw-flow__outcomes')
    + flowGroup('Normalization',
      flowStage('Normalized articles', normalizedArticles, initialCount, validCount, '', 'normalized_articles_processed',
        stageOptions('Valid articles processed into canonical forms.')));
  const pipelineSummary = hasFilterStages
    ? 'Percentages use the ' + formatNumber(initialCount) + '-result raw baseline'
    : formatNumber(initialCount) + ' captured input records';
  const pipelineFlow = '<div class="rw-flow rw-flow--pipeline">' + preparationStages + '</div>'
    + '<div class="rw-flow__continuation"><span>Prepared records continue through the research pipeline</span><b aria-hidden="true">\u2193</b></div>'
    + '<div class="rw-flow rw-flow--pipeline">' + researchStages + '</div>';
  phases += retentionPhase('Pipeline processing', 'Observed records move from loading through normalization.', pipelineSummary,
    pipelineFlow, 'rw-retention__phase--pipeline');

  var description = hasFilterStages
    ? 'Source selection and pipeline retention share the same initial raw-data denominator.'
    : 'Valid and discarded records branch after deduplication.';
  return panel('Retention flow', description,
    '<div class="rw-retention">' + phases + '</div>', 'span-all rw-panel--no-separator');
}

/** Returns a metric breakdown table with relative bars and optional total percentages. */
export function breakdown(title, source, valueLabel, useTotal) {
  if (!valueLabel) {
    valueLabel = 'Count';
  }
  const entries = metricEntries(source);
  if (!entries.length) {
    return panel(title, 'Recorded activity for this run.', '<p class="empty">Not recorded for this run.</p>');
  }

  var total;
  if (useTotal) {
    total = entries.reduce(function(sum, [, entry]) {
      return sum + number(entry);
    }, 0);
  } else {
    total = null;
  }

  const max = total || Math.max.apply(null, entries.map(function([, entry]) {
    return number(entry);
  }).concat([1]));

  /** Formats one breakdown value with availability and optional percentage. */
  function valueRender(row) {
    if (row.raw?.available === false) {
      return '<span class="ui faded text">Not recorded</span>';
    }
    if (!useTotal) {
      return formatNumber(row.raw);
    }
    var pct;
    if (total > 0) {
      pct = (number(row.raw) * 100 / total).toFixed(2) + '%';
    } else {
      pct = '—';
    }
    return formatNumber(row.raw) + ' (' + pct + ')';
  }

  /** Returns an accessible relative-volume bar for one breakdown row. */
  function barRender(row) {
    if (row.raw?.available === false) {
      return '<span class="ui faded text">—</span>';
    }
    const pct = Math.min(100, number(row.raw) / max * 100);
    return '<span class="ui progress" role="img" aria-label="' + esc(row.name) + ' ' + formatNumber(row.raw) + '">'
      + '<span class="bar" style="width:' + pct + '%"></span></span>';
  }

  const rows = entries.map(function([name, raw]) {
    return { name: name, raw: raw };
  });

  return table(title, 'Recorded activity for this run.', [
    { label: 'Metric', render: function(row) { return esc(row.name); } },
    { label: valueLabel, render: valueRender },
    { label: 'Relative volume', render: barRender }
  ], rows);
}

/** Returns the expected-versus-observed source export count table. */
export function sourceResultCountSummary(items, classes) {
  if (!classes) {
    classes = '';
  }

  /** Formats a source count or its unavailable state. */
  function count(raw) {
    if (raw == null) {
      return '<span class="muted">Not recorded</span>';
    }
    return formatNumber(raw);
  }

  /** Returns a status chip for a source-count comparison. */
  function comparison(raw) {
    if (raw) {
      return statusChip(raw);
    }
    return '<span class="muted">Not recorded</span>';
  }

  /** Escapes an export date or returns its unavailable state. */
  function date(raw) {
    if (raw) {
      return esc(raw);
    }
    return '<span class="muted">Not recorded</span>';
  }

  const rows = (items || []).map(function(item) {
    return {
      source: item.source_name || 'Unnamed source',
      expected: item.expected_result_count,
      observed: item.observed_result_count,
      comparison: item.result_count_comparison,
      exportDate: item.export_date,
    };
  });

  return table('Source export counts',
    'Expected is the count recorded when metadata was originally downloaded. Observed is the raw export count read by this run. The comparison is informational and never makes a run fail.',
    [
      { label: 'Source', render: function(row) { return esc(row.source); } },
      { label: 'Export date', render: function(row) { return date(row.exportDate); } },
      { label: 'Expected initial count', render: function(row) { return count(row.expected); } },
      { label: 'Observed raw records', render: function(row) { return count(row.observed); } },
      { label: 'Comparison', render: function(row) { return comparison(row.comparison); } },
    ], rows, classes);
}

/** Returns expandable exact-query markup for source exports. */
export function sourceSearchQueries(items, classes) {
  if (!classes) {
    classes = '';
  }
  if (!items || !items.length) {
    return '';
  }

  const rows = items.map(function(item) {
    const name = esc(item.source_name || 'Unnamed source');
    const q = esc(item.query || '');
    return '<details class="query-row"><summary><span class="query-source">' + name + '</span>'
      + '<span class="ui faded text">Inspect exact query</span></summary>'
      + '<div class="query-content"><code class="query-code">' + q + '</code>'
      + '<button type="button" class="ui button" data-copy-text="' + q.replace(/"/g, '&quot;') + '">Copy query</button>'
      + '</div></details>';
  });

  return panel('Search queries',
    'The exact search terms used to obtain each source export. Expand a source to inspect or copy its query.',
    rows.join(''), classes);
}

/** Returns chronological audit feed markup for generic event rows. */
export function timeline(rows) {
  if (!rows.length) {
    return '<p class="ui faded text">No records.</p>';
  }

  const items = rows.map(function(event) {
    const action = text(event, ['action', 'event_type', 'type']);
    const entityType = text(event, ['entity_type', 'entity', 'source']);
    const actor = event.actor ? esc(event.actor) : '';
    const entityId = event.entity_id ? esc(event.entity_id) : '';
    const meta = event.metadata_json;

    var detail = '';
    if (meta && typeof meta === 'object') {
      if (meta.field && meta.provider) {
        detail = 'Field <strong>' + esc(meta.field) + '</strong> enriched by <strong>' + esc(meta.provider) + '</strong>';
      } else if (meta.reasons) {
        detail = esc(Array.isArray(meta.reasons) ? meta.reasons.join('; ') : String(meta.reasons));
      } else if (meta.error) {
        detail = 'Error: ' + esc(meta.error);
      } else if (meta.status) {
        detail = 'Status: ' + esc(meta.status);
      } else if (meta.identity) {
        detail = esc(meta.identity);
      } else if (meta.reason) {
        detail = esc(meta.reason);
      } else if (meta.search_id) {
        detail = 'Search: ' + esc(meta.search_id);
        if (meta.revision) {
          detail = detail + ' / revision ' + esc(meta.revision);
        }
      }
    }

    var dotClass;
    if (action.startsWith('pipeline_')) {
      dotClass = 'pipeline-dot';
    } else if (action === 'field_enriched') {
      dotClass = 'enrich-dot';
    } else if (action.startsWith('validation_')) {
      dotClass = 'validation-dot';
    } else {
      dotClass = 'default-dot';
    }

    var actorHtml = '';
    if (actor) {
      actorHtml = '<span class="user">' + actor + '</span>';
    }

    var entityHtml = '';
    if (entityId) {
      entityHtml = '<span class="extra">' + esc(entityType) + ' #' + entityId + '</span>';
    }

    var detailHtml = '';
    if (detail) {
      detailHtml = '<span class="summary">' + detail + '</span>';
    }

    const timestamp = esc(formatTime(event.occurred_at || event.created_at || event.at || event.timestamp));

    return '<li class="event ' + dotClass + '">'
      + actorHtml + entityHtml
      + '<strong>' + esc(action) + '</strong><br>'
      + detailHtml + '<br>'
      + '<time>' + timestamp + '</time>'
      + '</li>';
  });

  return '<ol class="ui feed">' + items.join('') + '</ol>';
}

/** Builds a table whose columns are derived from the supplied detail records. */
export function detailTable(title, rows) {
  const records = list(rows, ['items', 'rows']);
  const columns = [...new Set(records.flatMap(function(record) {
    return Object.keys(record);
  }))];
  const colDefs = columns.map(function(key) {
    return {
      label: key,
      render: function(row) {
        return cell(row[key], key);
      }
    };
  });
  return table(title, '', colDefs, records);
}

/** Formats and links a table cell according to its column and table context. */
export function cell(item, column, tableName, options) {
  if (!tableName) {
    tableName = '';
  }
  if (!options) {
    options = {};
  }
  if (item === null || item === undefined) {
    return '<span class="ui faded text">NULL</span>';
  }

  const full = asJSON(item);
  var display;
  if (full.length > 140) {
    display = full.slice(0, 137) + '…';
  } else {
    display = full;
  }

  var href = '';
  if (column === 'article_id' || column === 'work_revision_id' || (tableName === 'work_revisions' && column === 'id')) {
    href = link({ view: 'article', article_id: item });
  }
  if (column === 'author_id' || column === 'author_occurrence_id' || (tableName === 'author_occurrences' && column === 'id')) {
    href = link({ view: 'author', author_id: item });
  }
  if (column === 'reference_id' || (tableName === 'reference_mentions' && column === 'id')) {
    href = link({ view: 'reference', reference_id: item });
  }

  const shown = '<span class="rw-cell" title="' + esc(full) + '">' + esc(display) + '</span>';
  if (href) {
    return '<a href="' + href + '">' + shown + '</a>';
  }
  if (full.length > 140 && options.expandLong !== false) {
    return '<details><summary>' + shown + '</summary><pre>' + esc(full) + '</pre></details>';
  }
  return shown;
}

/**
 * Bind copy-to-clipboard behavior for [data-copy-text] buttons.
 * Shows "Copied!" feedback for 2 seconds, falls back to prompt().
 */
export function bindCopyButtons() {
  document.querySelectorAll('[data-copy-text]').forEach(function(button) {
    button.addEventListener('click', async function() {
      var text = this.getAttribute('data-copy-text');
      try {
        await navigator.clipboard.writeText(text);
        this.textContent = 'Copied!';
        setTimeout(function() { this.textContent = 'Copy'; }.bind(this), 2000);
      } catch (_) {
        prompt('Copy query manually:', text);
      }
    });
  });
}

/**
 * Bind dismissible behavior for .ui.message elements with a .close child.
 * Clicking the close button fades out and removes the message.
 */
export function bindDismissibleMessages() {
  document.querySelectorAll('.ui.message > .close').forEach(function(button) {
    button.addEventListener('click', function() {
      var message = this.closest('.ui.message');
      if (message) {
        message.style.opacity = '0';
        setTimeout(function() { message.hidden = true; }, 150);
      }
    });
  });
}

/**
 * Bind loading state for buttons with [data-loading].
 * On click, the button shows a spinner and disables itself.
 */
export function bindLoadingButtons() {
  document.querySelectorAll('[data-loading]').forEach(function(button) {
    button.addEventListener('click', function() {
      button.classList.add('loading');
      button.disabled = true;
    });
  });
}
