# Research Workspace Frontend Design

## 1. Status and scope

This document is the implemented design reference for the local research workspace viewer. It defines current information architecture, interactions, visual language, responsive behavior, accessibility expectations, content rules, and acceptance criteria. [ARCHITECTURE.md](ARCHITECTURE.md) owns implementation structure, [CSS-REFERENCE.md](CSS-REFERENCE.md) owns active stylesheet details, [APP-USAGE.md](APP-USAGE.md) explains the user experience, [PROJECT-USAGE.md](PROJECT-USAGE.md) explains developer and operator workflows, and [STANDARDS.md](STANDARDS.md) defines frontend change rules.

The viewer is an inspection interface for historical research runs. It does not execute the pipeline, modify corpus records, restore or purge runs, add PDFs, or resolve identity candidates. Every screen must make that read-only boundary obvious.

## 2. Product goals

- Preserve research context across every navigation action so users always know which search, revision, execution plan, and run attempt they are inspecting.
- Separate evidence captured during execution from values derived from the database at view time.
- Make provenance, validation, cache use, identity uncertainty, and PDF inventory status inspectable without requiring direct SQL.
- Keep large corpora, audit streams, artifacts, and relationship graphs bounded and navigable.
- Use semantic HTML, visible focus, text alternatives, and table equivalents so essential information is not encoded only through color or graphics.
- Work from embedded local assets with no CDN or application-framework dependency.
- Preserve privacy by default through loopback serving, read-only database access, bounded previews, and explicit warnings about external exposure.

## 3. Design principles

### 3.1 Historical truth before dashboard polish

The interface must state whether a number was recorded by the pipeline or derived from current stored rows. Missing recorded evidence is displayed as not recorded, never silently converted to zero. Failed and discarded outcomes remain visible because they are part of the research record.

### 3.2 Context is persistent state

The canonical research context is `search_id`, `search_revision_id`, `plan_id`, and `run_id` in the URL. Navigation, filters, pagination, sorting, selected graph nodes, and provenance sections also use URL state when that makes a view reloadable or shareable. Every internal link uses the shared `link()` helper.

### 3.3 Bounded detail with progressive disclosure

Tables paginate, audit events load older cursor pages, graph endpoints report truncation, long cells expand on demand, advanced filters use disclosures, and artifact inspection fetches a bounded safe prefix. A user can move from summary to exact evidence without loading unbounded data into the browser.

### 3.4 Uncertainty remains explicit

Observed author names are occurrences, not automatically people. ORCID name-search results are candidates, not confirmed identifiers. Cache reuse, stale data, skipped stages, validation rejection, unavailable metrics, and missing PDFs use distinct labels and explanatory copy.

### 3.5 Visualizations require textual equivalents

The retention flow is supported by counts and tables. Relationship graphs include filters, cluster summaries, legends, selection details, and a paginated relationship table. Color is supplementary to labels, shape, text, and status wording.

## 4. Information architecture

The primary navigation order is Overview, Corpus, Relationships, Provenance, Evaluation, Advanced, and Trash. Article, author, and reference detail routes are contextual descendants of Corpus and keep Corpus marked as the current primary destination.

| View | Primary question | Required context |
|---|---|---|
| Overview | What happened in this run, and what does its current stored corpus contain? | Run required. |
| Corpus | Which normalized articles, observed authors, references, identity evidence, and source records belong to this run? | Run preferred; limited schema-backed fallback exists where supported. |
| Relationships | How are valid normalized articles connected to authors, citations, and references? | Run required. |
| Provenance | What audit events, artifacts, cache decisions, stages, and attempt metadata explain this run? | Audit may span runs; other sections require a run. |
| Evaluation | Which normalized articles have a manually inventoried PDF? | Run required. |
| Advanced | What values exist in each discovered metadata table? | Database required; run is not required. |
| Trash | Which runs are trashed, and what restoration evidence exists? | Database required; run is not required. |

## 5. Application shell

The shell begins with a skip link, then a site header containing the product identity, database health state, and a persistent Read-only marker. A mobile Menu button controls the primary navigation below 720px. The primary navigation follows the fixed order in section 4.

The Research context panel contains dependent Search, Search revision, Execution plan, and Run attempt selectors plus Clear context. Selectors initially show skeleton/loading or instructional states, disable unavailable descendants, select a sole available option automatically, and clear downstream identifiers when a parent changes.

The main region contains an alert notice, a polite live loading indicator, and the view container. View titles update `document.title` to `<page title> · Research workspace`. Global health, loading, and error states must remain understandable without inspecting developer tools.

