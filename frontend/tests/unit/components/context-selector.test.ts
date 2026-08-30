// Unit tests for components/context-selector.tsx bounded hierarchy controls.
import { afterEach, beforeEach, describe, it } from "node:test";
import assert from "node:assert/strict";

import "../setup.ts";
import { seedViewerState } from "../seed.ts";
import {
  clearContextOptionCache,
  focusContextSelector,
  hydrateSelectors,
  selects,
} from "../../../src/components/context-selector.tsx";
import { state, value } from "../../../src/state.tsx";

const originalFetch = globalThis.fetch;

/** Returns one API-compatible JSON response. */
function response(data: any, ok = true, status = 200): Promise<Response> {
  return Promise.resolve({
    ok: ok,
    status: status,
    json: function() {
      return Promise.resolve(ok ? { data: data } : { error: { message: data } });
    },
  } as unknown as Response);
}

/** Returns a complete bounded hierarchy fixture for the requested level. */
function hierarchyFixture(rawURL: string, sole = false): Promise<Response> {
  const url = new URL(rawURL, location.origin);
  const section = url.searchParams.get("section");
  if (section === "searches") {
    const items = sole ? [{ id: 1, search_id: "Only search" }] : [{ id: 1, search_id: "Systematic review" }, { id: 2, search_id: "Mapping study" }];
    if (url.searchParams.get("q") === "mapping") return response({ items: [items[items.length - 1]], has_more: false, next_cursor: "" });
    return response({ items: items, has_more: false, next_cursor: "" });
  }
  if (section === "revisions") return response({ items: [{ id: 11, label: "Revision 1" }], has_more: false, next_cursor: "" });
  if (section === "plans") return response({ items: [{ id: 21, execution_fingerprint: "fingerprint" }], has_more: false, next_cursor: "" });
  if (section === "attempts") return response({ items: [{ id: 31, status: "completed", started_at: "2026-08-01T12:00:00Z" }], has_more: false, next_cursor: "" });
  throw new Error(`Unexpected hierarchy request ${rawURL}`);
}

describe("context-selector.tsx hierarchy controls", function() {
  beforeEach(function() {
    clearContextOptionCache();
    state.searches = [];
    state.plans = [];
    state.runs = [];
    seedViewerState({ view: "overview" });
    globalThis.fetch = function(url) {
      return hierarchyFixture(String(url));
    } as typeof fetch;
  });

  afterEach(function() {
    globalThis.fetch = originalFetch;
  });

  it("exports all four native form-value controls", function() {
    assert.equal(selects.search.id, "search-select");
    assert.equal(selects.revision.id, "revision-select");
    assert.equal(selects.plan.id, "plan-select");
    assert.equal(selects.run.id, "run-select");
  });

  it("loads only the bounded search page until a parent is selected", async function() {
    const requests: string[] = [];
    globalThis.fetch = function(url) {
      requests.push(String(url));
      return hierarchyFixture(String(url));
    } as typeof fetch;

    await hydrateSelectors();

    assert.equal(state.searches.length, 2);
    assert.equal(selects.search.options.length, 3);
    assert.equal(selects.revision.disabled, true);
    assert.equal(requests.length, 1);
    assert.match(requests[0], /section=searches/);
  });

  it("performs server search and supports the complete listbox keyboard contract", async function() {
    const requests: string[] = [];
    globalThis.fetch = function(url) {
      requests.push(String(url));
      return hierarchyFixture(String(url));
    } as typeof fetch;
    await hydrateSelectors();
    const dropdown = document.querySelector<HTMLElement>("[data-context-dropdown=search]")!;
    const trigger = dropdown.querySelector<HTMLButtonElement>(".rw-search-dropdown__trigger")!;
    const optionsID = trigger.getAttribute("aria-controls");

    trigger.click();
    assert.equal(trigger.getAttribute("role"), "combobox");
    assert.equal(trigger.getAttribute("aria-expanded"), "true");
    assert.equal(document.getElementById(optionsID || "")?.getAttribute("role"), "listbox");
    const query = dropdown.querySelector<HTMLInputElement>(".rw-search-dropdown__query")!;
    query.value = "mapping";
    query.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise(function(resolve) {
      setTimeout(resolve, 220);
    });
    const options = dropdown.querySelectorAll<HTMLElement>("[role=option]");
    assert.equal(options.length, 1);
    assert.equal(options[0].textContent?.trim(), "Mapping study");
    assert.ok(requests.some(function(path) {
      return path.includes("q=mapping");
    }));
    options[0].dispatchEvent(new KeyboardEvent("keydown", { key: "Home", bubbles: true }));
    assert.equal(document.activeElement, options[0]);
    options[0].dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    assert.equal(selects.search.value, "2");
  });

  it("auto-selects a sole eligible child through the complete hierarchy", async function() {
    seedViewerState({ view: "overview" });
    globalThis.fetch = function(url) {
      return hierarchyFixture(String(url), true);
    } as typeof fetch;

    await hydrateSelectors();

    assert.equal(value("search_id"), "1");
    assert.equal(value("search_revision_id"), "11");
    assert.equal(value("plan_id"), "21");
    assert.equal(value("run_id"), "31");
  });

  it("replaces crossed ancestry with the selected run's canonical hierarchy", async function() {
    seedViewerState({ view: "overview", search_id: "2", search_revision_id: "22", plan_id: "33", run_id: "44" });
    globalThis.fetch = function(rawURL) {
      const target = String(rawURL);
      if (target.includes("/api/runs/44/context")) {
        return response({
          search: { id: 1, search_id: "canonical-search" },
          revision: { id: 11, label: "r1" },
          plan: { id: 12, execution_fingerprint: "canonical" },
          run: { id: 44, status: "completed", visibility_state: "active", started_at: "2026-08-01T00:00:00Z" },
        });
      }
      const url = new URL(target, location.origin);
      const section = url.searchParams.get("section");
      var selectedItem: any = null;
      if (section === "searches") selectedItem = { id: 1, search_id: "canonical-search" };
      if (section === "revisions") selectedItem = { id: 11, label: "r1" };
      if (section === "plans") selectedItem = { id: 12, execution_fingerprint: "canonical" };
      if (section === "attempts") selectedItem = { id: 44, status: "completed", started_at: "2026-08-01T00:00:00Z" };
      return response({ items: [], selected_item: selectedItem, has_more: false, next_cursor: "" });
    } as typeof fetch;

    await hydrateSelectors();

    assert.equal(value("search_id"), "1");
    assert.equal(value("search_revision_id"), "11");
    assert.equal(value("plan_id"), "12");
    assert.equal(value("run_id"), "44");
    assert.equal(selects.run.value, "44");
  });

  it("keeps successful parents visible when one child level fails", async function() {
    seedViewerState({ view: "overview", search_id: "1", search_revision_id: "11" });
    globalThis.fetch = function(rawURL) {
      const target = String(rawURL);
      if (target.includes("section=plans")) return response("Plan discovery failed.", false, 503);
      return hierarchyFixture(target, true);
    } as typeof fetch;

    await hydrateSelectors();

    assert.equal(selects.search.value, "1");
    assert.equal(selects.revision.value, "11");
    assert.equal(selects.plan.disabled, true);
    assert.match(selects.plan.closest(".ui.field")?.textContent || "", /Plan discovery failed/);
  });

  it("focuses the visible selector trigger instead of the hidden native select", async function() {
    await hydrateSelectors();
    focusContextSelector();
    const trigger = document.querySelector<HTMLButtonElement>("[data-context-dropdown=search] .rw-search-dropdown__trigger")!;
    assert.equal(document.activeElement, trigger);
  });
});
