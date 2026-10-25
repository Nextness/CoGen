# Project Standards

## 1. Purpose

This document defines the normative standards for repository changes. [ARCHITECTURE.md](ARCHITECTURE.md) explains the implemented system, [DATABASE.md](DATABASE.md) describes the current SQLite schema and persistence expectations, [PROJECT-USAGE.md](PROJECT-USAGE.md) gives commands, [DESIGN.md](DESIGN.md) defines frontend behavior, [CSS-REFERENCE.md](CSS-REFERENCE.md) lists active style conventions, and [PROJECT_CATALOG.md](PROJECT_CATALOG.md) maps declarations to source.

## 2. General change discipline

- Implement the smallest complete change that satisfies a concrete requirement and preserve unrelated behavior and user work.
- Inspect the current worktree, authoritative source, public interfaces, generated artifacts, and relevant tests before editing.
- Follow existing package and module boundaries unless repository evidence proves that a boundary causes the requested defect.
- Do not add speculative abstractions, dependencies, configuration, compatibility paths, feature flags, files, or cleanup.
- Modify generated output through its authoritative source and documented generator, then review the generated diff.
- Preserve public APIs, serialized formats, persisted data, and runtime defaults unless an approved requirement changes them.
- Handle invalid input and failures explicitly, use established error propagation, and never weaken security controls to pass verification.
- Review every changed file and the final diff for accidental formatting, debug output, secrets, temporary files, and unrelated changes.

## 3. Go standards

The module targets Go 1.25.0. Use standard Go formatting through `gofmt`, keep package names short and lowercase, use idiomatic mixedCaps identifiers, and keep exported symbols to the minimum required by current consumers.

- Keep application process wiring in `backend/cmd/analysis/` and importable command behavior in `backend/cli/`; maintained tool process entrypoints live under `tooling/go/cmd/` and their implementations under `tooling/go/`.
- Preserve package ownership described in [ARCHITECTURE.md](ARCHITECTURE.md); do not introduce reverse dependencies from foundations into orchestration.
- Prefer explicit control flow and ownership over hidden global state or unnecessary mutation.
- Wrap errors with useful operation context while preserving errors needed by `errors.Is` or `errors.As` callers.
- Do not swallow errors, log sensitive values, or duplicate nullable, JSON, SQL, hashing, or validation helpers already owned by a package.
- Close resources at the owning layer, propagate context cancellation, and keep goroutine shutdown and channel ownership deterministic.
- Use the standard library or an existing dependency when adequate. `modernc.org/sqlite` and the retained `github.com/pdfcpu/pdfcpu v0.15.0` dependency are the direct production Go dependencies; the pinned sqlc generator is a development tool and never enters the module graph. The pdfcpu dependency is Apache 2.0 licensed and remains isolated behind `backend/pdfannotate/`.
- Run `gofmt` on changed Go files and use `make format-check` before completion.

## 4. Source documentation comments

Go declaration comments use standard attached Go documentation comments and begin with the declared symbol name. JavaScript declarations use adjacent JSDoc. Signatures remain authoritative for syntactic inputs and outputs; comments describe intent, returned meaning, invariants, side effects, ownership, failure behavior, security requirements, or concurrency constraints without restating syntax.

The catalog copies attached comments exactly after whitespace normalization and writes `No source description` when a maintained declaration lacks a comment. It must not infer behavior from a name, and `make check-docs` rejects any maintained declaration with that fallback. Add or update the source comment, then regenerate the catalog instead of editing a generated description.

For JavaScript, use `@param` and `@returns` when JavaScript syntax alone cannot express meaningful input or returned-value behavior. Test titles are the descriptions for anonymous `test` and `it` callbacks. Do not add artificial comments to anonymous implementation closures or custom annotations that duplicate declaration names.

## 5. Go tests

Go tests use `testing` in external packages under `tests/<domain>/<component>/{unit,functional,integration}/`. Ordinary layer selection uses directories, without build tags. Production roots contain no tests, test-only fixtures, hooks, or imports from `tests/`; shared test construction belongs under `tests/support/`. Preserve private behavior assertions through public operations or cohesive production components used by runtime callers; do not add blanket exports, forwarding bridges, source copies, or compilation overlays. Offline E2E requires `e2e`; live E2E additionally requires `e2e_live` and the runtime opt-in, so both stay outside ordinary, race, and coverage selection.