## 6. URL and navigation behavior

The default view is Overview when `view` is absent. Primary view links retain the full research context. Changing search clears revision, plan, and run; changing revision clears plan and run; changing plan clears run; changing run preserves its ancestors.

The SPA intercepts same-page query links without modifier keys, pushes browser history, and renders without a full reload. Back and forward navigation re-renders from the URL. External links, downloads, modified clicks, and non-query links retain normal browser behavior.

Each render aborts the previous request controller and receives a monotonically increasing sequence number. Only the newest sequence may display errors, change the title, or clear the loading state. Aborted requests are silent.

View-specific URL keys must be namespaced when multiple tables coexist. Provenance uses keys such as `cache_page`, `stage_page`, and `audit_category`; graph state uses `mode`, filters, `article_limit`, and `node`; Corpus and Evaluation use `section`, `q`, `page`, `per_page`, `sort`, `order`, and expansion state as applicable.

## 7. Overview

Overview begins with the selected run identity and status, followed by the source-to-corpus retention flow. Source declarations show configured result counts, progressive filter counts, observed export counts, comparisons, export dates, and the exact search queries where recorded.

Captured metrics are grouped by input, parsing, deduplication, enrichment, validation, normalization, cache/network, and other evidence. Current coverage is presented separately because it is derived from stored rows rather than guaranteed to have been captured at attempt completion.

Corpus summary cards link to the relevant Corpus or Relationships destination without losing context. Breakdown panels cover enrichment, validation, normalization, source distribution, and cache activity. Normalization outcomes distinguish changed, already canonical, and unavailable fields against the assessed denominator.

Retention stages that correspond to inspectable records may navigate to a relevant Corpus section. Source filter stages are informational and are not falsely presented as clickable application stages.

## 8. Corpus

Corpus subnavigation contains Articles, Authors, References, Author identity / ORCID evidence, and Source records. The section description must identify the underlying entity semantics, especially that author occurrences do not imply global person identity and name-search candidates do not imply confirmed ORCID assignment.

Articles are valid normalized revisions for the selected run and present title, year, journal, source, DOI, and identifiers with expandable metadata. A message explains that discarded works remain in stage and provenance evidence rather than this analysis-ready list.

Authors are observed occurrence records linked through run revisions. The table exposes citation name, parsed names, observed ORCID, person link, article count, affiliation count, and captured time. It must not label same-name rows as one person unless repository identity evidence confirms that relationship.

References are ordered mentions attached to immutable revisions. Rows expose citing context, DOI, title, author string, year, source, and resolved-work identity when present. Article and reference links preserve the selected run so detail resolution remains historical.

Identity evidence summarizes resolutions, unclear matches, provider failures, and candidate counts. Candidate rows show rank, ORCID, display name, and links to preserved evidence without presenting a candidate as accepted identity.

Source records expose source name/type, record index, parse outcome, reject reason, content hash, and capture time. When source-level result counts are available, they appear above the table to connect configured retrieval and parsed evidence.

Corpus search is server-backed for scoped endpoints. Page size choices are 20, 50, 100, 200, and 500. Unsupported sort fields or page sizes must be corrected to safe defaults in the UI and rejected by the server if submitted directly.

## 9. Detail views

Article detail provides a return path to Corpus, a summary strip, property grid, authorships, reference mentions, stage outcomes, enrichment and validation evidence, raw record disclosure, and PDF status. The displayed revision remains anchored to the selected run context; audit may aggregate the selected work's related revisions inside that run.

The PDF panel distinguishes Available from Not Available, shows inventory timing when present, and exposes a PDF link only when the read-only API can join the available inventory row to stored content. The frontend never creates or changes inventory state.

Author detail describes one author occurrence and its linked articles, affiliations, person identity, and audit context. Reference detail describes one mention, its citing revision, captured fields, and resolved target when the selected run contains a suitable revision.

Related collections paginate in memory because detail APIs return bounded relationship sets. Breadcrumbs and return links preserve the view, section, run, and relevant entity identifier.

## 10. Relationships

Relationships defaults to Research network in the user interface. Available models are Research network, Articles and authors, Internal citations, and Articles and references. Changing model preserves article filters and clears stale selected-node state.

Common filters are title/DOI, year range, and source. Advanced filters are author, ORCID, reference, citation range, reference-count range, and article limit. The limit defaults to 2,000, must be between 1 and 2,000, and is omitted from the URL when it remains at the default.

