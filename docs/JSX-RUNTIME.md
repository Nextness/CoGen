# JSX Runtime

This document is the authoritative contract for the project-owned JSX runtime at `frontend/src/jsx/jsx-runtime.ts`. It records the design decisions, semantics, escaping model, event binding, wiring, migration mapping, and known limitations so that information is not lost as the codebase changes. Update this document whenever the runtime's semantics, attribute or children handling, escaping, event binding, or wiring change.

## 1. Design decisions and rationale

The viewer is a framework-free native ES-module SPA compiled per file by esbuild into `frontend/dist`. Rendering was originally string-template based: views assigned `app.innerHTML`, reusable components returned HTML strings, and every dynamic value was escaped manually through `esc()`. This runtime replaces that model with actual JSX components while preserving the framework-free property and the existing "render then bind" lifecycle.

- **DOM-building runtime, not a virtual DOM.** `h` returns a real `Node`. A view renders a fresh tree and then binds listeners, matching the existing lifecycle. There is no reconciler, no diffing, and no framework.
- **No dependency.** The runtime is project-owned and tiny. It keeps the documented no-framework-dependency property and avoids the vendoring, build, test-loader, and documentation costs of adopting React.
- **Classic JSX mode.** `tsconfig.json` uses `jsx: "react"`, `jsxFactory: "h"`, and `jsxFragmentFactory: "Fragment"`. Classic mode is required because the automatic `react-jsx` transform emits an import specifier that cannot resolve in a per-file, no-bundler, no-import-map build (see section 7).
- **Automatic escaping.** Text children are inserted through `document.createTextNode` and attribute values through `setAttribute`, so raw HTML in data is inert. This replaces manual `esc()` in JSX-rendered content.
- **Controlled raw-HTML escape hatch.** The container helpers (`panel`, `table`, `pageHeader`, `emptyState`, `emptyPanel`, `retentionFlow`, `breakdown`, `sourceSearchQueries`, `detailTable`) accept HTML-string bodies. The `raw()` helper builds a `Node` from a trusted, already-escaped string. It is used only for these container bodies during the migration bridge and is removed once callers are converted to JSX children.
- **Event binding via `addEventListener`.** `on*` props bind listeners at element creation (before insertion). Listeners on replaced nodes are released with the node, so no explicit cleanup is required.

## 2. Runtime API semantics

The runtime exports `h`, `Fragment`, `render`, `renderToString`, and `raw`. The exports `h`, `render`, `renderToString`, and `raw` are declared as `export function` so the doccheck catalog records them; `Fragment` is a function constant and is documented here only.

- `h(type, props, ...children): Node` — the JSX factory. If `type === Fragment`, returns a `DocumentFragment` containing the children. If `type` is a function, calls it with `{ ...props, children }` and returns its result (function component). Otherwise creates `document.createElement(type)` and applies attributes and children.
- `Fragment(props?): DocumentFragment` — a callable function returning a fragment containing its children. It must be callable (not a `Symbol`) because classic-mode `<>...</>` compiles to `Fragment(...)` and `tsc` requires a call signature.
- `render(node, host): void` — replaces `host` children with the node via `host.replaceChildren(node)`. If `node` is `null`/`undefined`, clears the host.
- `renderToString(node): string` — serializes a rendered node to an HTML string. For a single element, `node.outerHTML`. For a `DocumentFragment` or an array, concatenates each child's serialization. For `null`/`undefined`/empty, returns `""`. Used as the migration bridge.
- `raw(html: string): Node` — builds a `Node` from a trusted, already-escaped HTML string via a detached container's `innerHTML`. It must only be fed already-escaped or trusted content, never raw user data.

## 3. Attribute handling rules

`h` applies attributes in the order they appear in the JSX props object.

