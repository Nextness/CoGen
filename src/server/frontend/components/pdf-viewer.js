// Custom PDF.js rendering with bounded page lifecycle and accessible anchor selection.

const workerURL = '/vendor/pdfjs/pdf.worker.min.mjs';
const cMapURL = '/vendor/pdfjs/cmaps/';
const standardFontDataURL = '/vendor/pdfjs/standard_fonts/';

/** Converts displayed normalized rectangles back to unrotated page coordinates. */
export function unrotateRectangles(rectangles, rotation) {
  const angle = ((Number(rotation) % 360) + 360) % 360;
  return rectangles.map(function(rectangle) {
    const x = rectangle.x;
    const y = rectangle.y;
    const width = rectangle.width;
    const height = rectangle.height;
    let result;
    if (angle === 90) result = { x: y, y: 1 - x - width, width: height, height: width };
    else if (angle === 180) result = { x: 1 - x - width, y: 1 - y - height, width: width, height: height };
    else if (angle === 270) result = { x: 1 - y - height, y: x, width: height, height: width };
    else result = { x: x, y: y, width: width, height: height };
    return Object.fromEntries(Object.entries(result).map(function([key, value]) { return [key, Number(value.toFixed(12))]; }));
  });
}

/** Projects stored unrotated rectangles into the currently displayed page rotation. */
export function rotateRectangles(rectangles, rotation) {
  const angle = ((Number(rotation) % 360) + 360) % 360;
  return rectangles.map(function(rectangle) {
    const x = rectangle.x;
    const y = rectangle.y;
    const width = rectangle.width;
    const height = rectangle.height;
    let result;
    if (angle === 90) result = { x: 1 - y - height, y: x, width: height, height: width };
    else if (angle === 180) result = { x: 1 - x - width, y: 1 - y - height, width: width, height: height };
    else if (angle === 270) result = { x: y, y: 1 - x - width, width: height, height: width };
    else result = { x: x, y: y, width: width, height: height };
    return Object.fromEntries(Object.entries(result).map(function([key, value]) { return [key, Number(value.toFixed(12))]; }));
  });
}

