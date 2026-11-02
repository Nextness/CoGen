// Unit tests for state.tsx: shared state, DOM references, and utility components.
import { describe, it, beforeEach } from "node:test";
import assert from "node:assert/strict";

// Setup DOM before importing the module under test
import "../../../support/node/setup.ts";
import { seedViewerState } from "../../../support/node/seed.ts";
import type { HierarchyRun } from "../../../../frontend/src/api/types.ts";
import {
  app, notice, loading, state, pageSizes, corpusSections, provenanceSections, graphFilters,
  params, value, view, section, detailOrigin, viewPage, asJSON, list, pickID, text, numericEvidence, number, formatNumber,
  percent, formatTime, formatDate, formatDuration, formatBytes, humanLabel, parseObject, statusClass, StatusChip, metricEntries, selectedRun, showError,
  clearError, busy, link, linkTargetFor, contextChange, PageHeader, Breadcrumb, setBreadcrumb, EmptyState, Table, Subnav,
  FilterChips, MetricCard, FlowStage, RetentionFlow, Breakdown, SourceResultCountSummary,
  Cell, bindCopyButtons,
  initViewerState, pathView, pathFor, stateFor,
} from "../../../../frontend/src/state.tsx";
import { h } from "../../../../frontend/src/jsx/jsx-runtime.ts";
import { renderToString } from "../../../support/node/helpers/jsx-render.ts";

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
const cell = (item: any, column: string, tableName?: string): string => renderToString(Cell({ item: item, column: column, tableName: tableName }));

describe("state.tsx — constants", () => {

  it("maps supported views to their clean application paths", () => {
    assert.equal(viewPage.home, "/");
    assert.equal(viewPage.overview, "/overview");
    assert.equal(viewPage.article, "/article");
  });

  it("app is a DOM element", () => {
    assert.ok(app instanceof HTMLElement);
    assert.equal(app.id, "app");
  });

  it("notice is a DOM element", () => {
    assert.ok(notice instanceof HTMLElement);
    assert.equal(notice.id, "notice");
  });

  it("loading is a DOM element", () => {
    assert.ok(loading instanceof HTMLElement);
    assert.equal(loading.id, "loading");
  });

  it("state has expected keys", () => {
    assert.ok(Array.isArray(state.searches));
    assert.ok(Array.isArray(state.plans));
    assert.ok(Array.isArray(state.runs));
    assert.ok(Array.isArray(state.tables));
    assert.equal(typeof state.request, "number");
    assert.equal(state.controller, null);
  });

  it("pageSizes contains expected values", () => {
    assert.deepEqual(pageSizes, [20, 50, 100, 200, 500]);
  });

  it("corpusSections has expected keys", () => {
    assert.ok(corpusSections.articles);
    assert.ok(corpusSections.authors);
    assert.ok(corpusSections.references);
    assert.ok(corpusSections.identity_evidence);
    assert.ok(corpusSections.sources);
    assert.equal(corpusSections.articles.table, "work_revisions");
  });

  it("provenanceSections has expected keys", () => {
    assert.ok(provenanceSections.audit);
    assert.ok(provenanceSections.artifacts);
    assert.ok(provenanceSections.cache);
    assert.ok(provenanceSections.stages);
    assert.ok(provenanceSections.run);
    assert.equal(provenanceSections.audit[0], "Audit log");
  });

  it("graphFilters contains expected filters", () => {
    assert.ok(graphFilters.includes("mode"));
    assert.ok(graphFilters.includes("q"));
    assert.ok(graphFilters.includes("author"));
    assert.ok(graphFilters.includes("article_limit"));
  });

});

describe("state.tsx — params / value / view / section", () => {

  it("params returns URLSearchParams from viewerState", () => {
    seedViewerState({ view: "overview" });
    const p = params();
    assert.ok(p instanceof URLSearchParams);
    assert.equal(p.get("view"), "overview");
  });

  it("value reads a viewerState key", () => {
    seedViewerState({ view: "overview" });
    assert.equal(value("view"), "overview");
    assert.equal(value("nonexistent"), "");
  });

  it("view returns the current view or the pathname-derived default", () => {
    seedViewerState({ view: "overview" });
    assert.equal(view(), "overview");
    history.replaceState({}, "", "/");
    initViewerState();
    assert.equal(view(), "home");
    seedViewerState({ view: "overview" });
  });

  it("section returns the named parameter or fallback", () => {
    seedViewerState({ view: "overview" });
    assert.equal(section("section", "default"), "default");
    assert.equal(section("view", "fallback"), "overview");
  });

});

