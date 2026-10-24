// pdf-inspector.ts loads annotated PDFs with the vendored PDF.js build for independent contract checks.

import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

/** PDFAnnotation describes one annotation exposed by PDF.js. */
export interface PDFAnnotation {
  subtype: string;
  contents: string;
  title: string;
  subject: string;
  rect: number[];
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

/** inspectPDF loads PDF bytes with the vendored PDF.js build and extracts annotations and text. */
export async function inspectPDF(data: Uint8Array): Promise<PDFInspection> {
  const pdfjs = await import("../../../frontend/vendor/pdfjs/pdf.min.mjs") as unknown as PDFJSModule;
  pdfjs.GlobalWorkerOptions.workerSrc = pathToFileURL(resolve("frontend/vendor/pdfjs/pdf.worker.min.mjs")).href;
  const document = await pdfjs.getDocument({
    data,
    standardFontDataUrl: pathToFileURL(resolve("frontend/vendor/pdfjs/standard_fonts/")).href + "/",
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
        })),
        text: textContent.items.map((item) => item.str ?? "").join(" "),
      });
    }
    return { pageCount: document.numPages, pages };
  } finally {
    await document.destroy();
  }
}
