import { describe, it } from "node:test";
import assert from "node:assert/strict";

import "../setup.ts";
import { mountBacklinks } from "../../../src/components/backlinks.tsx";

/** Builds one successful mock API response. */
function response(data: unknown): Promise<Response> {
  return Promise.resolve({
    ok: true,
    status: 200,
    json: () => {
      return Promise.resolve({ data: data });
    },
  } as unknown as Response);
}

describe("backlinks.tsx - mountBacklinks", function() {
  it("traverses more than one hundred inbound notes without duplicates", async function() {
    const host = document.createElement("div");
    document.body.append(host);
    const originalFetch = globalThis.fetch;
    let requestCount = 0;
    globalThis.fetch = function(input) {
      requestCount += 1;
      const url = new URL(String(input), location.origin);
      const cursor = url.searchParams.get("cursor") || "page-0";
      const page = Number(cursor.replace("page-", ""));
      const start = page * 25;
      const count = page < 4 ? 25 : 1;
      const items = Array.from({ length: count }, (_, index) => {
        const id = start + index + 1;
        return {
          id: id,
          work_revision_id: id,
          version: { title: `Source note ${id}` },
        };
      });
      return response({
        items: items,
        has_more: page < 4,
        next_cursor: page < 4 ? `page-${page + 1}` : null,
      });
    } as typeof fetch;

    await mountBacklinks(host, {
      runID: 7,
      targetType: "pdf_page",
      targetID: "3",
      workRevisionID: 9,
    });
    assert.equal(host.querySelectorAll("li").length, 25);
    for (let page = 1; page <= 4; page += 1) {
      (host.querySelector("[data-backlinks-more]") as HTMLButtonElement).click();
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    }

    assert.equal(requestCount, 5);
    assert.equal(host.querySelectorAll("li").length, 101);
    assert.equal(new Set(Array.from(host.querySelectorAll("li"), (item) => item.textContent)).size, 101);
    assert.equal(host.querySelector("[data-backlinks-more]"), null);
    globalThis.fetch = originalFetch;
    host.remove();
  });
});
