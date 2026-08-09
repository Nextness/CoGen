# CSS Reference

## 1. Purpose

This document describes the active stylesheet layers, tokens, naming conventions, selector families, theme behavior, responsive rules, accessibility treatments, and dual-selector support in `src/server/frontend/`. Use [DESIGN.md](DESIGN.md) for product intent, [ARCHITECTURE.md](ARCHITECTURE.md) for module ownership, and [STANDARDS.md](STANDARDS.md) for frontend change rules.

The CSS source is authoritative. This reference describes current selectors without preserving previous naming stages or removal history.

## 2. Stylesheet loading order

`index.html` loads six independent stylesheets in this order. The cascade is flat and uses no preprocessor, CSS module system, or runtime style framework.

| Order | File | Ownership |
|---:|---|---|
| 1 | `styles/tokens.css` | Theme values, spacing, typography, focus, radii, shadows, graph clusters, and active token aliases. |
| 2 | `styles/base.css` | Reset, document layout, typography, forms, focus, skip link, header, and primary shell navigation. |
| 3 | `styles/elements.css` | Buttons, labels, messages, headers, loaders, segments, faded text, and active elemental aliases. |
| 4 | `styles/collections.css` | Grids, menus, context selectors, breadcrumbs, tables, pagination, forms, controls, responsive collections, and active layout aliases. |
| 5 | `styles/views.css` | Metrics, progress, retention flows, details, empty states, audit, artifacts, cache, stages, and active view aliases. |
| 6 | `styles/graph.css` | Relationship filters, viewport, canvas, cluster summary, legend, selection, edge table, fullscreen behavior, and active canvas alias. |

New rules go into the narrowest owning layer. Do not use import order to create hidden cross-layer overrides when an existing component selector or token expresses the relationship directly.

## 3. Naming conventions

- `ui` selectors implement local Semantic-UI-inspired primitives such as `.ui.button`, `.ui.label`, `.ui.message`, `.ui.segment`, `.ui.grid`, `.ui.menu`, `.ui.table`, and `.ui.progress`; no runtime Semantic UI package is loaded.
- `rw-` selectors identify research-viewer-specific shell, state, component, and view patterns.
- Research-viewer components use BEM-style elements and modifiers such as `.rw-graph__toolbar`, `.rw-audit-event__marker`, `.rw-nav__item--active`, and `.rw-status-dot--unavailable`.
- Plain element selectors cover universal behavior such as `button`, `input`, `select`, links, tables, code, details, and headings where component markup does not need another class.
- IDs are reserved for stable shell and state regions such as `#app`, `#loading`, `#notice`, and selector controls; reusable styles use classes.
- New code uses the owning `ui` or `rw-` family. Active alternate selectors remain current behavior until source markup, JavaScript, tests, and styles are changed together in an approved scope.

## 4. Core theme tokens

| Category | Tokens | Meaning |
|---|---|---|
| Canvas and surfaces | `--color-bg-canvas`, `--color-bg-surface`, `--color-bg-surface-raised`, `--color-bg-surface-muted` | Page background, panels, raised content, and muted controls. |
| Text | `--color-text-primary`, `--color-text-secondary`, `--color-text-muted` | Main content, supporting content, and lower-emphasis labels. |
| Borders | `--color-border-default`, `--color-border-strong` | Structural and interactive boundaries. |
| Accent | `--color-accent-default`, `--color-accent-hover`, `--color-accent-active`, `--color-accent-soft`, `--color-accent-contrast` | Links, selected state, primary actions, their interaction states, and contrasting text. |
| Success | `--color-status-success`, `--color-status-success-soft` | Completed, valid, available, matched, and hit outcomes. |
| Warning | `--color-status-warning`, `--color-status-warning-soft` | Skipped, stale, unclear, unresolved, negative, and not-available outcomes. |
| Danger | `--color-status-danger`, `--color-status-danger-soft` | Failed, discarded, invalid, rejected, and destructive lifecycle evidence. |
| Information | `--color-status-info`, `--color-status-info-soft` | Neutral recorded context and information messages. |
| Neutral | `--color-status-neutral`, `--color-status-neutral-soft` | Unknown, not recorded, read-only, and inactive state. |
| Review | `--color-status-review`, `--color-status-review-soft` | Human review and inherited review lineage. |
| Note | `--color-status-note`, `--color-status-note-soft` | Note content and note-specific classification. |
| Graph clusters | `--graph-cluster-1` through `--graph-cluster-6` | Connected-component colors; entity type remains shape-encoded. |
| Focus | `--focus-ring`, `--focus-ring-offset` | Visible keyboard focus outline and offset. |
| Elevation | `--shadow-low`, `--shadow-medium`, `--shadow-high` | Panel and overlay elevation. |
| Radius | `--radius-small`, `--radius-default`, `--radius-large`, `--radius-pill` | Controls, panels, and fully rounded status elements. |