describe("state.tsx — pathView / pathFor / initViewerState", () => {

  it("pathView derives the view from a pathname", () => {
    assert.equal(pathView("/overview"), "overview");
    assert.equal(pathView("/overview.html"), "overview");
    assert.equal(pathView("/trash"), "trash");
    assert.equal(pathView("/"), "home");
    assert.equal(pathView(""), "home");
    assert.equal(pathView("/corpus/"), "corpus");
  });

  it("pathFor returns the owning path for a state view", () => {
    assert.equal(pathFor({ view: "overview" }), "/overview");
    assert.equal(pathFor({ view: "trash" }), "/trash");
    assert.equal(pathFor({}), "/");
    assert.equal(pathFor({ view: "unsupported" }), "/");
  });

  it("initViewerState adopts history.state over sessionStorage and forces the view from the pathname", () => {
    history.replaceState({ view: "corpus", section: "articles", search_id: "1" }, "", "/corpus");
    initViewerState();
    assert.equal(view(), "corpus");
    assert.equal(value("section"), "articles");
    assert.equal(value("search_id"), "1");
    assert.equal(sessionStorage.getItem("rw-viewer-state"), JSON.stringify({ view: "corpus", section: "articles", search_id: "1" }));
    assert.equal(location.pathname, "/corpus");
  });

  it("initViewerState drops disallowed keys from the adopted state", () => {
    history.replaceState({ view: "corpus", section: "articles", q: "term", note_id: "9" }, "", "/corpus");
    initViewerState();
    assert.equal(value("section"), "articles");
    assert.equal(value("q"), "term");
    assert.equal(value("note_id"), "");
    assert.equal(value("search_id"), "");
  });

  it("initViewerState corrects the view from the pathname and keeps only adopted canonical context", () => {
    history.replaceState({ view: "article", article_id: "8", note_id: "9", search_id: "1", run_id: "4" }, "", "/overview");
    initViewerState();
    assert.equal(view(), "overview");
    assert.equal(value("article_id"), "");
    assert.equal(value("note_id"), "");
    assert.equal(value("run_id"), "4");
  });

});

describe("state.tsx — asJSON", () => {

  it("returns strings unchanged", () => {
    assert.equal(asJSON("hello"), "hello");
  });

  it("formats objects as pretty JSON", () => {
    const result = asJSON({ a: 1, b: 2 });
    assert.ok(result.includes(`"a": 1`));
    assert.ok(result.includes(`"b": 2`));
  });

  it("formats arrays as pretty JSON", () => {
    const result = asJSON([1, 2, 3]);
    assert.ok(result.includes("1"));
  });

  it("handles null", () => {
    assert.equal(asJSON(null), "null");
  });

});

describe("state.tsx — list", () => {

  it("returns the first matching key from data", () => {
    const data = { items: [1, 2, 3], rows: [4, 5] };
    assert.deepEqual(list(data, ["items", "rows"]), [1, 2, 3]);
  });

  it("returns data if it is an array", () => {
    assert.deepEqual(list([1, 2], ["items"]), [1, 2]);
  });

  it("returns empty array for non-array data", () => {
    assert.deepEqual(list(null, ["items"]), []);
    assert.deepEqual(list({}, ["items"]), []);
    assert.deepEqual(list(undefined, ["items"]), []);
  });

  it("returns empty array when no keys match", () => {
    assert.deepEqual(list({ items: "not array" }, ["items"]), []);
  });

});

describe("state.tsx — pickID", () => {

  it("prefers .id", () => {
    assert.equal(pickID({ id: "abc", search_id: "xyz" }), "abc");
  });

  it("falls back to search_id", () => {
    assert.equal(pickID({ search_id: "s1" }), "s1");
  });

  it("falls back to run_id", () => {
    assert.equal(pickID({ run_id: "r1" }), "r1");
  });

  it("falls back to plan_id", () => {
    assert.equal(pickID({ plan_id: "p1" }), "p1");
  });

  it("returns empty string when no id found", () => {
    assert.equal(pickID({}), "");
    assert.equal(pickID(null), "");
    assert.equal(pickID(undefined), "");
  });

});

describe("state.tsx — text", () => {

  it("returns the first non-empty field", () => {
    assert.equal(text({ a: "", b: "hello", c: "world" }, ["a", "b", "c"]), "hello");
  });

  it("returns Unnamed when all fields are empty", () => {
    assert.equal(text({ a: "", b: null, c: undefined }, ["a", "b", "c"]), "Unnamed");
  });

  it("returns Unnamed for null/undefined item", () => {
    assert.equal(text(null, ["name"]), "Unnamed");
    assert.equal(text(undefined, ["name"]), "Unnamed");
  });

});

