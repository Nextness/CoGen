# Research Workspace Frontend Design

## 1. Status and scope

This document is the implemented design reference for the local research workspace viewer. It defines current information architecture, interactions, visual language, responsive behavior, accessibility expectations, content rules, and acceptance criteria. [ARCHITECTURE.md](ARCHITECTURE.md) owns implementation structure, [CSS-REFERENCE.md](CSS-REFERENCE.md) owns active stylesheet details, [APP-USAGE.md](APP-USAGE.md) explains the user experience, [PROJECT-USAGE.md](PROJECT-USAGE.md) explains developer and operator workflows, and [STANDARDS.md](STANDARDS.md) defines frontend change rules.

The viewer inspects historical research runs, manages reversible run visibility, and records local run-scoped interpretations. It does not execute the pipeline, modify pipeline corpus records, physically delete or purge runs, add PDFs, or resolve identity candidates. Every screen distinguishes immutable pipeline evidence from append-only review versions and mutable context heads.

## 2. Product goals

- Preserve research context across every navigation action so users always know which search, revision, execution plan, and run attempt they are inspecting.
- Separate evidence captured during execution from values derived from the database at view time.
- Make provenance, validation, cache use, identity uncertainty, and PDF inventory status inspectable without requiring direct SQL.
- Let a user explicitly initialize one review context for a completed run, confirm lineage, record complete article status, compare immutable note history, follow resolved links, and create content-hash-bound PDF anchors without changing earlier run contexts.
- Keep large corpora, audit streams, artifacts, and relationship graphs bounded and navigable.
- Use semantic HTML, visible focus, text alternatives, and table equivalents so essential information is not encoded only through color or graphics.
- Work from local assets assembled from `frontend/` sources, with no CDN or application-framework dependency.
- Preserve privacy through mandatory loopback serving, existing-only metadata writes, a read-only PDF database, bounded mutation and preview inputs, escaped content, and no authentication claims.

## 3. Design principles

### 3.1 Historical truth before dashboard polish

The interface must state whether a number was recorded by the pipeline or derived from current stored rows. Missing recorded evidence is displayed as not recorded, never silently converted to zero. Failed and discarded outcomes remain visible because they are part of the research record.

### 3.2 Context is persistent state

The canonical research context is `search_id`, `search_revision_id`, `plan_id`, and `run_id` in the URL. Navigation, filters, pagination, sorting, selected graph nodes, provenance sections, focused `note_id`, focused `anchor_id`, and `pdf_page` also use URL state when that makes a view reloadable or shareable. Every internal link uses the shared `link()` helper.

### 3.3 Bounded detail with progressive disclosure

Tables paginate, audit events load older cursor pages, graph endpoints report truncation, long cells expand on demand, advanced filters use disclosures, and artifact inspection fetches a bounded safe prefix. A user can move from summary to exact evidence without loading unbounded data into the browser.

### 3.4 Uncertainty remains explicit

Observed author names are occurrences, not automatically people. ORCID name-search results are candidates, not confirmed identifiers. Cache reuse, stale data, skipped stages, validation rejection, unavailable metrics, and missing PDFs use distinct labels and explanatory copy.

### 3.5 Visualizations require textual equivalents

The retention flow is supported by counts and tables. Relationship graphs include filters, cluster summaries, legends, selection details, and a paginated relationship table. PDF highlights have a keyboard-operable anchor list with page, current or inherited state, availability, selected text, and history. Color is supplementary to labels, shape, text, and status wording.

### 3.6 Type hierarchy

The system font stack is used for interface text and the system monospace stack is used only for code, identifiers, hashes, URLs, JSON, and machine values. Body copy is 15px with a 1.55 line height on larger screens and 14px on mobile. The semantic heading hierarchy is H1 for the product identity at 24px, H2 for the page title at 28px, H3 for a parent panel at 20px, H4 for a section at 16px, H5 for a subsection or timeline event at 14px, and H6 for a 12px uppercase eyebrow. A subheader is sentence-case supporting copy beneath its owning heading, while an eyebrow is a short uppercase contextual label above it. Headings have no hover, selected, or disabled state because they are not controls; a linked heading follows the standard link states. Rendered review-note headings use a scoped visual hierarchy and do not create page-level H1 through H4 elements.

### 3.7 Container and spacing hierarchy

The page stack owns 24px separation between parent components. A parent panel occupies the available width, has no external margin, uses a 16px inset on desktop and 12px on mobile, and contains a header, body, and optional footer. The header-to-body and sibling-child separation is 16px, content groups use 12px, and inline controls use 8px. Child panels use a muted or raised surface with a border and no additional elevation. A component never creates an external gap that duplicates its parent layout. Side-panel and main-panel layouts place filters before results in source order, use a 20rem filter column only when the content benefits from it, and collapse to a single top-to-bottom flow at or below 1100px.

### 3.8 Color roles and interaction states

