// Unit tests for state.tsx: shared state, DOM references, and utility components.
import { describe, it, before, beforeEach, mock } from 'node:test';
import assert from 'node:assert/strict';

// Setup DOM before importing the module under test
import './setup.ts';
import type { HierarchyRun } from '../../src/api/types.ts';
import {
  app, notice, loading, state, pageSizes, corpusSections, provenanceSections, graphFilters,
  params, value, view, section, detailOrigin, viewPage, esc, asJSON, list, pickID, text, numericEvidence, number, formatNumber,
  percent, formatTime, formatDate, formatDuration, formatBytes, humanLabel, parseObject, statusClass, StatusChip, metricEntries, selectedRun, showError,
  clearError, busy, link, contextChange, PageHeader, Breadcrumb, setBreadcrumb, EmptyState, Table, Subnav,
  FilterChips, MetricCard, FlowStage, RetentionFlow, Breakdown, SourceResultCountSummary, Timeline,
  DetailTable, Cell, bindCopyButtons, bindDismissibleMessages, bindLoadingButtons,
} from '../../src/state.tsx';
import { h, renderToString } from "../../src/jsx/jsx-runtime.ts";

const statusChip = (raw: any): string => renderToString(StatusChip({ raw: raw }));
const pageHeader = (kicker: string, title: string, description: string, extra?: JSX.Element): string => renderToString(PageHeader({ kicker: kicker, title: title, description: description, extra: extra }));
const breadcrumb = (items: Array<{ href?: string; label: string }>): string => renderToString(Breadcrumb({ items: items }));
const emptyState = (title: string, detail: string): string => renderToString(EmptyState({ title: title, detail: detail }));
const table = (title: string, description: string, columns: any[], rows: any[]): string => renderToString(Table({ title: title, description: description, columns: columns, rows: rows }));
const subnav = (items: Array<[string, string]>, current: string, key: string): string => renderToString(Subnav({ items: items, current: current, key: key }));
const filterChips = (filters: Record<string, any>, labels?: Record<string, string>, options?: any): string => renderToString(FilterChips({ filters: filters, labels: labels, options: options }));
const metricCard = (name: string, metric: any, href?: string): string => renderToString(MetricCard({ name: name, metric: metric, href: href }));
const flowStage = (label: string, raw: any, base: any, previous: any, stageKey: string, options: any): string => {
  const stageMarkup = FlowStage({
    label: label,
    raw: raw,
    base: base,
    previous: previous,
    stageKey: stageKey,
    options: options,
  });
  return renderToString(stageMarkup);
};
const retentionFlow = (overview: any): string => renderToString(RetentionFlow({ overview: overview }));
const breakdown = (title: string, source: any, valueLabel?: string, useTotal?: boolean): string => renderToString(Breakdown({ title: title, source: source, valueLabel: valueLabel, useTotal: useTotal }));
const sourceResultCountSummary = (items: any[]): string => renderToString(SourceResultCountSummary({ items: items }));
const timeline = (rows: any[]): string => renderToString(Timeline({ rows: rows }));
const detailTable = (title: string, rows: any): string => renderToString(DetailTable({ title: title, rows: rows }));
const cell = (item: any, column: string, tableName?: string): string => renderToString(Cell({ item: item, column: column, tableName: tableName }));

describe('state.tsx — constants', function() {

  it('maps supported views to their owning page files', function() {
    assert.equal(viewPage.home, 'index.html');
    assert.equal(viewPage.overview, 'overview.html');
    assert.equal(viewPage.article, 'article.html');
  });

  it('app is a DOM element', function() {
    assert.ok(app instanceof HTMLElement);
    assert.equal(app.id, 'app');
  });

  it('notice is a DOM element', function() {
    assert.ok(notice instanceof HTMLElement);
    assert.equal(notice.id, 'notice');
  });

  it('loading is a DOM element', function() {
    assert.ok(loading instanceof HTMLElement);
    assert.equal(loading.id, 'loading');
  });

  it('state has expected keys', function() {
    assert.ok(Array.isArray(state.searches));
    assert.ok(Array.isArray(state.plans));
    assert.ok(Array.isArray(state.runs));
    assert.ok(Array.isArray(state.tables));
    assert.equal(typeof state.request, 'number');
    assert.equal(state.controller, null);
  });

  it('pageSizes contains expected values', function() {
    assert.deepEqual(pageSizes, [20, 50, 100, 200, 500]);
  });

  it('corpusSections has expected keys', function() {
    assert.ok(corpusSections.articles);
    assert.ok(corpusSections.authors);
    assert.ok(corpusSections.references);
    assert.ok(corpusSections.identity_evidence);
    assert.ok(corpusSections.sources);
    assert.equal(corpusSections.articles.table, 'work_revisions');
  });

  it('provenanceSections has expected keys', function() {
    assert.ok(provenanceSections.audit);
    assert.ok(provenanceSections.artifacts);
    assert.ok(provenanceSections.cache);
    assert.ok(provenanceSections.stages);
    assert.ok(provenanceSections.run);
    assert.equal(provenanceSections.audit[0], 'Audit timeline');
  });

  it('graphFilters contains expected filters', function() {
    assert.ok(graphFilters.includes('mode'));
    assert.ok(graphFilters.includes('q'));
    assert.ok(graphFilters.includes('author'));
    assert.ok(graphFilters.includes('article_limit'));
  });

});

