// Custom PDF.js rendering with bounded page lifecycle and accessible anchor selection.
import { h, Fragment, render as renderTree } from "../jsx/jsx-runtime.ts";

const workerURL = "/vendor/pdfjs/pdf.worker.min.mjs";
const cMapURL = "/vendor/pdfjs/cmaps/";
const standardFontDataURL = "/vendor/pdfjs/standard_fonts/";

/** One normalized page rectangle. */
export interface NormalizedRectangle {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** Converts displayed normalized rectangles back to unrotated page coordinates. */
export function unrotateRectangles(rectangles: NormalizedRectangle[], rotation: any): NormalizedRectangle[] {
  const angle = ((Number(rotation) % 360) + 360) % 360;
  return rectangles.map(function(rectangle) {
    const x = rectangle.x;
    const y = rectangle.y;
    const width = rectangle.width;
    const height = rectangle.height;
    let result: NormalizedRectangle;
    if (angle === 90) result = { x: y, y: 1 - x - width, width: height, height: width };
    else if (angle === 180) result = { x: 1 - x - width, y: 1 - y - height, width: width, height: height };
    else if (angle === 270) result = { x: 1 - y - height, y: x, width: height, height: width };
    else result = { x: x, y: y, width: width, height: height };
    return { x: Number(result.x.toFixed(12)), y: Number(result.y.toFixed(12)), width: Number(result.width.toFixed(12)), height: Number(result.height.toFixed(12)) };
  });
}

/** Projects stored unrotated rectangles into the currently displayed page rotation. */
export function rotateRectangles(rectangles: NormalizedRectangle[], rotation: any): NormalizedRectangle[] {
  const angle = ((Number(rotation) % 360) + 360) % 360;
  return rectangles.map(function(rectangle) {
    const x = rectangle.x;
    const y = rectangle.y;
    const width = rectangle.width;
    const height = rectangle.height;
    let result: NormalizedRectangle;
    if (angle === 90) result = { x: 1 - y - height, y: x, width: height, height: width };
    else if (angle === 180) result = { x: 1 - x - width, y: 1 - y - height, width: width, height: height };
    else if (angle === 270) result = { x: y, y: 1 - x - width, width: height, height: width };
    else result = { x: x, y: y, width: width, height: height };
    return { x: Number(result.x.toFixed(12)), y: Number(result.y.toFixed(12)), width: Number(result.width.toFixed(12)), height: Number(result.height.toFixed(12)) };
  });
}

/** Extracts bounded normalized rectangles from a same-page browser selection. */
export function selectionRectangles(selection: any, pageElement: HTMLElement | null, rotation: any): NormalizedRectangle[] {
  if (!selection || selection.isCollapsed || !selection.rangeCount || !pageElement) return [];
  const range: Range = selection.getRangeAt(0);
  if (!pageElement.contains(range.commonAncestorContainer)) return [];
  const pageRect = pageElement.getBoundingClientRect();
  if (!(pageRect.width > 0 && pageRect.height > 0)) return [];
  const rectangles: NormalizedRectangle[] = Array.from(range.getClientRects()).filter(function(rectangle: DOMRect) {
    return rectangle.width > 0 && rectangle.height > 0;
  }).slice(0, 64).map(function(rectangle: DOMRect) {
    return {
      x: Math.max(0, Math.min(1, (rectangle.left - pageRect.left) / pageRect.width)),
      y: Math.max(0, Math.min(1, (rectangle.top - pageRect.top) / pageRect.height)),
      width: Math.max(0, Math.min(1, rectangle.width / pageRect.width)),
      height: Math.max(0, Math.min(1, rectangle.height / pageRect.height)),
    };
  });
  return unrotateRectangles(rectangles, rotation).filter(function(rectangle) {
    return rectangle.width > 0 && rectangle.height > 0 && rectangle.x + rectangle.width <= 1.000001 && rectangle.y + rectangle.height <= 1.000001;
  });
}

/** One PDF viewer option set. */
export interface PDFViewerOptions {
  url: string;
  page?: any;
  onPageChange?: (page: number) => void;
  onSelection?: (selection: { page: number; selectedText: string; rectangles: NormalizedRectangle[] }) => void;
}

/** One anchor head used by the viewer highlight layer. */
export interface PDFAnchorHead {
  id: string;
  version: {
    page: number;
    rectangles?: NormalizedRectangle[];
  };
}

/** Mounts a project-styled PDF.js viewer and returns a lifecycle controller. */
export async function mountPDFViewer(host: HTMLElement, options: PDFViewerOptions, loader?: () => Promise<any>): Promise<any> {
  const loadPDFJS = loader || function() { return import("../../vendor/pdfjs/pdf.min.mjs"); };
  const pdfjs = await loadPDFJS();
  pdfjs.GlobalWorkerOptions.workerSrc = workerURL;
  let pageNumber = Math.max(1, Number(options.page || 1));
  let scale = 1.15;
  let rotation = 0;
  let destroyed = false;
  let anchors: PDFAnchorHead[] = [];
  let renderSequence = 0;
  const renderTasks = new Set<any>();

  renderTree(
    <section className="ui segment rw-pdf-viewer" aria-label="PDF reader">
      <div className="ui top attached header"><div><h3>Document reader</h3><p>One page is shown at a time. Select text to create a review anchor.</p></div></div>
      <div className="rw-pdf-toolbar" role="toolbar" aria-label="PDF controls">
        <div className="rw-pdf-toolbar__group" aria-label="Page navigation">
          <button type="button" className="ui basic button" data-pdf-previous aria-label="Previous PDF page">Previous</button>
          <label className="rw-pdf-page-control"><span>Page</span><input type="number" min={1} value={pageNumber} data-pdf-page aria-label="Current PDF page" /><span data-pdf-count></span></label>
          <button type="button" className="ui basic button" data-pdf-next aria-label="Next PDF page">Next</button>
        </div>
        <div className="rw-pdf-toolbar__group" aria-label="Display controls">
          <button type="button" className="ui icon basic button" data-pdf-zoom-out aria-label="Zoom out">{"\u2212"}</button>
          <span className="rw-pdf-zoom" data-pdf-zoom aria-live="polite">115%</span>
          <button type="button" className="ui icon basic button" data-pdf-zoom-in aria-label="Zoom in">+</button>
          <button type="button" className="ui basic button" data-pdf-rotate aria-label="Rotate PDF clockwise">Rotate</button>
        </div>
      </div>
      <div className="rw-pdf-pages" data-pdf-pages aria-label="PDF page viewport" aria-live="polite" tabindex={0}></div>
      <p className="rw-pdf-status ui faded text" data-pdf-status role="status">Loading PDF.</p>
    </section>,
    host
  );
  const loadingTask = pdfjs.getDocument({
    url: options.url,
    isEvalSupported: false,
    cMapUrl: cMapURL,
    cMapPacked: true,
    standardFontDataUrl: standardFontDataURL,
  });
  const document = await loadingTask.promise;
  pageNumber = Math.min(pageNumber, document.numPages);

  /** Synchronizes page boundaries, input bounds, and current zoom feedback. */
  function updateControls(): void {
    host.querySelector("[data-pdf-count]")!.textContent = "of " + document.numPages;
    (host.querySelector("[data-pdf-page]") as HTMLInputElement).max = String(document.numPages);
    (host.querySelector("[data-pdf-page]") as HTMLInputElement).value = String(pageNumber);
    (host.querySelector("[data-pdf-previous]") as HTMLButtonElement).disabled = pageNumber <= 1;
    (host.querySelector("[data-pdf-next]") as HTMLButtonElement).disabled = pageNumber >= document.numPages;
    host.querySelector("[data-pdf-zoom]")!.textContent = Math.round(scale * 100) + "%";
  }

  /** Replaces the single visible page and its selectable text and anchor layers. */
  async function render(): Promise<void> {
    if (destroyed) return;
    const sequence = ++renderSequence;
    const requestedPage = pageNumber;
    const requestedScale = scale;
    const requestedRotation = rotation;
    for (const task of renderTasks) task.cancel();
    renderTasks.clear();
    const pagesHost = host.querySelector("[data-pdf-pages]") as HTMLElement;
    pagesHost.setAttribute("aria-busy", "true");
    pagesHost.textContent = "";
    host.querySelector("[data-pdf-status]")!.textContent = "Loading page " + requestedPage + " of " + document.numPages + ".";
    updateControls();
    const page = await document.getPage(requestedPage);
    if (destroyed || sequence !== renderSequence) return;
    const viewport = page.getViewport({ scale: requestedScale, rotation: requestedRotation });
    const section = window.document.createElement("section");
    section.className = "rw-pdf-page rw-pdf-page--current";
    section.dataset.pdfPageNumber = String(requestedPage);
    section.dataset.rotation = String(requestedRotation);
    section.setAttribute("aria-label", "PDF page " + requestedPage);
    section.style.width = viewport.width + "px";
    section.style.height = viewport.height + "px";
    const canvas = window.document.createElement("canvas");
    const ratio = globalThis.devicePixelRatio || 1;
    canvas.width = Math.floor(viewport.width * ratio);
    canvas.height = Math.floor(viewport.height * ratio);
    canvas.style.width = viewport.width + "px";
    canvas.style.height = viewport.height + "px";
    section.append(canvas);
    const textLayer = window.document.createElement("div");
    textLayer.className = "textLayer";
    section.append(textLayer);
    const anchorLayer = window.document.createElement("div");
    anchorLayer.className = "rw-pdf-anchor-layer";
    section.append(anchorLayer);
    pagesHost.append(section);
    const context = canvas.getContext("2d");
    const renderTask = page.render({ canvasContext: context, viewport: viewport, transform: ratio === 1 ? null : [ratio, 0, 0, ratio, 0, 0] });
    renderTasks.add(renderTask);
    await renderTask.promise.catch(function(error: any) { if (error?.name !== "RenderingCancelledException") throw error; });
    renderTasks.delete(renderTask);
    if (destroyed || sequence !== renderSequence) return;
    const content = await page.getTextContent();
    if (destroyed || sequence !== renderSequence) return;
    renderSelectableText(pdfjs, content, textLayer, viewport);
    renderAnchors(anchorLayer, anchors.filter(function(anchor) { return Number(anchor.version.page) === requestedPage; }), requestedRotation);
    pagesHost.setAttribute("aria-busy", "false");
    host.querySelector("[data-pdf-status]")!.textContent = "PDF page " + requestedPage + " of " + document.numPages + ".";
    options.onPageChange?.(requestedPage);
  }

  /** Clamps and renders a requested current page. */
  function changePage(next: any): void {
    pageNumber = Math.max(1, Math.min(document.numPages, Number(next) || pageNumber));
    void render();
  }
  host.querySelector("[data-pdf-previous]")!.addEventListener("click", function() { changePage(pageNumber - 1); });
  host.querySelector("[data-pdf-next]")!.addEventListener("click", function() { changePage(pageNumber + 1); });
  host.querySelector("[data-pdf-page]")!.addEventListener("change", function(event) { changePage((event.target as HTMLInputElement).value); });
  host.querySelector("[data-pdf-zoom-out]")!.addEventListener("click", function() { scale = Math.max(0.6, scale - 0.15); void render(); });
  host.querySelector("[data-pdf-zoom-in]")!.addEventListener("click", function() { scale = Math.min(3, scale + 0.15); void render(); });
  host.querySelector("[data-pdf-rotate]")!.addEventListener("click", function() { rotation = (rotation + 90) % 360; void render(); });
  host.addEventListener("mouseup", function() {
    const selection = window.getSelection();
    const page = selection?.anchorNode?.parentElement?.closest?.(".rw-pdf-page") as HTMLElement | null;
    const rectangles = selectionRectangles(selection, page, Number(page?.dataset.rotation || 0));
    if (rectangles.length) options.onSelection?.({ page: Number(page!.dataset.pdfPageNumber), selectedText: selection!.toString().slice(0, 16384), rectangles: rectangles });
  });
  await render();
  return {
    goToPage: changePage,
    setAnchors: function(nextAnchors: PDFAnchorHead[] | any) {
      anchors = Array.isArray(nextAnchors) ? nextAnchors : [];
      void render();
    },
    destroy: async function(): Promise<void> {
      destroyed = true;
      renderSequence += 1;
      for (const task of renderTasks) task.cancel();
      renderTasks.clear();
      await document.destroy();
      await loadingTask.destroy();
      host.textContent = "";
    },
  };
}

/** Projects active content-matched anchor rectangles into one displayed page layer. */
function renderAnchors(container: HTMLElement, anchors: PDFAnchorHead[], rotation: any): void {
  for (const anchor of anchors) {
    for (const rectangle of rotateRectangles(anchor.version.rectangles || [], rotation)) {
      const highlight = window.document.createElement("span");
      highlight.className = "rw-pdf-anchor-highlight";
      highlight.dataset.anchorId = anchor.id;
      highlight.setAttribute("aria-hidden", "true");
      highlight.style.left = (rectangle.x * 100) + "%";
      highlight.style.top = (rectangle.y * 100) + "%";
      highlight.style.width = (rectangle.width * 100) + "%";
      highlight.style.height = (rectangle.height * 100) + "%";
      container.append(highlight);
    }
  }
}

/** Creates transparent positioned text spans from PDF.js text content and viewport transforms. */
function renderSelectableText(pdfjs: any, content: any, container: HTMLElement, viewport: any): void {
  const fragment = window.document.createDocumentFragment();
  for (const item of content.items || []) {
    if (!item.str) continue;
    const transform = pdfjs.Util.transform(viewport.transform, item.transform);
    const angle = Math.atan2(transform[1], transform[0]);
    const height = Math.hypot(transform[2], transform[3]);
    const span = window.document.createElement("span");
    span.textContent = item.str;
    span.style.left = transform[4] + "px";
    span.style.top = transform[5] - height + "px";
    span.style.fontSize = height + "px";
    span.style.fontFamily = content.styles?.[item.fontName]?.fontFamily || "sans-serif";
    if (angle) span.style.transform = `rotate(${angle}rad)`;
    fragment.append(span);
    if (item.hasEOL) fragment.append(window.document.createElement("br"));
  }
  container.append(fragment);
}