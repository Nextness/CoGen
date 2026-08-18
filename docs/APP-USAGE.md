# Application Usage

## 1. Purpose

The Research Corpus Viewer is a loopback-only local interface for inspecting one existing research-corpus metadata database, recording run-scoped immutable review history, and reading its bound PDF inventory without writing the PDF database. This guide explains what a user can expect, how research context and review evidence behave, and which limitations matter during interpretation. Developer commands, migration, and export workflows are in [PROJECT-USAGE.md](PROJECT-USAGE.md), the product contract is in [DESIGN.md](DESIGN.md), and technical behavior is in [ARCHITECTURE.md](ARCHITECTURE.md).

## 2. Start and access

From the repository root, migrate the database as needed, build the frontend assets, and start the viewer:

```sh
make migrate DB=corpus.metadata.db
make frontend-build
make serve DB=corpus.metadata.db ADDR=127.0.0.1:8080 ASSETS_DIR=frontend/dist
```

Open `http://127.0.0.1:8080`. The server requires an exact loopback IP address and rejects a different Host authority. It has no authentication or authorization, so local users and processes with host access remain inside the trust boundary.

The viewer requires an existing fully migrated metadata database. It does not create databases, run migrations, modify pipeline corpus evidence, physically delete or purge runs, acquire PDFs, change PDF inventory, or refresh provider data. It opens a separate existing-only metadata connection for bounded review and reversible run-visibility mutations while immutable evidence and the companion PDF database remain protected.

## 3. Research context

Home does not require a selected run. Choose Explore on a run attempt to open Deepdive with its search, search revision, execution plan, and run already selected. The Research context panel then offers four dependent searchable single-select controls; a child becomes available after its parent is known, a sole valid child may be selected automatically, and changing a parent clears invalid descendants.

The URL stores `search_id`, `search_revision_id`, `plan_id`, and `run_id`. Navigation, reload, browser back and forward, and internal links preserve that context. Filters, sorting, pagination, sections, expanded rows, graph state, artifact inspection, focused `note_id`, focused `anchor_id`, and `pdf_page` are also URL-backed where sharing or reload continuity matters.

Copied URLs may expose research identifiers and selected filters through browser history or messages. Treat them as research context rather than secret-bearing links, and do not share them where the identifiers are sensitive.

## 4. Navigation and breadcrumbs

The application opens on Home. Every route shows a breadcrumb beginning at Home. Deepdive has six tabular destinations in this order: Overview, Corpus, Relationships, Provenance, Evaluation, and Advanced. On narrow screens, the Menu button opens the same Deepdive navigation and reports its expanded state to assistive technology.

The header reports database health and always presents the viewer as Local review. Pipeline evidence remains immutable even though review heads can move to newly appended versions. A Skip to content link, navigation landmarks, headings, labels, live status regions, and visible keyboard focus support keyboard and assistive-technology use.

## 5. Home

Home summarizes the available search terms, search revisions, execution plans, and run attempts, including latest execution time, duration, outcome, and visibility. Search-history cards show how revisions, plans, and attempts relate, while the run-attempt table provides the direct Explore action for a complete Research context.

Move to trash hides a terminal attempt from the normal active-run selection without deleting its immutable evidence. Restore makes a trashed attempt active again. Both actions require confirmation, reject running attempts, update visibility and append a lifecycle audit event in one transaction, and remain available from Home; the application provides no physical-delete or purge control.

## 6. Overview

Overview explains how the selected run moved from declared source exports through parse, deduplication, enrichment, validation, and normalization. Captured execution metrics are distinguished from values derived from the current database so later interpretation does not confuse historical run evidence with current coverage.

The page begins with a compact run identity strip. Its retention flow uses three connected phases: Source selection, Pipeline processing, and Corpus enrichment. Each step displays the retained count, percentage, change from the previous step, and an information disclosure; inspectable processing stages link to a relevant Corpus or Provenance filter while source-only aggregate filters remain informational. Missing or unavailable values remain labeled rather than being silently converted to zero.

## 7. Corpus

Corpus uses one labeled collection selector for Analysis-ready articles, Authors, References, Author identity / ORCID evidence, and Source records instead of a nested tab row. Tables support bounded pagination, allowlisted sorting, search, page-size controls, and row disclosures. Multiple rows may remain expanded, and explicit controls remain available when row click also toggles expansion. The article list begins with DOI, Title, Year, Journal, and Source; the author and identity tables omit internal IDs, and identity-candidate payloads are excluded from the collection table.

Articles in the analysis-ready collection are valid normalized revisions for the selected run. Expanding an article row shows a Matched search terms block: how many of the run's recorded search terms matched the article and which terms matched in Title, Abstract, Keywords, and Keywords plus. Runs without stored term data show No search terms recorded. The matches are derived by the pipeline from the recorded source queries and are a local approximation of the database searches, not captured pipeline evidence. Author and reference collections can include occurrence or mention records connected to multiple revision snapshots; repeated conceptual names are possible and should be interpreted with revision and producer-stage context.