describe('state.tsx — params / value / view / section', function() {

  it('params returns URLSearchParams from location.search', function() {
    const p = params();
    assert.ok(p instanceof URLSearchParams);
    assert.equal(p.get('view'), 'overview');
  });

  it('value reads a query parameter', function() {
    assert.equal(value('view'), 'overview');
    assert.equal(value('nonexistent'), '');
  });

  it('view returns the current view or Home default', function() {
    assert.equal(view(), 'overview');
    const url = new URL(location.href);
    url.searchParams.delete('view');
    history.pushState({}, '', url.toString());
    assert.equal(view(), 'home');
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());
  });

  it('section returns the named parameter or fallback', function() {
    assert.equal(section('section', 'default'), 'default');
    assert.equal(section('view', 'fallback'), 'overview');
  });

});

describe('state.tsx — esc', function() {

  it('escapes HTML special characters', function() {
    const result = esc('<script>alert("xss")</script>');
    // jsdom's innerHTML does not escape quotes in text content
    assert.ok(result.includes('&lt;script&gt;'));
    assert.ok(result.includes('&lt;/script&gt;'));
  });

  it('handles null and undefined', function() {
    assert.equal(esc(null), '');
    assert.equal(esc(undefined), '');
  });

  it('converts numbers to strings', function() {
    assert.equal(esc(42), '42');
    assert.equal(esc(0), '0');
  });

  it('escapes ampersands', function() {
    assert.equal(esc('a & b'), 'a &amp; b');
  });

});

describe('state.tsx — asJSON', function() {

  it('returns strings unchanged', function() {
    assert.equal(asJSON('hello'), 'hello');
  });

  it('formats objects as pretty JSON', function() {
    const result = asJSON({ a: 1, b: 2 });
    assert.ok(result.includes('"a": 1'));
    assert.ok(result.includes('"b": 2'));
  });

  it('formats arrays as pretty JSON', function() {
    const result = asJSON([1, 2, 3]);
    assert.ok(result.includes('1'));
  });

  it('handles null', function() {
    assert.equal(asJSON(null), 'null');
  });

});

describe('state.tsx — list', function() {

  it('returns the first matching key from data', function() {
    const data = { items: [1, 2, 3], rows: [4, 5] };
    assert.deepEqual(list(data, ['items', 'rows']), [1, 2, 3]);
  });

  it('returns data if it is an array', function() {
    assert.deepEqual(list([1, 2], ['items']), [1, 2]);
  });

  it('returns empty array for non-array data', function() {
    assert.deepEqual(list(null, ['items']), []);
    assert.deepEqual(list({}, ['items']), []);
    assert.deepEqual(list(undefined, ['items']), []);
  });

  it('returns empty array when no keys match', function() {
    assert.deepEqual(list({ items: 'not array' }, ['items']), []);
  });

});

describe('state.tsx — pickID', function() {

  it('prefers .id', function() {
    assert.equal(pickID({ id: 'abc', search_id: 'xyz' }), 'abc');
  });

  it('falls back to search_id', function() {
    assert.equal(pickID({ search_id: 's1' }), 's1');
  });

  it('falls back to run_id', function() {
    assert.equal(pickID({ run_id: 'r1' }), 'r1');
  });

  it('falls back to plan_id', function() {
    assert.equal(pickID({ plan_id: 'p1' }), 'p1');
  });

  it('returns empty string when no id found', function() {
    assert.equal(pickID({}), '');
    assert.equal(pickID(null), '');
    assert.equal(pickID(undefined), '');
  });

});

describe('state.tsx — text', function() {

  it('returns the first non-empty field', function() {
    assert.equal(text({ a: '', b: 'hello', c: 'world' }, ['a', 'b', 'c']), 'hello');
  });

  it('returns Unnamed when all fields are empty', function() {
    assert.equal(text({ a: '', b: null, c: undefined }, ['a', 'b', 'c']), 'Unnamed');
  });

  it('returns Unnamed for null/undefined item', function() {
    assert.equal(text(null, ['name']), 'Unnamed');
    assert.equal(text(undefined, ['name']), 'Unnamed');
  });

});

describe('state.tsx — number', function() {

  it('parses numeric values', function() {
    assert.equal(number(42), 42);
    assert.equal(number('42'), 42);
  });

  it('does not conflate unavailable or invalid values with recorded zero', function() {
    assert.equal(Number.isNaN(number(null)), true);
    assert.equal(Number.isNaN(number(undefined)), true);
    assert.equal(Number.isNaN(number('abc')), true);
    assert.deepEqual(numericEvidence(null), { state: 'unavailable', value: null });
    assert.deepEqual(numericEvidence('abc'), { state: 'invalid', value: null });
    assert.deepEqual(numericEvidence(0), { state: 'recorded', value: 0 });
    assert.deepEqual(numericEvidence({ state: 'derived', value: 0 }), { state: 'derived', value: 0 });
  });

  it('handles { value } objects', function() {
    assert.equal(number({ value: 42 }), 42);
    assert.equal(Number.isNaN(number({ value: null })), true);
  });

  it('returns NaN for NaN and Infinity', function() {
    assert.equal(Number.isNaN(number(NaN)), true);
    assert.equal(Number.isNaN(number(Infinity)), true);
  });

});

describe('state.tsx — formatNumber', function() {

  it('formats numbers with locale separators', function() {
    const result = formatNumber(1234567);
    assert.ok(typeof result === 'string');
    assert.ok(result.length > 0);
  });

  it('handles zero', function() {
    assert.equal(formatNumber(0), '0');
  });

  it('handles object values', function() {
    const result = formatNumber({ value: 1000 });
    assert.ok(typeof result === 'string');
  });

  it('labels unavailable and invalid evidence explicitly', function() {
    assert.equal(formatNumber(null), 'Not recorded');
    assert.equal(formatNumber('not-a-number'), 'Invalid value');
  });

});

