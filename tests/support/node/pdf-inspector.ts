// pdf-inspector.ts loads annotated PDFs with the vendored PDF.js build for independent contract checks.

import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

/** PDFAnnotation describes one annotation exposed by PDF.js. */
export interface PDFAnnotation {
  subtype: string;
  contents: string;
  title: string;
  subject: string;
  rect: number[];
  quadPoints: { x: number; y: number }[];
}

/** PDFPage describes one page's annotations and text. */
export interface PDFPage {
  annotations: PDFAnnotation[];
  text: string;
}

/** PDFInspection is one parsed PDF document. */
export interface PDFInspection {
  pageCount: number;
  pages: PDFPage[];
}

interface PDFJSAnnotation {
  subtype?: string;
  contentsObj?: { str?: string };
  titleObj?: { str?: string };
  subjectObj?: { str?: string };
  rect?: number[];
  quadPoints?: unknown;
}

interface PDFJSPage {
  getAnnotations(): Promise<PDFJSAnnotation[]>;
  getTextContent(): Promise<{ items: { str?: string }[] }>;
}

interface PDFJSDocument {
  numPages: number;
  getPage(pageNumber: number): Promise<PDFJSPage>;
  destroy(): Promise<void>;
}

interface PDFJSModule {
  GlobalWorkerOptions: { workerSrc: string };
  getDocument(options: { data: Uint8Array; standardFontDataUrl?: string }): { promise: Promise<PDFJSDocument> };
}

/** normalizeQuadPoints flattens PDF.js single and multiple quad representations. */
function normalizeQuadPoints(value: unknown): { x: number; y: number }[] {
  if (!Array.isArray(value)) return [];
  const points: { x: number; y: number }[] = [];
  for (const item of value) {
    if (Array.isArray(item)) {
      points.push(...normalizeQuadPoints(item));
    } else if (item && typeof item === "object" && "x" in item && "y" in item) {
      points.push(item as { x: number; y: number });
    }
  }
  return points;
}

/** inspectPDF loads PDF bytes with the vendored PDF.js build and extracts annotations and text. */
export async function inspectPDF(data: Uint8Array): Promise<PDFInspection> {
  const pdfjs = await import("../../../frontend/vendor/pdfjs/pdf.min.mjs") as unknown as PDFJSModule;
  const vendorRoot = fileURLToPath(new URL("../../../frontend/vendor/pdfjs/", import.meta.url));
  pdfjs.GlobalWorkerOptions.workerSrc = pathToFileURL(join(vendorRoot, "pdf.worker.min.mjs")).href;
  const document = await pdfjs.getDocument({
    data,
    standardFontDataUrl: pathToFileURL(join(vendorRoot, "standard_fonts")).href + "/",
  }).promise;
  try {
    const pages: PDFPage[] = [];
    for (let pageNumber = 1; pageNumber <= document.numPages; pageNumber += 1) {
      const page = await document.getPage(pageNumber);
      const annotations = await page.getAnnotations();
      const textContent = await page.getTextContent();
      pages.push({
        annotations: annotations.map((annotation) => ({
          subtype: annotation.subtype ?? "",
          contents: annotation.contentsObj?.str ?? "",
          title: annotation.titleObj?.str ?? "",
          subject: annotation.subjectObj?.str ?? "",
          rect: annotation.rect ?? [],
          quadPoints: normalizeQuadPoints(annotation.quadPoints),
        })),
        text: textContent.items.map((item) => item.str ?? "").join(" "),
      });
    }
    return { pageCount: document.numPages, pages };
  } finally {
    await document.destroy();
  }
}
