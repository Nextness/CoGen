// Shared fixture-path inference for the metadata and PDF database pair.

/** Infers the companion PDF database path from a metadata database path. */
export function inferFixturePDF(metadataPath: string): string {
  return metadataPath.endsWith('.metadata.db') ? metadataPath.replace(/\.metadata\.db$/, '.pdf.db') : metadataPath.replace(/\.db$/, '.pdf.db');
}