describe('state.tsx — percent', function() {

  it('calculates percentage', function() {
    assert.equal(percent(25, 100), '25.0%');
  });

  it('returns em dash for zero denominator', function() {
    assert.equal(percent(25, 0), '—');
  });

  it('handles object values', function() {
    assert.equal(percent({ value: 50 }, { value: 200 }), '25.0%');
  });

});

describe('state.tsx — formatTime', function() {

  it('returns em dash for falsy input', function() {
    assert.equal(formatTime(null), '—');
    assert.equal(formatTime(undefined), '—');
    assert.equal(formatTime(''), '—');
  });

  it('returns the raw string for invalid dates', function() {
    assert.equal(formatTime('not-a-date'), 'not-a-date');
  });

  it('formats valid date strings', function() {
    const result = formatTime('2024-01-15T10:30:00Z');
    assert.ok(typeof result === 'string');
    assert.ok(result.length > 0);
    assert.notEqual(result, '—');
  });

  it("uses the documented English UTC calendar policy at a day boundary", function() {
    assert.equal(formatDate("2024-01-01T00:30:00+14:00"), "Dec 31, 2023");
    assert.equal(formatTime("2024-01-01T00:30:00+14:00"), "Dec 31, 2023, 10:30:00 AM");
  });

});

describe('state.tsx — formatDuration', function() {

  it('formats recorded elapsed time', function() {
    assert.equal(formatDuration('2024-01-01T00:00:00Z', '2024-01-01T01:02:03Z'), '1h 2m 3s');
  });

  it('rejects missing, invalid, or reversed timestamps', function() {
    assert.equal(formatDuration('', '2024-01-01T00:00:00Z'), '—');
    assert.equal(formatDuration('invalid', '2024-01-01T00:00:00Z'), '—');
    assert.equal(formatDuration('2024-01-02T00:00:00Z', '2024-01-01T00:00:00Z'), '—');
  });

});

describe('state.tsx — display helpers', function() {

  it('formats byte counts without treating kilobytes as decimal units', function() {
    assert.equal(formatBytes(512), '512 B');
    assert.match(formatBytes(1536), /^1[.,]5 KB$/);
  });

  it('turns stored field names into readable labels', function() {
    assert.equal(humanLabel('request_fingerprint'), 'Request Fingerprint');
  });

  it('parses object payloads and rejects scalar or malformed JSON', function() {
    assert.deepEqual(parseObject('{"stage":"normalize"}'), { stage: 'normalize' });
    assert.deepEqual(parseObject('not-json'), {});
    assert.deepEqual(parseObject('42'), {});
  });

  it('uses namespaced pagination updates when a filter chip is removed', function() {
    const originalURL = location.href;
    history.replaceState({}, '', '?view=provenance&section=cache');
    const html = filterChips({ cache_q: 'crossref' }, { cache_q: 'Search' }, {
      removeUpdates: { cache_page: 1 }, clearUpdates: { cache_q: '', cache_page: 1 }
    });
    const host = document.createElement('div');
    host.innerHTML = html;
    const url = new URL((host.querySelector('.rw-filter-chip') as HTMLAnchorElement).href);
    assert.equal(url.searchParams.get('cache_page'), '1');
    assert.equal(url.searchParams.get('page'), null);
    history.replaceState({}, '', originalURL);
  });

  it('renders one independently removable chip for each selected facet value', function() {
    const originalURL = location.href;
    history.replaceState({}, '', '?view=provenance&section=audit&audit_category=pipeline%2Creview');
    const html = filterChips({ audit_category: ['pipeline', 'review'] }, { audit_category: 'Category' });
    const host = document.createElement('div');
    host.innerHTML = html;
    const chips = Array.from(host.querySelectorAll<HTMLAnchorElement>('.rw-filter-chip'));
    assert.equal(chips.length, 2);
    assert.match(chips[0].textContent || '', /pipeline/);
    assert.equal(new URL(chips[0].href).searchParams.get('audit_category'), 'review');
    assert.equal(new URL(chips[1].href).searchParams.get('audit_category'), 'pipeline');
    history.replaceState({}, '', originalURL);
  });

});

describe('state.tsx — statusClass', function() {

  it('returns red for failure-related statuses', function() {
    assert.equal(statusClass('fail'), 'red');
    assert.equal(statusClass('discard'), 'red');
    assert.equal(statusClass('error'), 'red');
    assert.equal(statusClass('trash'), 'red');
    assert.equal(statusClass('invalid'), 'red');
  });

  it('returns green for completion-related statuses', function() {
    assert.equal(statusClass('complete'), 'green');
    assert.equal(statusClass('valid'), 'green');
    assert.equal(statusClass('success'), 'green');
    assert.equal(statusClass('hit'), 'green');
    assert.equal(statusClass('ready'), 'green');
  });

  it('returns orange for warning-like statuses', function() {
    assert.equal(statusClass('warning'), 'orange');
    assert.equal(statusClass('skipped'), 'orange');
    assert.equal(statusClass('stale'), 'orange');
    assert.equal(statusClass('unresolved'), 'orange');
    assert.equal(statusClass('unmatched'), 'orange');
    assert.equal(statusClass('orcid_is_unclear'), 'orange');
    assert.equal(statusClass('not_evaluated'), 'orange');
  });

  it('returns blue for informational statuses and neutral for unrecorded values', function() {
    assert.equal(statusClass('pending'), 'blue');
    assert.equal(statusClass('running'), 'blue');
    assert.equal(statusClass('recorded'), 'blue');
    assert.equal(statusClass(''), 'grey');
    assert.equal(statusClass(null), 'grey');
    assert.equal(statusClass('not_available'), 'orange');
    assert.equal(statusClass('no_orcid_candidate'), 'grey');
    assert.equal(statusClass('inherited'), 'violet');
  });

});