The server response is authoritative about matched articles, rendered nodes/edges, entity counts, limits, truncation, and truncation reason. The view must warn when the selected article set or related entities were truncated rather than implying a complete network.

The graph canvas supports deterministic initial layout, fit, zoom, wheel zoom around the pointer, pan, node drag, keyboard-accessible controls, node search, cluster overview selection, expansion mode, and PNG export. Destroying or leaving the view stops the simulation, observers, animation frames, and event state.

Entity type is encoded by shape and label: articles, observed authors, references, and raw referenced-author strings remain distinguishable without color. Connected component is encoded by one of six token colors, repeated when necessary. Edge style and the relationship legend distinguish authorship, citation, reference, coauthor, and shared-reference relationships.

Selection opens a details panel with entity metadata and neighboring relationships. The relationship table is the exact textual equivalent of the visualization and paginates independently. Graph search and table navigation must remain useful when canvas content is dense or inaccessible.

## 11. Provenance

Provenance subnavigation contains Audit timeline, Artifacts, Cache uses, Stage outcomes, and Run details. The page description consistently states that evidence is append-only and inspection-only.

Audit provides server-backed text search plus multi-select category, action, actor, and entity filters and optional stage/outcome fields. Active filters appear as removable chips in URL state. The newest 25 events load first, and Load 25 older events follows the cursor without replacing already visible evidence.

Each audit row identifies category, action, actor/source, entity, time, run context, outcome, and expandable metadata or before/after values. PDF events use the same event language but may be global to a work rather than owned by one pipeline attempt.

Artifacts show run context, role, producing/consuming steps, media type, byte size, content hash, time, preview availability, and download. Inspection requests 65,536 bytes, identifies truncation, formats complete JSON when possible, supports raw/formatted modes, line wrapping, clipboard copy, and original download. Binary or unsafe media remain download-only.

Cache uses are a scoped searchable table with independent page/sort/query keys. Rows explain provider, request identity, cache layer, outcome, expiry, payload artifact, and recorded time rather than reducing reuse to a single hit count.

Stage outcomes begin with an ordered run-level progression that reconciles run steps and per-work summaries. Each stage shows status, outcome counts, work-record count, duration, and linked artifacts. A separately paginated table retains individual work outcomes and reasons.

Run details show attempt identity, status, visibility, timestamps, duration, plan fingerprints, enrichment policy, summary, and exact configuration/manifest snapshot downloads. Legacy attempts may explicitly lack snapshot payloads.

## 12. Evaluation

Evaluation is a reading inventory, not a quality score. It lists every normalize-stage article in the selected run with title, DOI, source, inventory status, and inventory time.

Search is server-backed over title and DOI. Sorting is limited to title and DOI, and table controls use the shared page-size and pagination behavior. Article titles link to the selected revision detail while retaining research context.

Available means a PDF was manually validated and inserted into the bound companion store. Not Available means no selected PDF bytes exist for the registered DOI. The view does not imply that a missing PDF was searched for automatically, and it provides no add or edit action.

## 13. Advanced and Trash

Advanced discovers current metadata tables from the server and lets the user select one, inspect bounded rows, sort by an actual discovered column, change page size, and expand a row to see additional fields. It is implementation-level transparency, not a general SQL console, and it never accepts arbitrary table, column, or SQL expressions.

Trash lists trashed run attempts and the audit history of restorations. Restore and purge controls are intentionally absent because the viewer is read-only. Status copy must not suggest that inspection changes retention state.

## 14. Tables and data presentation

Shared tables render a visible subset of columns, optional expandable property grids, sortable headers, bounded page controls, result ranges, search controls, and long-cell clipping. The disclosure column is unlabeled but has an accessible control name and `aria-expanded` state.

Row click may toggle expansion only when it does not steal behavior from links, buttons, inputs, labels, selections, or text interaction. Multiple rows may remain expanded. Expansion state may be URL-backed when reload continuity is useful.

Empty states state what is absent and, when relevant, how to select context. Error states use the global alert or a local message near the failed operation. Loading buttons disable themselves and display a loading class until the operation settles.

IDs, hashes, URLs, raw JSON, and machine values use monospace or code treatment and remain copyable. Times use the browser locale for display while stored raw values remain available in detail or raw views. Numeric values use locale grouping; unavailable values use descriptive text or a neutral symbol with context.

## 15. Visual language

The interface uses a restrained research-tool aesthetic: cool neutral surfaces, dark slate text, blue interaction accents, semantic green/orange/red states, compact bordered panels, modest elevation, and dense but readable tables. Decoration must not compete with evidence.

