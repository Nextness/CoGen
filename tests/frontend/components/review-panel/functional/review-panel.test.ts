import { describe, it } from "node:test";
import assert from "node:assert/strict";

import "../../../../support/node/setup.ts";
import { seedViewerState } from "../../../../support/node/seed.ts";
import { mountArticleReview } from "../../../../../frontend/src/components/review-panel.tsx";
import type { ArticleDetailResponse, ArticleRecord } from "../../../../../frontend/src/api/types.ts";

describe("article review reader refresh", () => {
  for (const contextState of ["uninitialized", "unavailable"]) {
    it(`refreshes PDF state when review is ${contextState}`, async () => {
      seedViewerState({ view: "article", article_id: "1", run_id: "1" });
      const host = document.createElement("div");
      document.body.append(host);
      const originalFetch = globalThis.fetch;
      let statusRequests = 0;
      globalThis.fetch = async (input) => {
        if (String(input).includes("/review-context")) {
          if (contextState === "unavailable") throw new Error("Review context unavailable");
          return new Response(JSON.stringify({ context_initialized: false, proposed_parent: null }));
        }
        assert.match(String(input), /\/api\/works\/1\/pdf-status/);
        statusRequests += 1;
        return new Response(JSON.stringify({ status: "available", content_hash: "a".repeat(64) }));
      };
      try {
        const record = { id: 1, pipeline_run_id: 1, work_id: 1 } as ArticleRecord;
        const detail = { pdf_status: { status: "not_available" } } as ArticleDetailResponse;
        const controller = await mountArticleReview(host, null, record, detail);
        try {
          assert.equal(await controller.refreshPDF(), true);
          assert.equal(statusRequests, 1);
          assert.equal(await controller.refreshPDF(), false);
          assert.equal(statusRequests, 2);
        } finally {
          await controller.destroy();
        }
        assert.equal(await controller.refreshPDF(), false);
        assert.equal(statusRequests, 2);
      } finally {
        globalThis.fetch = originalFetch;
        host.remove();
      }
    });
  }
});
