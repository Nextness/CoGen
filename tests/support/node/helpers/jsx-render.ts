// Test-only helpers for serializing JSX node trees to HTML strings and
// building nodes from trusted markup. These are not part of the production
// JSX runtime; they exist solely to support unit-test assertions.

/** Serializes a rendered node tree to an HTML string. */
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
