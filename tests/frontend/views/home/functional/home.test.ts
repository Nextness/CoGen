// Unit tests for views/home.tsx bounded discovery and run lifecycle controls.
import { afterEach, beforeEach, describe, it } from "node:test";
import assert from "node:assert/strict";

import "../../../../support/node/setup.ts";
import { apiResponse } from "../../../../support/node/helpers/fetch.ts";
import { seedViewerState } from "../../../../support/node/seed.ts";
import { homeView } from "../../../../../frontend/src/views/home.tsx";
import { app } from "../../../../../frontend/src/state.tsx";

const originalFetch = globalThis.fetch;

/** Returns the hierarchy fixture section selected by one request URL. */
function hierarchyResponse(rawURL: string): Promise<Response> {
  const url = new URL(rawURL, location.origin);
  const section = url.searchParams.get("section");
  if (section === "summary") {
    return apiResponse({
      version: "1",
      totals: {
        searches: 140,
        revisions: 280,
        plans: 400,
        runs: 650,
        completed_runs: 600,
      },
      latest_run: {
        id: 31,
        execution_plan_id: 21,
        search_revision_id: 11,
        search_id: 1,
        attempt_number: 3,
        status: "completed",
        visibility_state: "active",
        started_at: "2024-01-03T10:00:00Z",
        finished_at: "2024-01-03T10:12:34Z",
      },
    });
  }
  if (section === "searches") {
    return apiResponse({
      version: "1",
      items: [{
        id: 1,
        search_id: "process-mining",
        created_at: "2024-01-01T00:00:00Z",
        revision_count: 120,
        plan_count: 190,
        run_count: 340,
        latest_revision_id: 11,
        latest_plan_id: 21,
        latest_run_id: 31,
      }],
      has_more: true,
      next_cursor: "next-searches",
    });
  }
  if (section === "revisions") {
    return apiResponse({
      version: "1",
      items: [{
        id: 11,
        label: "Revision 1",
        plan_count: 2,
        run_count: 3,
        latest_plan_id: 21,
        latest_run_id: 31,
      }],
      has_more: true,
      next_cursor: "next-revisions",
    });
  }
  return apiResponse({
    version: "1",
    items: [{
      id: 31,
      execution_plan_id: 21,
      search_revision_id: 11,
      search_id: 1,
      search_name: "process-mining",
      revision_label: "Revision 1",
      attempt_number: 3,
      status: "completed",
      visibility_state: "active",
      started_at: "2024-01-03T10:00:00Z",
      finished_at: "2024-01-03T10:12:34Z",
    }],
    has_more: true,
    next_cursor: "next-runs",
  });
}

describe("home.tsx homeView", () => {
  beforeEach(() => {
    seedViewerState({ view: "home" });
    globalThis.fetch = (input: RequestInfo | URL) => {
      return hierarchyResponse(String(input));
    };
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    app.replaceChildren();
  });

  it("renders bounded hierarchy metrics, filters, direct actions, and lazy revision pages", async () => {
    const requested: string[] = [];
    globalThis.fetch = (input: RequestInfo | URL) => {
      requested.push(String(input));
      return hierarchyResponse(String(input));
    };

    await homeView();

    const kpiLabels = Array.from(app.querySelectorAll(".rw-home-kpis .label"), (label) => {
      return label.textContent;
    });
    assert.deepEqual(kpiLabels, ["Search terms", "Search revisions", "Execution plans", "Run attempts"]);
    assert.match(app.textContent || "", /process-mining/);
    assert.match(app.textContent || "", /12m 34s/);
    assert.doesNotMatch(app.textContent || "", /Latest execution|Open latest run/);
    assert.equal(app.querySelectorAll(".rw-home-search-card__metrics > div").length, 3);
    assert.ok(app.querySelector(".rw-home-filters__advanced"));
    assert.equal(app.querySelectorAll("[data-home-run]").length, 1);
    assert.ok(app.querySelector("[data-more-searches]"));
    assert.ok(app.querySelector("[data-more-runs]"));
    const revisionRequests = requested.filter((path) => {
      return path.includes("section=revisions");
    });
    assert.equal(revisionRequests.length, 0);

    const details = app.querySelector<HTMLDetailsElement>("[data-home-search]")!;
    details.open = true;
    details.dispatchEvent(new Event("toggle"));
    await new Promise((resolve) => {
      setTimeout(resolve, 0);
    });
    assert.match(details.textContent || "", /Revision 1/);
    assert.ok(details.querySelector("[data-more-revisions]"));

    const explore = Array.from(app.querySelectorAll("a")).find((anchor) => {
      return anchor.textContent?.trim() === "Explore";
    }) as HTMLAnchorElement;
    assert.equal(explore.href, "http://127.0.0.1:8080/overview");
    assert.match(explore.dataset.state || "", /"search_id":"1"/);
    assert.match(explore.dataset.state || "", /"search_revision_id":"11"/);
    assert.match(explore.dataset.state || "", /"plan_id":"21"/);
    assert.match(explore.dataset.state || "", /"run_id":"31"/);
  });

  it("uses a native dialog and restores focus to the lifecycle opener after cancellation", async () => {
    await homeView();
    const lifecycle = app.querySelector<HTMLButtonElement>("[data-run-visibility=trashed]")!;
    lifecycle.focus();
    lifecycle.click();
    const dialog = app.querySelector<HTMLDialogElement>("[data-run-dialog]")!;

    assert.equal(dialog.hasAttribute("open"), true);
    assert.match(dialog.textContent || "", /without deleting immutable evidence/);
    dialog.querySelector<HTMLButtonElement>("[data-run-dialog-close]")!.click();
    assert.equal(dialog.hasAttribute("open"), false);
    assert.equal(document.activeElement, lifecycle);
  });

  it("retains successful sibling sections when run discovery fails", async () => {
    globalThis.fetch = (input: RequestInfo | URL) => {
      const url = new URL(String(input), location.origin);
      if (url.searchParams.get("section") === "runs") return Promise.reject(new Error("Run query failed."));
      return hierarchyResponse(String(input));
    };

    await homeView();

    assert.match(app.textContent || "", /140/);
    assert.match(app.textContent || "", /process-mining/);
    assert.match(app.textContent || "", /Run query failed/);
  });
});