### 4.1 Complete palette values

| Role | Light | Dark | Purpose and use |
|---|---|---|---|
| Canvas | `#f5f6f7` | `#111a20` | Application page background. |
| Surface | `#ffffff` | `#17232b` | Parent panels, tables, and default controls. |
| Raised surface | `#fbfcfc` | `#1b2932` | Child cards, KPI cards, timeline events, and contained filters. |
| Muted surface | `#eef1f3` | `#23333e` | Table headings, disabled controls, selected support areas, and secondary groups. |
| Primary text | `#18232c` | `#e7eff4` | Headings, values, and main body copy. |
| Secondary text | `#5c6974` | `#b6c3cc` | Supporting copy, labels, and metadata. |
| Muted text and neutral status | `#5f6c76` | `#91a3b0` for text and `#a4b4be` for status | Lower-emphasis text, inactive state, unknown state, and not-recorded state. |
| Default border | `#d2d9de` | `#3c4f5b` | Structural boundaries and table rules. |
| Strong border | `#adb9c2` | `#617380` | Interactive boundaries and emphasized separators. |
| Accent default | `#0b5e8e` | `#73c1ed` | Links, primary actions, navigation, and selected state. |
| Accent hover | `#084b72` | `#9bd4f4` | Hovered links and interactive emphasis. |
| Accent active | `#063c5c` | `#b9e2f8` | Pressed links and active actions. |
| Accent soft | `#e5f1f7` | `#1a3c4e` | Selected and hovered supporting surfaces. |
| Accent contrast | `#ffffff` | `#10212a` | Text on solid accent controls. |
| Success | `#2f6f52` on `#e8f3ed` | `#7cc99f` on `#1a3a2a` | Completed, valid, available, matched, and resolved. |
| Warning | `#8f5f00` on `#fff3d9` | `#efbd65` on `#3a2e1a` | Incomplete, stale, skipped, unavailable, not evaluated, and unresolved. |
| Danger | `#a22f3f` on `#fbeaea` | `#fa9ca7` on `#3a1a1f` | Failed, invalid, rejected, discarded, and destructive. |
| Information | `#365f91` on `#e9f1fa` | `#7cb8e8` on `#1a2a3a` | Recorded explanatory facts and neutral action feedback. |
| Neutral status surface | `#eef1f3` | `#25343e` | Unknown, not recorded, read-only, and inactive labels. |
| Review | `#7753a6` on `#f1ebf8` | `#c3a5ee` on `#332744` | Human review, inherited evidence, and review timeline markers. |
| Note | `#a0446f` on `#faeaf2` | `#efa6ca` on `#402332` | Note classification and PDF provenance markers. |
| Focus | `#d36d00` | `#ffc066` | Keyboard focus ring; it is intentionally distinct from selection blue. |
| Graph cluster 1 | `#236b8e` | `#68b7df` | First connected component only. |
| Graph cluster 2 | `#8b6417` | `#e4b85d` | Second connected component only. |
| Graph cluster 3 | `#35765b` | `#78c69d` | Third connected component only. |
| Graph cluster 4 | `#6e58a3` | `#b5a0ec` | Fourth connected component only. |
| Graph cluster 5 | `#985468` | `#e99caf` | Fifth connected component only. |
| Graph cluster 6 | `#3b7c83` | `#75c4ca` | Sixth connected component only. |

The only media-specific color exceptions are white PDF paper, a 30% blue native text-selection overlay, a 30% gold saved-anchor overlay with an accent boundary, and the 72% near-black review-dialog backdrop. Light-theme shadows use near-black at 8%, 10%, and 12%; dark mode removes decorative shadows. The graph renderer repeats the token values as defensive JavaScript fallbacks so a canvas remains legible if computed custom properties are unavailable.

