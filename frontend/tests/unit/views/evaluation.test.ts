import { describe, it, before } from "node:test";
import assert from "node:assert/strict";

import "../setup.ts";
import { evaluationView } from "../../../src/views/evaluation.tsx";
import { app, state } from "../../../src/state.tsx";

/** Sets the Evaluation URL state used by one unit test. */
function setLocation(values: Record<string, string>): void {
  const url = new URL(location.href);
  const keys = ["view", "run_id", "page", "per_page", "sort", "order", "q", "pdf_status", "review_status", "review_source", "qualifier", "source", "reviewed"];
  keys.forEach((key) => {
    url.searchParams.delete(key);
  });
  Object.entries(values).forEach(([key, raw]) => {
    url.searchParams.set(key, raw);
  });
  history.pushState({}, "", url.toString());
}

/** Builds a successful mock API response. */
function response(data: unknown): Promise<Response> {
  return Promise.resolve({
    ok: true,
    status: 200,
    json: () => {
      return Promise.resolve({ data: data });
    },
  } as unknown as Response);
}

/** Builds one invariant Evaluation response with optional overrides. */
function evaluationResponse(overrides: Record<string, any> = {}): Record<string, any> {
  return {
    review_context_initialized: false,
    review_context: null,
    proposed_parent: null,
    run_writable: true,
    queue_navigation: { previous_work_revision_id: 2, next_work_revision_id: 1 },
    review_summary: {
      total: 2,
      reviewed: 0,
      unreviewed: 2,
      pdf_available: 1,
      pdf_not_available: 1,
      percent_reviewed: 0,
      facets: {
        pdf_status: [{ value: "available", count: 1 }, { value: "not_available", count: 1 }],
        review_status: [{ value: "not_evaluated", count: 2 }],
        review_source: [{ value: "not_started", count: 2 }],
        qualifier: [],
        source: [{ value: "crossref", count: 2 }],
      },
    },
    columns: ["title", "doi", "source", "inventory_status", "inventoried_at", "review_status", "review_inherited", "review_sub_statuses"],
    rows: [
      { work_revision_id: 1, title: "Available article", doi: "10.1000/available", source: "crossref", inventory_status: "available", inventoried_at: "2026-07-29T12:00:00Z", review_status: "not_evaluated", review_inherited: false, review_version_id: null, review_sub_statuses: [] },
      { work_revision_id: 2, title: "Missing article", doi: "10.1000/missing", source: "crossref", inventory_status: "not_available", inventoried_at: null, review_status: "not_evaluated", review_inherited: false, review_version_id: null, review_sub_statuses: [] },
    ],
    pagination: { page: 1, per_page: 50, total_rows: 2, total_pages: 1 },
    ...overrides,
  };
}

describe("evaluation.tsx - evaluationView", function() {
  before(function() {
    state.searches = [];
    state.plans = [];
    state.runs = [];
  });

  it("requires a selected run without requesting evaluation data", async function() {
    const originalFetch = globalThis.fetch;
    var requested = false;
    globalThis.fetch = function() {
      requested = true;
      return response([]);
    } as typeof fetch;
    setLocation({ view: "evaluation" });

    await evaluationView();

    assert.equal(requested, false);
    assert.match(app.textContent || "", /Select a run attempt/);
    globalThis.fetch = originalFetch;
  });

  it("renders queue progress, explicit lineage, facets, and inventory states", async function() {
    const originalFetch = globalThis.fetch;
    var requested = "";
    globalThis.fetch = function(input) {
      requested = String(input);
      return response(evaluationResponse());
    } as typeof fetch;
    setLocation({ view: "evaluation", run_id: "7" });

    await evaluationView();

    assert.match(requested, /\/api\/runs\/7\/evaluation/);
    assert.match(app.textContent || "", /Review progress/);
    assert.match(app.textContent || "", /PDF/);
    assert.equal(app.querySelectorAll(".ui.green.label").length, 1);
    assert.equal(app.querySelectorAll(".ui.orange.label").length >= 3, true);
    assert.match(app.querySelector(".ui.green.label")?.textContent || "", /Available/);
    assert.match(app.querySelector(".rw-evaluation-table")?.textContent || "", /Not started/);
    assert.ok(app.querySelector("[data-start-review]"));
    const nextUnreviewed = Array.from(app.querySelectorAll<HTMLAnchorElement>("a")).find((anchor) => {
      return anchor.textContent?.includes("Next unreviewed");
    });
    assert.match(decodeURIComponent(nextUnreviewed?.href || ""), /origin=.*view=evaluation/);
    globalThis.fetch = originalFetch;
  });

  it("uses top-level initialized state when a filtered page has no rows", async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return response(evaluationResponse({
        review_context_initialized: true,
        review_context: { id: 11, pipeline_run_id: 7 },
        rows: [],
        pagination: { page: 9, per_page: 50, total_rows: 0, total_pages: 0 },
      }));
    } as typeof fetch;
    setLocation({ view: "evaluation", run_id: "7", q: "no match", page: "9" });

    await evaluationView();

    assert.match(app.textContent || "", /Review initialized/);
    assert.equal(app.querySelector("[data-start-review]"), null);
    globalThis.fetch = originalFetch;
  });

  it("submits every queue filter as destination-owned URL state", async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return response(evaluationResponse());
    } as typeof fetch;
    setLocation({ view: "evaluation", run_id: "7" });
    await evaluationView();

    const form = app.querySelector<HTMLFormElement>("[data-evaluation-filters]")!;
    form.querySelector<HTMLInputElement>("[name=q]")!.value = "methods";
    form.querySelector<HTMLSelectElement>("[name=pdf_status]")!.value = "available";
    form.querySelector<HTMLSelectElement>("[name=reviewed]")!.value = "unreviewed";
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

    const current = new URL(location.href);
    assert.equal(current.searchParams.get("q"), "methods");
    assert.equal(current.searchParams.get("pdf_status"), "available");
    assert.equal(current.searchParams.get("reviewed"), "unreviewed");
    globalThis.fetch = originalFetch;
  });
});