Detail pages remain within Corpus navigation and use breadcrumbs instead of redundant return links. Article detail places full-width PDF status above an equal-width Document reader and Article review workspace, followed by full-width Provenance summary, Bibliographic metadata, and a Search term coverage panel, then related authors and references, stage history, an article-scoped audit timeline, and raw evidence. The coverage panel lists matched terms per field, an All search terms disclosure with matched and unmatched terms and their source databases, and a Not recorded or No matched terms state per field. Author detail contains the uncertain ORCID candidate rows and links to preserved provider evidence; author and reference details retain observed or mentioned values rather than presenting uncertain data as confirmed identity.

## 8. Identity evidence

An exact observed ORCID can support confirmed identity behavior. ORCID name-search results are uncertain candidates only. Candidate rank, provider data, and evidence context are inspectable, but a name-search match does not assign an ORCID to an occurrence or person.

Resolution statuses include matched, no candidate, unclear, and provider error. No candidate uses a neutral label, while unclear uses a warning label; treat both and provider-error outcomes as recorded evidence about an attempted resolution, not proof of identity or absence.

## 9. Relationships

Relationships supports research network, article-author, citation, and article-reference modes. Filters and selected graph state are stored in the URL. Shape encodes entity type and color encodes connected component; selection details and a paginated relationship table provide the textual equivalent of canvas content.

Graph responses are bounded. The server caps article nodes at 2,000, related nodes at 10,000, and edges at 20,000. When results are truncated, the page reports the condition and reason; the browser does not stream the remainder.

Canvas controls support fit, zoom, drag, selection, search, cluster overview, expansion, and PNG export. Keyboard and nonvisual users should rely on the controls, summaries, selection panel, and relationship table rather than canvas pixels.

## 10. Provenance

Provenance has Audit, Artifacts, Cache, Stages, and Run detail sections. Audit presents a top-to-bottom timeline grouped by day and supports text search plus category, action, actor, entity, stage, outcome, and run-context filters with bounded cursor pagination. Loading older events appends them without closing already-open Recorded data disclosures and reports the loaded count or beginning of history beside the action. Pipeline, enrichment, validation, PDF, and review actions retain their own semantic labels. Review-decision events visibly compare the complete previous and new status, optional reason, and every sub-status; review metadata identifies contexts and versions without duplicating note bodies, selected text, reviewer email, or browser drafts.

Artifacts list exact captured inputs, manifests, provider payloads, and pipeline outputs associated with the selected run. Preview requests retrieve only a bounded prefix: 64 KiB by default and no more than 256 KiB. Recognized text, JSON, and SOMETHING content can be previewed; the original bounded server download remains separate.

Cache evidence distinguishes hits, misses, negative results, stale entries, invalid payloads, and network fetches. Stage views show run-level progression and per-work outcomes so discarded or failed work remains inspectable.

## 11. Evaluation, review, and PDFs

Evaluation lists normalized work and overlays companion PDF inventory plus the selected run context's current review status. `not_available` means the DOI is registered but no validated selected bytes are present. `available` means validated PDF bytes and an inventory timestamp exist. Review status is a contextual interpretation and is not an intrinsic property of the PDF.

The viewer never downloads PDFs from external services and never changes inventory state. An available PDF is rendered locally through the custom vendored PDF.js interface and remains available from the local API with download-safe headers. Manual inventory is performed outside the application using the supported tool described in [PROJECT-USAGE.md](PROJECT-USAGE.md).

Metadata and PDF databases are one portable bundle after inventory begins. If the companion binding or schema is unavailable, the viewer reports unavailable PDF evidence rather than mutating or repairing the store.

A completed non-trashed run has at most one review context. Select Start review to open a modal setup surface where you can confirm the proposed earlier parent, choose another same-search context, explicitly include all earlier searches, or start empty. Same-search and all-search loading report their result in the context message below the scope control. Close, Cancel, the dimmed backdrop, or Escape dismiss setup without initializing review. The default proposal prefers an earlier context from the same execution plan and then the same stable search. Initialization freezes matching stable-work version heads at that moment; later parent edits do not change the child.

The initialized Article review panel separates Decision, Notes, and PDF anchors into keyboard-operable tabs. Left and Right Arrow move between tabs, while Home and End move to the first and last tab. Article status choices are Not Evaluated, In-progress, Approved, Not approved, and Removed. The optional reason is available for every status. Not approved and Removed additionally permit Redacted, Unrelated, Out of scope, Duplicate, Retracted, Withdrawn, Superseded, Predatory/low quality, Copyright/licensing, and Not peer-reviewed as a multi-select set. Saving records one complete immutable state with reviewer and time attribution. A conflict message means another version moved the same context head; current textarea and form input remain available for comparison and retry.

Notes support headings through level four, paragraphs, block quotes, bullet or `1.` lists, fenced code, and simple tables. Open Note syntax and link examples beside the editor to see the supported grammar. Link forms are `[[note:123|display]]`, `[[article:10.1000/example|display]]`, `[[pdf:page=5|display]]`, `[[anchor:methods-1|display]]`, and `[[ext:https://example.test|display]]`; a DOI therefore uses the `article` target type, not `doi`. Preview content is escaped. A visible Unresolved link label means syntax was accepted but the selected context does not currently contain the target; it may resolve later without rewriting the note version.