/** Extracts bounded normalized rectangles from a same-page browser selection. */
export function selectionRectangles(selection, pageElement, rotation) {
  if (!selection || selection.isCollapsed || !selection.rangeCount || !pageElement) return [];
  const range = selection.getRangeAt(0);
  if (!pageElement.contains(range.commonAncestorContainer)) return [];
  const pageRect = pageElement.getBoundingClientRect();
  if (!(pageRect.width > 0 && pageRect.height > 0)) return [];
  const rectangles = Array.from(range.getClientRects()).filter(function(rectangle) {
    return rectangle.width > 0 && rectangle.height > 0;
  }).slice(0, 64).map(function(rectangle) {
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

/** Mounts a project-styled PDF.js viewer and returns a lifecycle controller. */
export async function mountPDFViewer(host, options, loader) {
  const loadPDFJS = loader || function() { return import('../vendor/pdfjs/pdf.min.mjs'); };
  const pdfjs = await loadPDFJS();
  pdfjs.GlobalWorkerOptions.workerSrc = workerURL;
  let pageNumber = Math.max(1, Number(options.page || 1));
  let scale = 1.15;
  let rotation = 0;
  let destroyed = false;
  let anchors = [];
  const renderTasks = new Set();

  host.innerHTML = '<section class="rw-pdf-viewer" aria-label="PDF reader"><div class="rw-pdf-toolbar">'
    + '<button type="button" data-pdf-previous aria-label="Previous PDF page">Previous</button>'
    + '<label>Page <input type="number" min="1" value="' + pageNumber + '" data-pdf-page></label><span data-pdf-count></span>'
    + '<button type="button" data-pdf-next aria-label="Next PDF page">Next</button>'
    + '<button type="button" data-pdf-zoom-out aria-label="Zoom out">−</button>'
    + '<button type="button" data-pdf-zoom-in aria-label="Zoom in">+</button>'
    + '<button type="button" data-pdf-rotate aria-label="Rotate PDF clockwise">Rotate</button></div>'
    + '<div class="rw-pdf-pages" data-pdf-pages aria-label="PDF pages" aria-live="polite" tabindex="0"></div><p class="rw-pdf-status" data-pdf-status>Loading PDF.</p></section>';
  const loadingTask = pdfjs.getDocument({
    url: options.url,
    isEvalSupported: false,
    cMapUrl: cMapURL,
    cMapPacked: true,
    standardFontDataUrl: standardFontDataURL,
  });
  const document = await loadingTask.promise;
  pageNumber = Math.min(pageNumber, document.numPages);
  host.querySelector('[data-pdf-count]').textContent = 'of ' + document.numPages;

  /** Replaces the bounded nearby-page set and its selectable text and anchor layers. */
  async function render() {
    if (destroyed) return;
    for (const task of renderTasks) task.cancel();
    renderTasks.clear();
    const pagesHost = host.querySelector('[data-pdf-pages]');
    pagesHost.textContent = '';
    const numbers = [pageNumber - 1, pageNumber, pageNumber + 1].filter(function(number) { return number >= 1 && number <= document.numPages; });
    for (const number of numbers) {
      const page = await document.getPage(number);
      if (destroyed) return;
      const viewport = page.getViewport({ scale: scale, rotation: rotation });
      const section = window.document.createElement('section');
      section.className = 'rw-pdf-page' + (number === pageNumber ? ' rw-pdf-page--current' : '');
      section.dataset.pdfPageNumber = String(number);
      section.dataset.rotation = String(rotation);
      section.setAttribute('aria-label', 'PDF page ' + number);
      section.style.width = viewport.width + 'px';
      section.style.height = viewport.height + 'px';
      const canvas = window.document.createElement('canvas');
      const ratio = globalThis.devicePixelRatio || 1;
      canvas.width = Math.floor(viewport.width * ratio);
      canvas.height = Math.floor(viewport.height * ratio);
      canvas.style.width = viewport.width + 'px';
      canvas.style.height = viewport.height + 'px';
      section.append(canvas);
      const textLayer = window.document.createElement('div');
      textLayer.className = 'textLayer';
      section.append(textLayer);
      const anchorLayer = window.document.createElement('div');
      anchorLayer.className = 'rw-pdf-anchor-layer';
      section.append(anchorLayer);
      pagesHost.append(section);
      const context = canvas.getContext('2d');
      const renderTask = page.render({ canvasContext: context, viewport: viewport, transform: ratio === 1 ? null : [ratio, 0, 0, ratio, 0, 0] });
      renderTasks.add(renderTask);
      await renderTask.promise.catch(function(error) { if (error?.name !== 'RenderingCancelledException') throw error; });
      renderTasks.delete(renderTask);
      if (destroyed) return;
      const content = await page.getTextContent();
      renderSelectableText(pdfjs, content, textLayer, viewport);
      renderAnchors(anchorLayer, anchors.filter(function(anchor) { return Number(anchor.version.page) === number; }), rotation);
    }
    host.querySelector('[data-pdf-page]').value = pageNumber;
    host.querySelector('[data-pdf-status]').textContent = 'PDF page ' + pageNumber + ' of ' + document.numPages + '.';
    options.onPageChange?.(pageNumber);
  }

  /** Clamps and renders a requested current page. */
  function changePage(next) {
    pageNumber = Math.max(1, Math.min(document.numPages, Number(next) || pageNumber));
    void render();
  }
  host.querySelector('[data-pdf-previous]').addEventListener('click', function() { changePage(pageNumber - 1); });
  host.querySelector('[data-pdf-next]').addEventListener('click', function() { changePage(pageNumber + 1); });
  host.querySelector('[data-pdf-page]').addEventListener('change', function(event) { changePage(event.target.value); });
  host.querySelector('[data-pdf-zoom-out]').addEventListener('click', function() { scale = Math.max(0.6, scale - 0.15); void render(); });
  host.querySelector('[data-pdf-zoom-in]').addEventListener('click', function() { scale = Math.min(3, scale + 0.15); void render(); });
  host.querySelector('[data-pdf-rotate]').addEventListener('click', function() { rotation = (rotation + 90) % 360; void render(); });
  host.addEventListener('mouseup', function() {
    const selection = window.getSelection();
    const page = selection?.anchorNode?.parentElement?.closest?.('.rw-pdf-page');
    const rectangles = selectionRectangles(selection, page, Number(page?.dataset.rotation || 0));
    if (rectangles.length) options.onSelection?.({ page: Number(page.dataset.pdfPageNumber), selectedText: selection.toString().slice(0, 16384), rectangles: rectangles });
  });
  await render();
  return {
    goToPage: changePage,
    setAnchors: function(nextAnchors) {
      anchors = Array.isArray(nextAnchors) ? nextAnchors : [];
      void render();
    },
    destroy: async function() {
      destroyed = true;
      for (const task of renderTasks) task.cancel();
      renderTasks.clear();
      await document.destroy();
      await loadingTask.destroy();
      host.textContent = '';
    },
  };
}

/** Projects active content-matched anchor rectangles into one displayed page layer. */
function renderAnchors(container, anchors, rotation) {
  for (const anchor of anchors) {
    for (const rectangle of rotateRectangles(anchor.version.rectangles || [], rotation)) {
      const highlight = window.document.createElement('span');
      highlight.className = 'rw-pdf-anchor-highlight';
      highlight.dataset.anchorId = anchor.id;
      highlight.setAttribute('aria-hidden', 'true');
      highlight.style.left = (rectangle.x * 100) + '%';
      highlight.style.top = (rectangle.y * 100) + '%';
      highlight.style.width = (rectangle.width * 100) + '%';
      highlight.style.height = (rectangle.height * 100) + '%';
      container.append(highlight);
    }
  }
}

/** Creates transparent positioned text spans from PDF.js text content and viewport transforms. */
function renderSelectableText(pdfjs, content, container, viewport) {
  const fragment = window.document.createDocumentFragment();
  for (const item of content.items || []) {
    if (!item.str) continue;
    const transform = pdfjs.Util.transform(viewport.transform, item.transform);
    const angle = Math.atan2(transform[1], transform[0]);
    const height = Math.hypot(transform[2], transform[3]);
    const span = window.document.createElement('span');
    span.textContent = item.str;
    span.style.left = transform[4] + 'px';
    span.style.top = transform[5] - height + 'px';
    span.style.fontSize = height + 'px';
    span.style.fontFamily = content.styles?.[item.fontName]?.fontFamily || 'sans-serif';
    if (angle) span.style.transform = `rotate(${angle}rad)`;
    fragment.append(span);
    if (item.hasEOL) fragment.append(window.document.createElement('br'));
  }
  container.append(fragment);
}
