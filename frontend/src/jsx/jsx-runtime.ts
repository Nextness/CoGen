// Project-owned JSX runtime that builds real DOM nodes.
//
// This module is the authoritative implementation of the custom JSX runtime
// described in docs/JSX-RUNTIME.md. It is framework-free: it provides the
// classic-mode JSX factory (h), the fragment factory (Fragment), a render
// function that mounts a node tree into a host, a renderToString bridge used
// during migration, and a controlled raw-HTML escape hatch (raw) for trusted,
// already-escaped markup.
//
// Escaping is automatic: text children are inserted through
// document.createTextNode and attribute values through setAttribute, so raw
// HTML in data is inert. The raw() helper is the only path that inserts markup
// and must only ever be fed already-escaped or trusted content.

/** Returns a document fragment containing the supplied children. */
export const Fragment = function(props?: { children?: unknown }): DocumentFragment {
  const fragment = document.createDocumentFragment();
  appendChildren(fragment, props?.children);
  return fragment;
};

/** Appends one child value to a parent, recursing through arrays. */
function appendChildren(parent: Node, children: unknown): void {
  if (children == null || children === false || children === true) {
    return;
  }
  if (Array.isArray(children)) {
    for (const child of children) {
      appendChildren(parent, child);
    }
    return;
  }
  if (children instanceof Node) {
    parent.appendChild(children);
    return;
  }
  parent.appendChild(document.createTextNode(String(children)));
}

/** HTML boolean attributes that serialize as a bare attribute when true. */
const htmlBooleanAttributes = new Set([
  "async", "autofocus", "autoplay", "checked", "controls", "default", "defer",
  "disabled", "formnovalidate", "hidden", "itemscope", "loop", "multiple",
  "muted", "novalidate", "open", "playsinline", "readonly", "required",
  "selected", "truespeed", "typemustmatch",
]);

/** Applies one JSX attribute to a created element. */
function setAttribute(element: Element, name: string, value: unknown): void {
  if (value == null || value === undefined) {
    return;
  }
  if (name === "className") {
    element.setAttribute("class", String(value));
    return;
  }
  if (name === "htmlFor") {
    element.setAttribute("for", String(value));
    return;
  }
  if (name === "style") {
    if (typeof value === "object") {
      Object.assign((element as HTMLElement).style, value as Record<string, string>);
    } else {
      element.setAttribute("style", String(value));
    }
    return;
  }
  if (name.startsWith("on") && typeof value === "function") {
    const eventName = name.slice(2).toLowerCase() === "doubleclick" ? "dblclick" : name.slice(2).toLowerCase();
    element.addEventListener(eventName, value as EventListener);
    return;
  }
  if (name.startsWith("aria-")) {
    element.setAttribute(name, value === true ? "true" : value === false ? "false" : String(value));
    return;
  }
  if (htmlBooleanAttributes.has(name)) {
    if (value === true) {
      element.setAttribute(name, "");
    }
    return;
  }
  if (value === true) {
    element.setAttribute(name, "");
    return;
  }
  element.setAttribute(name, String(value));
}

/** Creates a DOM node from a JSX type, props, and children. */
export function h(type: any, props: Record<string, unknown> | null, ...children: any[]): Node {
  if (type === Fragment) {
    const fragment = document.createDocumentFragment();
    for (const child of children) {
      appendChildren(fragment, child);
    }
    return fragment;
  }
  if (typeof type === "function") {
    return type({ ...(props || {}), children: children.length === 1 ? children[0] : children });
  }
  const element = document.createElement(type as string);
  if (props) {
    for (const [name, value] of Object.entries(props)) {
      setAttribute(element, name, value);
    }
  }
  for (const child of children) {
    appendChildren(element, child);
  }
  return element;
}

/** Replaces the children of a host element with a rendered node tree. */
export function render(node: Node | null | undefined, host: HTMLElement): void {
  if (node == null) {
    host.replaceChildren();
    return;
  }
  host.replaceChildren(node);
}

/** Serializes a rendered node tree to an HTML string for the migration bridge. */
export function renderToString(node: unknown): string {
  if (node == null) {
    return "";
  }
  if (Array.isArray(node)) {
    return node.map(renderToString).join("");
  }
  if (node instanceof DocumentFragment) {
    return Array.from(node.childNodes).map(renderToString).join("");
  }
  if (node instanceof Node) {
    return (node as Element).outerHTML ?? "";
  }
  return String(node);
}

/** Builds a node from a trusted, already-escaped HTML string. */
export function raw(html: string): Node {
  const container = document.createElement("div");
  container.innerHTML = html;
  const fragment = document.createDocumentFragment();
  while (container.firstChild) {
    fragment.appendChild(container.firstChild);
  }
  return fragment;
}