describe("state.tsx — number", () => {

  it("parses numeric values", () => {
    assert.equal(number(42), 42);
    assert.equal(number("42"), 42);
  });

  it("does not conflate unavailable or invalid values with recorded zero", () => {
    assert.equal(Number.isNaN(number(null)), true);
    assert.equal(Number.isNaN(number(undefined)), true);
    assert.equal(Number.isNaN(number("abc")), true);
    assert.deepEqual(numericEvidence(null), { state: "unavailable", value: null });
    assert.deepEqual(numericEvidence("abc"), { state: "invalid", value: null });
    assert.deepEqual(numericEvidence(0), { state: "recorded", value: 0 });
    assert.deepEqual(numericEvidence({ state: "derived", value: 0 }), { state: "derived", value: 0 });
  });

  it("handles { value } objects", () => {
    assert.equal(number({ value: 42 }), 42);
    assert.equal(Number.isNaN(number({ value: null })), true);
  });

  it("returns NaN for NaN and Infinity", () => {
    assert.equal(Number.isNaN(number(NaN)), true);
    assert.equal(Number.isNaN(number(Infinity)), true);
  });

});

describe("state.tsx — formatNumber", () => {

  it("formats numbers with locale separators", () => {
    const result = formatNumber(1234567);
    assert.ok(typeof result === "string");
    assert.ok(result.length > 0);
  });

  it("handles zero", () => {
    assert.equal(formatNumber(0), "0");
  });

  it("handles object values", () => {
    const result = formatNumber({ value: 1000 });
    assert.ok(typeof result === "string");
  });

  it("labels unavailable and invalid evidence explicitly", () => {
    assert.equal(formatNumber(null), "Not recorded");
    assert.equal(formatNumber("not-a-number"), "Invalid value");
  });

});

describe("state.tsx — percent", () => {

  it("calculates percentage", () => {
    assert.equal(percent(25, 100), "25.0%");
  });

  it("returns em dash for zero denominator", () => {
    assert.equal(percent(25, 0), "—");
  });

  it("handles object values", () => {
    assert.equal(percent({ value: 50 }, { value: 200 }), "25.0%");
  });

});

describe("state.tsx — formatTime", () => {

  it("returns em dash for falsy input", () => {
    assert.equal(formatTime(null), "—");
    assert.equal(formatTime(undefined), "—");
    assert.equal(formatTime(""), "—");
  });

  it("returns the raw string for invalid dates", () => {
    assert.equal(formatTime("not-a-date"), "not-a-date");
  });

  it("formats valid date strings", () => {
    const result = formatTime("2024-01-15T10:30:00Z");
    assert.ok(typeof result === "string");
    assert.ok(result.length > 0);
    assert.notEqual(result, "—");
  });

  it("uses the documented English UTC calendar policy at a day boundary", () => {
    assert.equal(formatDate("2024-01-01T00:30:00+14:00"), "Dec 31, 2023");
    assert.equal(formatTime("2024-01-01T00:30:00+14:00"), "Dec 31, 2023, 10:30:00 AM");
  });

});

describe("state.tsx — formatDuration", () => {

  it("formats recorded elapsed time", () => {
    assert.equal(formatDuration("2024-01-01T00:00:00Z", "2024-01-01T01:02:03Z"), "1h 2m 3s");
  });

  it("rejects missing, invalid, or reversed timestamps", () => {
    assert.equal(formatDuration("", "2024-01-01T00:00:00Z"), "—");
    assert.equal(formatDuration("invalid", "2024-01-01T00:00:00Z"), "—");
    assert.equal(formatDuration("2024-01-02T00:00:00Z", "2024-01-01T00:00:00Z"), "—");
  });

});

describe("state.tsx — display helpers", () => {

  it("formats byte counts without treating kilobytes as decimal units", () => {
    assert.equal(formatBytes(512), "512 B");
    assert.match(formatBytes(1536), /^1[.,]5 KB$/);
  });

  it("turns stored field names into readable labels", () => {
    assert.equal(humanLabel("request_fingerprint"), "Request Fingerprint");
  });

  it("parses object payloads and rejects scalar or malformed JSON", () => {
    assert.deepEqual(parseObject(`{"stage":"normalize"}`), { stage: "normalize" });
    assert.deepEqual(parseObject("not-json"), {});
    assert.deepEqual(parseObject("42"), {});
  });

  it("uses namespaced pagination updates when a filter chip is removed", () => {
    seedViewerState({ view: "provenance", section: "cache" });
    const html = filterChips({ cache_q: "crossref" }, { cache_q: "Search" }, {
      removeUpdates: { cache_page: 1 }, clearUpdates: { cache_q: "", cache_page: 1 }
    });
    const host = document.createElement("div");
    host.innerHTML = html;
    const chip = host.querySelector(".rw-filter-chip") as HTMLAnchorElement;
    const state = JSON.parse(chip.dataset.state || "{}");
    assert.equal(state.cache_page, "1");
    assert.equal(state.page, undefined);
    assert.equal(chip.getAttribute("href"), "/provenance");
  });

  it("renders one independently removable chip for each selected facet value", () => {
    seedViewerState({ view: "provenance", section: "audit" });
    const html = filterChips({ audit_category: ["pipeline", "review"] }, { audit_category: "Category" });
    const host = document.createElement("div");
    host.innerHTML = html;
    const chips = Array.from(host.querySelectorAll<HTMLAnchorElement>(".rw-filter-chip"));
    assert.equal(chips.length, 2);
    assert.match(chips[0].textContent || "", /pipeline/);
    assert.equal(JSON.parse(chips[0].dataset.state || "{}").audit_category, "review");
    assert.equal(JSON.parse(chips[1].dataset.state || "{}").audit_category, "pipeline");
  });

});

