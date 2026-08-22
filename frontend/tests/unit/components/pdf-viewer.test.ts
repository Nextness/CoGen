import { describe, it } from "node:test";
import assert from "node:assert/strict";

import "../setup.ts";
import { mountPDFViewer, rotateRectangles, unrotateRectangles } from "../../../src/components/pdf-viewer.tsx";

/** Waits until one PDF status message reaches the expected value. */
async function waitForStatus(host: HTMLElement, expected: string): Promise<void> {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    if (host.querySelector("[data-pdf-status]")?.textContent === expected) return;
    await new Promise((resolve) => {
      setTimeout(resolve, 0);
    });
  }
  assert.equal(host.querySelector("[data-pdf-status]")?.textContent, expected);
}

describe("pdf-viewer.tsx", function() {
  it("projects displayed rectangles back through all supported rotations", function() {
    const rectangle = { x: 0.1, y: 0.2, width: 0.3, height: 0.4 };
    assert.deepEqual(unrotateRectangles([rectangle], 0)[0], rectangle);
    assert.deepEqual(unrotateRectangles([rectangle], 90)[0], { x: 0.2, y: 0.6, width: 0.4, height: 0.3 });
    assert.deepEqual(unrotateRectangles([rectangle], 180)[0], { x: 0.6, y: 0.4, width: 0.3, height: 0.4 });
    assert.deepEqual(unrotateRectangles([rectangle], 270)[0], { x: 0.4, y: 0.1, width: 0.4, height: 0.3 });
  });

  it("round-trips stored rectangles into each displayed rotation", function() {
    const rectangle = { x: 0.1, y: 0.2, width: 0.3, height: 0.4 };
    for (const rotation of [0, 90, 180, 270]) {
      assert.deepEqual(unrotateRectangles(rotateRectangles([rectangle], rotation), rotation)[0], rectangle);
    }
  });

  it("caches pages and text, redraws anchors without rerendering, and supports keyboard paging", async function() {
    const renderedPages: number[] = [];
    const requestedPages: number[] = [];
    const requestedText: number[] = [];
    const documentMock = {
      numPages: 3,
      getPage: async (pageNumber: number) => {
        requestedPages.push(pageNumber);
        return {
          getViewport: () => {
            return { width: 240, height: 320, transform: [1, 0, 0, 1, 0, 0] };
          },
          render: () => {
            renderedPages.push(pageNumber);
            return { cancel: () => {}, promise: Promise.resolve() };
          },
          getTextContent: async () => {
            requestedText.push(pageNumber);
            return {
              items: [{ str: `Page ${pageNumber}`, transform: [1, 0, 0, 12, 10, 20], fontName: "sans" }],
              styles: { sans: { fontFamily: "sans-serif" } },
            };
          },
        };
      },
      destroy: async () => {},
    };
    const loadingTask = { promise: Promise.resolve(documentMock), destroy: async () => {} };
    const pdfjsMock = {
      GlobalWorkerOptions: {},
      Util: {
        transform: (_viewport: unknown, item: unknown) => {
          return item;
        },
      },
      getDocument: () => {
        return loadingTask;
      },
    };
    const host = document.createElement("div");
    document.body.append(host);
    const changes: number[] = [];
    const controller = await mountPDFViewer(host, {
      url: "/fixture.pdf",
      page: 1,
      onPageChange: (page) => {
        changes.push(page);
      },
    }, async () => {
      return pdfjsMock;
    });

    assert.equal(host.querySelectorAll(".rw-pdf-page").length, 1);
    assert.equal((host.querySelector(".rw-pdf-page") as HTMLElement).dataset.pdfPageNumber, "1");
    assert.equal(host.querySelector("[data-pdf-pages]")?.hasAttribute("aria-live"), false);
    assert.ok(host.querySelector("[data-pdf-fit-width]"));

    controller.setAnchors([{ id: "a1", version: { page: 1, rectangles: [{ x: 0.1, y: 0.1, width: 0.2, height: 0.1 }] } }]);
    assert.equal(host.querySelectorAll(".rw-pdf-anchor-highlight").length, 1);
    assert.deepEqual(renderedPages, [1]);
    assert.deepEqual(requestedText, [1]);

    const viewport = host.querySelector("[data-pdf-pages]") as HTMLElement;
    viewport.dispatchEvent(new KeyboardEvent("keydown", { key: "PageDown", bubbles: true }));
    await waitForStatus(host, "PDF page 2 of 3.");
    assert.equal((host.querySelector(".rw-pdf-page") as HTMLElement).dataset.pdfPageNumber, "2");

    controller.goToPage(1);
    await waitForStatus(host, "PDF page 1 of 3.");
    assert.deepEqual(requestedPages, [1, 2]);
    assert.deepEqual(requestedText, [1, 2]);
    assert.deepEqual(changes, [1, 2, 1]);
    await controller.destroy();
    host.remove();
  });
});