- Unit tests isolate deterministic package behavior without database files, network access, or process-wide services.
- Functional tests exercise a complete language or component behavior across internal phases without external infrastructure.
- Integration tests use real temporary SQLite files, production migrations, HTTP handlers, controlled processes, or boundaries whose behavior depends on integration.
- Database integration tests use temporary directories and production migration configuration rather than manually approximated schema.
- Provider tests use controlled mock HTTP servers and never call live Crossref, OpenAlex, ORCID, or credentialed services, except for the bounded public-provider target `make test-e2e-live E2E_LIVE=1`, which must remain explicitly enabled and structurally asserted.
- Tagged E2E tests invoke the Makefile-built `build/analysis`, `build/pdf-store`, and `build/something-printer`, write only beneath `build/e2e/`, keep deterministic and mocked variants offline, and verify persisted pipeline and A1/A2 review database evidence before comparing APIs and browser UI.
- Regression tests assert observable behavior and reproduce the failure before the fix when practical.
- Avoid nondeterministic sleeps, shared external state, implementation-detail assertions, and weakening assertions to accept a failure.
- Use `make test-go DOMAIN=backend COMPONENT=server LAYER=integration TEST='^TestName$'` for focused Go work; `PACKAGE` accepts a canonical `./tests/...` package override. `make test` runs every ordinary domain and layer, including Node and browser suites; invalid or empty selections and unmatched Go `TEST` expressions fail. Use `make test-e2e` for offline cross-layer behavior, and `make test-race` for concurrency, cache, HTTP-client, database, or lifecycle work.

Test counts are derived from source and are not maintained as prose. Use `rg -g '*_test.go' '^func Test' tests | wc -l` when a current count is needed.

## 6. Coverage and static verification

`make coverage` explicitly instruments all production packages under `backend/`, `tooling/go/`, and `internal/`, runs those packages plus all ordinary test packages with atomic coverage, includes uncovered production statements, excludes test support from production metrics, writes `build/coverage/coverage.out`, and uses `tooling/go/coveragecheck/` with [config/coverage_policy.something](../config/coverage_policy.something). Package and file floors are regression signals rather than proof of correctness.

- Do not lower a coverage floor merely to accept a regression.
- Raise a floor only when meaningful tests justify the durable threshold.
- Inspection tools excluded by the policy do not become exempt from their focused unit suites.
- The repository has no separate lint target; supported static checks are `gofmt`, `make format-check`, `make vet` or `make check`, the `make check-sql` raw SQL ownership check and the `make check-imports` import and transaction graph check that `make check` and the CI backend gate run, the `make sqlc-check` generated-output freshness and query-analysis check that the CI backend gate runs and every family SQL input change requires, and the `make gitleaks` secret scan that `make check` and the CI merge gate run.

## 7. SOMETHING standards

[something.spec.md](something.spec.md) is the behavioral language specification. Language changes must preserve the ordered lexer, parser, structural directive-generator, type-checker, and evaluator phases unless an approved design changes the whole contract.

- Internal language-phase failures intentionally panic with `*SomethingError`; `LoadSomethingFile` and `LoadSomethingBytes` remain the recovery boundaries.
- Preserve source locations, source order, declaration visibility, and deterministic iteration keys through every phase.
- Keep `#assert`, `#if`, `#error`, boolean and comparison operators, `#match`, and `#len` through structural generation for later checking and evaluation.
- Use centralized typed accessors and conversion checks instead of scattered `any` assertions.
- Add unit tests for isolated syntax and semantics, functional tests across compiler phases, and integration tests where repository configuration consumes the behavior.
- Update the specification, fixtures, configuration examples, and generated declarations when an approved language contract changes.

## 8. Configuration standards

- Keep SOMETHING configuration names descriptive, lowercase, and consistent with existing setup and iteration forms.
- Define defaults explicitly, validate invalid values, and preserve unambiguous precedence between configuration and command-line overrides.
- Do not add a configuration option for behavior that does not need to vary.
- `config/workspace.something` owns workspace selection and includes `config/baseline.something`; `config/database.something` independently owns migration-chain selection.
- Optional reviewer username and email remain backward-compatible nested defaults, are trimmed and bounded, are captured once for every attempt including failure, and remain excluded from resolved manifests, input manifests, and execution fingerprints.
- Keep provider registration and ordering in code; configuration values customize registered providers but do not create providers.
- Update documentation and tests whenever a supported configuration field, default, or validation rule changes.