describe("state.tsx — statusClass", () => {

  it("returns red for failure-related statuses", () => {
    assert.equal(statusClass("fail"), "red");
    assert.equal(statusClass("discard"), "red");
    assert.equal(statusClass("error"), "red");
    assert.equal(statusClass("trash"), "red");
    assert.equal(statusClass("invalid"), "red");
  });

  it("returns green for completion-related statuses", () => {
    assert.equal(statusClass("complete"), "green");
    assert.equal(statusClass("valid"), "green");
    assert.equal(statusClass("success"), "green");
    assert.equal(statusClass("hit"), "green");
    assert.equal(statusClass("ready"), "green");
  });

  it("returns orange for warning-like statuses", () => {
    assert.equal(statusClass("warning"), "orange");
    assert.equal(statusClass("skipped"), "orange");
    assert.equal(statusClass("stale"), "orange");
    assert.equal(statusClass("unresolved"), "orange");
    assert.equal(statusClass("unmatched"), "orange");
    assert.equal(statusClass("orcid_is_unclear"), "orange");
    assert.equal(statusClass("not_evaluated"), "orange");
  });

  it("returns blue for informational statuses and neutral for unrecorded values", () => {
    assert.equal(statusClass("pending"), "blue");
    assert.equal(statusClass("running"), "blue");
    assert.equal(statusClass("recorded"), "blue");
    assert.equal(statusClass(""), "grey");
    assert.equal(statusClass(null), "grey");
    assert.equal(statusClass("not_available"), "orange");
    assert.equal(statusClass("no_orcid_candidate"), "grey");
    assert.equal(statusClass("inherited"), "violet");
  });

});

describe("state.tsx — statusChip", () => {

  it("wraps status in a span with class", () => {
    const result = statusChip("complete");
    assert.ok(result.includes(`class="ui green label"`));
    assert.ok(result.includes("complete"));
  });

  it("uses Not recorded for null", () => {
    const result = statusChip(null);
    assert.ok(result.includes("Not recorded"));
  });

});

describe("state.tsx — metricEntries", () => {

  it("converts arrays of objects to entries", () => {
    const input = [
      { metric: "articles", value: 10, source: "crossref" },
      { metric: "authors", value: 5 },
    ];
    const result = metricEntries(input);
    assert.equal(result.length, 2);
    assert.equal(result[0][0], "articles (crossref)");
    assert.equal(result[1][0], "authors");
  });

  it("converts object to entries", () => {
    const result = metricEntries({ a: 1, b: 2 });
    assert.deepEqual(result, [["a", 1], ["b", 2]]);
  });

  it("returns empty array for null/undefined", () => {
    assert.deepEqual(metricEntries(null), []);
    assert.deepEqual(metricEntries(undefined), []);
  });

});

describe("state.tsx — selectedRun", () => {

  it("returns undefined when no run_id is set", () => {
    assert.equal(selectedRun(), undefined);
  });

  it("finds a run by run_id", () => {
    state.runs = [{ id: "run-1", status: "complete" } as unknown as HierarchyRun];
    seedViewerState({ view: "overview", run_id: "run-1" });

    const result = selectedRun();
    assert.ok(result);
    assert.equal(result.id, "run-1");
    state.runs = [];
  });

});

describe("state.tsx — showError / clearError / busy", () => {

  it("showError sets notice text and removes hidden", () => {
    notice.hidden = true;
    showError(new Error("test error"));
    assert.equal(notice.textContent, "test error");
    assert.equal(notice.hidden, false);
  });

  it("showError handles string errors", () => {
    showError("string error");
    assert.equal(notice.textContent, "string error");
  });

  it("clearError hides notice and clears text", () => {
    clearError();
    assert.equal(notice.hidden, true);
    assert.equal(notice.textContent, "");
  });

  it("busy toggles loading visibility", () => {
    busy(true);
    assert.equal(loading.hidden, false);
    busy(false);
    assert.equal(loading.hidden, true);
  });

});