## 5. Spacing and typography tokens

| Category | Tokens | Values and use |
|---|---|---|
| Spacing | `--space-1`, `--space-2`, `--space-3`, `--space-4`, `--space-5`, `--space-6`, `--space-8`, `--space-10`, `--space-12` | A 4px base scale from 4px through 48px. |
| Text sizes | `--text-xxs`, `--text-xs`, `--text-sm`, `--text-base`, `--text-lg`, `--text-md`, `--text-xl`, `--text-2xl`, `--text-3xl` | Labels and metadata through page headings. |
| Line heights | `--leading-tight`, `--leading-heading`, `--leading-body` | Product identity, headings, and readable body copy. |
| Font weights | `--weight-normal`, `--weight-medium`, `--weight-bold`, `--weight-black` | Normal content, controls, labels, and strongest headings. |
| Breakpoint references | `--bp-mobile`, `--bp-tablet` | Documented values 720px and 1100px; media queries use literal values because custom properties do not participate in media conditions. |

The font stack is system UI for normal content and the system monospace stack for code, identifiers, hashes, URLs, JSON, and machine values.

## 6. Active token aliases

The following custom properties are active aliases in `tokens.css`. New rules use the descriptive `--color-*`, `--focus-ring`, `--shadow-*`, and `--radius-*` tokens, while the aliases remain part of current source behavior.

| Active alias | Resolves to |
|---|---|
| `--canvas` | `--color-bg-canvas` |
| `--surface` | `--color-bg-surface` |
| `--surface-raised` | `--color-bg-surface-raised` |
| `--surface-muted` | `--color-bg-surface-muted` |
| `--text` | `--color-text-primary` |
| `--muted` | `--color-text-secondary` |
| `--faint` | `--color-text-muted` |
| `--border` | `--color-border-default` |
| `--border-strong` | `--color-border-strong` |
| `--accent` | `--color-accent-default` |
| `--accent-soft` | `--color-accent-soft` |
| `--accent-contrast` | `--color-accent-contrast` |
| `--success` | `--color-status-success` |
| `--warning` | `--color-status-warning` |
| `--danger` | `--color-status-danger` |
| `--focus` | `--focus-ring` |
| `--shadow` | `--shadow-low` |
| `--radius` | `--radius-default` |

## 7. Base and shell selectors

| Family | Principal selectors | Responsibility |
|---|---|---|
| Reset and page | `*`, `html`, `body`, `a`, `::selection`, headings, paragraphs, `code`, `pre` | Box sizing, canvas, bounded body width, type hierarchy, link states, selection, paragraph rhythm, and monospace values. |
| Forms | `button`, `input`, `select` | Inherited type and minimum touch target height. |
| Focus | `:focus-visible` | Three-pixel orange focus outline with tokenized offset. |
| Skip link | `.rw-skip-link`, `.skip-link` | Hidden-until-focused navigation to main content. |
| Header | `.rw-header`, `.rw-header__brand`, `.rw-header__eyebrow`, `.rw-header__status` | Product identity, research framing, health, and local-review state. |
| Status | `.rw-status-dot`, `.rw-status-dot--unavailable`, `.rw-label` | Health indicator and persistent textual status label. |
| Navigation | `.rw-nav`, `.rw-nav__item`, `.rw-nav__item--active` | Deepdive navigation and active destination. |

The shell constrains the body to 1680px, prevents page-level horizontal overflow, and lets wide evidence collections manage their own scroll region.

## 8. Element primitives