Editing, removing, and restoring a note append immutable versions. History compares bounded line sets and retains tombstones. Unsaved drafts are best-effort browser-local values scoped by corpus, run, article revision, logical note, and expected version. Successful saves clear only the matching draft; conflicts or storage failures do not clear the editor. Browser clearing, quota, privacy mode, or another profile can lose drafts, and OSF preparation does not copy them.

The custom PDF reader renders one current page with selectable text, page navigation, zoom, and rotation. One Previous or Next activation loads exactly one adjacent page, and the controls become unavailable at the first and last page. The document remains inside its own scrollable viewport instead of covering later article content. Selecting text on one page opens the PDF anchors section and a named anchor form; supply an ID beginning with a letter and containing only letters, digits, `.`, `_`, or `-`. The keyboard-operable anchor list identifies current or inherited state, page, availability, selected text, and history. An anchor tied to different PDF content remains recorded but is labeled unavailable and is not drawn over the current PDF.

Articles without available PDF bytes remain readable but cannot save status, note, or anchor changes. This protects the review contract that every mutation is associated with an available document while leaving immutable pipeline evidence accessible.

## 12. Advanced

Advanced exposes discovered non-system SQLite tables through an evidence-only browser. Table names and sort columns are validated against discovered schema, page sizes are limited to 20, 50, 100, 200, or 500, and raw values remain escaped and copyable. It does not provide arbitrary SQL or direct table mutation. Run visibility actions remain on Home rather than appearing as a separate Trash destination.

## 13. Loading, errors, and unavailable data

Global navigation displays a loading state and cancels stale requests. A slower previous route cannot overwrite a newer route. Request failures, invalid JSON, unavailable context, and local progressive-action errors use explicit messages rather than blank content.

Empty means the selected scope has no matching records. Unavailable or Not recorded means required evidence is absent or cannot be derived. Truncated means the server deliberately bounded a larger result. These states are not interchangeable and should remain distinct during analysis.

Local actions such as loading older audit events, inspecting an artifact prefix, copying preview text, graph interactions, saving complete review state, editing notes, or creating anchors keep existing content visible when possible and report failure near the operation. `409 Conflict` preserves form or draft input and requires reloading current history before retrying.

## 14. Privacy and security expectations

- The server accepts only an exact loopback IP listener; it cannot be deliberately exposed on another interface through supported serving options.
- Do not treat the application as authenticated, authorized, encrypted, multi-user, or safe for untrusted networks.
- Review databases and artifact contents before sharing screenshots, reports, downloads, URLs, or browser profiles.
- The frontend escapes dynamic content and the server bounds previews, but the viewer can still reveal the research data stored in the selected database.
- Production assets are served from the assembled `frontend/dist` directory and make no CDN request; provider APIs are not contacted while serving the viewer.
- Credentials, tokens, private keys, and sensitive environment values must never be stored as artifacts intended for viewing.
- Reviewer username and email may appear as local version attribution but are not copied into audit metadata. Empty and sanitized reviewer identity displays as `Anonymous or redacted`; review the bundle before sharing screenshots or browser profiles.

## 15. Accessibility and responsive expectations

All actions have a keyboard path and visible focus. Disclosure controls expose `aria-expanded`, navigation exposes current state, status regions announce loading and failures, and color is not the sole status indicator. Canvas information remains available through DOM content.

Below 720px, navigation, selectors, summaries, toolbars, graph layout, and detail panels adapt to the viewport. Wide tables remain horizontally scrollable rather than dropping columns or evidence.

Light and dark themes follow system preference. Reduced-motion preference suppresses unnecessary smooth effects, while force-layout computation does not make essential state depend on animation.

## 16. Operational checks

The server health endpoint is `GET /api/health`. A healthy response confirms that the process can read selected metadata, exposes review capability and the opaque corpus draft namespace, and has passed startup schema protection checks; it does not prove that every optional artifact, PDF, or research-context record exists.

If the viewer cannot start, confirm that the metadata path exists, is a file, is writable by the local reviewer, has V00025 and required review triggers, and has been migrated explicitly. If context selectors are empty, confirm that the database contains searches, revisions, plans, and runs. If PDF status is unavailable, confirm the relative companion binding and database move together with metadata. On a database migrated only to V00024, the corpus and article detail pages still work but show No search terms recorded because the term-coverage tables do not exist.

## 17. Current limitations

- The viewer has no application authentication or authorization.
- Browser drafts are not database evidence or export content and may be lost.
- Author and reference collections can repeat conceptual values across revision snapshots.
- Graph and collection endpoints are bounded and do not stream beyond their limits.
- URLs preserve useful research context but can reveal its identifiers.
- String-template rendering relies on deliberate escaping for each new dynamic value.
- Visual snapshots and fixtures represent controlled data and tested browser environments, not every platform.