describe('state.tsx — statusChip', function() {

  it('wraps status in a span with class', function() {
    const result = statusChip('complete');
    assert.ok(result.includes('class="ui green label"'));
    assert.ok(result.includes('complete'));
  });

  it('uses Not recorded for null', function() {
    const result = statusChip(null);
    assert.ok(result.includes('Not recorded'));
  });

});

describe('state.tsx — metricEntries', function() {

  it('converts arrays of objects to entries', function() {
    const input = [
      { metric: 'articles', value: 10, source: 'crossref' },
      { metric: 'authors', value: 5 },
    ];
    const result = metricEntries(input);
    assert.equal(result.length, 2);
    assert.equal(result[0][0], 'articles (crossref)');
    assert.equal(result[1][0], 'authors');
  });

  it('converts object to entries', function() {
    const result = metricEntries({ a: 1, b: 2 });
    assert.deepEqual(result, [['a', 1], ['b', 2]]);
  });

  it('returns empty array for null/undefined', function() {
    assert.deepEqual(metricEntries(null), []);
    assert.deepEqual(metricEntries(undefined), []);
  });

});

describe('state.tsx — selectedRun', function() {

  it('returns undefined when no run_id is set', function() {
    assert.equal(selectedRun(), undefined);
  });

  it('finds a run by run_id', function() {
    state.runs = [{ id: 'run-1', status: 'complete' } as unknown as HierarchyRun];
    // Set run_id in URL
    const url = new URL(location.href);
    url.searchParams.set('run_id', 'run-1');
    history.pushState({}, '', url.toString());

    const result = selectedRun();
    assert.ok(result);
    assert.equal(result.id, 'run-1');

    // Cleanup
    url.searchParams.delete('run_id');
    history.pushState({}, '', url.toString());
    state.runs = [];
  });

});

describe('state.tsx — showError / clearError / busy', function() {

  it('showError sets notice text and removes hidden', function() {
    notice.hidden = true;
    showError(new Error('test error'));
    assert.equal(notice.textContent, 'test error');
    assert.equal(notice.hidden, false);
  });

  it('showError handles string errors', function() {
    showError('string error');
    assert.equal(notice.textContent, 'string error');
  });

  it('clearError hides notice and clears text', function() {
    clearError();
    assert.equal(notice.hidden, true);
    assert.equal(notice.textContent, '');
  });

  it('busy toggles loading visibility', function() {
    busy(true);
    assert.equal(loading.hidden, false);
    busy(false);
    assert.equal(loading.hidden, true);
  });

});

describe('state.tsx — link', function() {

  beforeEach(function() {
    history.replaceState({}, '', '/?view=overview');
  });

  it('builds a query string from updates', function() {
    const result = link({ view: 'corpus', section: 'articles' });
    assert.ok(result.startsWith('corpus.html?'));
    assert.ok(result.includes('view=corpus'));
    assert.ok(result.includes('section=articles'));
  });

  it('removes keys with empty values but keeps the Home default', function() {
    const result = link({ view: '' });
    assert.ok(result.startsWith('index.html?'));
    assert.ok(result.includes('view=home'));
  });

  it('removes keys with null values but keeps the Home default', function() {
    const result = link({ view: null });
    assert.ok(result.startsWith('index.html?'));
    assert.ok(result.includes('view=home'));
  });

  it('ensures view defaults to Home', function() {
    const url = new URL(location.href);
    url.searchParams.delete('view');
    history.pushState({}, '', url.toString());
    const result = link({ section: 'articles' });
    assert.ok(result.includes('view=home'));
    url.searchParams.set('view', 'overview');
    history.pushState({}, '', url.toString());
  });

  it('uses the current view page when no updates are supplied', function() {
    const result = link();
    assert.ok(result.startsWith('overview.html?'));
  });

  it('uses the home page for an unsupported destination', function() {
    const result = link({ view: 'unsupported' });
    assert.ok(result.startsWith('index.html?'));
    assert.ok(result.includes('view=unsupported'));
  });

  it("keeps only canonical context and destination-owned route state", () => {
    history.replaceState({}, "", "?view=corpus&search_id=1&search_revision_id=2&plan_id=3&run_id=4&section=articles&q=term&page=2&note_id=9&audit_q=stale");

    const provenance = new URL(link({ view: "provenance", section: "audit" }), location.origin).searchParams;
    assert.equal(provenance.get("run_id"), "4");
    assert.equal(provenance.get("section"), "audit");
    assert.equal(provenance.has("q"), false);
    assert.equal(provenance.has("page"), false);
    assert.equal(provenance.has("note_id"), false);

    const home = new URL(link({ view: "home" }), location.origin).searchParams;
    assert.deepEqual(Array.from(home.keys()), ["view"]);
    history.replaceState({}, "", "?view=overview");
  });

  it("returns detail context changes to the corresponding collection without record focus", () => {
    history.replaceState({}, "", "?view=article&search_id=1&search_revision_id=2&plan_id=3&run_id=4&article_id=8&note_id=9&anchor_id=a1&pdf_page=2");
    const updates = contextChange({ run_id: "5" });
    const target = new URL(link(updates), location.origin).searchParams;
    assert.equal(target.get("view"), "corpus");
    assert.equal(target.get("section"), "articles");
    assert.equal(target.get("run_id"), "5");
    assert.equal(target.has("article_id"), false);
    assert.equal(target.has("note_id"), false);
    assert.equal(target.has("anchor_id"), false);
    assert.equal(target.has("pdf_page"), false);
    history.replaceState({}, "", "?view=overview");
  });

  it("clears article-local note, anchor, and page focus when the article changes", () => {
    history.replaceState({}, "", "?view=article&run_id=4&article_id=8&note_id=9&anchor_id=a1&pdf_page=2");
    const target = new URL(link({ view: "article", article_id: "10" }), location.origin).searchParams;
    assert.equal(target.get("article_id"), "10");
    assert.equal(target.has("note_id"), false);
    assert.equal(target.has("anchor_id"), false);
    assert.equal(target.has("pdf_page"), false);
    history.replaceState({}, "", "?view=overview");
  });

  it("returns a detail origin through the origin view page", () => {
    const origin = new URLSearchParams({
      view: "corpus",
      search_id: "1",
      search_revision_id: "2",
      plan_id: "3",
      run_id: "4",
      section: "articles",
    });
    const detail = new URLSearchParams({
      view: "article",
      search_id: "1",
      search_revision_id: "2",
      plan_id: "3",
      run_id: "4",
      article_id: "8",
      origin: origin.toString(),
    });
    history.replaceState({}, "", `?${detail.toString()}`);

    const result = detailOrigin();

    assert.ok(result);
    assert.ok(result.href.startsWith("corpus.html?"));
    assert.equal(new URL(result.href, location.origin).searchParams.get("section"), "articles");
    history.replaceState({}, "", "?view=overview");
  });

});