describe("state.tsx — link", () => {

  it("builds a path from updates", () => {
    seedViewerState({ view: "overview" });
    const result = link({ view: "corpus", section: "articles" });
    assert.equal(result, "/corpus");
  });

  it("builds matching path and serialized state from one update", () => {
    seedViewerState({ view: "overview", run_id: "12" });
    const target = linkTargetFor({ view: "corpus", section: "authors" });
    assert.equal(target.href, "/corpus");
    assert.deepEqual(target.state, { view: "corpus", run_id: "12", section: "authors" });
  });

  it("removes keys with empty values but keeps the Home default", () => {
    seedViewerState({ view: "overview" });
    const result = link({ view: "" });
    assert.equal(result, "/");
  });

  it("removes keys with null values but keeps the Home default", () => {
    seedViewerState({ view: "overview" });
    const result = link({ view: null });
    assert.equal(result, "/");
  });

  it("ensures view defaults to Home", () => {
    seedViewerState({});
    const result = link({ section: "articles" });
    assert.equal(result, "/");
  });

  it("uses the current view path when no updates are supplied", () => {
    seedViewerState({ view: "overview" });
    const result = link();
    assert.equal(result, "/overview");
  });

  it("uses the home path for an unsupported destination", () => {
    seedViewerState({ view: "overview" });
    const result = link({ view: "unsupported" });
    assert.equal(result, "/");
  });

  it("keeps only canonical context and destination-owned route state", () => {
    seedViewerState({ view: "corpus", search_id: "1", search_revision_id: "2", plan_id: "3", run_id: "4", section: "articles", q: "term", page: "2", note_id: "9", audit_q: "stale" });

    const provenance = stateFor({ view: "provenance", section: "audit" });
    assert.equal(provenance.run_id, "4");
    assert.equal(provenance.section, "audit");
    assert.equal(provenance.q, undefined);
    assert.equal(provenance.page, undefined);
    assert.equal(provenance.note_id, undefined);

    const home = stateFor({ view: "home" });
    assert.deepEqual(Object.keys(home), ["view"]);
  });

  it("returns detail context changes to the corresponding collection without record focus", () => {
    seedViewerState({ view: "article", search_id: "1", search_revision_id: "2", plan_id: "3", run_id: "4", article_id: "8", note_id: "9", anchor_id: "a1", pdf_page: "2" });
    const updates = contextChange({ run_id: "5" });
    const target = stateFor(updates);
    assert.equal(target.view, "corpus");
    assert.equal(target.section, "articles");
    assert.equal(target.run_id, "5");
    assert.equal(target.article_id, undefined);
    assert.equal(target.note_id, undefined);
    assert.equal(target.anchor_id, undefined);
    assert.equal(target.pdf_page, undefined);
  });

  it("clears article-local note, anchor, and page focus when the article changes", () => {
    seedViewerState({ view: "article", run_id: "4", article_id: "8", note_id: "9", anchor_id: "a1", pdf_page: "2" });
    const target = stateFor({ view: "article", article_id: "10" });
    assert.equal(target.article_id, "10");
    assert.equal(target.note_id, undefined);
    assert.equal(target.anchor_id, undefined);
    assert.equal(target.pdf_page, undefined);
  });

  it("returns a detail origin through the origin view path", () => {
    const origin = new URLSearchParams({
      view: "corpus",
      search_id: "1",
      search_revision_id: "2",
      plan_id: "3",
      run_id: "4",
      section: "articles",
    });
    seedViewerState({
      view: "article",
      search_id: "1",
      search_revision_id: "2",
      plan_id: "3",
      run_id: "4",
      article_id: "8",
      origin: origin.toString(),
    });

    const result = detailOrigin();

    assert.ok(result);
    assert.equal(result.href, "/corpus");
    assert.equal(result.state.section, "articles");
  });

});

describe("state.tsx — pageHeader", () => {

  it("renders a page header with kicker, title, description", () => {
    const result = pageHeader("Kicker", "Title", "Description");
    assert.ok(result.includes("rw-page-header"));
    assert.ok(result.includes("rw-page-header__kicker"));
    assert.ok(result.includes("rw-page-header__description"));
    assert.ok(result.includes("Kicker"));
    assert.ok(result.includes("Title"));
    assert.ok(result.includes("Description"));
  });

  it("includes extra content when provided", () => {
    const result = pageHeader("K", "T", "D", h("a", null, "link"));
    assert.ok(result.includes("<a>link</a>"));
  });

  it("escapes HTML in inputs", () => {
    const result = pageHeader("<script>", "Title", "Description");
    assert.ok(!result.includes("<script>"));
    assert.ok(result.includes("&lt;script&gt;"));
  });

});

describe("state.tsx — breadcrumb", () => {

  it("returns empty string when no context is set", () => {
    assert.equal(breadcrumb([]), "");
  });

  it("renders an explicit ordered page hierarchy", () => {
    const result = breadcrumb([{ label: "Home", href: "?view=home" }, { label: "Deepdive", href: "?view=overview" }, { label: "Corpus" }]);
    assert.ok(result.includes("rw-breadcrumb"));
    assert.ok(result.includes("Home"));
    assert.ok(result.includes("Deepdive"));
    assert.ok(result.includes("Corpus"));
    assert.equal((result.match(/class="divider"/g) || []).length, 2);
  });

  it("marks only the final item as the current page and mounts it in the shell", () => {
    const items = [{ label: "Home", href: "?view=home" }, { label: "Article revision" }];
    const result = breadcrumb(items);
    assert.equal((result.match(/aria-current="page"/g) || []).length, 1);
    setBreadcrumb(items);
    assert.ok(document.querySelector("#workspace-breadcrumb")!.textContent.includes("Article revision"));
  });

});

