// Unit tests for content-coordinate projection used by PDF anchors.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { mountPDFViewer, rotateRectangles, unrotateRectangles } from '../../../src/components/pdf-viewer.js';

describe('pdf-viewer.js', function() {
  it('projects displayed rectangles back through all supported rotations', function() {
    const rectangle = { x: 0.1, y: 0.2, width: 0.3, height: 0.4 };
    assert.deepEqual(unrotateRectangles([rectangle], 0)[0], rectangle);
    assert.deepEqual(unrotateRectangles([rectangle], 90)[0], { x: 0.2, y: 0.6, width: 0.4, height: 0.3 });
    assert.deepEqual(unrotateRectangles([rectangle], 180)[0], { x: 0.6, y: 0.4, width: 0.3, height: 0.4 });
    assert.deepEqual(unrotateRectangles([rectangle], 270)[0], { x: 0.4, y: 0.1, width: 0.4, height: 0.3 });
  });

  it('round-trips stored rectangles into each displayed rotation', function() {
    const rectangle = { x: 0.1, y: 0.2, width: 0.3, height: 0.4 };
    for (const rotation of [0, 90, 180, 270]) {
      assert.deepEqual(unrotateRectangles(rotateRectangles([rectangle], rotation), rotation)[0], rectangle);
    }
  });

  it('renders one page and advances it with one Next activation', async function() {
    const renderedPages = [];
    const documentMock = {
      numPages: 3,
      getPage: async function(pageNumber) {
        return {
          getViewport: function() { return { width: 240, height: 320, transform: [1, 0, 0, 1, 0, 0] }; },
          render: function() {
            renderedPages.push(pageNumber);
            return { cancel: function() {}, promise: Promise.resolve() };
          },
          getTextContent: async function() { return { items: [{ str: `Page ${pageNumber}`, transform: [1, 0, 0, 12, 10, 20], fontName: 'sans' }], styles: { sans: { fontFamily: 'sans-serif' } } }; },
        };
      },
      destroy: async function() {},
    };
    const loadingTask = { promise: Promise.resolve(documentMock), destroy: async function() {} };
    const pdfjsMock = {
      GlobalWorkerOptions: {},
      Util: { transform: function(_viewport, item) { return item; } },
      getDocument: function() { return loadingTask; },
    };
    const host = document.createElement('div');
    document.body.append(host);
    const changes = [];
    const controller = await mountPDFViewer(host, { url: '/fixture.pdf', page: 1, onPageChange: function(page) { changes.push(page); } }, async function() { return pdfjsMock; });

    assert.equal(host.querySelectorAll('.rw-pdf-page').length, 1);
    assert.equal(host.querySelector('.rw-pdf-page').dataset.pdfPageNumber, '1');
    assert.equal(host.querySelector('[data-pdf-previous]').disabled, true);
    host.querySelector('[data-pdf-next]').click();
    for (let attempt = 0; attempt < 10 && host.querySelector('[data-pdf-status]').textContent !== 'PDF page 2 of 3.'; attempt += 1) await new Promise(function(resolve) { setTimeout(resolve, 0); });

    assert.equal(host.querySelectorAll('.rw-pdf-page').length, 1);
    assert.equal(host.querySelector('.rw-pdf-page').dataset.pdfPageNumber, '2');
    assert.equal(host.querySelector('[data-pdf-page]').value, '2');
    assert.deepEqual(renderedPages, [1, 2]);
    assert.deepEqual(changes, [1, 2]);
    await controller.destroy();
    host.remove();
  });
});
