// Unit tests for the project-owned JSX runtime. The runtime is exercised
// directly through h() and Fragment() because Node's native type stripping
// cannot parse JSX in a .ts entry file; the JSX-through-loader path is covered
// by the migrated .tsx source modules.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import './setup.ts';
import { h, Fragment, classAdd, classHas, classRemove, classToggle, cx, render, renderToString, raw } from "../../src/jsx/jsx-runtime.ts";

describe("jsx-runtime - class helpers", () => {
  it("joins registered tokens and omits conditional gaps", () => {
    assert.equal(cx("ui", false, null, "button", undefined), "ui button");
  });

  it("applies typed DOM class operations", () => {
    const node = document.createElement("div");
    classAdd(node, ["ui", "button"]);
    assert.equal(classHas(node, "button"), true);
    assert.equal(classToggle(node, "active", true), true);
    classRemove(node, "ui");
    assert.equal(node.className, "button active");
  });
});

describe('jsx-runtime — h and Fragment', function() {
  it('builds an intrinsic element with attributes and text children', function() {
    const node = h("a", { className: "brand", href: "/x" }, "Hello");
    assert.ok(node instanceof HTMLAnchorElement);
    assert.equal(node.getAttribute("class"), "brand");
    assert.equal(node.getAttribute('href'), '/x');
    assert.equal(node.textContent, 'Hello');
  });

  it('builds a fragment with multiple children', function() {
    const node = h(Fragment, null, h('span', null, 'one'), h('span', null, 'two'));
    assert.ok(node instanceof DocumentFragment);
    assert.equal(node.childNodes.length, 2);
  });

  it('calls function components with props and children', function() {
    /** A function component used to verify the function-component path. */
    function Greeting(props: { name: string }): Node {
      return h("span", { className: "rw-page-header" }, "Hello ", props.name);
    }
    const node = h(Greeting, { name: 'world' });
    assert.ok(node instanceof HTMLSpanElement);
    assert.equal(node.textContent, 'Hello world');
  });

  it('escapes text children automatically', function() {
    const node = h('p', null, '<script>alert(1)</script>') as HTMLParagraphElement;
    assert.equal(node.querySelector('script'), null);
    assert.ok(node.textContent.includes('<script>'));
  });

  it('stringifies aria boolean attributes', function() {
    const node = h('button', { 'aria-expanded': true, 'aria-hidden': false }, 'x') as HTMLButtonElement;
    assert.equal(node.getAttribute('aria-expanded'), 'true');
    assert.equal(node.getAttribute('aria-hidden'), 'false');
  });

  it('handles HTML boolean attributes', function() {
    const node = h('button', { disabled: true, hidden: false }, 'x') as HTMLButtonElement;
    assert.equal(node.hasAttribute('disabled'), true);
    assert.equal(node.hasAttribute('hidden'), false);
  });

  it('passes string style values through', function() {
    const node = h('span', { style: 'width:50%' }, 'x') as HTMLSpanElement;
    assert.equal(node.getAttribute('style'), 'width:50%');
  });

  it("creates SVG elements in the SVG namespace", function() {
    const path = h("path", { d: "M0 0h16v16z" });
    const node = h("svg", { viewBox: "0 0 16 16" }, path);
    assert.equal(node.namespaceURI, "http://www.w3.org/2000/svg");
    assert.equal((node.firstChild as SVGElement | null)?.namespaceURI, "http://www.w3.org/2000/svg");
    assert.equal((node.firstChild as SVGPathElement).getAttribute("d"), "M0 0h16v16z");
  });
});

describe('jsx-runtime — render and renderToString', function() {
  it('renders a node into a host', function() {
    const host = document.createElement('div');
    render(h('p', null, 'content'), host);
    assert.equal(host.innerHTML, '<p>content</p>');
  });

  it('clears the host for a null node', function() {
    const host = document.createElement('div');
    host.innerHTML = '<p>old</p>';
    render(null, host);
    assert.equal(host.innerHTML, '');
  });

  it('serializes a single element', function() {
    assert.equal(renderToString(h('a', { href: '/x' }, 'Hi')), '<a href="/x">Hi</a>');
  });

  it('serializes a fragment by concatenating children', function() {
    const node = h(Fragment, null, h('span', null, 'a'), h('span', null, 'b'));
    assert.equal(renderToString(node), '<span>a</span><span>b</span>');
  });

  it('returns empty string for null', function() {
    assert.equal(renderToString(null), '');
  });
});

describe('jsx-runtime — raw', function() {
  it('builds a node from trusted markup', function() {
    const node = raw('<p class="x">body</p>');
    assert.ok(node instanceof DocumentFragment);
    assert.equal(renderToString(node), '<p class="x">body</p>');
  });
});