| Family | Principal selectors | Responsibility |
|---|---|---|
| Buttons | `.ui.button`, `.ui.basic.button`, `.ui.primary.button`, `.ui.danger.button`, `.ui.icon.button`, `.ui.loading.button` | Default, quiet, primary, destructive, icon, selected, disabled, and loading actions. |
| Labels | `.ui.label`, `.ui.green.label`, `.ui.orange.label`, `.ui.red.label`, `.ui.blue.label`, `.ui.grey.label`, `.ui.violet.label`, `.ui.pink.label` | Neutral, status, review, and note classification with selected, link, and disabled states. |
| Messages | `.ui.message`, `.ui.negative.message`, `.ui.error.message`, `.ui.warning.message`, `.ui.info.message`, `.ui.success.message`, `.ui.review.message` | Global and local feedback with a symbol, text, semantic boundary, and soft color. |
| Headers | `.ui.header`, `.ui.header .sub.header`, `.ui.top.attached.header`, `.rw-page-header`, `.rw-eyebrow` | Product, page, panel, section, eyebrow, and supporting-heading hierarchy. |
| Loading | `.ui.loader`, `.ui.active.loader` | Progress state with accessible surrounding status text. |
| Segments | `.ui.segment`, `.ui.attached.segment`, `.rw-panel`, `.rw-panel__header`, `.rw-panel__body` | Bordered parent surfaces with internal structure and no component-owned external spacing. |
| Supporting text | `.ui.faded.text` | Secondary content that retains readable contrast. |

Active elemental alternates include raw `button` states, `.empty`, `.muted`, `.loading`, `.notice`, `#loading`, and hidden attached segments or expansion rows. They are current selectors and must remain covered if their source users remain.

## 9. Collections and layout

| Family | Principal selectors | Responsibility |
|---|---|---|
| Grid | `.ui.grid`, width classes, `.selection-grid`, `.dashboard-grid`, `.span-all` | Flexible and fixed-column layouts with full-width spans. |
| Menus | `.ui.menu`, `.ui.secondary.pointing.menu`, `.ui.tabular.menu`, `.ui.pagination.menu`, `.rw-section-tabs` | Deepdive, URL-backed section, local tab, and pagination navigation. |
| Context | `.context-panel`, `.context-heading`, `.ui.search.selection.dropdown`, `.rw-search-dropdown__trigger`, `.rw-search-dropdown__menu`, `.rw-search-dropdown__query`, `.rw-search-dropdown__options`, `.rw-search-dropdown__option` | Hierarchical searchable single selection for search, revision, plan, and run, including keyboard and unavailable states. |
| Breadcrumb | `.rw-shell-breadcrumb`, `.ui.breadcrumb`, `.breadcrumb` | Persistent Home-first route hierarchy and context-preserving detail navigation. |
| Corpus collection | `.rw-corpus-collection` | One labeled collection selector below the Deepdive tabs without introducing nested tab navigation. |
| Tables | `.ui.table`, `.table-wrap`, `.rw-table-controls`, `.toggle-cell`, `.expansion-row`, `.rw-table-number`, `.rw-table-time` | Bounded records, sortable headers, selectable or expandable row states, stable disclosure, data alignment, and horizontal overflow. |
| Pagination | `.ui.pagination.menu`, `.rw-pagination__summary`, `.pagination`, `.pagination-pages`, `.pagination-actions` | Result summary and boundary-aware page controls. |
| Forms and controls | `.ui.form`, `.controls`, `.rw-table-controls`, `.rw-filter-disclosure`, `.filter-chips` | Shared control states, filter bars, advanced filters, active chips, sorting, search, and page-size inputs. |
| Panels and stacks | `.rw-panel`, `.rw-panel__header`, `.rw-panel__body`, `.rw-page-stack`, `.rw-section-stack`, `.rw-content-stack`, `.rw-inline-group`, `.panel`, `.panel-heading`, `.panel-body` | Parent surfaces, layout-owned separation, current panel markup, and legacy aliases. |
| Query evidence | `.query-row`, `.query-source`, `.query-code` | Source query and captured input presentation. |

Tables use a scroll wrapper rather than shrinking evidence beyond readability. Only selectable or expandable rows use hover treatment. Sortable headers expose `aria-sort`; expansion buttons expose stable `aria-expanded` and `aria-controls` relationships; selected, expanded, focus, and disabled states use borders, text, icons, attributes, and surface treatment in addition to color.

## 10. View selectors

