// Custom PDF.js rendering with bounded page lifecycle and accessible anchor selection.
import { h, Fragment, render as renderTree, cx } from "../jsx/jsx-runtime.ts";

/** Typed compound class names used by this module. */
const classNames = {
  rwPdfStatusUiFadedText: cx("rw-pdf-status", "ui", "faded", "text"),
  uiBasicButton: cx("ui", "basic", "button"),
  uiIconBasicButton: cx("ui", "icon", "basic", "button"),
  uiPrimaryButton: cx("ui", "primary", "button"),
  uiSegmentRwPdfViewer: cx("ui", "segment", "rw-pdf-viewer"),
  uiTopAttachedHeader: cx("ui", "top", "attached", "header"),
};

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
  return rectangles.map(({ x, y, width, height }) => {
    let result: NormalizedRectangle = { x: x, y: y, width: width, height: height };
    if (angle === 90) {
      result = {
        x: y,
        y: 1 - x - width,
        width: height,
        height: width,
      };
    } else if (angle === 180) {
      result = {
        x: 1 - x - width,
        y: 1 - y - height,
        width: width,
        height: height,
      };
    } else if (angle === 270) {
      result = {
        x: 1 - y - height,
        y: x,
        width: height,
        height: width,
      };
    }
    return {
      x: Number(result.x.toFixed(12)),
      y: Number(result.y.toFixed(12)),
      width: Number(result.width.toFixed(12)),
      height: Number(result.height.toFixed(12)),
    };
  });
}

/** Projects stored unrotated rectangles into the currently displayed page rotation. */
export function rotateRectangles(rectangles: NormalizedRectangle[], rotation: any): NormalizedRectangle[] {
  const angle = ((Number(rotation) % 360) + 360) % 360;
  return rectangles.map(({ x, y, width, height }) => {
    let result: NormalizedRectangle = { x: x, y: y, width: width, height: height };
    if (angle === 90) {
      result = {
        x: 1 - y - height,
        y: x,
        width: height,
        height: width,
      };
    } else if (angle === 180) {
      result = {
        x: 1 - x - width,
        y: 1 - y - height,
        width: width,
        height: height,
      };
    } else if (angle === 270) {
      result = {
        x: y,
        y: 1 - x - width,
        width: height,
        height: width,
      };
    }
    return {
      x: Number(result.x.toFixed(12)),
      y: Number(result.y.toFixed(12)),
      width: Number(result.width.toFixed(12)),
      height: Number(result.height.toFixed(12)),
    };
  });
}

