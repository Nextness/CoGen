import { describe, it } from "node:test";
import assert from "node:assert/strict";

import "../../../../support/node/setup.ts";
import { paginationPages, Pagination } from "../../../../../frontend/src/components/pagination.tsx";
import { renderToString } from "../../../../support/node/helpers/jsx-render.ts";

const pagination = (result: any, options?: any): string => {
  return renderToString(Pagination({
    result,
    options,
  }));
};

describe("pagination.tsx", () => {
  it("keeps a bounded page window around the current page", () => {
    assert.deepEqual(paginationPages(1, 10, 5), [1, 2, 3, 4, 5]);
    assert.deepEqual(paginationPages(6, 10, 5), [4, 5, 6, 7, 8]);
    assert.deepEqual(paginationPages(10, 10, 5), [6, 7, 8, 9, 10]);
  });

  it("renders range, page count, numbered pages, and boundary controls", () => {
    const html = pagination({
      page: 2,
      per_page: 20,
      total_rows: 95,
      total_pages: 5,
    }, { itemLabel: "articles" });
    assert.ok(html.includes("<strong>21–40</strong>"));
    assert.ok(html.includes("of 95 articles"));
    assert.ok(html.includes("Page 2 of 5"));
    assert.ok(html.includes("First"));
    assert.ok(html.includes("Last"));
    assert.ok(html.includes(`aria-current="page"`));
  });

  it("handles an empty result without inventing a visible row", () => {
    const html = pagination({
      page: 1,
      per_page: 50,
      total_rows: 0,
      total_pages: 0,
    });
    assert.ok(html.includes("<strong>0–0</strong>"));
    assert.ok(html.includes("Page 1 of 1"));
  });
});