## 9. SQLite, SQL, and generated-code standards

- Use `backend/bundle/` preparation for paired schema upgrades, `database.OpenWritable` with `database.New` for the pipeline after preparation, `database.OpenExisting` for existing-only metadata operations, and `server.Open` for the viewer's coordinated metadata read, review-write, PDF evidence-read, and optional PDF write lifecycle.
- Keep foreign keys enabled, retain connection busy timeouts, and preserve `BEGIN IMMEDIATE` migration serialization.
- Use family-local nullable and JSON helpers and parameterized SQL; never construct value-bearing SQL through string concatenation.
- Keep raw SQL calls inside classified ownership locations: platform migrations and connections, the viewer query-budget driver wrapper, dynamic table introspection, sanitized export maintenance, the cross-database outbox, or a retained `*_dynamic.go` reader. Dynamic SQL is an explicit exception rather than a default: prefer finite named query variants, and confine any retained builder to closed SQL fragments, bound values, allowlisted identifiers, and hard limits with injection and ordering tests. Run `make check-sql` after adding or moving a call; an unclassified call or a stale ownership entry fails with the file and the expected remediation, and `make sql-report` prints the current inventory.
- Keep the package dependency direction: family production code depends only on its own subtree and shared internal, logging, manifest, or notes packages, so sibling families cannot call one another inside transactions; the composition root imports families, the companion store coordinates with them, and consumers import both; shared low-level packages never import orchestration. Run `make check-imports` after changing imports; generated `internal/sql` output stays inside its owning package tree, and every sqlc generation unit keeps its schema, queries, and output under one owner directory.
- Keep every maintained family declaration and named query under `backend/database/<family>/schema/` or `backend/pdfstore/schema/`, and keep generated Go under the reserved `internal/sql/` package of the same owner; change the inputs, run `make sqlc-generate` from the committed `config/sqlc.yaml`, and review the generated diff instead of editing generated files.
- Map generated row types to family application types inside the owning package. Generated packages and types stay private to their family tree and never appear in another family's or a consumer's handwritten API.
- Put relationship SQL with the family that owns the returned resource or the mutation. A family may read another family's table through a specifically named relationship query, but cross-table writes and transactions stay in the family owning the use case and never call a sibling family store.
- Create integration, fixture, and E2E databases only through the production migration chains. Family `schema/` declarations are compile-time inputs for typed queries and never feed runtime migrations or SOMETHING versioning.
- Run `make sqlc-check` after changing any family schema declaration or named query; it proves committed generated output is current and every maintained query still passes sqlc analysis. Normal `make build`, `make tools`, `make test`, and `make check` consume committed generated output and never invoke sqlc.
- Validate dynamic identifiers against discovered or explicit allowlists before quoting them.
- Keep prerequisite checks before relationship inserts because foreign-key violations are not neutralized by `INSERT OR IGNORE`.
- Preserve immutable work revisions, authorships, reference mentions, review contexts and versions, append-only audit, and content-addressed artifact invariants. Only context head tables may move, and they move through repository transactions with expected-version compare-and-swap. Review audit metadata remains identifier-only; decision events deliberately record the complete bounded previous and new decision in the audit before/after fields.
- Context creation requires a completed non-trashed run, an earlier acyclic parent when supplied, and stable-work head materialization. Parent heads are frozen at creation and never propagate later changes.
- Review-decision and Note mutations require a completed active run, initialized review context, and exact work membership. PDF anchor creation or restoration additionally requires the matching available PDF bytes. Note bodies, selected PDF text, reviewer email, and browser drafts must not enter audit metadata or generic before/after state. A review decision's status, optional reason, and all sub-statuses are deliberate bounded before/after audit evidence.
- Bound collection, graph, preview, table-browser, review history, note, anchor, backlink, and candidate queries even though the viewer runs locally.

## 10. Migration standards

Metadata migrations live in `migrations/corpus.metadata/`, PDF migrations live in `migrations/corpus.pdf/`, and each new file is appended to the matching chain referenced by `config/database.something`.