Accent blue is reserved for links, navigation, primary actions, focus-adjacent selection, and selected rows. Information blue explains recorded facts. Success green means completed, valid, available, matched, or resolved. Warning amber means incomplete, stale, skipped, unavailable, not evaluated, or unresolved. Danger red means failed, invalid, rejected, discarded, or destructive. Neutral gray means unknown, not recorded, read-only, or inactive. Review violet identifies human review or inherited lineage, and note pink identifies note content. Every role has a soft surface treatment, every interactive accent has default, hover, and active values, and disabled controls use a muted surface, muted text, a normal boundary, and explicit disabled semantics instead of opacity alone. Graph cluster colors encode connected components only and never substitute for status colors.

### 3.9 Messages, labels, and tags

Messages contain explanatory text and may include a short title. Information, success, warning, danger, neutral, and review messages combine a semantic boundary, soft background, symbol, and text so color is never the only cue. Status labels use the same semantic meanings as messages. Neutral compact labels show counts and recorded categories, review labels show inherited or human-authored evidence, note labels identify note content, and removable filter chips expose an accessible removal action. Linked labels have hover and active treatment; disabled labels remain legible and noninteractive.

### 3.10 Navigation and disclosure

Home is the application entry point. The six URL-backed Deepdive destinations use one tabular menu and mark the current link with `aria-current`; view-specific collections such as Corpus use a labeled selector rather than another competing tab row. Mutually exclusive content inside one panel, such as Decision, Notes, and PDF anchors, uses a keyboard-operable tab list with selected tab and tab-panel semantics. A radio-like model selector uses a segmented control with `aria-pressed`. Progressive or optional content uses native disclosure semantics with a consistent chevron, focus, hover, and open treatment; table-row detail uses the table expansion contract rather than a standalone tab or an unrelated disclosure style.

### 3.11 Forms and filters

Inputs, selects, and textareas share a 38px default control height, label placement, help text, required indication, and visible default, hover, focus, invalid, read-only, and disabled states. A table filter bar contains search, page size, sort, and actions and wraps without changing source order. A complex filter panel presents the most common controls first, places advanced controls in a native disclosure, shows the selected scope or result summary, and exposes active URL-backed values as removable chips. The primary Apply or Search action precedes a basic Reset action.

### 3.12 Tables and row expansion

Evidence tables use 13px body text with a 1.45 line height, 10px by 12px cells, sticky headers where supported, tabular right-aligned numeric values, monospace identifiers, nonwrapping times, semantic labels in status columns, and horizontal overflow owned by the table region. Only selectable or expandable rows receive hover treatment. Sortable headers expose `aria-sort`, selected rows use an accent edge and soft background, disabled rows retain legible muted text, and an empty result occupies the full table width. Long text is constrained only in declared long-text columns and expands through an explicit accessible control.

An expandable table row uses one chevron button of at least 34px, a stable row key, `aria-expanded`, `aria-controls`, a selected parent row, and a sibling detail row spanning all columns. Multiple rows may remain open, URL state preserves expansion where the view requires reload continuity, and expanding a row does not discard a user text selection. Inline disclosure is reserved for compact cell content, row expansion is used for record detail, and an accordion is used for standalone progressive sections.

### 3.13 Metrics

Headline totals use two through six KPI cards with a 13px label, a 28px tabular value, and an optional 12px unit or basis. Category, provider, field, enrichment, and normalization distributions use metric rows with a 14px label, a 20px value, an optional percentage, a full-width bar, and explicit denominator or scale wording. Exhaustive captured evidence uses a compact metric table with metric, value, and scope columns. Distribution bars state whether they show share of total or relative volume, provider names do not introduce arbitrary colors, and cards never add filler text such as Recorded value.

### 3.14 Selection and content states

Normal text selection uses the soft accent color, links retain accent color after visiting, link hover increases emphasis, and active links use the stronger accent. Interactive selected state is conveyed through `aria-current`, `aria-selected`, `aria-pressed`, or `aria-expanded` plus visual treatment. Unavailable content remains visible with a warning or neutral explanation and a recovery action when one exists; it is not represented by a disabled control appearing without context.

## 4. Information architecture

Home is the context-independent entry point. Selecting Explore opens the Deepdive workspace, whose tab order is Overview, Corpus, Relationships, Provenance, Evaluation, and Advanced. Article, author, and reference detail routes are contextual descendants of Corpus and keep Corpus marked as the current Deepdive destination.

| View | Primary question | Required context |
|---|---|---|
| Home | Which searches, revisions, plans, and attempts exist, and which run should be explored or moved through the reversible visibility lifecycle? | Database required; run is not required. |
| Overview | What happened in this run, and what does its current stored corpus contain? | Run required. |
| Corpus | Which normalized articles, observed authors, references, identity evidence, and source records belong to this run? | Run preferred; limited schema-backed fallback exists where supported. |
| Relationships | How are valid normalized articles connected to authors, citations, and references? | Run required. |
| Provenance | What audit events, artifacts, cache decisions, stages, and attempt metadata explain this run? | Audit may span runs; other sections require a run. |
| Evaluation | Which normalized articles have a PDF, what is their current run-context review status, and has review been initialized? | Run required. |
| Advanced | What values exist in each discovered metadata table? | Database required; run is not required. |

## 5. Application shell