| Family | Principal selectors | Responsibility |
|---|---|---|
| Home and metrics | `.rw-kpi-grid`, `.rw-kpi`, `.rw-home-kpis`, `.rw-home-searches`, `.rw-home-search-card*`, `.rw-home-revisions`, `.rw-home-runs`, `.ui.statistics`, `.ui.statistic`, `.metric-grid`, `.rw-metric-table`, `.rw-metric-table__value`, `.rw-metric-table__percent` | Home hierarchy, run lifecycle inventory, headline KPI cards, categorical distributions with explicit scale, and compact exhaustive metric evidence. |
| Progress | `.ui.progress`, `.ui.progress .bar`, `.bar` | Retention and percentage visualization with textual values. |
| Retention | `.rw-run-identity-strip`, `.rw-run-identity`, `.rw-retention`, `.rw-retention__phase*`, `.rw-flow`, `.rw-flow__step`, `.rw-flow__info`, `.rw-flow__progress`, `.rw-flow__delta`, `.rw-flow__outcome-values` | Borderless run identity plus three-phase connected source-selection, pipeline-processing, and corpus-enrichment evidence. |
| Summaries | `.rw-summary-strip`, `.rw-record-summary`, `.rw-record-comparison` | Definition-list summaries and comparisons. |
| Empty state | `.rw-empty-state` | Explicit absence, unavailable state, and recovery actions. |
| Details | `.rw-record-detail`, `.rw-pdf-status-strip`, `.rw-pdf-status-strip__action`, `.rw-reading-workspace`, `.rw-identity-resolution`, `.rw-identity-candidate-list`, `.rw-keywords`, `.property-grid`, collection mounts | Article summary and PDF state, equal-width reader/review workspace, author candidate evidence, reference detail, and related-record content. |
| Lifecycle modal | `.rw-modal-open`, `.rw-modal-backdrop`, `.rw-modal`, `.rw-modal__header`, `.rw-modal__body`, `.rw-modal__actions` | Dimmed confirmation hierarchy and reversible Home run-visibility action. |
| Review layout | `.rw-article-split`, `.rw-article-split__pdf`, `.rw-article-split__review`, `.rw-review-panel`, `.rw-review-onboarding`, `.rw-review-dialog*`, `.rw-review-nav`, `.rw-review-section*`, `.rw-review-form*`, `.rw-review-history*` | Responsive PDF and review columns, contained modal lineage setup, local section navigation, complete decision state, inherited labels, feedback, and immutable history. |
| Notes | `.rw-note-list`, `.rw-note-card*`, `.rw-note-workspace`, `.rw-note-form*`, `.rw-note-content`, `.rw-note-heading*`, `.rw-note-table`, `.rw-note-syntax`, `.rw-note-preview*`, `.rw-note-history`, `.rw-note-link--unresolved`, `.rw-note-comparison`, `.rw-note-diff*` | Safe rendered note cards, scoped heading hierarchy, compact tables, visible syntax help, side-by-side editing and preview, unresolved text treatment, drafts and history, bounded comparison, and added or removed lines. |
| Anchors | `.rw-anchor-panel`, `.rw-anchor-candidate`, `.rw-anchor-list`, `.rw-anchor-card*`, `.rw-anchor-history`, `.rw-pdf-anchor-layer`, `.rw-pdf-anchor-highlight` | Contextual anchor creation, keyboard-operable evidence cards, immutable history, and supplemental page highlights. |
| PDF viewer | `.rw-pdf-viewer`, `.rw-pdf-toolbar*`, `.rw-pdf-page-control`, `.rw-pdf-pages`, `.rw-pdf-page`, `.rw-pdf-status`, `.textLayer` descendants | Contained sticky desktop reader, grouped boundary-aware controls, one scrollable current page, canvas, selectable text, status, and overlay stacking. |
| Disclosure | `.rw-disclosure`, `.rw-filter-disclosure`, `.rw-event-details`, `.rw-query-row`, `.rw-note-history`, `.rw-keyword-details` | Keyboard-accessible expandable record, filter, event, query, history, and raw-value content with one chevron language. |
| Audit | `.rw-audit-layout`, `.rw-audit-filter`, `.rw-audit-stream`, `.rw-audit-day`, `.rw-audit-events`, `.rw-audit-event*`, `.rw-review-audit-change`, `.rw-review-audit-state`, `.rw-review-audit-substatuses`, `.rw-audit-load`, `.rw-record-audit*` | Full-width filters, date-grouped vertical timeline, continuous rail, category markers, visible review-decision comparisons, safe recorded-data disclosure, append status, and compact record investigation. |
| Artifacts | `.rw-artifact-*`, `.artifact-actions`, `.artifact-inspector-toolbar` | Artifact lists, bounded inspection, copy, and download actions. |
| Stages | `.rw-stage-*` | Ordered run progression, status, metrics, and artifact evidence. |