describe('state.tsx — pageHeader', function() {

  it('renders a page header with kicker, title, description', function() {
    const result = pageHeader('Kicker', 'Title', 'Description');
    assert.ok(result.includes('rw-page-header'));
    assert.ok(result.includes('rw-page-header__kicker'));
    assert.ok(result.includes('rw-page-header__description'));
    assert.ok(result.includes('Kicker'));
    assert.ok(result.includes('Title'));
    assert.ok(result.includes('Description'));
  });

  it('includes extra content when provided', function() {
    const result = pageHeader('K', 'T', 'D', h("a", null, "link"));
    assert.ok(result.includes('<a>link</a>'));
  });

  it('escapes HTML in inputs', function() {
    const result = pageHeader('<script>', 'Title', 'Description');
    assert.ok(!result.includes('<script>'));
    assert.ok(result.includes('&lt;script&gt;'));
  });

});

describe('state.tsx — breadcrumb', function() {

  it('returns empty string when no context is set', function() {
    assert.equal(breadcrumb([]), '');
  });

  it('renders an explicit ordered page hierarchy', function() {
    const result = breadcrumb([{ label: 'Home', href: '?view=home' }, { label: 'Deepdive', href: '?view=overview' }, { label: 'Corpus' }]);
    assert.ok(result.includes('rw-breadcrumb'));
    assert.ok(result.includes('Home'));
    assert.ok(result.includes('Deepdive'));
    assert.ok(result.includes('Corpus'));
    assert.equal((result.match(/class="divider"/g) || []).length, 2);
  });

  it('marks only the final item as the current page and mounts it in the shell', function() {
    const items = [{ label: 'Home', href: '?view=home' }, { label: 'Article revision' }];
    const result = breadcrumb(items);
    assert.equal((result.match(/aria-current="page"/g) || []).length, 1);
    setBreadcrumb(items);
    assert.ok(document.querySelector('#workspace-breadcrumb')!.textContent.includes('Article revision'));
  });

});

describe('state.tsx — emptyState', function() {

  it('renders an empty state with page header and panel', function() {
    const result = emptyState('Title', 'Detail');
    assert.ok(result.includes('Read-only workspace'));
    assert.ok(result.includes('Title'));
    assert.ok(result.includes('Detail'));
  });

});

describe('state.tsx — table', function() {

  it('renders a table with header and rows', function() {
    const columns = ['name', 'age'];
    const rows = [{ name: 'Alice', age: 30 }, { name: 'Bob', age: 25 }];
    const result = table('People', 'A list', columns, rows);
    assert.ok(result.includes('People'));
    assert.ok(result.includes('Alice'));
    assert.ok(result.includes('Bob'));
    assert.ok(result.includes('<th scope="col">'));
    assert.ok(result.includes('<td>'));
    assert.ok(result.includes('<tr>'));
  });

  it('renders empty state when no rows', function() {
    const result = table('Empty', 'No data', ['col'], []);
    assert.ok(result.includes('No records.'));
  });

});

describe('state.tsx — subnav', function() {

  it('renders subnavigation links', function() {
    const items: Array<[string, string]> = [['a', 'Alpha'], ['b', 'Beta']];
    const result = subnav(items, 'a', 'section');
    assert.ok(result.includes('ui tabular menu'));
    assert.ok(result.includes('item active'));
    assert.ok(result.includes('Alpha'));
    assert.ok(result.includes('Beta'));
    assert.ok(result.includes('aria-current="page"'));
  });

});

describe('state.tsx — metricCard', function() {

  it('renders a metric card with value', function() {
    const result = metricCard('Articles', { value: 42 });
    assert.ok(result.includes('ui statistic'));
    assert.ok(result.includes('Articles'));
    assert.ok(result.includes('42'));
  });

  it('renders unavailable state', function() {
    const result = metricCard('Articles', { available: false });
    assert.ok(result.includes('Not recorded'));
  });

  it('includes href when provided', function() {
    const result = metricCard('Articles', 42, '?view=corpus');
    assert.ok(result.includes('href="?view=corpus"'));
  });

  it('shows percentage when denominator is present', function() {
    const result = metricCard('Test', { value: 10, denominator: 100 });
    assert.ok(result.includes('10 of'));
    assert.ok(result.includes('100'));
  });

});