- Use the next contiguous `VNNNNN_description.sql` filename and include both `-- ==UP==` and `-- ==DOWN==` sections.
- Never rewrite an applied migration.
- Treat SOMETHING iteration order and filename as execution identity; `previous` and `upgrade` are descriptive rather than ordering inputs.
- Use `supersedes` only when a migration was renamed after equivalent SQL was applied under the earlier filename; retain the historical row, record the canonical filename transactionally without rerunning SQL, and test both fresh and legacy upgrade paths.
- Account for existing records, nullability, defaults, indexes, constraints, transaction behavior, runtime cost, and application versions that may overlap during deployment.
- Coordinate metadata and PDF upgrades through `backend/bundle/` preparation, which validates both configured chains and migration inputs before changing either store, applies metadata then PDF, verifies both targets, and requires exclusive maintenance ownership; retain committed progress and resume forward instead of downgrading.
- Preserve data and transactional integrity; destructive or irreversible transformations require explicit approval and documented rollout safeguards.
- Run `make check-docs` so configured filenames, present SQL files, contiguous versions, and documented boundaries remain consistent.

## 11. Enrichment and cache standards

- Keep provider HTTP and decoding in `backend/enrich/` and database persistence and provenance in workspace or repository layers.
- Use `enrich.Client.Fetch` or `FetchAll` so every request follows shared rate limiting, timeout, retry, and cancellation behavior.
- Honor each provider's `fill_missing_only` setting and keep Crossref, OpenAlex, and ORCID ordering consistent with registered orchestration.
- Validate successful payload envelopes before storing or using them, store deliberate negative results only with policy TTL, and continue the read order after stale data.
- Record cache hit, miss, negative, stale, invalid-payload, and network-fetch evidence at run and provider scope.
- Treat ORCID name-search matches as uncertain candidates and never assign confirmed identity without confirmed evidence.

## 12. PDF and audit standards

- Register normalized DOIs as `not_available`; only the validated manual tool stores bytes and changes a DOI to `available`.
- Enforce the 20,000,000-byte limit, `%PDF-` signature, SHA-256 content identity, prior normalized registration, and unchanged selection for already available documents.
- Keep metadata and companion paths bundle-relative and reject absolute or escaping bindings.
- Write PDF audit evidence through the transactional outbox and preserve idempotent cross-database delivery.
- Verify the companion V00003 document-management columns, content-hash index, outbox evidence columns, and protection triggers before enabling writes; a current-schema companion without write permission may still serve evidence with an explicit non-writable capability.
- Do not add automatic acquisition, replacement, deletion, or filesystem compatibility without an approved requirement.
- Keep review context, note, link, anchor, geometry, and reviewer data in metadata. The viewer may read PDF availability and bytes but never writes the companion for review behavior.
- Bind anchor geometry to an exact work revision and PDF content hash, validate one-page finite normalized rectangles, and refuse to project geometry when the current content hash differs.
- `prepare-osf` creates a new temporary sibling bundle and atomically renames it only after WAL-safe snapshots, reviewer redaction, syntax-aware configuration and raw-artifact replacement, corpus-ID regeneration, foreign-key and artifact validation, and manifest hashing succeed. It never overwrites output or mutates source databases, source configuration, or browser drafts.
- OSF configuration traversal remains inside the selected configuration root including symbolic-link resolution, fails closed for unprovable reviewer sources, and does not search and replace arbitrary note, provider, or research text.

## 13. HTTP and security standards

- Evidence routes remain GET or HEAD. Review POST and PUT routes are explicitly allowlisted and require JSON content type, bounded bodies, unknown-field and trailing-value rejection, allowed query keys, same-origin validation when `Origin` is present, no-store responses, and stable structured errors.
- Return stable JSON errors without exposing SQL, credentials, tokens, private keys, raw environment values, or sensitive research content.
- Escape all dynamic HTML through shared helpers and do not interpret provider or artifact content as markup.
- Preserve `Content-Disposition`, `Cache-Control`, content-type validation, and `X-Content-Type-Options` behavior for downloads and previews.
- Require an exact loopback IP listener, reject a Host authority different from the bound listener, and do not describe the viewer as authenticated or safe for untrusted network exposure.
- Keep graph information available through textual summaries and relationship tables rather than relying on canvas alone.

## 14. Frontend module standards