Status classes reuse the semantic token families. New statuses must include readable text and not depend on background color alone.

## 11. Graph selectors

| Family | Principal selectors | Responsibility |
|---|---|---|
| Filters | `.rw-graph__controls`, `.rw-graph__filter-fields`, `.rw-graph__filter-actions`, `.rw-graph__common-filters` | Mode-specific and shared bounded query controls. |
| Toolbar | `.rw-graph__toolbar`, `.rw-segmented-control` | Fit, export, expand, mode, and status actions. |
| Viewport | `.rw-graph__viewport`, `.rw-graph__wrap`, `.rw-graph__canvas`, `.graph-canvas` | Canvas sizing, focus, touch behavior, and fullscreen layout. |
| Model and cluster | `.rw-graph-model*`, `.rw-graph-cluster-summary` | Current model explanation and connected-component overview. |
| Legend | `.rw-graph__legend`, `.rw-graph__legend-mark`, `.rw-graph__legend-line` | Shape, color, and edge interpretation. |
| Selection | `.rw-graph__selection`, `.rw-graph-empty` | Selected node evidence and explicit empty state. |
| Relationships | `.rw-graph__edges`, relationship table selectors | Textual and paginated equivalent to canvas edges. |

Canvas uses `touch-action: none`, a grab cursor, explicit focus treatment, and responsive height. Fullscreen rules expand the viewport and canvas without removing controls or evidence.

## 12. Theme behavior

`:root` declares `color-scheme: light dark` and the light palette. `@media (prefers-color-scheme: dark)` replaces surface, text, border, accent, semantic, and graph-cluster tokens while preserving selector structure.

Dark mode maintains visible focus and semantic contrast and removes or reduces decorative elevation where specified. New hard-coded colors require a concrete rendering need that cannot be represented by an existing semantic or graph token and must be reviewed in both themes.

## 13. Responsive behavior

The principal responsive boundaries are 70rem for the article reading workspace, 1100px for other dense analysis layouts, and 720px for mobile Deepdive navigation and stacked controls.

- At or below 1100px, relationship layout becomes one column and its filter area stops using sticky positioning.
- At or below 70rem, the equal-width article reader/review workspace becomes one column, the PDF viewer stops using sticky positioning while retaining a contained scrollable height, and review forms plus note editing become one column.
- At or below 720px, Deepdive navigation uses the Menu control, context selectors and Home KPI grids stack, dashboard and record grids collapse, review and lifecycle dialogs plus actions stack, PDF toolbar groups use available width, graph controls use available width, and the canvas height reduces.
- Wide tables remain horizontally scrollable, and pagination controls wrap while retaining result summaries and boundary actions.
- Responsive rules preserve labels, status meaning, disclosures, evidence, and actions rather than hiding content required for analysis.

## 14. Accessibility treatments

- `:focus-visible` supplies a strong global focus ring, and graph canvas focus adds an inset ring.
- The skip link moves into view on focus.
- Disabled buttons retain visible state and suppress pointer action without removing their labels.
- Loading controls use visible state plus surrounding live or status content.
- Labels, messages, selected rows, statuses, graph entities, and truncation use text, shapes, attributes, or borders in addition to color.
- PDF anchor highlights are supplemental to a keyboard-operable textual list that names the anchor, page, current or inherited state, content availability, selected text, and history.
- `prefers-reduced-motion` rules suppress unnecessary transitions and smooth behavior while preserving state changes.
- Horizontal overflow is owned by tables and graph regions so keyboard focus and page layout remain usable.

## 15. Change checklist

- Identify the owning stylesheet and reuse an existing token or component family before adding a selector.
- Verify source markup and JavaScript use the documented selector and do not change generated vendor content.
- Preserve the six-file cascade order unless an approved architecture change requires another layer.
- Test light, dark, desktop, tablet, mobile, keyboard focus, reduced motion, loading, error, and unavailable states affected by the change.
- Run frontend unit, server, Playwright, accessibility, and visual verification required by [STANDARDS.md](STANDARDS.md).
- Update this reference when active tokens, selector families, aliases, cascade ownership, theme behavior, or responsive behavior changes.
