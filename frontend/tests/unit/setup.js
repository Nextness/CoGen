// Shared JSDOM setup for frontend unit tests.
// Import this as a side-effect before importing any frontend module:
//   import './setup.js';
//   import { esc } from '../../src/server/frontend/state.js';

import { JSDOM } from 'jsdom';

const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', {
  url: 'http://127.0.0.1:8080/?view=overview',
  pretendToBeVisual: true,
});

const window = dom.window;
const document = window.document;

// --- Build the DOM structure the app expects ---
document.body.innerHTML = `
  <div id="app"></div>
  <div id="notice" hidden></div>
  <div id="loading" hidden></div>
  <div id="health-status"></div>
  <div id="selection-summary"></div>
  <button id="clear-context"></button>
  <select id="search-select"></select>
  <select id="revision-select"></select>
  <select id="plan-select"></select>
  <select id="run-select"></select>
  <div id="graph-edge-rows"></div>
  <div id="graph-selection"></div>
  <div id="graph-layout-status"></div>
  <button id="graph-run-layout"></button>
  <button id="graph-fit"></button>
  <button id="graph-clear-selection"></button>
  <canvas class="rw-graph__canvas" width="800" height="600"></canvas>
  <div id="artifact-inspector"><div class="panel-body"></div></div>
`;

// --- Assign window globals FIRST so polyfills can reference them ---
globalThis.window = window;
globalThis.document = document;
globalThis.location = window.location;
globalThis.history = window.history;
globalThis.getComputedStyle = window.getComputedStyle.bind(window);
globalThis.HTMLElement = window.HTMLElement;
globalThis.HTMLSelectElement = window.HTMLSelectElement;
globalThis.HTMLInputElement = window.HTMLInputElement;
globalThis.HTMLButtonElement = window.HTMLButtonElement;
globalThis.HTMLCanvasElement = window.HTMLCanvasElement;
globalThis.HTMLDivElement = window.HTMLDivElement;
globalThis.HTMLSpanElement = window.HTMLSpanElement;
globalThis.HTMLTableElement = window.HTMLTableElement;
globalThis.HTMLTableRowElement = window.HTMLTableRowElement;
globalThis.HTMLTableCellElement = window.HTMLTableCellElement;
globalThis.CustomEvent = window.CustomEvent;
globalThis.KeyboardEvent = window.KeyboardEvent;
globalThis.MouseEvent = window.MouseEvent;
globalThis.FocusEvent = window.FocusEvent;
globalThis.UIEvent = window.UIEvent;
globalThis.FormData = window.FormData;
globalThis.URL = window.URL;
globalThis.URLSearchParams = window.URLSearchParams;
globalThis.MutationObserver = window.MutationObserver || class { observe() {} disconnect() {} };
globalThis.devicePixelRatio = 1;

// --- Polyfill missing browser APIs ---

// matchMedia
if (!globalThis.matchMedia) {
  globalThis.matchMedia = function() {
    return { matches: false, addEventListener: function() {}, removeEventListener: function() {} };
  };
}

// requestAnimationFrame / cancelAnimationFrame
if (!globalThis.requestAnimationFrame) {
  globalThis.requestAnimationFrame = function(cb) { return setTimeout(cb, 0); };
}
if (!globalThis.cancelAnimationFrame) {
  globalThis.cancelAnimationFrame = function(id) { clearTimeout(id); };
}

// ResizeObserver
if (!globalThis.ResizeObserver) {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

// HTMLCanvasElement.getContext('2d') mock — always override since jsdom
// provides a stub that throws "Not implemented"
HTMLCanvasElement.prototype.getContext = function() {
    return {
      canvas: this,
      setTransform: function() {},
      clearRect: function() {},
      fillRect: function() {},
      fillStyle: '',
      strokeStyle: '',
      lineWidth: 1,
      globalAlpha: 1,
      font: '',
      textBaseline: 'alphabetic',
      beginPath: function() {},
      moveTo: function() {},
      lineTo: function() {},
      closePath: function() {},
      fill: function() {},
      stroke: function() {},
      arc: function() {},
      measureText: function() { return { width: 0 }; },
      fillText: function() {},
      translate: function() {},
      scale: function() {},
      save: function() {},
      restore: function() {},
    };
  };

// navigator.clipboard
if (!globalThis.navigator) {
  globalThis.navigator = {};
}
if (!globalThis.navigator.clipboard) {
  globalThis.navigator.clipboard = {
    writeText: function() { return Promise.resolve(); },
    readText: function() { return Promise.resolve(''); },
  };
}

// fetch
if (!globalThis.fetch) {
  globalThis.fetch = function() {
    return Promise.resolve({
      ok: true,
      status: 200,
      json: function() { return Promise.resolve({ data: [] }); },
    });
  };
}

// AbortController (should exist in Node 26, but just in case)
if (!globalThis.AbortController) {
  globalThis.AbortController = class {
    constructor() { this.signal = { aborted: false }; }
    abort() { this.signal.aborted = true; }
  };
}