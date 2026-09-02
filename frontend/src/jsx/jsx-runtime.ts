// Project-owned JSX runtime that builds real DOM nodes.
//
// This module is the authoritative implementation of the custom JSX runtime
// described in docs/JSX-RUNTIME.md. It is framework-free: it provides the
// classic-mode JSX factory (h), the fragment factory (Fragment), a render
// function that mounts a node tree into a host, and typed class composition
// and DOM class helpers.
//
// The renderToString serialization bridge and the raw() escape hatch are
// test-only helpers and live in tests/unit/helpers/jsx-render.ts, not here.
//
// Escaping is automatic: text children are inserted through
// document.createTextNode and attribute values through setAttribute, so raw
// HTML in data is inert.

import type { ClassName } from "./classes.ts";

/** Prevents ordinary strings from satisfying the ClassNames type. */
declare const classNamesBrand: unique symbol;

/** A space-separated class string assembled exclusively from registered tokens. */
export type ClassNames = string & { readonly [classNamesBrand]: true };

/** A function component accepted by the project-owned JSX factory. */
export type ComponentType<P = any> = (props: P) => JSX.Element | null;

/** An intrinsic tag, function component, or Fragment accepted by the JSX factory. */
export type ElementType = ComponentType<any> | keyof JSX.IntrinsicElements | typeof Fragment;

/** Extracts the props accepted by one function component type. */
export type ComponentProps<T> = T extends ComponentType<infer P> ? P : never;

/** Joins registered class tokens while omitting false, null, and undefined values. */
export function cx(...tokens: Array<ClassName | false | null | undefined>): ClassNames {
  const presentTokens = tokens.filter(Boolean);
  return presentTokens.join(" ") as ClassNames;
}

/** Adds registered class tokens to an element. */
export function classAdd(element: Element, tokens: readonly ClassName[]): void {
  element.classList.add(...tokens);
}

/** Removes one registered class token from an element. */
export function classRemove(element: Element, token: ClassName): void {
  element.classList.remove(token);
}

/** Toggles one registered class token on an element. */
export function classToggle(element: Element, token: ClassName, force?: boolean): boolean {
  return element.classList.toggle(token, force);
}

/** Returns whether an element has one registered class token. */
export function classHas(element: Element, token: ClassName): boolean {
  return element.classList.contains(token);
}

/** Returns a document fragment containing the supplied children. */
export const Fragment = function(props?: { children?: JSX.Node }): DocumentFragment {
  const fragment = document.createDocumentFragment();
  appendChildren(fragment, props?.children);
  return fragment;
};

/** Appends one child value to a parent, recursing through arrays. */
function appendChildren(parent: Node, children: JSX.Node): void {
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

/** SVG tag names that do not overlap HTML tag names and therefore have an unambiguous namespace. */
const svgElementNames = new Set<string>([
  "animate", "animateMotion", "animateTransform", "circle", "clipPath", "defs",
  "desc", "ellipse", "feBlend", "feColorMatrix", "feComponentTransfer",
  "feComposite", "feConvolveMatrix", "feDiffuseLighting", "feDisplacementMap",
  "feDistantLight", "feDropShadow", "feFlood", "feFuncA", "feFuncB", "feFuncG",
  "feFuncR", "feGaussianBlur", "feImage", "feMerge", "feMergeNode",
  "feMorphology", "feOffset", "fePointLight", "feSpecularLighting", "feSpotLight",
  "feTile", "feTurbulence", "filter", "foreignObject", "g", "image", "line",
  "linearGradient", "marker", "mask", "metadata", "mpath", "path", "pattern",
  "polygon", "polyline", "radialGradient", "rect", "set", "stop", "svg", "switch",
  "symbol", "text", "textPath", "tspan", "use", "view",
]);

/** Namespace used for unambiguous SVG intrinsic elements. */
const svgNamespace = "http://www.w3.org/2000/svg";

/** Applies one JSX attribute to a created element. */
function setAttribute(element: Element, name: string, value: unknown): void {
  if (value == null) {
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
    const eventName = name.slice(2).toLowerCase();
    element.addEventListener(eventName === "doubleclick" ? "dblclick" : eventName, value as EventListener);
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
export function h(type: typeof Fragment, props: { children?: JSX.Node } | null, ...children: JSX.Node[]): DocumentFragment;
/** Creates a DOM node from a JSX type, props, and children. */
export function h<P>(type: ComponentType<P>, props: P | null, ...children: JSX.Node[]): Node;
/** Creates a DOM node from a JSX type, props, and children. */
export function h<K extends keyof JSX.IntrinsicElements>(type: K, props: JSX.IntrinsicElements[K] | null, ...children: JSX.Node[]): Element;
/** Creates a DOM node from a JSX type, props, and children. */
export function h(type: any, props: Record<string, unknown> | null, ...children: JSX.Node[]): Node {
  if (type === Fragment) {
    return Fragment({ children: children });
  }
  if (typeof type === "function") {
    // TypeScript checks component props at the overload boundary. The internal
    // cast preserves the runtime's permissive null return used by child append.
    return type({ ...(props || {}), children: children.length === 1 ? children[0] : children }) as Node;
  }
  const element = svgElementNames.has(type as string)
    ? document.createElementNS(svgNamespace, type as string)
    : document.createElement(type as string);
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