describe("state.tsx — emptyState", () => {

  it("renders an empty state with page header and panel", () => {
    const result = emptyState("Title", "Detail");
    assert.ok(result.includes("Read-only workspace"));
    assert.ok(result.includes("Title"));
    assert.ok(result.includes("Detail"));
  });

});

describe("state.tsx — table", () => {

  it("renders a table with header and rows", () => {
    const columns = ["name", "age"];
    const rows = [{ name: "Alice", age: 30 }, { name: "Bob", age: 25 }];
    const result = table("People", "A list", columns, rows);
    assert.ok(result.includes("People"));
    assert.ok(result.includes("Alice"));
    assert.ok(result.includes("Bob"));
    assert.ok(result.includes(`<th scope="col">`));
    assert.ok(result.includes("<td>"));
    assert.ok(result.includes("<tr>"));
  });

  it("renders empty state when no rows", () => {
    const result = table("Empty", "No data", ["col"], []);
    assert.ok(result.includes("No records."));
  });

});

describe("state.tsx — subnav", () => {

  it("renders subnavigation links", () => {
    const items: Array<[string, string]> = [["a", "Alpha"], ["b", "Beta"]];
    const result = subnav(items, "a", "section");
    assert.ok(result.includes("ui tabular menu"));
    assert.ok(result.includes("item active"));
    assert.ok(result.includes("Alpha"));
    assert.ok(result.includes("Beta"));
    assert.ok(result.includes(`aria-current="page"`));
  });

});

describe("state.tsx — metricCard", () => {

  it("renders a metric card with value", () => {
    const result = metricCard("Articles", { value: 42 });
    assert.ok(result.includes("ui statistic"));
    assert.ok(result.includes("Articles"));
    assert.ok(result.includes("42"));
  });

  it("renders unavailable state", () => {
    const result = metricCard("Articles", { available: false });
    assert.ok(result.includes("Not recorded"));
  });

  it("includes href when provided", () => {
    const result = metricCard("Articles", 42, "?view=corpus");
    assert.ok(result.includes(`href="?view=corpus"`));
  });

  it("shows percentage when denominator is present", () => {
    const result = metricCard("Test", { value: 10, denominator: 100 });
    assert.ok(result.includes("10 of"));
    assert.ok(result.includes("100"));
  });

});

describe("state.tsx — flowStage", () => {

  it("renders a flow stage with count", () => {
    const result = flowStage("Parsed", 100, 200, null, "", {});
    assert.ok(result.includes("rw-flow__step"));
    assert.ok(result.includes("Parsed"));
    assert.ok(result.includes("100"));
    assert.ok(result.includes("50.00%"));
  });

  it("shows input baseline for null previous", () => {
    const result = flowStage("Input", 100, 100, null, "", {});
    assert.ok(result.includes("Input baseline"));
  });

  it("shows diff from prior", () => {
    const result = flowStage("Parsed", 80, 100, 100, "", {});
    assert.ok(result.includes("from prior"));
  });

  it("handles unavailable state", () => {
    const result = flowStage("Test", { available: false }, 100, null, "", {});
    assert.ok(result.includes("Not recorded"));
  });

  it("handles null raw", () => {
    const result = flowStage("Test", null, 100, null, "", {});
    assert.ok(result.includes("Not recorded"));
  });

});