/** Extracts bounded normalized rectangles from a same-page browser selection. */
export function selectionRectangles(selection: any, pageElement: HTMLElement | null, rotation: any): NormalizedRectangle[] {
  if (!selection || selection.isCollapsed || !selection.rangeCount || !pageElement) return [];
  const range: Range = selection.getRangeAt(0);
  if (!pageElement.contains(range.commonAncestorContainer)) return [];
  const pageRect = pageElement.getBoundingClientRect();
  if (!(pageRect.width > 0 && pageRect.height > 0)) return [];
  const clientRects = Array.from(range.getClientRects());
  const visibleRects = clientRects.filter(({ width, height }) => {
    return width > 0 && height > 0;
  });
  const boundedRects = visibleRects.slice(0, 64);
  const rectangles: NormalizedRectangle[] = boundedRects.map(({ left, top, width, height }) => {
    return {
      x: Math.max(0, Math.min(1, (left - pageRect.left) / pageRect.width)),
      y: Math.max(0, Math.min(1, (top - pageRect.top) / pageRect.height)),
      width: Math.max(0, Math.min(1, width / pageRect.width)),
      height: Math.max(0, Math.min(1, height / pageRect.height)),
    };
  });
  const rotatedRects = unrotateRectangles(rectangles, rotation);
  return rotatedRects.filter(({ x, y, width, height }) => {
    return width > 0 && height > 0 && x + width <= 1.000001 && y + height <= 1.000001;
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
  const loadPDFJS = loader || (() => {
    return import("../../vendor/pdfjs/pdf.min.mjs");
  });
  const pdfjs = await loadPDFJS();
  pdfjs.GlobalWorkerOptions.workerSrc = workerURL;
  let pageNumber = Math.max(1, Number(options.page || 1));
  let scale = 1.15;
  let rotation = 0;
  let destroyed = false;
  let anchors: PDFAnchorHead[] = [];
  let renderSequence = 0;
  const renderTasks = new Set<any>();
  const pageCache = new Map<number, Promise<any>>();
  const textCache = new Map<number, Promise<any>>();
  let lastSelectionIdentity = "";
  let pendingSelection: { page: number; selectedText: string; rectangles: NormalizedRectangle[] } | null = null;

  const headerMarkup = (
    <div className={classNames.uiTopAttachedHeader}>
      <div>
        <h3>Document reader</h3>
        <p>One page is shown at a time. Select text to create a review anchor.</p>
      </div>
    </div>
  );
  const pageToolbar = (
    <div className="rw-pdf-toolbar__group" aria-label="Page navigation">
      <button type="button" className={classNames.uiBasicButton} data-pdf-previous aria-label="Previous PDF page">Previous</button>
      <label className="rw-pdf-page-control">
        <span>Page</span>
        <input type="number" min={1} value={pageNumber} data-pdf-page aria-label="Current PDF page" />
        <span data-pdf-count></span>
      </label>
      <button type="button" className={classNames.uiBasicButton} data-pdf-next aria-label="Next PDF page">Next</button>
    </div>
  );
  const displayToolbar = (
    <div className="rw-pdf-toolbar__group" aria-label="Display controls">
      <button type="button" className={classNames.uiIconBasicButton} data-pdf-zoom-out aria-label="Zoom out">{"\u2212"}</button>
      <span className="rw-pdf-zoom" data-pdf-zoom aria-live="polite">115%</span>
      <button type="button" className={classNames.uiIconBasicButton} data-pdf-zoom-in aria-label="Zoom in">+</button>
      <button type="button" className={classNames.uiBasicButton} data-pdf-fit-width aria-label="Fit PDF page to reader width">Fit width</button>
      <button type="button" className={classNames.uiBasicButton} data-pdf-rotate aria-label="Rotate PDF clockwise">Rotate</button>
      <button type="button" className={classNames.uiPrimaryButton} data-pdf-review-selection hidden>Review selection</button>
    </div>
  );
  const statusText = <p className={classNames.rwPdfStatusUiFadedText} data-pdf-status role="status">Loading PDF.</p>;
  const viewerMarkup = (
    <section className={classNames.uiSegmentRwPdfViewer} aria-label="PDF reader">
      {headerMarkup}
      <div className="rw-pdf-toolbar" role="toolbar" aria-label="PDF controls">
        {pageToolbar}
        {displayToolbar}
      </div>
      <div className="rw-pdf-pages" data-pdf-pages aria-label="PDF page viewport" tabindex={0}></div>
      {statusText}
    </section>
  );
  renderTree(viewerMarkup, host);
  const loadingTask = pdfjs.getDocument({
    url: options.url,
    isEvalSupported: false,
    cMapUrl: cMapURL,
    cMapPacked: true,
    standardFontDataUrl: standardFontDataURL,
  });
  const document = await loadingTask.promise;
  pageNumber = Math.min(pageNumber, document.numPages);

  /** Returns one cached PDF.js page object without repeating document parsing. */
  function cachedPage(requestedPage: number): Promise<any> {
    const cached = pageCache.get(requestedPage);
    if (cached) return cached;
    const requested = Promise.resolve(document.getPage(requestedPage));
    pageCache.set(requestedPage, requested);
    return requested;
  }

  /** Returns one cached text-content projection for a loaded page. */
  function cachedText(requestedPage: number, page: any): Promise<any> {
    const cached = textCache.get(requestedPage);
    if (cached) return cached;
    const requested = Promise.resolve(page.getTextContent());
    textCache.set(requestedPage, requested);
    return requested;
  }

  /** Synchronizes page boundaries, input bounds, and current zoom feedback. */
  function updateControls(): void {
    host.querySelector("[data-pdf-count]")!.textContent = `of ${document.numPages}`;
    (host.querySelector("[data-pdf-page]") as HTMLInputElement).max = String(document.numPages);
    (host.querySelector("[data-pdf-page]") as HTMLInputElement).value = String(pageNumber);
    (host.querySelector("[data-pdf-previous]") as HTMLButtonElement).disabled = pageNumber <= 1;
    (host.querySelector("[data-pdf-next]") as HTMLButtonElement).disabled = pageNumber >= document.numPages;
    host.querySelector("[data-pdf-zoom]")!.textContent = `${Math.round(scale * 100)}%`;
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
    host.querySelector("[data-pdf-status]")!.textContent = `Loading page ${requestedPage} of ${document.numPages}.`;
    updateControls();
    const page = await cachedPage(requestedPage);
    if (destroyed || sequence !== renderSequence) return;
    const viewport = page.getViewport({ scale: requestedScale, rotation: requestedRotation });
    const section = window.document.createElement("section");
    section.className = "rw-pdf-page";
    section.dataset.pdfPageNumber = String(requestedPage);
    section.dataset.rotation = String(requestedRotation);
    section.setAttribute("aria-label", `PDF page ${requestedPage}`);
    section.style.width = `${viewport.width}px`;
    section.style.height = `${viewport.height}px`;
    const canvas = window.document.createElement("canvas");
    const ratio = globalThis.devicePixelRatio || 1;
    canvas.width = Math.floor(viewport.width * ratio);
    canvas.height = Math.floor(viewport.height * ratio);
    canvas.style.width = `${viewport.width}px`;
    canvas.style.height = `${viewport.height}px`;
    section.append(canvas);
    const textLayer = window.document.createElement("div");
    textLayer.className = "textLayer";
    section.append(textLayer);
    const anchorLayer = window.document.createElement("div");
    anchorLayer.className = "rw-pdf-anchor-layer";
    section.append(anchorLayer);
    const context = canvas.getContext("2d");
    let transform: number[] | null = [ratio, 0, 0, ratio, 0, 0];
    if (ratio === 1) transform = null;
    const renderTask = page.render({
      canvasContext: context,
      viewport: viewport,
      transform: transform,
    });
    renderTasks.add(renderTask);
    await renderTask.promise.catch((error: any) => {
      if (error?.name !== "RenderingCancelledException") throw error;
    });
    renderTasks.delete(renderTask);
    if (destroyed || sequence !== renderSequence) return;
    const content = await cachedText(requestedPage, page);
    if (destroyed || sequence !== renderSequence) return;
    renderSelectableText(pdfjs, content, textLayer, viewport);
    const pageAnchors = anchors.filter(({ version }) => {
      return Number(version.page) === requestedPage;
    });
    renderAnchors(anchorLayer, pageAnchors, requestedRotation);
    pagesHost.replaceChildren(section);
    pagesHost.setAttribute("aria-busy", "false");
    host.querySelector("[data-pdf-status]")!.textContent = `PDF page ${requestedPage} of ${document.numPages}.`;
    options.onPageChange?.(requestedPage);
  }

  /** Runs one render with a local retry state while preserving the previous completed frame. */
  async function requestRender(): Promise<void> {
    try {
      await render();
    } catch (error: any) {
      if (destroyed || error?.name === "RenderingCancelledException") return;
      const pagesHost = host.querySelector<HTMLElement>("[data-pdf-pages]");
      pagesHost?.setAttribute("aria-busy", "false");
      const status = host.querySelector<HTMLElement>("[data-pdf-status]");
      if (!status) return;
      const errorMarkup = (
        <Fragment>
          Page {pageNumber} could not be rendered: {error.message}
          <button type="button" className={classNames.uiBasicButton} data-pdf-retry>Retry page</button>
        </Fragment>
      );
      renderTree(errorMarkup, status);
      status.querySelector<HTMLButtonElement>("[data-pdf-retry]")?.addEventListener("click", () => {
        void requestRender();
      });
    }
  }

  /** Clamps and renders a requested current page. */
  function changePage(next: any): void {
    pageNumber = Math.max(1, Math.min(document.numPages, Number(next) || pageNumber));
    void requestRender();
  }
  host.querySelector("[data-pdf-previous]")!.addEventListener("click", () => { changePage(pageNumber - 1); });
  host.querySelector("[data-pdf-next]")!.addEventListener("click", () => { changePage(pageNumber + 1); });
  host.querySelector("[data-pdf-page]")!.addEventListener("change", (event) => { changePage((event.target as HTMLInputElement).value); });
  host.querySelector("[data-pdf-zoom-out]")!.addEventListener("click", () => { scale = Math.max(0.6, scale - 0.15); void requestRender(); });
  host.querySelector("[data-pdf-zoom-in]")!.addEventListener("click", () => { scale = Math.min(3, scale + 0.15); void requestRender(); });
  host.querySelector("[data-pdf-rotate]")!.addEventListener("click", () => { rotation = (rotation + 90) % 360; void requestRender(); });
  host.querySelector("[data-pdf-fit-width]")!.addEventListener("click", async () => {
    const page = await cachedPage(pageNumber);
    const viewport = page.getViewport({ scale: 1, rotation: rotation });
    const pagesHost = host.querySelector("[data-pdf-pages]") as HTMLElement;
    const availableWidth = Math.max(1, pagesHost.clientWidth - 32);
    scale = Math.max(0.6, Math.min(3, availableWidth / viewport.width));
    await requestRender();
  });

  /** Hands one mouse- or keyboard-originated same-page text selection to review controls. */
  function captureSelection(): void {
    const selection = window.getSelection();
    const anchorNode = selection?.anchorNode;
    const page = anchorNode?.parentElement?.closest?.(".rw-pdf-page") as HTMLElement | null;
    const rectangles = selectionRectangles(selection, page, Number(page?.dataset.rotation || 0));
    if (rectangles.length) {
      const identity = `${page?.dataset.pdfPageNumber}:${selection!.toString()}:${JSON.stringify(rectangles)}`;
      if (identity === lastSelectionIdentity) return;
      lastSelectionIdentity = identity;
      pendingSelection = {
        page: Number(page!.dataset.pdfPageNumber),
        selectedText: selection!.toString().slice(0, 16384),
        rectangles: rectangles,
      };
      const handoff = host.querySelector<HTMLButtonElement>("[data-pdf-review-selection]")!;
      handoff.hidden = false;
      host.querySelector("[data-pdf-status]")!.textContent = "Text selected. Activate Review selection to create a PDF anchor.";
    }
  }
  host.addEventListener("mouseup", captureSelection);
  const pagesHost = host.querySelector("[data-pdf-pages]") as HTMLElement;
  pagesHost.addEventListener("keyup", captureSelection);
  host.querySelector("[data-pdf-review-selection]")!.addEventListener("click", () => {
    if (!pendingSelection) return;
    options.onSelection?.(pendingSelection);
    pendingSelection = null;
    (host.querySelector("[data-pdf-review-selection]") as HTMLButtonElement).hidden = true;
  });
  pagesHost.addEventListener("keydown", (event) => {
    if (event.key === "PageUp" || event.key === "ArrowLeft") {
      event.preventDefault();
      changePage(pageNumber - 1);
    } else if (event.key === "PageDown" || event.key === "ArrowRight") {
      event.preventDefault();
      changePage(pageNumber + 1);
    } else if (event.key === "+" || event.key === "=") {
      event.preventDefault();
      scale = Math.min(3, scale + 0.15);
      void requestRender();
    } else if (event.key === "-") {
      event.preventDefault();
      scale = Math.max(0.6, scale - 0.15);
      void requestRender();
    }
  });
  await requestRender();
  return {
    goToPage: changePage,
    setAnchors: (nextAnchors: PDFAnchorHead[] | any) => {
      let effectiveAnchors: PDFAnchorHead[] = [];
      if (Array.isArray(nextAnchors)) effectiveAnchors = nextAnchors;
      anchors = effectiveAnchors;
      const section = host.querySelector<HTMLElement>(".rw-pdf-page");
      const anchorLayer = section?.querySelector<HTMLElement>(".rw-pdf-anchor-layer");
      if (anchorLayer && section) {
        anchorLayer.textContent = "";
        const visible = anchors.filter(({ version }) => {
          return Number(version.page) === Number(section.dataset.pdfPageNumber);
        });
        renderAnchors(anchorLayer, visible, Number(section.dataset.rotation || 0));
      }
    },
    destroy: async (): Promise<void> => {
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
      highlight.style.left = `${rectangle.x * 100}%`;
      highlight.style.top = `${rectangle.y * 100}%`;
      highlight.style.width = `${rectangle.width * 100}%`;
      highlight.style.height = `${rectangle.height * 100}%`;
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
    const [scaleX, skewY, skewX, scaleY, offsetX, offsetY] = transform;
    const angle = Math.atan2(skewY, scaleX);
    const height = Math.hypot(skewX, scaleY);
    const span = window.document.createElement("span");
    span.textContent = item.str;
    span.style.left = `${offsetX}px`;
    span.style.top = `${offsetY - height}px`;
    span.style.fontSize = `${height}px`;
    span.style.fontFamily = content.styles?.[item.fontName]?.fontFamily || "sans-serif";
    if (angle) span.style.transform = `rotate(${angle}rad)`;
    fragment.append(span);
    if (item.hasEOL) fragment.append(window.document.createElement("br"));
  }
  container.append(fragment);
}