Frontend production source is TypeScript with native ES modules and no application framework, compiled per file by `make frontend-build` into the assembled `build/frontend/dist` served root. The build reads the single `frontend/index.html` shell template and generates one identified HTML document per Home, Deepdive, and detail view. Relative imports use explicit `.ts` or `.tsx` specifiers, type-only imports use `import type` (`verbatimModuleSyntax` and `erasableSyntaxOnly` are enforced by `tsc --noEmit`), and new exports carry required type annotations plus the maintained adjacent JSDoc prose description. Rendering uses the project-owned classic-mode JSX runtime; [JSX-RUNTIME.md](JSX-RUNTIME.md) is the authoritative contract for its semantics and must be updated whenever the runtime's attribute/children handling, escaping, event binding, or wiring change. `state.tsx` owns URL, view-to-page mapping, and shared rendering state, `api.tsx` owns JSON transport, `router.tsx` owns dispatch, cross-view navigation, same-view history, and abort lifecycle, views fetch page data, assign the JSX tree to a descriptive binding, and pass that binding to `render`, while components own reusable JSX elements or behavior. The generated `frontend/src/jsx/classes.ts` registry is derived from the six authoritative stylesheets and is checked, together with non-JSX class uses, before `make check-frontend` runs `tsc --noEmit`.

- Code style and identifier naming are normative in [FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md); read it before changing frontend source.
- Add a CSS selector in its owning stylesheet before regenerating `frontend/src/jsx/classes.ts` with `make frontend-classes`; use `make frontend-classes-check` as the non-mutating freshness and static-use proof, and never edit the registry directly.
- Every internal link uses `link()`, targets the clean per-view path for its destination, and preserves `search_id`, `search_revision_id`, `plan_id`, `run_id`, focused `note_id`, focused `anchor_id`, and `pdf_page` unless a parent or target change invalidates descendant context. Cross-view transitions use native document navigation with the destination state persisted first; same-view filters, pages, cursors, modes, and sections use history state plus an in-document render. Anchors carrying state beyond the view path must render `data-state` JSON.
- Bind DOM listeners after rendering a tree into the app or a sub-container, abort stale requests, and prevent older renders from overwriting current state.
- Components do not import views; new code does not expand the intentional router and context-selector cycle.
- Rely on the JSX runtime's automatic escaping for text and attributes, compose application markup as JSX Nodes, format machine data through shared helpers, and provide explicit empty, loading, error, unavailable, and truncation states. Application views and components do not use the `raw` or `renderToString` compatibility helpers.
- API response contracts live in `frontend/src/api/types.ts` and are enforced at the fetch boundary through `api<T>` and `mutate<T>`; every nullable field is `| null` and every conditionally-present field is `?`. The serial `api-shapes.spec.ts` fetches every consumed endpoint from the fixture server and validates the payloads against those interfaces, so server drift in nullability or field names fails the browser suite.
- Keep server-backed collections bounded and URL-addressable where reload or sharing matters.
- The Go binary contains no frontend assets. `serve` requires `--assets-dir` (normally the assembled `build/frontend/dist` produced by `make frontend-build`), and serving makes no CDN request.
- Change `vendor/d3-force.js` only through its dependency and `make frontend-vendor`, then review the generated diff.
- Change `vendor/pdfjs/` only through the exact `pdfjs-dist` dependency and `make frontend-pdfjs-vendor`; run `make frontend-pdfjs-vendor-check` to verify matching core and worker versions plus deterministic CMap, font, and license output.
- The custom PDF viewer renders exactly one bounded current page and selectable text, advances one page per Previous or Next activation, cancels stale render work, destroys render, document, loading, and worker lifecycles on route navigation, and treats highlights as supplemental to keyboard-operable textual anchor evidence.
- Review notes render only through the project parser and resolver, escape raw HTML, label unresolved targets with text and an accessible name, and keep conflicted or unsaved drafts under the corpus-scoped browser key.

## 15. Frontend tests

Frontend unit and functional tests live under `tests/frontend/<component>/{unit,functional}/`, are authored in TypeScript, use Node's built-in `node:test` and `node:assert` with jsdom, and import `setup.ts` as a side effect when DOM shims are required.

