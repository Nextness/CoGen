import { describe, it, before } from "node:test";
import assert from "node:assert/strict";

import "../../../../support/node/setup.ts";
import { apiResponse } from "../../../../support/node/helpers/fetch.ts";
import { seedViewerState } from "../../../../support/node/seed.ts";
import { articleCollectionView } from "../../../../../frontend/src/views/evaluation.tsx";
import { app, state, value } from "../../../../../frontend/src/state.tsx";

/** Sets the Evaluation viewer state used by one unit test. */
function setLocation(values: Record<string, string>): void {
  seedViewerState({ view: "corpus", ...values });
}

/** Waits for one observable asynchronous state instead of a fixed delay. */
async function waitFor(predicate: () => boolean): Promise<void> {
  for (var attempt = 0; attempt < 100; attempt += 1) {
    if (predicate()) return;
    await new Promise(function(resolve) { setTimeout(resolve, 10); });
  }
  assert.fail("Timed out waiting for evaluation state");
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
    columns: ["title", "doi", "year", "journal", "source", "inventory_status", "inventoried_at", "review_status", "review_inherited", "review_sub_statuses"],
    rows: [
      { work_revision_id: 1, title: "Available article", doi: "10.1000/available", source: "crossref", inventory_status: "available", inventoried_at: "2026-07-29T12:00:00Z", content_hash: "a".repeat(64), byte_size: 512, review_status: "not_evaluated", review_inherited: false, review_version_id: null, review_sub_statuses: [] },
      { work_revision_id: 2, title: "Missing article", doi: "10.1000/missing", source: "crossref", inventory_status: "not_available", inventoried_at: null, content_hash: null, byte_size: null, review_status: "not_evaluated", review_inherited: false, review_version_id: null, review_sub_statuses: [] },
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
      return apiResponse([]);
    } as typeof fetch;
    setLocation({ view: "corpus" });

    await articleCollectionView(document.createElement("div"), async () => {});

    assert.equal(requested, false);
    assert.match(app.textContent || "", /Select a run attempt/);
    globalThis.fetch = originalFetch;
  });

  it("renders queue progress, explicit lineage, facets, and inventory states", async function() {
    const originalFetch = globalThis.fetch;
    var requested = "";
    globalThis.fetch = function(input) {
      requested = String(input);
      return apiResponse(evaluationResponse());
    } as typeof fetch;
    setLocation({ view: "corpus", run_id: "7" });

    await articleCollectionView(document.createElement("div"), async () => {});

    assert.match(requested, /\/api\/runs\/7\/corpus\/articles/);
    assert.match(app.textContent || "", /Review progress/);
    assert.match(app.textContent || "", /PDF/);
    assert.equal(app.querySelectorAll(".ui.green.label").length, 1);
    assert.equal(app.querySelectorAll(".ui.orange.label").length >= 3, true);
    assert.match(app.querySelector(".ui.green.label")?.textContent || "", /Available/);
    assert.match(app.querySelector(".rw-corpus-table")?.textContent || "", /Not started/);
    assert.deepEqual(Array.from(app.querySelectorAll(".rw-corpus-table thead th"), (cell) => {
      return cell.textContent?.trim();
    }), ["", "", "DOI", "Title", "Year", "Journal", "Source", "PDF", "Review status", "Review source"]);
    assert.equal(app.querySelector(".rw-corpus-table time")?.textContent, "Jul 29, 2026");
    assert.ok(app.querySelector(".rw-evaluation-filters__advanced"));
    assert.ok(app.querySelector("[data-start-review]"));
    assert.ok(app.querySelector("[data-export-selected]"));
    assert.equal(app.querySelector<HTMLButtonElement>("[data-export-selected]")?.disabled, true);
    assert.match(app.querySelector("[data-export-count]")?.textContent || "", /0 selected/);
    assert.match(app.querySelector("[data-export-unavailable]")?.textContent || "", /1 PDF is unavailable/);
    assert.equal(app.querySelector<HTMLInputElement>('[data-select-row="1"]')?.disabled, false);
    assert.equal(app.querySelector<HTMLInputElement>('[data-select-row="2"]')?.disabled, true);
    const nextUnreviewed = Array.from(app.querySelectorAll<HTMLAnchorElement>("a")).find((anchor) => {
      return anchor.textContent?.includes("Next unreviewed");
    });
    assert.match(nextUnreviewed?.dataset.state || "", /origin.*view=corpus/);
    globalThis.fetch = originalFetch;
  });

  it("uses top-level initialized state when a filtered page has no rows", async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return apiResponse(evaluationResponse({
        review_context_initialized: true,
        review_context: { id: 11, pipeline_run_id: 7 },
        rows: [],
        pagination: { page: 9, per_page: 50, total_rows: 0, total_pages: 0 },
      }));
    } as typeof fetch;
    setLocation({ view: "corpus", run_id: "7", q: "no match", page: "9" });

    await articleCollectionView(document.createElement("div"), async () => {});

    assert.match(app.textContent || "", /Review initialized/);
    assert.equal(app.querySelector("[data-start-review]"), null);
    globalThis.fetch = originalFetch;
  });


  it("downloads only the selected current-page PDFs and reports partial results", async function() {
    const originalFetch = globalThis.fetch;
    const originalCreate = URL.createObjectURL;
    const originalRevoke = URL.revokeObjectURL;
    const originalClick = HTMLAnchorElement.prototype.click;
    var exportBody: any = null;
    var clicked = 0;
    URL.createObjectURL = function() { return "blob:selected"; };
    URL.revokeObjectURL = function() {};
    HTMLAnchorElement.prototype.click = function() { clicked += 1; };
    globalThis.fetch = function(input: RequestInfo | URL, init?: RequestInit) {
      if (init?.method === "POST") {
        exportBody = JSON.parse(String(init.body));
        return Promise.resolve({
          ok: true,
          status: 200,
          headers: new Headers({
            "Content-Type": "application/zip",
            "Content-Disposition": "attachment; filename=selected.zip",
            "Content-Length": "3",
            "X-PDF-Export-Included": "1",
            "X-PDF-Export-Skipped": "0",
            "X-PDF-Export-Annotated": "0",
            "X-PDF-Export-Comments": "0",
          }),
          body: null,
          arrayBuffer: function() { return Promise.resolve(new TextEncoder().encode("zip").buffer); },
        } as unknown as Response);
      }
      return apiResponse(evaluationResponse());
    } as typeof fetch;
    setLocation({ view: "corpus", run_id: "7" });
    await articleCollectionView(document.createElement("div"), async () => {});

    const selectAll = app.querySelector<HTMLInputElement>("[data-select-all]")!;
    selectAll.checked = true;
    selectAll.dispatchEvent(new Event("change", { bubbles: true }));
    assert.equal(app.querySelector<HTMLButtonElement>("[data-export-selected]")?.disabled, false);
    assert.match(app.querySelector("[data-export-count]")?.textContent || "", /1 selected/);
    const comments = app.querySelector<HTMLInputElement>("[data-export-include-comments]")!;
    assert.equal(comments.checked, false);

    app.querySelector<HTMLButtonElement>("[data-export-selected]")!.click();
    await waitFor(() => (app.querySelector("[data-export-feedback]")?.textContent || "").includes("1 included, 0 skipped"));
    assert.deepEqual(exportBody, {
      articles: [{ work_revision_id: 1, content_hash: "a".repeat(64) }],
      include_comments: false,
    });
    assert.equal(clicked, 1);
    assert.match(app.querySelector("[data-export-feedback]")?.textContent || "", /selected\.zip/);

    globalThis.fetch = originalFetch;
    URL.createObjectURL = originalCreate;
    URL.revokeObjectURL = originalRevoke;
    HTMLAnchorElement.prototype.click = originalClick;
  });

  it("sends the comments toggle for selected-set exports and reports inclusion", async function() {
    const originalFetch = globalThis.fetch;
    const originalCreate = URL.createObjectURL;
    const originalRevoke = URL.revokeObjectURL;
    const originalClick = HTMLAnchorElement.prototype.click;
    var exportBody: any = null;
    URL.createObjectURL = function() { return "blob:selected-comments"; };
    URL.revokeObjectURL = function() {};
    HTMLAnchorElement.prototype.click = function() {};
    globalThis.fetch = function(input: RequestInfo | URL, init?: RequestInit) {
      if (init?.method === "POST") {
        exportBody = JSON.parse(String(init.body));
        return Promise.resolve({
          ok: true,
          status: 200,
          headers: new Headers({
            "Content-Type": "application/zip",
            "Content-Disposition": "attachment; filename=selected.zip",
            "Content-Length": "3",
            "X-PDF-Export-Included": "1",
            "X-PDF-Export-Skipped": "0",
            "X-PDF-Export-Annotated": "1",
            "X-PDF-Export-Comments": "2",
          }),
          body: null,
          arrayBuffer: function() { return Promise.resolve(new TextEncoder().encode("zip").buffer); },
        } as unknown as Response);
      }
      return apiResponse(evaluationResponse());
    } as typeof fetch;
    setLocation({ view: "corpus", run_id: "7" });
    await articleCollectionView(document.createElement("div"), async () => {});

    const selectAll = app.querySelector<HTMLInputElement>("[data-select-all]")!;
    selectAll.checked = true;
    selectAll.dispatchEvent(new Event("change", { bubbles: true }));
    const comments = app.querySelector<HTMLInputElement>("[data-export-include-comments]")!;
    comments.checked = true;
    comments.dispatchEvent(new Event("change"));
    app.querySelector<HTMLButtonElement>("[data-export-selected]")!.click();
    await waitFor(() => (app.querySelector("[data-export-feedback]")?.textContent || "").includes("saved comments included"));

    assert.deepEqual(exportBody, {
      articles: [{ work_revision_id: 1, content_hash: "a".repeat(64) }],
      include_comments: true,
    });
    assert.match(app.querySelector("[data-export-feedback]")?.textContent || "", /2 saved comments included in 1 file/);

    globalThis.fetch = originalFetch;
    URL.createObjectURL = originalCreate;
    URL.revokeObjectURL = originalRevoke;
    HTMLAnchorElement.prototype.click = originalClick;
  });

  it("limits select-all to twenty current-page rows and clears selection on rerender", async function() {
    const originalFetch = globalThis.fetch;
    const rows = Array.from({ length: 25 }, function(_, index) {
      return {
        work_revision_id: index + 1,
        title: `Article ${index + 1}`,
        doi: `10.1000/${index + 1}`,
        source: "crossref",
        inventory_status: "available",
        inventoried_at: "2026-07-29T12:00:00Z",
        content_hash: String(index + 1).padStart(64, "a"),
        byte_size: 512,
        review_status: "not_evaluated",
        review_inherited: false,
        review_version_id: null,
        review_sub_statuses: [],
      };
    });
    globalThis.fetch = function() {
      return apiResponse(evaluationResponse({
        rows: rows,
        pagination: { page: 1, per_page: 50, total_rows: 25, total_pages: 1 },
      }));
    } as typeof fetch;
    setLocation({ view: "corpus", run_id: "7" });
    await articleCollectionView(document.createElement("div"), async () => {});

    const selectAll = app.querySelector<HTMLInputElement>("[data-select-all]")!;
    selectAll.checked = true;
    selectAll.dispatchEvent(new Event("change", { bubbles: true }));
    assert.match(app.querySelector("[data-export-count]")?.textContent || "", /20 selected/);
    assert.match(app.querySelector("[data-export-feedback]")?.textContent || "", /first 20/);

    await articleCollectionView(document.createElement("div"), async () => {});
    assert.match(app.querySelector("[data-export-count]")?.textContent || "", /0 selected/);
    assert.equal(app.querySelector<HTMLButtonElement>("[data-export-selected]")?.disabled, true);

    globalThis.fetch = originalFetch;
  });

  it("submits every queue filter as destination-owned URL state", async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return apiResponse(evaluationResponse());
    } as typeof fetch;
    setLocation({ view: "corpus", run_id: "7" });
    await articleCollectionView(document.createElement("div"), async () => {});

    const form = app.querySelector<HTMLFormElement>("[data-evaluation-filters]")!;
    form.querySelector<HTMLInputElement>("[name=q]")!.value = "methods";
    form.querySelector<HTMLSelectElement>("[name=pdf_status]")!.value = "available";
    form.querySelector<HTMLSelectElement>("[name=reviewed]")!.value = "unreviewed";
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

    assert.equal(value("q"), "methods");
    assert.equal(value("pdf_status"), "available");
    assert.equal(value("reviewed"), "unreviewed");
    globalThis.fetch = originalFetch;
  });
});
