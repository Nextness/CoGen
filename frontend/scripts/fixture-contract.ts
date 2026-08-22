import { DatabaseSync } from "node:sqlite";

/** Current generated viewer-fixture data contract version. */
export const viewerFixtureContractVersion = 1;

const metadataSchemaVersion = "V00026_review_anchor_labels.sql";
const pdfSchemaVersion = "V00002_normalized_inventory.sql";

/** Validates the generated metadata and PDF fixture pair before a browser server starts. */
export function validateViewerFixture(metadataPath: string, pdfPath: string): void {
  var metadata: DatabaseSync | null = null;
  var pdf: DatabaseSync | null = null;
  try {
    metadata = new DatabaseSync(metadataPath, { readOnly: true });
    pdf = new DatabaseSync(pdfPath, { readOnly: true });
    requireNumber(metadata, "PRAGMA user_version", "user_version", viewerFixtureContractVersion, "metadata contract version");
    requireNumber(pdf, "PRAGMA user_version", "user_version", viewerFixtureContractVersion, "PDF contract version");
    requireString(metadata, "SELECT filename FROM schema_migrations ORDER BY rowid DESC LIMIT 1", "filename", metadataSchemaVersion, "metadata schema");
    requireString(pdf, "SELECT filename FROM schema_migrations ORDER BY rowid DESC LIMIT 1", "filename", pdfSchemaVersion, "PDF schema");
    requireNumber(metadata, "SELECT COUNT(*) AS count FROM searches WHERE search_id='deep-learning-nlp'", "count", 1, "canonical search sentinel");
    requireNumber(metadata, "SELECT COUNT(*) AS count FROM run_search_terms WHERE pipeline_run_id=1", "count", 10, "search-term sentinel");
    requireNumber(pdf, "SELECT COUNT(*) AS count FROM pdf_documents WHERE doi='10.1000/1' AND status='available'", "count", 1, "available PDF sentinel");
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`viewer fixture contract validation failed: ${message}. Run make fixture.`);
  } finally {
    pdf?.close();
    metadata?.close();
  }
}

/** Requires one numeric scalar query result to equal its fixture-contract value. */
function requireNumber(db: DatabaseSync, query: string, field: string, expected: number, label: string): void {
  const row = db.prepare(query).get() as Record<string, unknown> | undefined;
  const actual = Number(row?.[field]);
  if (actual !== expected) {
    throw new Error(`${label} is ${Number.isFinite(actual) ? actual : "missing"}; expected ${expected}`);
  }
}

/** Requires one string scalar query result to equal its fixture-contract value. */
function requireString(db: DatabaseSync, query: string, field: string, expected: string, label: string): void {
  const row = db.prepare(query).get() as Record<string, unknown> | undefined;
  const actual = row?.[field];
  if (actual !== expected) {
    throw new Error(`${label} is ${String(actual ?? "missing")}; expected ${expected}`);
  }
}