The shell begins with a skip link, then a site header containing Local research workspace, Research workspace, the database health state, and a persistent Local review marker. This header and a breadcrumb are present on every route. Home hides run-scoped controls; Deepdive displays the research context followed by its fixed tab order. A mobile Menu button controls the Deepdive tabs below 720px.

The compact context surface contains only dependent Search, Search revision, Execution plan, and Run attempt searchable single-select controls. It has no separate title or Clear context action. Each control presents a bounded server-searchable page eligible under its current parent, supports keyboard listbox navigation, preserves an exact selected option outside the current page, selects a sole available child automatically, and clears downstream identifiers when a parent changes. Context is intentionally singular because one Deepdive route represents one immutable search, revision, plan, and run chain.

The main region contains an alert notice, a polite live loading indicator, and the view container. One authoritative shell template generates the Home, six Deepdive, and three detail HTML documents with a static `rw-page` marker and initial title. Rendered view titles update `document.title` to `<page title> · Research workspace`. Global health, loading, and error states must remain understandable without inspecting developer tools.

## 6. URL and navigation behavior

The default view is Home when `view` is absent. Explore links establish the complete research context and open Overview; Deepdive links retain that context. Changing search clears revision, plan, and run; changing revision clears plan and run; changing plan clears run; changing run preserves its ancestors. Every view supplies an explicit breadcrumb beginning at Home, with detail routes extending through Deepdive and Corpus.

Every application-generated URL contains the owning page file and the `view` query parameter. Cross-view links perform a native full-page load, so browser back, forward, reload, and bookmarks use document navigation. Same-view links and programmatic state changes without modifier keys push or replace browser history and render without reloading the document. External links, downloads, modified clicks, and non-application paths retain normal browser behavior. Direct legacy root URLs such as `/?view=overview` remain supported through `index.html`; application links canonicalize the same view to `overview.html?view=overview`.

Each render aborts the previous request controller and receives a monotonically increasing sequence number. Only the newest sequence may display errors, change the title, or clear the loading state. Aborted requests are silent.

View-specific URL keys must be namespaced when multiple tables coexist. Provenance uses keys such as `cache_page`, `stage_page`, and `audit_category`; graph state uses `mode`, filters, `article_limit`, and `node`; Corpus and Evaluation use `section`, `q`, `page`, `per_page`, `sort`, `order`, and expansion state as applicable; article review links use `note_id`, `anchor_id`, and `pdf_page`.

## 7. Overview

Overview begins with a compact borderless run identity strip containing attempt, start, finish, duration, plan, outcome, and visibility. The retention flow follows in exactly three phases: Source selection covers initial raw results, publication range, document type, and language; Pipeline processing covers input records, parsed articles, and deduplicated articles; Corpus enrichment covers candidate articles, accepted plus discarded outcomes, and normalization. Each connected step keeps its count, percentage, delta, and progress bar visible while explanatory detail moves into an information disclosure.

Captured metrics are grouped by input, parsing, deduplication, enrichment, validation, normalization, cache/network, and other evidence. Current coverage is presented separately because it is derived from stored rows rather than guaranteed to have been captured at attempt completion. Headline totals use KPI cards, meaningful categorical distributions use metric rows with an explicit percentage or relative-volume basis, and exhaustive recorded values use a compact metric table.

Corpus summary cards link to the relevant Corpus or Relationships destination without losing context. Breakdown panels cover enrichment, validation, normalization, source distribution, and cache activity. Normalization outcomes distinguish changed, already canonical, and unavailable fields against the assessed denominator.

Retention stages that correspond to inspectable record sets navigate with a bounded Corpus or Provenance filter that explains the selected stage. Source-filter aggregate steps remain informational because the current data contract does not expose a record-level filter mapping for those counts.

## 8. Corpus

Corpus uses one labeled collection selector for Analysis-ready articles, Authors, References, Author identity / ORCID evidence, and Source records, avoiding a second tab hierarchy beneath Deepdive. The section description must identify the underlying entity semantics, especially that author occurrences do not imply global person identity and name-search candidates do not imply confirmed ORCID assignment.

Articles are valid normalized revisions for the selected run and present DOI, title, year, journal, and source in that order, with identifiers and further metadata available through expansion. A message explains that discarded works remain in stage and provenance evidence rather than this analysis-ready list. Article row expansion includes a Matched search terms block that summarizes how many of the run's recorded search terms matched the article and lists the matched terms per field (Title, Abstract, Keywords, Keywords plus); a run without stored term data shows No search terms recorded. The matches are derived data stored by the pipeline, not captured pipeline evidence, and matching is deterministic: every word in the term and field value is stemmed before whole-word matching so inflected forms match their base term.

Authors are observed occurrence records linked through run revisions. The table omits internal IDs and exposes citation name, parsed names, observed ORCID, person link, article count, affiliation count, and captured time. It must not label same-name rows as one person unless repository identity evidence confirms that relationship.

References are ordered mentions attached to immutable revisions. Rows expose citing context, DOI, title, author string, year, source, and resolved-work identity when present. Article and reference links preserve the selected run so detail resolution remains historical.