describe("state.tsx — retentionFlow", () => {

  it("renders not recorded when input is missing", () => {
    const result = retentionFlow({ retention_funnel: {} });
    assert.ok(result.includes("Not recorded"));
  });

  it("renders not recorded when input is unavailable", () => {
    const result = retentionFlow({ retention_funnel: { input_records: { available: false } } });
    assert.ok(result.includes("Not recorded"));
  });

  it("renders full retention flow with valid data", () => {
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
    assert.ok(result.includes("Retention flow"));
    // formatNumber(1000) uses toLocaleString() — locale-dependent
    assert.ok(result.includes("Input records") || result.includes("rw-flow"));
    assert.ok(result.includes("50"));
    assert.ok(result.includes("85") || result.includes("90"));
  });

  it("uses the initial unfiltered source total for every retention percentage", () => {
    const overview = {
      source_filter_counts: [
        { source: "alpha", filters: ["NO_FILTER"], count: 100 },
        { source: "alpha", filters: ["NO_FILTER", "RANGE_10_YEARS"], count: 60 },
        { source: "alpha", filters: ["NO_FILTER", "RANGE_10_YEARS", "ARTICLE_ONLY"], count: 30 },
        { source: "alpha", filters: ["NO_FILTER", "RANGE_10_YEARS", "ARTICLE_ONLY", "ENGLISH_ONLY"], count: 25 },
        { source: "beta", filters: ["NO_FILTER"], count: 200 },
        { source: "beta", filters: ["NO_FILTER", "RANGE_10_YEARS"], count: 100 },
        { source: "beta", filters: ["NO_FILTER", "RANGE_10_YEARS", "ARTICLE_ONLY"], count: 50 },
        { source: "beta", filters: ["NO_FILTER", "RANGE_10_YEARS", "ARTICLE_ONLY", "ENGLISH_ONLY"], count: 45 },
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
    assert.equal(document.querySelectorAll(".rw-retention__phase").length, 3);
    assert.deepEqual(Array.from(document.querySelectorAll(".rw-retention__phase-header h4"), (label) => {
      return label.textContent;
    }), ["Source selection", "Pipeline processing", "Corpus enrichment"]);
    assert.equal(document.querySelectorAll(".rw-flow--source > .rw-flow__step").length, 4);
    assert.equal(document.querySelectorAll(`[data-retention-phase="pipeline"] > .rw-flow > .rw-flow__step`).length, 3);
    assert.equal(document.querySelectorAll(`[data-retention-phase="corpus"] > .rw-flow > .rw-flow__step`).length, 3);
    assert.match((document.querySelector(`[data-retention-phase="source"] .ui.label`) as HTMLElement).textContent, /300 initial raw results across 2 sources/);
    assert.equal((document.querySelector(`[data-flow-stage="input_records"] .rw-flow__percentage`) as HTMLElement).textContent, "22.67%");
    assert.equal((document.querySelector(`[data-flow-stage="parsed_articles"] .rw-flow__percentage`) as HTMLElement).textContent, "20.00%");
    assert.equal((document.querySelector(`[data-flow-stage="deduplicated_articles"] .rw-flow__percentage`) as HTMLElement).textContent, "16.67%");
    assert.equal((document.querySelector(`[data-flow-stage="enrichment_candidates"] .rw-flow__percentage`) as HTMLElement).textContent, "16.67%");
    assert.equal((document.querySelector(`[data-flow-stage="validation_outcomes"] .rw-flow__percentage`) as HTMLElement).textContent, "16.67%");
    assert.deepEqual(Array.from(document.querySelectorAll(`[data-flow-stage="validation_outcomes"] .rw-flow__outcome-values span`), (item) => {
      return item.textContent;
    }), ["45 accepted", "5 discarded"]);
    assert.equal((document.querySelector(`[data-flow-stage="normalized_articles_processed"] .rw-flow__percentage`) as HTMLElement).textContent, "15.00%");
    assert.ok((document.querySelector(`[data-flow-stage="parsed_articles"] a`) as HTMLAnchorElement).dataset.state!.includes(`"stage_q":"parse"`));
    assert.ok((document.querySelector(`[data-flow-stage="input_records"] a`) as HTMLAnchorElement).dataset.state!.includes(`"section":"sources"`));
    assert.equal(document.querySelectorAll(".rw-flow__info").length, 10);
  });

  it("does not repeat a shorter source sequence into later aggregate stages", () => {
    const overview = {
      source_filter_counts: [
        { source: "alpha", filters: ["NO_FILTER"], count: 100 },
        { source: "alpha", filters: ["NO_FILTER", "RANGE_10_YEARS"], count: 60 },
        { source: "alpha", filters: ["NO_FILTER", "RANGE_10_YEARS", "ARTICLE_ONLY"], count: 30 },
        { source: "beta", filters: ["NO_FILTER"], count: 200 },
        { source: "beta", filters: ["NO_FILTER", "RANGE_10_YEARS"], count: 100 },
      ],
      retention_funnel: { input_records: 68, parsed_articles: 60, deduplicated_articles: 50, valid_articles: 45, discarded_articles: 5 },
      enrichment_breakdown: { enrichment_candidates: 50 },
      normalization_breakdown: { normalized_articles_processed: 45 },
    };

    document.body.innerHTML = retentionFlow(overview);
    const sourceValues = Array.from(document.querySelectorAll(".rw-flow--source .rw-flow__step")).map((stage) => {
      return (stage.querySelector(".rw-flow__value strong, .rw-flow__unavailable") as HTMLElement).textContent;
    });
    assert.deepEqual(sourceValues, ["300", "160", "Not recorded", "Not recorded"]);
  });

});

describe("state.tsx — breakdown", () => {

  it("renders not recorded for empty entries", () => {
    const result = breakdown("Test", null, "Count");
    assert.ok(result.includes("Not recorded"));
  });

  it("renders a breakdown table with entries", () => {
    const source = { crossref: { value: 100 }, openalex: { value: 50 } };
    const result = breakdown("Sources", source, "Count");
    assert.ok(result.includes("Sources"));
    assert.ok(result.includes("Crossref"));
    assert.ok(result.includes("Openalex"));
  });

  it("renders with total when useTotal is true", () => {
    const source = { a: { value: 10 }, b: { value: 20 } };
    const result = breakdown("Test", source, "Count", true);
    assert.ok(result.includes("a"));
    assert.ok(result.includes("b"));
  });

});

describe("state.tsx — sourceResultCountSummary", () => {

  it("renders a table with source counts", () => {
    const items = [
      { source_name: "PubMed", expected_result_count: 100, observed_result_count: 95, result_count_comparison: "consistent" },
    ];
    const result = sourceResultCountSummary(items);
    assert.ok(result.includes("Source export counts"));
    assert.ok(result.includes("PubMed"));
    assert.ok(result.includes("100"));
    assert.ok(result.includes("95"));
  });

  it("handles empty items", () => {
    const result = sourceResultCountSummary([]);
    assert.ok(result.includes("No records."));
  });

});

describe("state.tsx — cell", () => {

  it("renders NULL for null/undefined", () => {
    const result = cell(null, "col");
    assert.ok(result.includes("NULL"));
    assert.equal(cell(undefined, "col"), cell(null, "col"));
  });

  it("renders short values inline", () => {
    const result = cell("hello", "col");
    assert.ok(result.includes("hello"));
    assert.ok(!result.includes("details"));
  });

  it("truncates long values with details", () => {
    const long = "x".repeat(200);
    const result = cell(long, "col");
    assert.ok(result.includes("details"));
    assert.ok(result.includes("…"));
  });

  it("creates article link for article_id column", () => {
    const result = cell("a1", "article_id");
    assert.ok(result.includes(`href="/article"`));
    assert.ok(result.includes("&quot;article_id&quot;:&quot;a1&quot;"));
  });

  it("creates author link for author_id column", () => {
    const result = cell("auth1", "author_id");
    assert.ok(result.includes(`href="/author"`));
    assert.ok(result.includes("&quot;author_id&quot;:&quot;auth1&quot;"));
  });

  it("creates reference link for reference_id column", () => {
    const result = cell("ref1", "reference_id");
    assert.ok(result.includes(`href="/reference"`));
    assert.ok(result.includes("&quot;reference_id&quot;:&quot;ref1&quot;"));
  });

  it("creates article link for work_revision id with tableName", () => {
    const result = cell("w1", "id", "work_revisions");
    assert.ok(result.includes("&quot;article_id&quot;:&quot;w1&quot;"));
  });

  it("creates author link for author_occurrences id with tableName", () => {
    const result = cell("a1", "id", "author_occurrences");
    assert.ok(result.includes("&quot;author_id&quot;:&quot;a1&quot;"));
  });

  it("creates reference link for reference_mentions id with tableName", () => {
    const result = cell("r1", "id", "reference_mentions");
    assert.ok(result.includes("&quot;reference_id&quot;:&quot;r1&quot;"));
  });

});

describe("state.tsx — bindCopyButtons", () => {

  beforeEach(() => {
    document.querySelectorAll("[data-copy-text]").forEach((el) => { el.remove(); });
  });

  it("copies text and shows Copied! feedback", async () => {
    let btn = document.createElement("button");
    btn.setAttribute("data-copy-text", "hello");
    btn.textContent = "Copy query";
    document.body.appendChild(btn);

    // Mock clipboard
    let written = "";
    let originalClipboard = navigator.clipboard;
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: (text: string) => { written = text; return Promise.resolve(); } },
      configurable: true,
    });

    bindCopyButtons();
    btn.click();
    // Allow microtask to flush
    await new Promise((resolve) => { setTimeout(resolve, 10); });
    assert.equal(written, "hello");
    assert.equal(btn.textContent, "Copied!");

    // Original label is restored after the feedback timeout
    await new Promise((resolve) => { setTimeout(resolve, 2010); });
    assert.equal(btn.textContent, "Copy query");

    Object.defineProperty(navigator, "clipboard", { value: originalClipboard, configurable: true });
    btn.remove();
  });

  it("falls back to prompt when clipboard fails", async () => {
    let btn = document.createElement("button");
    btn.setAttribute("data-copy-text", "fail");
    btn.textContent = "Copy";
    document.body.appendChild(btn);

    let originalClipboard = navigator.clipboard;
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: () => { return Promise.reject(new Error("denied")); } },
      configurable: true,
    });

    let originalPrompt = globalThis.prompt;
    let prompted = "";
    globalThis.prompt = ((_msg: string, val?: string) => { prompted = val ?? ""; return ""; }) as typeof prompt;

    bindCopyButtons();
    btn.click();
    await new Promise((resolve) => { setTimeout(resolve, 10); });
    assert.equal(prompted, "fail");

    globalThis.prompt = originalPrompt;
    Object.defineProperty(navigator, "clipboard", { value: originalClipboard, configurable: true });
    btn.remove();
  });

});
