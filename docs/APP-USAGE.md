# Application Usage

## 1. Purpose

The Research Corpus Viewer is a local, read-only interface for inspecting one existing research-corpus metadata database and its bound PDF inventory. This guide explains what a user can expect, how research context and evidence behave, and which limitations matter during interpretation. Developer commands and tests are in [PROJECT-USAGE.md](PROJECT-USAGE.md), the product contract is in [DESIGN.md](DESIGN.md), and technical behavior is in [ARCHITECTURE.md](ARCHITECTURE.md).

## 2. Start and access

From the repository root, start the viewer with embedded assets:

```sh
make serve DB=corpus.metadata.db ADDR=127.0.0.1:8080
```

Open `http://127.0.0.1:8080`. The default loopback address keeps access on the local machine. If the address is changed to a non-loopback interface, the server warns but does not add authentication or authorization.

The viewer requires an existing metadata database. It does not create files, run migrations, enable writable SQLite settings, modify records, restore runs, acquire PDFs, or refresh provider data.

## 3. Research context

The selectors establish a hierarchy of search, search revision, execution plan, and run. A child selector becomes available after its parent is known. When only one valid child exists, the application may select it automatically; changing a parent clears invalid descendant context.

The URL stores `search_id`, `search_revision_id`, `plan_id`, and `run_id`. Navigation, reload, browser back and forward, and internal links preserve that context. Filters, sorting, pagination, sections, expanded rows, graph state, and artifact inspection are also URL-backed where sharing or reload continuity matters.

Copied URLs may expose research identifiers and selected filters through browser history or messages. Treat them as research context rather than secret-bearing links, and do not share them where the identifiers are sensitive.

## 4. Primary navigation

The primary destinations are Overview, Corpus, Relationships, Provenance, Evaluation, Advanced, and Trash. On narrow screens, the Menu button opens the same navigation and reports its expanded state to assistive technology.

The header reports database health and always presents the viewer as Read-only. A Skip to content link, navigation landmarks, headings, labels, live status regions, and visible keyboard focus support keyboard and assistive-technology use.

## 5. Overview

Overview explains how the selected run moved from declared source exports through parse, deduplication, enrichment, validation, and normalization. Captured execution metrics are distinguished from values derived from the current database so later interpretation does not confuse historical run evidence with current coverage.

The page shows retention flow, source-result provenance, metrics, and breakdowns. Missing or unavailable values remain labeled rather than being silently converted to zero. Counts link to their relevant bounded collections while preserving selected research context.

## 6. Corpus

Corpus provides Articles, Authors, References, Sources, and Identity evidence. Tables support bounded pagination, allowlisted sorting, search, page-size controls, and row disclosures. Multiple rows may remain expanded, and explicit controls remain available when row click also toggles expansion.

Articles in the analysis-ready collection are valid normalized revisions for the selected run. Author and reference collections can include occurrence or mention records connected to multiple revision snapshots; repeated conceptual names are possible and should be interpreted with revision and producer-stage context.

Detail pages remain within Corpus navigation. Article detail combines the selected revision, run evidence, related authors and references, audit context, and PDF status. Author and reference details retain observed or mentioned values rather than presenting uncertain data as confirmed identity.

## 7. Identity evidence

An exact observed ORCID can support confirmed identity behavior. ORCID name-search results are uncertain candidates only. Candidate rank, provider data, and evidence context are inspectable, but a name-search match does not assign an ORCID to an occurrence or person.

Resolution statuses include matched, no match, unclear, and provider error. Treat unclear and provider-error outcomes as recorded evidence about an attempted resolution, not proof of identity or absence.

## 8. Relationships

Relationships supports research network, article-author, citation, and article-reference modes. Filters and selected graph state are stored in the URL. Shape encodes entity type and color encodes connected component; selection details and a paginated relationship table provide the textual equivalent of canvas content.

Graph responses are bounded. The server caps article nodes at 2,000, related nodes at 10,000, and edges at 20,000. When results are truncated, the page reports the condition and reason; the browser does not stream the remainder.

Canvas controls support fit, zoom, drag, selection, search, cluster overview, expansion, and PNG export. Keyboard and nonvisual users should rely on the controls, summaries, selection panel, and relationship table rather than canvas pixels.

## 9. Provenance