- Unit tests cover state, rendering helpers, URL behavior, API behavior, components, routing, and views without a browser or server.
- Browser tests use `*.spec.ts` under `tests/frontend/` and follow navigate, assert, interact, assert URL, assert content.
- Playwright targets split each invocation into a read-only suite and a serial mutation suite, copy the ignored generated fixture metadata and PDF pair separately for each browser project and mutation spec, start an isolated viewer on an operating-system-assigned loopback port, and isolate output under `build/playwright/`. No browser test writes the base fixture.
- `viewer.spec.ts` covers evidence browsing; serial `review.spec.ts` covers review mutations and PDF rendering; serial `pdf-management.spec.ts` covers visible PDF upload, replacement, removal, single download, selected-set ZIP download and manifest inspection, failure, stale-tab, and draft-preservation behavior; serial `api-shapes.spec.ts` validates every consumed endpoint payload against the typed contracts; `ui-quality.spec.ts` covers accessibility, responsive behavior, and reviewed screenshots; `e2e.spec.ts` is guarded by `E2E_SPEC=1`, uses only a target-owned mutation database, and is run through `make test-e2e` before a Go database-evidence verifier.
- Run `make test-frontend-unit` for frontend logic, `make test-go DOMAIN=backend COMPONENT=server` for served frontend or API integration, and the focused Playwright suite for browser behavior.
- Run `make test-frontend-visual` for visual or accessibility changes and review snapshots rather than replacing them blindly.

## 16. CSS standards

Styles load in cascade order `tokens.css`, `base.css`, `elements.css`, `collections.css`, `views.css`, and `graph.css`. The active selector and token reference is [CSS-REFERENCE.md](CSS-REFERENCE.md).

- Use custom properties from `tokens.css` for color, spacing, typography, focus, radii, elevation, and graph clusters.
- Use `ui` classes for local Semantic-UI-inspired primitives and `rw-` with BEM-style suffixes for research-viewer-specific components.
- Place reset and shell rules in base, primitives in elements, shared layout and collection rules in collections, page-specific rules in views, and relationship explorer rules in graph.
- Preserve light and dark contrast, visible focus, reduced-motion behavior, keyboard targets, responsive layout, and textual status meaning.
- Do not hide evidence on smaller screens; use stacking, wrapping, and horizontal table overflow.
- Do not retain or document a selector merely for historical comparison; describe only selectors and aliases active in current source.

## 17. Accessibility and design standards

[DESIGN.md](DESIGN.md) is the implemented product contract. All views retain logical headings, landmarks, labels, visible focus, keyboard operation, status announcements, accessible disclosure state, and non-color meaning.

- Canvas and visual summaries are supplemental to equivalent text or table content.
- Interactive rows cannot be the only way to trigger an action and must not steal behavior from links, controls, selection, or text interaction.
- Mobile behavior preserves context, evidence, pagination, status, and actions.
- Loading keeps context understandable, errors identify the failed operation, and unavailable data is distinguished from empty data.
- Privacy text must accurately describe immutable pipeline evidence, local append-only review behavior, mandatory loopback access, browser-draft limitations, and read-only PDF ownership without implying authentication, arbitrary mutation, live refresh, or automatic PDF acquisition.

## 18. Documentation standards

- Keep every prose paragraph, bullet, numbered item, and table row on one physical Markdown line; fenced code and ASCII diagrams may span lines.
- Use repository-local Markdown links for navigable document and source references.
- Describe only current supported state; use Git history for superseded designs, completed phases, renamed concepts, and deprecated workflows.
- Update [ARCHITECTURE.md](ARCHITECTURE.md) for system behavior, boundaries, state, configuration, routes, decisions, assumptions, or current issues.
- Update [DESIGN.md](DESIGN.md) for frontend behavior, interaction, visual, responsive, accessibility, or content contracts.
- Update [FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) when frontend naming, control-flow, or layout conventions change.
- Run `make docs-catalog-update` after maintained Go or JavaScript declarations move or change, then review [PROJECT_CATALOG.md](PROJECT_CATALOG.md).
- Run `make docs-state-update` only after every affected document and dependency listed in [DOC-STATE.md](DOC-STATE.md) has been reviewed.
- Run `make check-docs` after Markdown, migration configuration, migration filenames, documentation-tool code, or generated documentation changes.

## 19. Verification selection

Use the narrowest relevant test first, then the affected package or frontend layer, then broader required checks. Documentation and tool changes run `make test-docs`, catalog and state updates, `make check-docs`, and `make check`; family SQL input changes additionally run `make sqlc-generate` with a reviewed diff and `make sqlc-check`; production changes add package, integration, race, coverage, frontend, or build verification in proportion to risk.
