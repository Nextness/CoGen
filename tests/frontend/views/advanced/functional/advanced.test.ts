// Unit tests for views/advanced.tsx safe table selection and local errors.
import { afterEach, beforeEach, describe, it } from "node:test";
import assert from "node:assert/strict";

import "../../../../support/node/setup.ts";
import { apiResponse } from "../../../../support/node/helpers/fetch.ts";
import { seedViewerState } from "../../../../support/node/seed.ts";
import { advancedView } from "../../../../../frontend/src/views/advanced.tsx";
import { app, state, value } from "../../../../../frontend/src/state.tsx";

const originalFetch = globalThis.fetch;

describe("advanced.tsx advancedView", () => {
  beforeEach(() => {
    state.tables = [];
    const url = new URL(location.href);
    url.search = "?view=advanced";
    history.replaceState({}, "", url.toString());
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    state.tables = [];
    app.replaceChildren();
  });

  it("renders a safe table projection without cached discovery counts", async () => {
    let fetchCount = 0;
    globalThis.fetch = (url: RequestInfo | URL) => {
      fetchCount += 1;
      if (String(url).endsWith("/api/tables")) {
        return apiResponse({
          tables: [{
            name: "works",
            columns: [{ name: "id" }, { name: "title" }],
          }],
        });
      }
      return apiResponse({
        table: {
          name: "works",
          columns: [{ name: "id" }, { name: "title" }],
        },
        rows: [{
          id: 1,
          title: "Test",
        }],
        truncated_fields: {},
        pagination: {
          page: 1,
          per_page: 50,
          total_pages: 1,
          total_rows: 1,
        },
      });
    };

    await advancedView();

    assert.match(app.textContent || "", /Advanced database inspection/);
    assert.equal(app.querySelector("#table-select option")?.textContent, "works");
    assert.ok(app.querySelector("th.toggle-cell"));
    assert.ok(app.querySelector("td.toggle-cell .expand-toggle"));
    assert.equal(fetchCount, 2);
  });

  it("replaces an invalid table key with a valid table and explains the correction", async () => {
    seedViewerState({
      table: "missing_table",
      view: "advanced",
    });
    globalThis.fetch = (url: RequestInfo | URL) => {
      if (String(url).endsWith("/api/tables")) {
        return apiResponse({
          tables: [{
            name: "works",
            columns: [{ name: "id" }],
          }],
        });
      }
      return apiResponse({
        table: {
          name: "works",
          columns: [{ name: "id" }],
        },
        rows: [{ id: 1 }],
        truncated_fields: {},
        pagination: {
          page: 1,
          per_page: 50,
          total_pages: 1,
          total_rows: 1,
        },
      });
    };

    await advancedView();

    assert.equal(value("table"), "works");
    assert.match(app.textContent || "", /requested table was not found/i);
  });

  it("keeps the Advanced shell visible when the selected table request fails", async () => {
    globalThis.fetch = (url: RequestInfo | URL) => {
      if (String(url).endsWith("/api/tables")) {
        return apiResponse({
          tables: [{
            name: "works",
            columns: [{ name: "id" }],
          }],
        });
      }
      return Promise.resolve({
        ok: false,
        status: 503,
        json: () => {
          return Promise.resolve({ error: { message: "The selected table is temporarily unavailable." } });
        },
      } as unknown as Response);
    };

    await advancedView();

    assert.match(app.textContent || "", /Advanced database inspection/);
    assert.match(app.textContent || "", /temporarily unavailable/);
    assert.ok(app.querySelector("[role=alert]"));
  });
});