describe('state.tsx — flowStage', function() {

  it('renders a flow stage with count', function() {
    const result = flowStage("Parsed", 100, 200, null, "", {});
    assert.ok(result.includes('rw-flow__step'));
    assert.ok(result.includes('Parsed'));
    assert.ok(result.includes('100'));
    assert.ok(result.includes('50.00%'));
  });

  it('shows input baseline for null previous', function() {
    const result = flowStage("Input", 100, 100, null, "", {});
    assert.ok(result.includes('Input baseline'));
  });

  it('shows diff from prior', function() {
    const result = flowStage("Parsed", 80, 100, 100, "", {});
    assert.ok(result.includes('from prior'));
  });

  it('handles unavailable state', function() {
    const result = flowStage("Test", { available: false }, 100, null, "", {});
    assert.ok(result.includes('Not recorded'));
  });

  it('handles null raw', function() {
    const result = flowStage("Test", null, 100, null, "", {});
    assert.ok(result.includes('Not recorded'));
  });

});

describe('state.tsx — retentionFlow', function() {

  it('renders not recorded when input is missing', function() {
    const result = retentionFlow({ retention_funnel: {} });
    assert.ok(result.includes('Not recorded'));
  });

  it('renders not recorded when input is unavailable', function() {
    const result = retentionFlow({ retention_funnel: { input_records: { available: false } } });
    assert.ok(result.includes('Not recorded'));
  });

  it('renders full retention flow with valid data', function() {
    const overview = {
      retention_funnel: {
        input_records: 1000,
        parsed_articles: 950,
        deduplicated_articles: 900,
        valid_articles: 850,
        discarded_articles: 50,
      }
    };
    const result = retentionFlow(overview);
    assert.ok(result.includes('Retention flow'));
    // formatNumber(1000) uses toLocaleString() — locale-dependent
    assert.ok(result.includes('Input records') || result.includes('rw-flow'));
    assert.ok(result.includes('50'));
    assert.ok(result.includes('85') || result.includes('90'));
  });

  it('uses the initial unfiltered source total for every retention percentage', function() {
    const overview = {
      source_filter_counts: [
        { source: 'alpha', filters: ['NO_FILTER'], count: 100 },
        { source: 'alpha', filters: ['NO_FILTER', 'RANGE_10_YEARS'], count: 60 },
        { source: 'alpha', filters: ['NO_FILTER', 'RANGE_10_YEARS', 'ARTICLE_ONLY'], count: 30 },
        { source: 'alpha', filters: ['NO_FILTER', 'RANGE_10_YEARS', 'ARTICLE_ONLY', 'ENGLISH_ONLY'], count: 25 },
        { source: 'beta', filters: ['NO_FILTER'], count: 200 },
        { source: 'beta', filters: ['NO_FILTER', 'RANGE_10_YEARS'], count: 100 },
        { source: 'beta', filters: ['NO_FILTER', 'RANGE_10_YEARS', 'ARTICLE_ONLY'], count: 50 },
        { source: 'beta', filters: ['NO_FILTER', 'RANGE_10_YEARS', 'ARTICLE_ONLY', 'ENGLISH_ONLY'], count: 45 },
      ],
      retention_funnel: {
        input_records: 68,
        parsed_articles: 60,
        deduplicated_articles: 50,
        valid_articles: 45,
        discarded_articles: 5,
      },
      enrichment_breakdown: { enrichment_candidates: 50 },
      normalization_breakdown: { normalized_articles_processed: 45 },
    };

    document.body.innerHTML = retentionFlow(overview);
    assert.equal(document.querySelectorAll('.rw-retention__phase').length, 3);
    assert.deepEqual(Array.from(document.querySelectorAll('.rw-retention__phase-header h4'), function(label) {
      return label.textContent;
    }), ['Source selection', 'Pipeline processing', 'Corpus enrichment']);
    assert.equal(document.querySelectorAll('.rw-flow--source > .rw-flow__step').length, 4);
    assert.equal(document.querySelectorAll(`[data-retention-phase="pipeline"] > .rw-flow > .rw-flow__step`).length, 3);
    assert.equal(document.querySelectorAll(`[data-retention-phase="corpus"] > .rw-flow > .rw-flow__step`).length, 3);
    assert.match((document.querySelector(`[data-retention-phase="source"] .ui.label`) as HTMLElement).textContent, /300 initial raw results across 2 sources/);
    assert.equal((document.querySelector('[data-flow-stage="input_records"] .rw-flow__percentage') as HTMLElement).textContent, '22.67%');
    assert.equal((document.querySelector('[data-flow-stage="parsed_articles"] .rw-flow__percentage') as HTMLElement).textContent, '20.00%');
    assert.equal((document.querySelector('[data-flow-stage="deduplicated_articles"] .rw-flow__percentage') as HTMLElement).textContent, '16.67%');
    assert.equal((document.querySelector('[data-flow-stage="enrichment_candidates"] .rw-flow__percentage') as HTMLElement).textContent, '16.67%');
    assert.equal((document.querySelector('[data-flow-stage="validation_outcomes"] .rw-flow__percentage') as HTMLElement).textContent, '16.67%');
    assert.deepEqual(Array.from(document.querySelectorAll('[data-flow-stage="validation_outcomes"] .rw-flow__outcome-values span'), function(item) {
      return item.textContent;
    }), ['45 accepted', '5 discarded']);
    assert.equal((document.querySelector('[data-flow-stage="normalized_articles_processed"] .rw-flow__percentage') as HTMLElement).textContent, '15.00%');
    assert.ok((document.querySelector('[data-flow-stage="parsed_articles"] a') as HTMLAnchorElement).href.includes('stage_q=parse'));
    assert.ok((document.querySelector('[data-flow-stage="input_records"] a') as HTMLAnchorElement).href.includes('section=sources'));
    assert.equal(document.querySelectorAll('.rw-flow__info').length, 10);
  });

  it('does not repeat a shorter source sequence into later aggregate stages', function() {
    const overview = {
      source_filter_counts: [
        { source: 'alpha', filters: ['NO_FILTER'], count: 100 },
        { source: 'alpha', filters: ['NO_FILTER', 'RANGE_10_YEARS'], count: 60 },
        { source: 'alpha', filters: ['NO_FILTER', 'RANGE_10_YEARS', 'ARTICLE_ONLY'], count: 30 },
        { source: 'beta', filters: ['NO_FILTER'], count: 200 },
        { source: 'beta', filters: ['NO_FILTER', 'RANGE_10_YEARS'], count: 100 },
      ],
      retention_funnel: { input_records: 68, parsed_articles: 60, deduplicated_articles: 50, valid_articles: 45, discarded_articles: 5 },
      enrichment_breakdown: { enrichment_candidates: 50 },
      normalization_breakdown: { normalized_articles_processed: 45 },
    };

    document.body.innerHTML = retentionFlow(overview);
    const sourceValues = Array.from(document.querySelectorAll('.rw-flow--source .rw-flow__step')).map(function(stage) {
      return (stage.querySelector('.rw-flow__value strong, .rw-flow__unavailable') as HTMLElement).textContent;
    });
    assert.deepEqual(sourceValues, ['300', '160', 'Not recorded', 'Not recorded']);
  });

});