Identity evidence summarizes resolutions, unclear matches, provider failures, and candidate counts without embedding candidate payloads in the collection table. Candidate rows move to the relevant author detail page, where rank, ORCID, display name, and preserved provider evidence remain visibly uncertain rather than accepted identity. `no_orcid_candidate` uses a neutral status treatment while `orcid_is_unclear` uses warning treatment so the outcomes remain distinguishable without relying only on wording.

Source records expose source name/type, record index, parse outcome, reject reason, content hash, and capture time. When source-level result counts are available, they appear above the table to connect configured retrieval and parsed evidence.

Corpus search is server-backed for scoped endpoints. Page size choices are 20, 50, 100, 200, and 500. Unsupported sort fields or page sizes must be corrected to safe defaults in the UI and rejected by the server if submitted directly.

## 9. Detail views

Article detail uses the breadcrumb Home / Deepdive / Corpus / Analysis-ready articles / DOI instead of a redundant back control. It presents the revision title and summary, a full-width PDF status strip, then a responsive equal-width Document reader and Article review workspace with a defined gap. Full-width Provenance summary and Bibliographic metadata panels follow, then a Search term coverage panel, before authorships, reference mentions, stage outcomes, the article-scoped audit timeline, and advanced raw data. The displayed revision remains anchored to the selected run context; audit may aggregate the selected work's related revisions inside that run.

The Search term coverage panel is derived from the run's recorded queries and the revision's stored fields. It shows a summary line of how many of the run's search terms matched the article, one row per field (Title, Abstract, Keywords, Keywords plus) with matched terms as chips, and an All search terms disclosure listing matched and unmatched terms with their source badges. A field with no recorded value shows Not recorded, a recorded field with no matches shows No matched terms, and a run without stored term data shows No search terms recorded. The panel states that matching is deterministic and stems every word before whole-word matching.

The PDF panel distinguishes Available from Not Available, shows inventory timing when present, and exposes content only when the read-only PDF API can join the available inventory row to stored bytes. The custom PDF.js interface renders exactly one current page with selectable text, boundary-aware page controls, zoom, rotation, and active anchor highlights without the default PDF.js viewer UI. One Previous or Next activation changes the current page once, concurrent display changes cancel stale rendering, the contained page viewport owns document overflow, and navigation destroys rendering and worker tasks. The frontend never creates or changes PDF inventory state.

Review is started explicitly for one completed non-trashed run. Start review opens a modal dialog with a dimmed backdrop, visible title and close action, deterministic proposed parent, same-search alternatives loaded first, and a deliberate action that expands to all earlier searches. The expansion action reports loading, added, empty, and failed outcomes in one contextual message region instead of disappearing or exposing an unexplained disabled control. Starting empty and dismissal without initialization are always available. Creation freezes inherited version IDs for stable work matches; inherited labels identify reused heads and later parent edits do not propagate.

Complete review state is organized into keyboard-operable Decision, Notes, and PDF anchors tabs so the dense editing surfaces remain locally navigable. Left and Right Arrow move between adjacent tabs, Home and End move to the first and last tab, and each selected tab identifies its tab panel. Decision uses `Not Evaluated`, `In-progress`, `Approved`, `Not approved`, or `Removed`, an optional reason, and multi-select qualifiers only for `Not approved` and `Removed`. Qualifier labels are `Redacted`, `Unrelated`, `Out of scope`, `Duplicate`, `Retracted`, `Withdrawn`, `Superseded`, `Predatory/low quality`, `Copyright/licensing`, and `Not peer-reviewed`. Saving appends a version, shows reviewer and time attribution, treats identical input as a no-op, preserves edited input when a stale expected version returns a conflict, and appends an audit comparison containing the complete previous and new status, optional reason, and every qualifier.

Review notes use a bounded project grammar with headings, paragraphs, quotes, lists, fenced code, simple tables, and links shaped as `[[note:123|display]]`, `[[article:10.1000/example|display]]`, `[[pdf:page=5|display]]`, `[[anchor:methods-1|display]]`, and `[[ext:target|display]]`. The side-by-side editor and safe preview precede the saved-note search and cards. Insert evidence link and Note syntax and link examples remain optional disclosures inside the editor; DOI links use the `article` target type rather than `doi`. Preview escapes raw HTML, displays parser diagnostics before saving, labels unresolved targets with text and an accessible name, and resolves targets when read so later-created targets do not rewrite history. Edits, removal tombstones, and restoration append versions; history provides bounded line comparison with side-by-side fallback for large bodies.

Unsaved note drafts remain only in browser storage under a key containing the opaque corpus ID, run, revision, logical note, and expected version. A successful save clears only the matching draft. Storage failures and version conflicts keep textarea content and display a warning. OSF export does not scan or copy browser drafts.

Selecting text on one rendered PDF page creates one through 64 normalized rectangles and opens an anchor form with a work-scoped human label. The repository generates the corpus-unique opaque anchor ID used by stable note links. Anchors retain exact work revision, PDF content hash, selected text, geometry, immutable history, and inherited or current labels. A hash mismatch displays the anchor as unavailable and does not project stale geometry or offer an unqualified page action. The textual anchor list is keyboard operable, can navigate to a matching page, shows history and tombstones, and remains the non-visual equivalent of highlights.

