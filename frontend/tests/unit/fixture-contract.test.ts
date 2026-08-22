import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { DatabaseSync } from "node:sqlite";
import { describe, it } from "node:test";

import { validateViewerFixture, viewerFixtureContractVersion } from "../../scripts/fixture-contract.ts";

/** Creates a minimal fixture pair satisfying the browser runner contract. */
function createFixturePair(directory: string, contractVersion = viewerFixtureContractVersion): { metadataPath: string; pdfPath: string } {
  const metadataPath = path.join(directory, "workspace.fixture.db");
  const pdfPath = path.join(directory, "workspace.fixture.pdf.db");
  const metadata = new DatabaseSync(metadataPath);
  metadata.exec(`
    CREATE TABLE schema_migrations (filename TEXT NOT NULL);
    INSERT INTO schema_migrations VALUES ('V00026_review_anchor_labels.sql');
    CREATE TABLE searches (search_id TEXT NOT NULL);
    INSERT INTO searches VALUES ('deep-learning-nlp');
    CREATE TABLE run_search_terms (pipeline_run_id INTEGER NOT NULL, term TEXT NOT NULL);
    INSERT INTO run_search_terms VALUES
      (1, 'one'), (1, 'two'), (1, 'three'), (1, 'four'), (1, 'five'),
      (1, 'six'), (1, 'seven'), (1, 'eight'), (1, 'nine'), (1, 'ten');
    PRAGMA user_version=${contractVersion};
  `);
  metadata.close();

  const pdf = new DatabaseSync(pdfPath);
  pdf.exec(`
    CREATE TABLE schema_migrations (filename TEXT NOT NULL);
    INSERT INTO schema_migrations VALUES ('V00002_normalized_inventory.sql');
    CREATE TABLE pdf_documents (doi TEXT NOT NULL, status TEXT NOT NULL);
    INSERT INTO pdf_documents VALUES ('10.1000/1', 'available');
    PRAGMA user_version=${contractVersion};
  `);
  pdf.close();
  return { metadataPath, pdfPath };
}

describe("viewer fixture contract", () => {
  it("accepts the current generated metadata and PDF contract", async () => {
    const directory = await mkdtemp(path.join(tmpdir(), "cogen-fixture-contract-"));
    try {
      const fixture = createFixturePair(directory);
      assert.doesNotThrow(() => validateViewerFixture(fixture.metadataPath, fixture.pdfPath));
    } finally {
      await rm(directory, { recursive: true });
    }
  });

  it("rejects a stale fixture contract with a regeneration instruction", async () => {
    const directory = await mkdtemp(path.join(tmpdir(), "cogen-fixture-contract-"));
    try {
      const fixture = createFixturePair(directory, viewerFixtureContractVersion - 1);
      assert.throws(
        () => validateViewerFixture(fixture.metadataPath, fixture.pdfPath),
        /metadata contract version is 0; expected 1.*Run make fixture/,
      );
    } finally {
      await rm(directory, { recursive: true });
    }
  });
});