describe('state.tsx — breakdown', function() {

  it('renders not recorded for empty entries', function() {
    const result = breakdown('Test', null, 'Count');
    assert.ok(result.includes('Not recorded'));
  });

  it('renders a breakdown table with entries', function() {
    const source = { crossref: { value: 100 }, openalex: { value: 50 } };
    const result = breakdown('Sources', source, 'Count');
    assert.ok(result.includes('Sources'));
    assert.ok(result.includes('Crossref'));
    assert.ok(result.includes('Openalex'));
  });

  it('renders with total when useTotal is true', function() {
    const source = { a: { value: 10 }, b: { value: 20 } };
    const result = breakdown('Test', source, 'Count', true);
    assert.ok(result.includes('a'));
    assert.ok(result.includes('b'));
  });

});

describe('state.tsx — sourceResultCountSummary', function() {

  it('renders a table with source counts', function() {
    const items = [
      { source_name: 'PubMed', expected_result_count: 100, observed_result_count: 95, result_count_comparison: 'consistent' },
    ];
    const result = sourceResultCountSummary(items);
    assert.ok(result.includes('Source export counts'));
    assert.ok(result.includes('PubMed'));
    assert.ok(result.includes('100'));
    assert.ok(result.includes('95'));
  });

  it('handles empty items', function() {
    const result = sourceResultCountSummary([]);
    assert.ok(result.includes('No records.'));
  });

});

describe('state.tsx — timeline', function() {

  it('renders empty state for no rows', function() {
    assert.equal(timeline([]), '<p class="ui faded text">No records.</p>');
  });

  it('renders timeline items', function() {
    const events = [
      { action: 'pipeline_started', entity_type: 'run', occurred_at: '2024-01-15T10:00:00Z' },
    ];
    const result = timeline(events);
    assert.ok(result.includes('ui feed'));
    assert.ok(result.includes('pipeline_started'));
    assert.ok(result.includes('pipeline-dot'));
  });

  it('renders enrichment detail', function() {
    const events = [
      { action: 'field_enriched', metadata_json: { field: 'title', provider: 'crossref' }, occurred_at: '2024-01-15T10:00:00Z' },
    ];
    const result = timeline(events);
    assert.ok(result.includes('field_enriched'));
    assert.ok(result.includes('enrich-dot'));
    assert.ok(result.includes('title'));
    assert.ok(result.includes('crossref'));
  });

  it('renders validation detail', function() {
    const events = [
      { action: 'validation_discarded', metadata_json: { reasons: ['Invalid DOI', 'Missing title'] }, occurred_at: '2024-01-15T10:00:00Z' },
    ];
    const result = timeline(events);
    assert.ok(result.includes('validation_discarded'));
    assert.ok(result.includes('validation-dot'));
    assert.ok(result.includes('Invalid DOI'));
  });

  it('renders error detail', function() {
    const events = [
      { action: 'fetch_failed', metadata_json: { error: 'Connection timeout' }, occurred_at: '2024-01-15T10:00:00Z' },
    ];
    const result = timeline(events);
    assert.ok(result.includes('Error:'));
    assert.ok(result.includes('Connection timeout'));
  });

  it('renders status detail', function() {
    const events = [
      { action: 'status_update', metadata_json: { status: 'running' }, occurred_at: '2024-01-15T10:00:00Z' },
    ];
    const result = timeline(events);
    assert.ok(result.includes('Status: running'));
  });

  it('renders identity detail', function() {
    const events = [
      { action: 'identity_resolved', metadata_json: { identity: '0000-0001-2345-6789' }, occurred_at: '2024-01-15T10:00:00Z' },
    ];
    const result = timeline(events);
    assert.ok(result.includes('0000-0001-2345-6789'));
  });

  it('renders search detail with revision', function() {
    const events = [
      { action: 'search_executed', metadata_json: { search_id: 's1', revision: 'r1' }, occurred_at: '2024-01-15T10:00:00Z' },
    ];
    const result = timeline(events);
    assert.ok(result.includes('Search: s1'));
    assert.ok(result.includes('revision r1'));
  });

  it('includes actor when present', function() {
    const events = [
      { action: 'test', actor: 'system', occurred_at: '2024-01-15T10:00:00Z' },
    ];
    const result = timeline(events);
    assert.ok(result.includes('system'));
    assert.ok(result.includes('class="user"'));
  });

  it('includes entity when present', function() {
    const events = [
      { action: 'test', entity_type: 'work', entity_id: 'w1', occurred_at: '2024-01-15T10:00:00Z' },
    ];
    const result = timeline(events);
    assert.ok(result.includes('work'));
    assert.ok(result.includes('w1'));
    assert.ok(result.includes('class="extra"'));
  });

});