Author detail describes one author occurrence and its linked articles, affiliations, person identity, and audit context. Reference detail describes one mention, its citing revision, captured fields, and resolved target when the selected run contains a suitable revision.

Related article and author collections use counted, run-owned cursor subresources. Each section keeps independent URL cursor state, loads one bounded page at a time, and retains the current page when continuation fails. Breadcrumbs and return links preserve the view, section, run, and relevant entity identifier.

## 10. Relationships

Relationships defaults to Research network in the user interface. Available models are Research network, Articles and authors, Internal citations, and Articles and references. Changing model preserves article filters and clears stale selected-node state.

Common filters are title/DOI, year range, and source. Advanced filters are author, ORCID, reference, citation range, reference-count range, and article limit. The limit defaults to 2,000, must be between 1 and 2,000, and is omitted from the URL when it remains at the default.

The server response is authoritative about matched articles, rendered nodes/edges, entity counts, limits, truncation, and truncation reason. The view must warn when the selected article set or related entities were truncated rather than implying a complete network.

The graph canvas supports deterministic initial layout, fit, zoom, wheel zoom around the pointer, pan, keyboard-accessible controls, a bounded keyboard-operable node result list, cluster overview selection, expansion mode, and PNG export with run and filter context. The exact relationship table remains the authoritative keyboard-accessible evidence. Destroying or leaving the view stops the simulation, observers, animation frames, and event state.

Entity type is encoded by shape and label: articles, observed authors, references, and raw referenced-author strings remain distinguishable without color. Connected component is encoded by one of six token colors, repeated when necessary. Edge style and the relationship legend distinguish authorship, citation, reference, coauthor, and shared-reference relationships.

Selection opens a details panel with entity metadata and neighboring relationships. The relationship table is the exact textual equivalent of the visualization and paginates independently. Graph search and table navigation must remain useful when canvas content is dense or inaccessible.

## 11. Provenance

Provenance subnavigation contains Audit timeline, Artifacts, Cache uses, Stage outcomes, and Run details. The page description consistently states that evidence is append-only and inspection-only.

Audit provides server-backed text search plus multi-select category, action, actor, and entity filters and optional stage/outcome fields. Active filters appear as removable chips in URL state. The newest 25 events load first, and Load 25 older events follows the cursor without replacing already visible evidence.

Audit is a top-to-bottom chronology grouped by date with one continuous rail, category marker, and event card. Each card keeps action, category, actor/source, affected entity, time, run context, and outcome visible while event identifiers, correlation identifiers, recorded metadata, and before/after values remain in a Recorded data disclosure. PDF events use the same event language but may be global to a work rather than owned by one pipeline attempt. Review events identify context and old or new version IDs without duplicating note bodies, selected PDF text, reviewer email, or browser drafts. Loading older events appends and deduplicates cards, preserves open disclosures, reports the loaded count in a live region, and replaces the action with a persistent beginning-of-history state only when the cursor is exhausted.

Artifacts show run context, role, producing/consuming steps, media type, byte size, content hash, time, preview availability, and download. The inventory uses the shared rows-per-page selector and complete First, Previous, numbered, Next, and Last pagination controls. Inspection requests 65,536 bytes, identifies truncation, formats complete JSON when possible, supports raw/formatted modes, line wrapping, clipboard copy, and original download. Binary or unsafe media remain download-only.

Cache uses are a scoped searchable table with independent page/sort/query keys. Rows explain provider, request identity, cache layer, outcome, expiry, payload artifact, and recorded time rather than reducing reuse to a single hit count.

Stage outcomes begin with an ordered run-level progression that reconciles run steps and per-work summaries. Each stage shows status, outcome counts, work-record count, persisted duration, and linked artifacts. New run-step boundaries are recorded as microsecond-precision UTC timestamps so subsecond work is not rounded to a one-second tick; legacy rows retain the precision originally stored. A separately paginated table retains individual work outcomes and reasons.

Run details show attempt identity, status, visibility, timestamps, duration, plan fingerprints, enrichment policy, summary, and exact configuration/manifest snapshot downloads. Legacy attempts may explicitly lack snapshot payloads.

## 12. Evaluation

Evaluation combines reading inventory and current selected-context review state; neither is an intrinsic quality score. Its queue lists every normalize-stage article in the selected run with title, DOI, inventory status, inventory calendar date, review status, and current-context or inherited lineage. Source and qualifier remain advanced server filters and article-detail evidence rather than queue columns.

Search is server-backed over title and DOI. Sorting is limited to title and DOI, and table controls use the shared page-size and pagination behavior. Article titles link to the selected revision detail while retaining research context.

Available means a PDF was manually validated and inserted into the bound companion store. Not Available means no selected PDF bytes exist for the registered DOI. The view does not imply that a missing PDF was searched for automatically and provides no PDF add action. A completed run without a context presents Start review; after initialization, article rows link to the detail review workspace. Articles without an available PDF remain readable and permit review-decision and Note mutations when the active-run context and work membership are valid; only PDF anchor creation or restoration remains unavailable.

## 13. Home lifecycle and Advanced