`tokens.css` defines the light and dark palettes, six graph cluster colors, orange focus ring, 4px spacing scale, system font stack, typography scale, radii, and shadows. Dark mode follows `prefers-color-scheme` and removes decorative shadows while maintaining semantic contrast.

The cascade order is tokens, base, elements, collections, views, and graph. Reusable application patterns use the `rw-` prefix. Existing `ui` classes are local Semantic-UI-inspired primitives, not proof that a runtime Semantic UI dependency exists.

Status treatments combine text and semantic color. Success covers completed, valid, available, matched, and cache hit states; warning covers skipped, stale, negative, unclear, unresolved, and not available states where appropriate; danger covers failed, discarded, invalid, rejected, trashed, and purge states; neutral/information covers other recorded states.

## 16. Responsive behavior

Desktop is the primary dense-analysis layout, with multi-column summaries, side-by-side graph filters/network, and wide tables in horizontal overflow containers. Tablet collapses selected grids without removing data or controls.

Below 720px, primary navigation is controlled by the Menu button, the context selectors stack, summary grids collapse, toolbars wrap, graph and detail panels use the available width, and tables remain horizontally scrollable. The mobile control must update `aria-expanded` and close after navigation.

Responsive changes must preserve context labels, status meaning, pagination, disclosures, and action targets. Hiding nonessential decoration is acceptable; hiding evidence or replacing controls with pointer-only interactions is not.

## 17. Accessibility

The document includes a Skip to content link, landmark header/navigation/main regions, logical headings, explicit form labels, status live regions, and visible keyboard focus. Primary navigation uses `aria-current`; disclosure controls use `aria-expanded`; context loading and errors are announced without forcing focus.

All actions must be operable by keyboard and have discernible text or an accessible name. Interactive rows cannot be the only way to expand a record. Focus order follows visual order, and disabled selectors or buttons communicate why through nearby text.

Canvas is supplemental. Graph filters, summaries, legend, selection panel, and relationship table expose equivalent information. Screenshots or decorative images require appropriate empty alt text; meaningful images require descriptive alt text.

Light and dark themes must retain readable contrast for text, borders, focus, status chips, graph shapes, and selected rows. Color cannot be the sole indicator of entity type, outcome, selection, or truncation.

Reduced-motion preferences must avoid unnecessary smooth transitions or animated effects. The force layout may compute positions, but interaction feedback and essential state changes cannot depend on motion.

## 18. Loading, errors, and resilience

Global view fetches show the shared loading region and clear prior errors. The API helper requires JSON, extracts the standard error message when present, and reports invalid JSON or request failures in plain language. Aborted stale renders never overwrite a newer view.

Local progressive actions such as loading older audit events, inspecting artifacts, copying preview text, or running graph controls keep existing content visible and report failures near the action. A failed preview always leaves original download available when the server permits it.

Missing optional columns or metrics must degrade to Not recorded, unavailable panels, or omitted optional controls. A missing required run or table produces an explicit empty state, not a blank application container.

## 19. Privacy and security presentation

The header always displays Read-only. Copy explains that data is historical and local. The UI must not claim authentication, authorization, encryption, automatic PDF acquisition, live provider refresh, or mutation capabilities that the server does not provide.

Artifact and PDF downloads are direct local API links. Preview content is escaped before insertion, JSON is formatted only after parsing, dynamic labels and table cells use the shared escaping helper, and no provider payload is interpreted as HTML.

The interface must not render credentials, tokens, private keys, raw environment values, or newly exposed sensitive fields. New table/detail surfaces require an explicit review of whether values are safe for a local viewer and whether preview/download behavior remains bounded.

## 20. Frontend module ownership