Provenance has Audit, Artifacts, Cache, Stages, and Run detail sections. The Audit stream supports category, action, actor, entity, correlation, and run context filters with bounded cursor pagination. Pipeline, enrichment, validation, and PDF categories retain their own semantic labels.

Artifacts list exact captured inputs, manifests, provider payloads, and pipeline outputs associated with the selected run. Preview requests retrieve only a bounded prefix: 64 KiB by default and no more than 256 KiB. Recognized text, JSON, and SOMETHING content can be previewed; the original bounded server download remains separate.

Cache evidence distinguishes hits, misses, negative results, stale entries, invalid payloads, and network fetches. Stage views show run-level progression and per-work outcomes so discarded or failed work remains inspectable.

## 10. Evaluation and PDFs

Evaluation lists normalized DOI-bearing work and overlays companion PDF inventory status. `not_available` means the DOI is registered but no validated selected bytes are present. `available` means validated PDF bytes and an inventory timestamp exist.

The viewer never downloads PDFs from external services and never changes inventory state. An available PDF can be delivered through the local API with download-safe headers. Manual inventory is performed outside the application using the supported tool described in [PROJECT-USAGE.md](PROJECT-USAGE.md).

Metadata and PDF databases are one portable bundle after inventory begins. If the companion binding or schema is unavailable, the viewer reports unavailable PDF evidence rather than mutating or repairing the store.

## 11. Advanced and Trash

Advanced exposes discovered non-system SQLite tables through a read-only browser. Table names and sort columns are validated against discovered schema, page sizes are limited to 20, 50, 100, 200, or 500, and raw values remain escaped and copyable.

Trash lists trashed-run and restore history as lifecycle evidence. The viewer does not provide trash, restore, purge, or other mutation controls.

## 12. Loading, errors, and unavailable data

Global navigation displays a loading state and cancels stale requests. A slower previous route cannot overwrite a newer route. Request failures, invalid JSON, unavailable context, and local progressive-action errors use explicit messages rather than blank content.

Empty means the selected scope has no matching records. Unavailable or Not recorded means required evidence is absent or cannot be derived. Truncated means the server deliberately bounded a larger result. These states are not interchangeable and should remain distinct during analysis.

Local actions such as loading older audit events, inspecting an artifact prefix, copying preview text, or graph interactions keep existing content visible when possible and report failure near the operation.

## 13. Privacy and security expectations

- Use loopback unless there is an explicit, reviewed reason to expose the viewer on another interface.
- Do not treat the application as authenticated, authorized, encrypted, multi-user, or safe for untrusted networks.
- Review databases and artifact contents before sharing screenshots, reports, downloads, URLs, or browser profiles.
- The frontend escapes dynamic content and the server bounds previews, but the viewer can still reveal the research data stored in the selected database.
- Production assets are embedded and make no CDN request; provider APIs are not contacted while serving the viewer.
- Credentials, tokens, private keys, and sensitive environment values must never be stored as artifacts intended for viewing.

## 14. Accessibility and responsive expectations

All actions have a keyboard path and visible focus. Disclosure controls expose `aria-expanded`, navigation exposes current state, status regions announce loading and failures, and color is not the sole status indicator. Canvas information remains available through DOM content.

Below 720px, navigation, selectors, summaries, toolbars, graph layout, and detail panels adapt to the viewport. Wide tables remain horizontally scrollable rather than dropping columns or evidence.

Light and dark themes follow system preference. Reduced-motion preference suppresses unnecessary smooth effects, while force-layout computation does not make essential state depend on animation.

## 15. Operational checks

The server health endpoint is `GET /api/health`. A healthy response confirms that the HTTP process can access the selected read-only metadata database; it does not prove that every optional artifact, PDF, or research-context record exists.

If the viewer cannot start, confirm that the metadata path exists, is a file, has the expected schema, and is readable. If context selectors are empty, confirm that the database contains searches, revisions, plans, and runs. If PDF status is unavailable, confirm the relative companion binding and database move together with metadata.

## 16. Current limitations

- The viewer has no application authentication or authorization.
- Author and reference collections can repeat conceptual values across revision snapshots.
- Graph and collection endpoints are bounded and do not stream beyond their limits.
- URLs preserve useful research context but can reveal its identifiers.
- String-template rendering relies on deliberate escaping for each new dynamic value.
- Visual snapshots and fixtures represent controlled data and tested browser environments, not every platform.