Advanced discovers current metadata tables from the server and lets the user select one, inspect bounded rows, sort by an actual discovered column, change page size, and expand a row to see additional fields. It is implementation-level transparency, not a general SQL console, and it never accepts arbitrary table, column, or SQL expressions.

Home summarizes counts and timing for searches, revisions, plans, and attempts, groups immutable research history by search, and lists every attempt with Explore plus a lifecycle action. Move to trash and Restore are explicit reversible mutations guarded by a confirmation dialog, origin and JSON validation, terminal-run checks, one transaction, and an append-only audit event. The interface exposes no physical deletion or purge action; Advanced remains available for evidence inspection without changing its current behavior.

## 14. Tables and data presentation

Shared tables render a visible subset of columns, optional expandable property grids, sortable headers, bounded page controls, result ranges, search controls, and long-cell clipping. The disclosure column is unlabeled but has an accessible control name and `aria-expanded` state.

Row click may toggle expansion only when it does not steal behavior from links, buttons, inputs, labels, selections, or text interaction. Multiple rows may remain expanded. Expansion state may be URL-backed when reload continuity is useful.

Empty states state what is absent and, when relevant, how to select context. Error states use the global alert or a local message near the failed operation. Loading buttons disable themselves and display a loading class until the operation settles. Review mutations announce success, parser failure, unavailable-PDF state, and optimistic conflict near the affected form without discarding user input.

IDs, hashes, URLs, raw JSON, and machine values use monospace or code treatment and remain copyable. The English-only interface formats recorded timestamps consistently with the `en-US` locale and UTC time zone and emits machine-readable datetime attributes where time is presented. Numeric values use locale grouping; unavailable values use descriptive text or a neutral symbol with context.

## 15. Visual language

The interface uses a restrained research-tool aesthetic: cool neutral surfaces, dark slate text, blue interaction accents, semantic green/orange/red states, compact bordered panels, modest elevation, and dense but readable tables. Decoration must not compete with evidence.

`tokens.css` defines the light and dark palettes, six graph cluster colors, orange focus ring, 4px spacing scale, system font stack, typography scale, radii, and shadows. Dark mode follows `prefers-color-scheme` and removes decorative shadows while maintaining semantic contrast.

The cascade order is tokens, base, elements, collections, views, and graph. Reusable application patterns use the `rw-` prefix. Existing `ui` classes are local Semantic-UI-inspired primitives, not proof that a runtime Semantic UI dependency exists.

Status treatments combine text and semantic color. Success covers completed, valid, available, matched, and cache hit states; warning covers skipped, stale, negative, unclear, unresolved, and not available states where appropriate; danger covers failed, discarded, invalid, rejected, trashed, and purge states; neutral/information covers other recorded states.

## 16. Responsive behavior

Desktop is the primary dense-analysis layout, with multi-column summaries, side-by-side graph filters/network, and wide tables in horizontal overflow containers. Tablet collapses selected grids without removing data or controls.

Below 720px, Deepdive navigation is controlled by the Menu button, the context selectors and Home summaries stack, toolbars wrap, graph and detail panels use the available width, and tables remain horizontally scrollable. The mobile control must update `aria-expanded` and close after navigation.

Responsive changes must preserve context labels, status meaning, pagination, disclosures, and action targets. Hiding nonessential decoration is acceptable; hiding evidence or replacing controls with pointer-only interactions is not.

## 17. Accessibility

The document includes a Skip to content link, landmark header/navigation/main regions, logical headings, explicit form labels, status live regions, and visible keyboard focus. Primary navigation uses `aria-current`; disclosure controls use `aria-expanded`; context loading and errors are announced without forcing focus. A generated page-file load focuses the rendered page title after the busy region becomes interactive, while a direct legacy root load retains normal top-of-document focus and the Skip to content entry path.

All actions must be operable by keyboard and have discernible text or an accessible name. Interactive rows cannot be the only way to expand a record. Focus order follows visual order, and disabled selectors or buttons communicate why through nearby text.

Canvas is supplemental. Graph filters, summaries, legend, selection panel, and relationship table expose equivalent information. Screenshots or decorative images require appropriate empty alt text; meaningful images require descriptive alt text.

Light and dark themes must retain readable contrast for text, borders, focus, status chips, graph shapes, and selected rows. Color cannot be the sole indicator of entity type, outcome, selection, or truncation.

Reduced-motion preferences must avoid unnecessary smooth transitions or animated effects. The force layout may compute positions, but interaction feedback and essential state changes cannot depend on motion.

## 18. Loading, errors, and resilience

Global view fetches show the shared loading region and clear prior errors. Cross-view loads repeat shell health, hierarchy hydration, run-context reconciliation, and view requests; same-view history renders retain the abortable request lifecycle without reloading the shell. The API helper requires JSON, extracts the standard error message when present, and reports invalid JSON or request failures in plain language. Aborted stale renders never overwrite a newer view.

Local progressive actions such as loading older audit events, inspecting artifacts, copying preview text, or running graph controls keep existing content visible and report failures near the action. A failed preview always leaves original download available when the server permits it.