| Path | Design responsibility |
|---|---|
| `index.html` | Accessible shell, navigation, context selectors, status, loading, notice, and app mount point. |
| `app.js` | Global event binding, history interception, shell initialization, and first render. |
| `state.js` | URL values, DOM state, escaping, formatting, shared panels/tables/flows, links, statuses, and global UI behavior. |
| `api.js` | Abort-aware JSON fetching, endpoint construction, shared error extraction, and table discovery cache. |
| `router.js` | Request sequencing, abort lifecycle, selector hydration, view dispatch, primary-nav state, and document title. |
| `components/context-selector.js` | Dependent selector hydration, loading skeletons, clear controls, auto-selection, and selection summary. |
| `components/data-table.js` | Shared rows, sorting, search, page size, expansion, and control binding. |
| `components/pagination.js` | First/Previous/numbered/Next/Last controls and result ranges. |
| `components/audit-events.js` | Audit classification, summaries, metadata disclosures, timeline markup, and optional investigation export. |
| `components/graph.js` | Graph query, connected components, canvas lifecycle, simulation, drawing, interactions, export, selection, and relationship table. |
| `components/shell.js` | Health state and responsive primary-navigation toggle. |
| `views/*.js` | Page-level fetching, rendering, and post-render event binding for the destinations in section 4. |
| `styles/tokens.css` | Theme values, spacing, type, status, graph colors, focus, radius, and elevation. |
| `styles/base.css` | Reset, document layout, typography, links, landmarks, and reduced motion. |
| `styles/elements.css` | Buttons, labels, messages, loaders, headers, and segment primitives. |
| `styles/collections.css` | Navigation, selectors, grids, forms, tables, pagination, breadcrumbs, and responsive collections. |
| `styles/views.css` | Overview, details, provenance, audit, artifacts, evaluation, and stage-specific presentation. |
| `styles/graph.css` | Relationship layout, controls, canvas, overview, legend, selection, and edge table. |
| `vendor/d3-force.js` | Generated pinned force-simulation implementation; never edit it manually. |

Components may import shared state, API, router helpers, pagination, and the pinned D3 module as required, but they must not import view modules. Views own `app.innerHTML`; reusable components return markup or bind behavior to caller-owned DOM.

## 21. Testing and acceptance

Frontend unit tests use `node:test`, `node:assert`, and jsdom. The suite verifies URL state, API behavior, routing, selectors, tables, pagination, graph transformation and interactions, shell behavior, shared render helpers, and every view module; counts are derived from source rather than maintained here.

The main Playwright suite runs against an isolated fixture viewer and verifies context selection, navigation, URL preservation, table controls, details, graph behavior, provenance, evaluation, error states, responsive layouts, dark/light preferences, landmarks, and interaction semantics. The UI-quality suite adds axe-core checks and reviewed screenshots for core views.

Every frontend change must run `make test-frontend-unit`. Changes under `src/server/frontend/` must also run `make test-go PACKAGE=./server` and `make test-frontend TEST_FILE=tests/viewer.spec.cjs`. Visual or accessibility changes must run `make test-frontend-visual` and review rather than blindly replace snapshots.

Acceptance requires no hard-coded context-dropping internal links, no unbounded new collection, no hidden mutation, no new external asset request, no inaccessible graph-only fact, no raw unescaped provider content, no unexplained unavailable state, and no paragraph or list item split across physical Markdown lines in this document.

## 22. Known design limitations

- The viewer has no application authentication or authorization and is intended for loopback use; a warning is the only built-in protection against deliberate non-loopback binding.
- Corpus author and reference lists include relationships from all revision snapshots in a run, so repeated conceptual values are possible and must remain labeled as occurrences or mentions.
- Graph limits can truncate large networks; the endpoint reports this, but the browser does not stream beyond the configured bounds.
- The frontend is string-template based, so every new dynamic value must deliberately use `esc`; there is no framework-level automatic escaping.
- Router and context-selector currently form one ES-module cycle to connect selector navigation with render-time hydration; it is initialized safely, but new modules should not add dependencies to that cycle.
- The URL contains research context and filters, which is useful for reproducibility but can reveal search/run identifiers through copied links or browser history.
- The local fixture and screenshot baselines represent controlled test data and Chromium/Linux rendering; they do not prove visual identity across every platform font and browser.
- Active CSS aliases remain part of current source behavior and can change only with a verified, scoped update to styles, markup, JavaScript, tests, and [CSS-REFERENCE.md](CSS-REFERENCE.md).

## 23. Change checklist

- Confirm the proposed behavior exists in or is supported by the read-only API before designing a control for it.
- Preserve search, revision, plan, and run context through `link()` and clear only invalid descendant state.
- Keep server-backed collections bounded, sortable only by allowlisted fields, and explicit about truncation or unavailable evidence.
- Escape every dynamic value and keep artifact/PDF handling download-safe and preview-bounded.
- Provide keyboard behavior, visible focus, labels, status text, and a non-visual equivalent for graphical information.
- Place reusable logic in the established state/component boundary and avoid view-to-view imports or new framework dependencies.
- Add or update unit, server integration, Playwright, accessibility, and visual tests in proportion to the changed behavior.
- Review light, dark, desktop, tablet, and mobile behavior and retain the permanent Read-only presentation.
- Update this document and [ARCHITECTURE.md](ARCHITECTURE.md) when the implemented contract or module responsibilities change.