describe('state.tsx — detailTable', function() {

  it('renders a table from an array of records', function() {
    const rows = [{ name: 'Alice', age: 30 }, { name: 'Bob', age: 25 }];
    const result = detailTable('People', { items: rows });
    assert.ok(result.includes('People'));
    assert.ok(result.includes('Alice'));
    assert.ok(result.includes('Bob'));
    assert.ok(result.includes('name'));
    assert.ok(result.includes('age'));
  });

  it('handles empty rows', function() {
    const result = detailTable('Empty', { items: [] });
    assert.ok(result.includes('No records.'));
  });

});

describe('state.tsx — cell', function() {

  it('renders NULL for null/undefined', function() {
    const result = cell(null, 'col');
    assert.ok(result.includes('NULL'));
    assert.equal(cell(undefined, 'col'), cell(null, 'col'));
  });

  it('renders short values inline', function() {
    const result = cell('hello', 'col');
    assert.ok(result.includes('hello'));
    assert.ok(!result.includes('details'));
  });

  it('truncates long values with details', function() {
    const long = 'x'.repeat(200);
    const result = cell(long, 'col');
    assert.ok(result.includes('details'));
    assert.ok(result.includes('…'));
  });

  it('creates article link for article_id column', function() {
    const result = cell('a1', 'article_id');
    assert.ok(result.includes('href='));
    assert.ok(result.includes('article_id=a1'));
  });

  it('creates author link for author_id column', function() {
    const result = cell('auth1', 'author_id');
    assert.ok(result.includes('href='));
    assert.ok(result.includes('author_id=auth1'));
  });

  it('creates reference link for reference_id column', function() {
    const result = cell('ref1', 'reference_id');
    assert.ok(result.includes('href='));
    assert.ok(result.includes('reference_id=ref1'));
  });

  it('creates article link for work_revision id with tableName', function() {
    const result = cell('w1', 'id', 'work_revisions');
    assert.ok(result.includes('article_id=w1'));
  });

  it('creates author link for author_occurrences id with tableName', function() {
    const result = cell('a1', 'id', 'author_occurrences');
    assert.ok(result.includes('author_id=a1'));
  });

  it('creates reference link for reference_mentions id with tableName', function() {
    const result = cell('r1', 'id', 'reference_mentions');
    assert.ok(result.includes('reference_id=r1'));
  });

});

describe('state.tsx — bindCopyButtons', function() {

  beforeEach(function() {
    document.querySelectorAll('[data-copy-text]').forEach(function(el) { el.remove(); });
  });

  it('copies text and shows Copied! feedback', async function() {
    var btn = document.createElement('button');
    btn.setAttribute('data-copy-text', 'hello');
    btn.textContent = 'Copy';
    document.body.appendChild(btn);

    // Mock clipboard
    var written = '';
    var originalClipboard = navigator.clipboard;
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: function(text: string) { written = text; return Promise.resolve(); } },
      configurable: true,
    });

    bindCopyButtons();
    btn.click();
    // Allow microtask to flush
    await new Promise(function(resolve) { setTimeout(resolve, 10); });
    assert.equal(written, 'hello');
    assert.equal(btn.textContent, 'Copied!');

    Object.defineProperty(navigator, 'clipboard', { value: originalClipboard, configurable: true });
    btn.remove();
  });

  it('falls back to prompt when clipboard fails', async function() {
    var btn = document.createElement('button');
    btn.setAttribute('data-copy-text', 'fail');
    btn.textContent = 'Copy';
    document.body.appendChild(btn);

    var originalClipboard = navigator.clipboard;
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: function() { return Promise.reject(new Error('denied')); } },
      configurable: true,
    });

    var originalPrompt = globalThis.prompt;
    var prompted = '';
    globalThis.prompt = function(_msg: string, val?: string) { prompted = val ?? ''; return ''; } as typeof prompt;

    bindCopyButtons();
    btn.click();
    await new Promise(function(resolve) { setTimeout(resolve, 10); });
    assert.equal(prompted, 'fail');

    globalThis.prompt = originalPrompt;
    Object.defineProperty(navigator, 'clipboard', { value: originalClipboard, configurable: true });
    btn.remove();
  });

});

describe('state.tsx — bindDismissibleMessages', function() {

  beforeEach(function() {
    document.querySelectorAll('.ui.message').forEach(function(el) { el.remove(); });
  });

  it('fades out and hides message on close click', async function() {
    var msg = document.createElement('div');
    msg.className = 'ui message';
    msg.innerHTML = '<button class="close">&times;</button><span>Content</span>';
    document.body.appendChild(msg);

    bindDismissibleMessages();
    var close = msg.querySelector('.close') as HTMLButtonElement;
    close.click();

    assert.equal(msg.style.opacity, '0');
    // hidden is set after 150ms timeout
    await new Promise(function(resolve) { setTimeout(resolve, 160); });
    assert.equal(msg.hidden, true);
    msg.remove();
  });

});

describe('state.tsx — bindLoadingButtons', function() {

  beforeEach(function() {
    document.querySelectorAll('[data-loading]').forEach(function(el) { el.remove(); });
  });

  it('adds loading class and disables button on click', function() {
    var btn = document.createElement('button');
    btn.setAttribute('data-loading', '');
    btn.textContent = 'Submit';
    document.body.appendChild(btn);

    bindLoadingButtons();
    btn.click();

    assert.ok(btn.classList.contains('loading'));
    assert.equal(btn.disabled, true);
    btn.remove();
  });

});