Missing optional columns or metrics must degrade to Not recorded, unavailable panels, or omitted optional controls. A missing required run or table produces an explicit empty state, not a blank application container.

## 19. Privacy and security presentation

The header always displays Local review. Copy explains that pipeline evidence is immutable, review history is append-only and local, and context heads are the only review state moved by saves. The UI must not claim authentication, authorization, encryption, automatic PDF acquisition, live provider refresh, arbitrary corpus mutation, or PDF-store mutation.

Artifact and PDF downloads are direct local API links. Preview content is escaped before insertion, JSON is formatted only after parsing, dynamic labels and table cells use the shared escaping helper, and no provider payload is interpreted as HTML.

The interface must not render credentials, tokens, private keys, raw environment values, or newly exposed sensitive fields. Review version attribution exposes only the optional reviewer username; reviewer email remains local run metadata and is not returned in version responses or duplicated in audit data. Empty or sanitized identity appears as `Anonymous or redacted`. New table/detail surfaces require an explicit review of whether values are safe for a local viewer and whether preview, mutation, and download behavior remains bounded.

## 20. Frontend module ownership

| Path | Design responsibility |
|---|---|
| `index.html` | Authoritative accessible shell template for every generated page document. |
| `scripts/build.ts` | Compiled-source assembly, per-view page generation and markers, and served-root validation. |
| `scripts/generate-classes.ts` and `scripts/check-classes.ts` | Generated CSS token registry, non-mutating freshness verification, non-JSX class validation, and defined-without-static-use reporting. |
| `app.tsx` | Global event binding, same-view history interception, protected native cross-view navigation, shell initialization, and first render. |
| `jsx/classes.ts` | Committed generated union of class tokens defined by the six authoritative stylesheets; never edit it directly. |
| `jsx/jsx-runtime.ts` | The project-owned classic-mode JSX runtime: `h`, `Fragment`, typed class composition and DOM class helpers, `render`, `renderToString`, and the controlled `raw` escape hatch. Authored contract in [JSX-RUNTIME.md](JSX-RUNTIME.md). |
| `jsx/jsx.d.ts` | The ambient global `JSX` namespace (`Element = Node`, class names narrowed to generated tokens or branded combinations, and otherwise permissive intrinsic attributes). |
| `state.tsx` | URL values, view-to-page ownership, DOM state, escaping, formatting, shared JSX panels/tables/flows/labels, links, statuses, and global UI behavior. It imports only the leaf JSX runtime. |
| `api.tsx` | Abort-aware JSON reads and mutations, endpoint construction, structured error extraction, and table discovery cache. |
| `router.tsx` | Request sequencing, abort lifecycle, selector hydration, view dispatch, native cross-view navigation, same-view history state, primary-nav state, document title, and focus. |
| `components/context-selector.tsx` | Dependent bounded searchable single-select hydration, loading skeletons, keyboard listbox interaction, exact selected options, and sole-child auto-selection. |
| `components/data-table.tsx` | Shared rows, sorting, search, page size, expansion, and control binding. |
| `components/pagination.tsx` | First/Previous/numbered/Next/Last controls and result ranges. |
| `components/audit-events.tsx` | Audit classification, summaries, metadata disclosures, timeline markup, and optional investigation export. |
| `components/graph.tsx` | Graph query, connected components, canvas lifecycle, simulation, drawing, interactions, export, selection, and relationship table. |
| `components/shell.tsx` | Health state and responsive primary-navigation toggle. |
| `components/note-parser.tsx` | Bounded note parsing, diagnostics, safe preview, unresolved labels, and resolved context-preserving links. |
| `components/note-editor.tsx` | Draft lifecycle, active versions, tombstones, restoration, history, and bounded comparison. |
| `components/pdf-viewer.tsx` | PDF.js worker and page lifecycle, single-page rendering, selectable text, geometry projection, boundary-aware controls, and highlights. |
| `components/review-panel.tsx` | Explicit lineage initialization, complete status state, conflicts, history, notes, PDF integration, and accessible anchors. |
| `views/home.tsx` | Context-independent history metrics, Explore links, and reversible run-visibility dialog behavior. |
| `views/*.tsx` | Page-level fetching, rendering, and post-render event binding for the Deepdive destinations and detail routes in section 4. |
| `styles/tokens.css` | Theme values, spacing, type, status, graph colors, focus, radius, and elevation. |
| `styles/base.css` | Reset, document layout, typography, links, landmarks, and reduced motion. |
| `styles/elements.css` | Buttons, labels, messages, loaders, headers, and segment primitives. |
| `styles/collections.css` | Navigation, selectors, grids, forms, tables, pagination, breadcrumbs, and responsive collections. |
| `styles/views.css` | Overview, details, provenance, audit, artifacts, evaluation, and stage-specific presentation. |
| `styles/graph.css` | Relationship layout, controls, canvas, overview, legend, selection, and edge table. |
| `vendor/d3-force.js` | Generated pinned force-simulation implementation; never edit it manually. |
| `vendor/pdfjs/` | Generated pinned PDF.js core, exact worker, CMaps, standard fonts, and license assets; never edit them manually. |

