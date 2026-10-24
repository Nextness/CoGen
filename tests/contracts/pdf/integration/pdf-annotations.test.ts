// pdf-annotations.test.ts independently validates annotation output produced by the internal worker.

import { after, before, describe, it } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { inspectPDF } from "../../../support/node/pdf-inspector.ts";

const root = fileURLToPath(new URL("../../../../", import.meta.url));
let buildDirectory: string;
let worker: string;
let fixture: Buffer;

/** runWorker runs the worker with one JSON request and returns its output. */
function runWorker(request: string): Buffer {
  return execFileSync(worker, ["pdf-annotate"], { input: request, cwd: root });
}

describe("pdf annotations contract", function() {
  before(function() {
    mkdirSync(join(root, "build", "pdf-trial"), { recursive: true });
    buildDirectory = mkdtempSync(join(root, "build", "pdf-trial", "pdfannotate-contract-"));
    worker = join(buildDirectory, "analysis");
    execFileSync("make", ["build", `BIN=${worker}`], { cwd: root, stdio: "inherit" });
    fixture = execFileSync("go", ["run", "./tests/support/go/cmd/pdf-fixture", "-complex", "Complex fixture page"], { cwd: root });
  });

  after(function() {
    if (buildDirectory) rmSync(buildDirectory, { recursive: true, force: true });
  });

  it("adds text and highlight annotations without changing source bytes", async function() {
    const original = Buffer.from(fixture);
    const request = JSON.stringify({
      pdf: fixture.toString("base64"),
      annotations: [
        {
          Page: 1,
          Type: "Text",
          Rect: { X1: 10, Y1: 10, X2: 20, Y2: 20 },
          Contents: "Unicode: 日本語 😀",
          Title: "Reviewer",
          Subject: "Comment",
        },
        {
          Page: 1,
          Type: "Highlight",
          Rect: { X1: 30, Y1: 30, X2: 40, Y2: 40 },
          Contents: "Highlighted text",
          Title: "Reviewer",
          Subject: "Highlight",
        },
      ],
    });
    const annotated = runWorker(request);
    assert.ok(annotated.length > 0);
    assert.ok(annotated.subarray(0, 5).equals(Buffer.from("%PDF-")));
    assert.ok(!annotated.equals(original));
    assert.ok(fixture.equals(original));
    const inspection = await inspectPDF(new Uint8Array(annotated));
    assert.equal(inspection.pageCount, 1);
    const page = inspection.pages[0];
    assert.equal(page.annotations.length, 3);
    const text = page.text;
    assert.ok(text.includes("Complex fixture page"));
    const existing = page.annotations.find((annotation) => annotation.contents === "Existing note");
    assert.ok(existing);
    const textAnnotation = page.annotations.find((annotation) => annotation.contents === "Unicode: 日本語 😀");
    assert.ok(textAnnotation);
    assert.equal(textAnnotation.subtype, "Text");
    const highlight = page.annotations.find((annotation) => annotation.contents === "Highlighted text");
    assert.ok(highlight);
    assert.equal(highlight.subtype, "Highlight");
  });

  it("rejects malformed requests safely", function() {
    assert.throws(() => {
      runWorker('{"pdf":');
    });
  });

  it("rejects unsupported annotation types safely", function() {
    const request = JSON.stringify({
      pdf: fixture.toString("base64"),
      annotations: [
        {
          Page: 1,
          Type: "Unsupported",
          Rect: { X1: 10, Y1: 10, X2: 20, Y2: 20 },
        },
      ],
    });
    assert.throws(() => {
      runWorker(request);
    });
  });
});