- `className` → `setAttribute("class", value)`.
- `htmlFor` → `setAttribute("for", value)`.
- `style` object → `Object.assign(element.style, value)`. `style` string → `setAttribute("style", value)` (pass-through, preserves exact serialization).
- `on<Event>` with a function value → `addEventListener(eventName, handler)`. The event-name mapping strips `on` and lowercases, with a `dblclick` special case for `onDoubleClick`.
- `aria-*` attributes → always stringified: `true` → `"true"`, `false` → `"false"`, never omitted. ARIA attributes carry literal `"true"`/`"false"` values, unlike HTML boolean attributes.
- HTML boolean attributes (`disabled`, `hidden`, `checked`, `selected`, and the other members of the runtime's boolean set) → `true` → `setAttribute(name, "")`; `false` → omitted.
- `null`/`undefined` → attribute omitted.
- Any other value → `setAttribute(name, String(value))`.

## 4. Children handling rules

- `null`, `undefined`, `false`, `true` → skipped. Note: `0` is NOT skipped (renders the text `"0"`); use ternaries or `count > 0 && ...`.
- Arrays → each child appended recursively.
- `Node` → appended directly.
- Any other value → `document.createTextNode(String(value))` (automatic escaping).

## 5. Escaping model

Text children are escaped automatically through `document.createTextNode`, and attribute values through `setAttribute`. This replaces manual `esc()` in JSX-rendered content. `esc()` must be removed from JSX content at conversion time (both text children and attribute values) to avoid double-escaping. The `raw()` helper is the only path that inserts markup and must only ever be fed already-escaped or trusted content; no `esc()` removal may occur on any path that still serializes through `innerHTML`.

## 6. Event binding model

`on*` props bind listeners at element creation, before insertion, and are released with the node when `render` replaces the host children. `on*` handlers are incompatible with `renderToString` bridging because serialization drops listeners; keep event binding in the existing `bind*` functions during the migration bridge. `onClick` on internal `?`-links must call `preventDefault` to avoid double handling with the document-level navigation delegation in `app.tsx`.

## 7. Wiring and classic-mode rationale

`tsconfig.json` uses `jsx: "react"`, `jsxFactory: "h"`, and `jsxFragmentFactory: "Fragment"`. The automatic `react-jsx` transform emits `import { jsx } from "<jsxImportSource>/jsx-runtime"`. In a per-file, no-bundler, no-import-map build, that specifier is emitted verbatim: with the default `jsxImportSource` it is a bare specifier that cannot resolve, and with a relative `jsxImportSource` it is only correct for files at one directory depth, but this repo has JSX files at `src/`, `src/components/`, and `src/views/`. Classic mode emits `h(...)` with an explicit relative `import { h } from "./jsx-runtime.ts"` that flows through the existing `.ts`/`.tsx` → `.js` rewrite in `build.mjs`.

`build.mjs` requires no change: the existing rewrite `/\.tsx?(["'])/g` → `.js$1` handles the `.ts` runtime import, and the stale-specifier check covers `.ts`/`.tsx`. `frontend/scripts/tsx-loader.mjs` requires `jsx: 'transform'`, `jsxFactory: 'h'`, and `jsxFragment: 'Fragment'` in its `transformSync` call because the esbuild `transform` API does not read `tsconfig.json` (only the `build` API does). Every `.tsx` file that uses JSX imports `import { h, Fragment } from "../jsx/jsx-runtime.ts"` (path depends on directory depth).

## 8. Migration mapping

The migration converts string-returning helpers and components to JSX components. The `renderToString` bridge keeps string-asserting tests green approximately; serialization diverges for boolean attributes, style normalization, and attribute escaping, so affected assertions are updated by review, not treated as byte-parity. Container helpers that accept HTML-string bodies use the `raw` escape hatch during the bridge, and markup-returning helpers (`statusChip`, `cell`, render callbacks) become Node-returning components. `esc()` is removed from each helper at conversion time. The `render` runtime export is aliased in views that also use the router `render` (for example `import { render as renderTree } from "../jsx/jsx-runtime.ts"`).

## 9. Known limitations

- Attribute types are permissive (`[attr: string]: unknown`); attribute names and event-handler shapes are not validated. Tightening is an optional follow-up.
- There is no VDOM reconciliation and no automatic re-render; a view re-renders by building a fresh tree and calling `render`.
- SVG and namespaced elements are not supported (no current usage); SVG would require `createElementNS`.
- The `0` child is not skipped; use ternaries or `count > 0 && ...`.
- `renderToString` serialization diverges from hand-written templates for boolean attributes (`disabled=""` vs ` disabled`), style normalization (`style="width: 50%;"` vs `width:50%`), and attribute escaping.
- `createElement`/`createTextNode` is generally somewhat slower than `innerHTML` parsing for trees with many small nodes, but the difference is negligible for this viewer's bounded collections. The `renderToString` bridge adds a temporary DOM-to-string-to-DOM round trip on the largest collections. `createTextNode` is more efficient for large text payloads.

## 10. How to add a new component

Create a `.tsx` file under `frontend/src/components/` or `frontend/src/views/` that imports `h` and `Fragment` from the runtime and exports a function returning `JSX.Element`. Use JSX for markup, rely on automatic escaping for text and attributes, and bind behavior either through `on*` props or through a `bind*` function called after `render`. Add a unit test under `frontend/tests/unit/` that imports the component and asserts on the rendered DOM or `renderToString` output. Run `make check-frontend`, `make frontend-build`, and `make test-frontend-unit` after the change.