Components may import shared state, API, router helpers, the JSX runtime, pagination, and the pinned D3 module as required, but they must not import view modules. Views assign a built JSX tree to a descriptive binding and pass that binding to `render`; reusable components return JSX elements, and behavior helpers bind to caller-owned DOM after render.

## 21. Testing and acceptance

Frontend unit tests use `node:test`, `node:assert`, and jsdom. The suite verifies URL state, API reads and mutations, routing and cleanup, selectors, tables, pagination, graph transformation and interactions, note parser conformance and safe rendering, draft and comparison helpers, PDF geometry projection and one-activation pagination, shell behavior, shared render helpers, and every view module; counts are derived from source rather than maintained here.

The main Playwright suite runs against an isolated fixture copy and verifies context selection, native page navigation, browser history, page markers, URL preservation, focus, table controls, details, graph behavior, provenance, evaluation, error states, responsive layouts, dark/light preferences, landmarks, and interaction semantics. The serial review suite verifies status, note, anchor, custom PDF rendering, and reload persistence without mutating the base fixture. The UI-quality suite adds axe-core checks for both legacy root and generated page URLs plus reviewed screenshots for core views.

Every frontend change must run `make test-frontend-unit`. Changes under `frontend/` must also run `make test-go PACKAGE=./server` and `make test-frontend TEST_FILE=tests/viewer.spec.ts`. Visual or accessibility changes must run `make test-frontend-visual` and review rather than blindly replace snapshots.

Acceptance requires no hard-coded context-dropping internal links, no unbounded new collection, no mutation outside the declared review and run-visibility controls, no new external asset request, no inaccessible graph or highlight-only fact, no raw unescaped provider or note content, no unexplained unavailable state, and no paragraph or list item split across physical Markdown lines in this document.

## 22. Known design limitations

- The viewer has no application authentication or authorization and therefore rejects non-loopback binding; local users and processes with access to the host remain inside the trust boundary.
- Browser note drafts are best-effort local storage, are not part of database history or OSF export, and can be lost through storage clearing, quota, privacy mode, or another browser profile.
- Corpus author and reference lists include relationships from all revision snapshots in a run, so repeated conceptual values are possible and must remain labeled as occurrences or mentions.
- Graph limits can truncate large networks; the endpoint reports this, but the browser does not stream beyond the configured bounds.
- The frontend uses the project-owned JSX runtime in [JSX-RUNTIME.md](JSX-RUNTIME.md). Text and attribute values are escaped automatically; the controlled `raw` escape hatch accepts only trusted, already-escaped markup.
- The JSX runtime has no VDOM reconciliation and no automatic re-render; a view re-renders by building a fresh tree and calling `render`. Application views and components compose Nodes directly. `renderToString` remains a test serializer, and `raw` has no application caller.
- Router and context-selector currently form one ES-module cycle to connect selector navigation with render-time hydration; it is initialized safely, but new modules should not add dependencies to that cycle.
- The URL contains research context and filters, which is useful for reproducibility but can reveal search/run identifiers through copied links or browser history.
- Canonical application links use page-file URLs with a duplicate `view` query parameter, while legacy `/?view=...` URLs remain supported, so one rendered view has two valid URL forms and Home links use `index.html?view=home`.
- A page file without its matching `view` query parameter renders Home because the query parameter remains the dispatch source of truth.
- A cross-view load repeats approximately seven to nine health, hierarchy, reconciliation, and view requests through the server's single read connection; same-view filters and pagination avoid this cost by rendering in place.
- Cross-view navigation discards in-memory graph simulation positions, zoom and pan, and appended audit pages. The URL-backed graph selection, filters, and other route state remain restorable.
- The filesystem server uses `Last-Modified` revalidation without an explicit project cache policy, so rebuilding page or module assets within one timestamp-resolution interval can leave a stale browser response until revalidation observes the new modification time.
- The local fixture and screenshot baselines represent controlled test data and Chromium/Linux rendering; they do not prove visual identity across every platform font and browser.
- Active CSS aliases remain part of current source behavior and can change only with a verified, scoped update to styles, markup, JavaScript, tests, and [CSS-REFERENCE.md](CSS-REFERENCE.md).

## 23. Change checklist

- Confirm evidence reads or narrowly scoped immutable review writes exist in the server contract before designing a control.
- Preserve search, revision, plan, run, note, anchor, and PDF-page context through `link()` and clear only invalid descendant state.
- Keep server-backed collections bounded, sortable only by allowlisted fields, and explicit about truncation or unavailable evidence.
- Escape every dynamic value, keep artifact and PDF handling download-safe and preview-bounded, and render note links only through the project parser and resolver.
- Provide keyboard behavior, visible focus, labels, status text, and a non-visual equivalent for graphical information.
- Place reusable logic in the established state/component boundary and avoid view-to-view imports or new framework dependencies.
- Add or update unit, server integration, Playwright, accessibility, and visual tests in proportion to the changed behavior.
- Review light, dark, desktop, tablet, and mobile behavior and retain the permanent Local review presentation plus immutable pipeline-evidence explanation.
- Update this document and [ARCHITECTURE.md](ARCHITECTURE.md) when the implemented contract or module responsibilities change.
