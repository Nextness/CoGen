# Frontend Code Style Guide

## 1. Purpose and scope

This document is the normative style reference for frontend TypeScript and JSX source under `frontend/src`. It is mandatory reading for any agent or developer who changes frontend code, together with [AGENTS.md](../AGENTS.md), section 14 of [STANDARDS.md](STANDARDS.md), [JSX-RUNTIME.md](JSX-RUNTIME.md) for runtime semantics, [DESIGN.md](DESIGN.md) for behavior contracts, and [CSS-REFERENCE.md](CSS-REFERENCE.md) for active styles.

The conventions below are demonstrated by existing views, with `SearchTermCoveragePanel` in [frontend/src/views/detail.tsx](../frontend/src/views/detail.tsx) as the canonical example. New code must match the established pattern of the file it extends; this guide states that pattern so a change stays consistent without requiring the reader to reconstruct the whole file. [STANDARDS.md](STANDARDS.md) section 14 defines the module, import, escaping, and build rules that this guide complements.

## 2. Identifier naming

Use camelCase for every TypeScript identifier. Snake_case names such as `fields_elements`, `panel_body`, and `all_match` are wrong in frontend source and must be renamed to `fieldElements`, `panelBody`, and `fieldMatches`.

Keep API wire keys intact on the payload: `matched_total`, `term_total`, and `terms_with_sources` remain server keys, and their values are copied into descriptive camelCase bindings such as `matchedTotal`, `termTotal`, and `termEntries`.

Name a binding after its content: `matchedTermNames` is a `Set` of term names matched in any field, `termEntries` pairs each recorded term with its sources, `fieldMatches` is the match list of one field, and `termTags` are the label elements rendered for one field.

Avoid filler words and vague generics: `allTerms` does not say what the variable holds, and `elements` does not say elements of what. Prefer `termEntries` and `termTags`, which state content and role.

Destructure object pairs and tuples at the use site instead of indexing: write `([term, sources])` and `([key, item])` in `map` and `filter` callbacks rather than `entry[0]` and `entry[1]`. The file already uses this destructuring in `rawRecord`; tuple indexing was the reason the matched and unmatched term maps were hard to read.

Keep related names parallel: `matchedTerms` and `unmatchedTerms` are both term entries derived from `termEntries`, while `matchedTermNames` is the set used to split them. Never reuse one name for two shapes.

## 3. String quoting

Prefer double quotes for JavaScript string literals in frontend source: write `"title"` and `"Matched terms"` rather than `'title'` and `'Matched terms'`. This covers string literals in declarations, property keys, comparisons, and JSX attribute values; template literals remain unchanged.

Use a template literal whenever a string interpolates a value: write `` `[data-detail-collection-host="${key}"]` `` instead of `'[data-detail-collection-host="' + key + '"]'`. Do not build interpolated strings with the plus operator; concatenation is harder to read than a backtick literal with `${...}` placeholders, so new and edited code uses template literals.

Use a template literal when a string contains nested double quotes, such as `` `[data-table-owner="work_revisions"]` `` or `` `class="ui table"``, instead of escaping those quotes inside a double-quoted string.

The existing codebase mixes single- and double-quoted literals, so the convention applies to new and edited code: use double quotes in lines you add or touch and do not convert quoting in unrelated lines of the same file.

## 4. Object and array literals

Format multi-property object literals with one property per line and a trailing comma, as in the `updates` map in `detailLink` and the `link({...})` calls in `backToCorpus`. Single-property objects such as `{ run_id: value("run_id") }` stay on one line.

Extract array literals used in `.includes()` checks into a named variable: `const checkList = ["article", "author", "reference"]; if (checkList.includes(currentKind) && currentID) {...}`. A literal inside the call hides what the list means.

## 5. Control flow and variables

Prefer `if` and `else` with pre-declared variables over inline ternary chains for multi-branch rendering. Declare the fallback first, then reassign in each branch, as in the per-field cell:

```tsx
var content: JSX.Element = <span className="ui faded text">No matched terms</span>;
if (!field.recorded) {
  content = <span className="ui faded text">Not recorded</span>;
} else if (fieldMatches.length) {
  const termTags: JSX.Element[] = fieldMatches.map((term) => {
    return <span className="ui label">{term}</span>;
  });
  content = <span className="rw-keyword-tags">{termTags}</span>;
}
```

Use the same default-first pattern for whole regions: `disclosureContent` starts as the "No search terms recorded." paragraph and is replaced by the built matched and unmatched sections only when `termEntries.length` is set.

Guard early for absent data before any derived work: `SearchTermCoveragePanel` returns the empty-state `Panel` immediately when `props.matches` is null or undefined, so the rest of the function can assume a payload exists.

Write an `if` whose body is exactly one short statement without braces on a single line, as in `if (typeof raw !== "string") return recorded(raw);`. Keep the braced multi-line form when the body has more than one statement or the single statement is long, as in the per-field cell above.

Single-statement `else if` branches use the same one-line form: `if (kind === "author") section = "authors"; else if (kind === "reference") section = "references";`.

Break long method chains into intermediary variables. Assign each chained segment to a variable that names its result before calling the next method, as in `const pageButtons = container.querySelectorAll<HTMLButtonElement>("[data-detail-page]"); pageButtons.forEach(...)` instead of calling `forEach` directly on the query result. Chains hide intermediate shapes and make each step harder to review.

Separate logical groups of declarations and statements with blank lines so each group reads as one unit. In `CollectionMarkup`, pagination sizing, the body rows, the table markup, the pagination configuration, and the final count each form their own block, and a blank line marks where one concern ends and the next begins. Do not blank-line every statement; group only what belongs together.

Build region arrays with `push` inside `if` blocks instead of nesting ternaries: the matched and unmatched sections of the disclosure are appended to `sections` only when each list is non-empty.

Do not keep nested ternary chains in JSX. The original disclosure body nested a ternary over `termEntries.length` around ternaries over `matchedTerms.length` and `unmatchedTerms.length`; refactor such chains into the default-first variable form even when the rendered output does not change.

Convert value-assignment ternaries to a defaulted variable with `if`: write `var content = recorded(entry.value); if (entry.html) content = raw(entry.value);` instead of `const content = entry.html ? raw(entry.value) : recorded(entry.value);`. The `if` form reads as a default with an override instead of a fork.

## 6. Derive data before markup

Compute every derived value before building the JSX tree. `SearchTermCoveragePanel` first records the `fields` descriptors, then derives `matchedTermNames`, `termEntries`, `matchedTerms`, and `unmatchedTerms`, then builds `fieldElements`, `disclosureContent`, and `panelBody`, and only then returns the `Panel`.

Keep JSX expressions free of filtering, splitting, and counting logic; reference the derived variables instead. A JSX subtree should read as structure, with data already shaped.

Do not alias props into local variables: use `props.record` and `props.data` directly instead of `const record = props.record;`. An alias adds a name without adding meaning; derive only genuinely new values such as `validation`, `summary`, or `pdfGrid`.

## 7. JSX layout

Keep JSX narrow and extract larger subtrees into named variables: `fieldElements`, `disclosureContent`, and `panelBody` hold the pieces rendered inside the panel so each element stays short enough to scan.

Never pass a JSX element literal to a function call. Assign the element to a variable first and pass the variable, as in `const collectionMarkup = (<CollectionMarkup ... />); renderTree(collectionMarkup, container);`. An element literal inside a call buries the configuration and makes the call hard to read.

Use `Fragment` when a group has multiple roots, as the matched and unmatched term sections do inside the disclosure content.

Elements with two or more children put each child on its own line, as the `dt`/`dd` rows and the collection header do; single-child elements stay inline.

Write callback functions that are passed as parameters with arrow syntax: `(button) => {...}`, `([term, sources]) => {...}`, and `() => {...}`. Keep the named `function` syntax for module-level helpers and other declarations, which are not callbacks; do not pass anonymous `function () {...}` expressions as arguments.

Inside an element, JSX expressions may reference variables only; do not inline complete statements such as `{reasons.map((reason) => {...})}` into the markup. Compute the mapped or filtered values into a named variable before the element: `const reasonsMap = reasons.map((reason) => ...); return <ul className="rw-mapping-values">{reasonsMap}</ul>;`. Mapped arrays used in both matched and unmatched sections are derived once, before the markup, exactly like `matchedTermTags` and `unmatchedTermTags`.

The generated `ClassName` type permits a single defined token as a literal. Compound or conditional classes use `cx`, and every computed class is extracted before markup: `const gridClass = cx("rw-property-grid", ...(classes || []));` feeds `className={gridClass}`. Repeated fixed combinations may use one documented module-local `classNames` object whose properties call `cx`; JSX references the named property and never calls `cx` inline. Markup-producing calls follow the same rule: `const mappingResult = mappingValue(item);` feeds `{mappingResult}`.

Keep `map` and `filter` callbacks small: destructure the incoming entry, use the resulting names, and return one element or one boolean.

Do not restate presentation logic in markup; the classes `rw-term-field`, `rw-keyword-tags`, `rw-term-sources`, `ui label`, and `ui faded text` stay in the JSX, and every state decided by `if` branches arrives through a variable.

## 8. Reuse and helpers

Extract a named helper with attached JSDoc when the same markup pattern appears more than once in a view. The matched and unmatched term maps render identical label markup and share one helper or one destructured map body rather than two divergent copies.

Follow the file conventions for existing helpers: `recorded`, `keywordMarkup`, and `stageReasonMarkup` each render one repeated presentation and each carries a one-line JSDoc describing what it renders.

Every maintained declaration requires the adjacent JSDoc description that the catalog copies into [PROJECT_CATALOG.md](PROJECT_CATALOG.md); a new helper without a comment fails `make check-docs`.

## 9. Types

Annotate the `JSX.Element` return type on every component and helper that returns markup, and type the props object explicitly, as `SearchTermCoveragePanel(props: { matches: any; record: any }): JSX.Element` does.

Do not widen or overload types during a readability refactor; rename and restructure only, and leave every payload shape and rendered node the same. Class-producing props and local mappings use `ClassName`, arrays use `readonly ClassName[]` where mutation is unnecessary, and compound results use the branded `ClassNames` type returned by `cx`.

Use `const` for bindings that are never reassigned and `let` for bindings that are, matching the surrounding declaration style of the file being changed; the views currently declare mutable markup slots with `var` and immutable bindings with `const`.

## 10. Canonical example

The disclosure content of `SearchTermCoveragePanel` before refactoring nested ternaries and tuple indexing:

```tsx
{allTerms.length
  ? <Fragment>
    {matchedTerms.length ? <Fragment><p className="ui faded text">Matched terms</p><div className="rw-keyword-tags">{matchedTerms.map(function(entry) {
      return <span className="ui label">{entry[0]}<span className="rw-term-sources">({entry[1].join(', ')})</span></span>;
    })}</div></Fragment> : null}
    {unmatchedTerms.length ? <Fragment><p className="ui faded text">Unmatched terms</p><div className="rw-keyword-tags">{unmatchedTerms.map(function(entry) {
      return <span className="ui label">{entry[0]}<span className="rw-term-sources">({entry[1].join(', ')})</span></span>;
    })}</div></Fragment> : null}
  </Fragment>
  : <p className="ui faded text">No search terms recorded.</p>}
```

The same output built with named variables and `if` blocks:

```tsx
var disclosureContent: JSX.Element = <p className="ui faded text">No search terms recorded.</p>;
if (termEntries.length) {
  const matchedTermTags = matchedTerms.map(([term, sources]) => {
    return <span className="ui label">{term}<span className="rw-term-sources">({sources.join(', ')})</span></span>;
  });
  const unmatchedTermTags = unmatchedTerms.map(([term, sources]) => {
    return <span className="ui label">{term}<span className="rw-term-sources">({sources.join(', ')})</span></span>;
  });
  const sections: JSX.Element[] = [];
  if (matchedTerms.length) {
    sections.push(
      <Fragment>
        <p className="ui faded text">Matched terms</p>
        <div className="rw-keyword-tags">{matchedTermTags}</div>
      </Fragment>
    );
  }
  if (unmatchedTerms.length) {
    sections.push(
      <Fragment>
        <p className="ui faded text">Unmatched terms</p>
        <div className="rw-keyword-tags">{unmatchedTermTags}</div>
      </Fragment>
    );
  }
  disclosureContent = <Fragment>{sections}</Fragment>;
}
```

The two versions render identical markup; readability comes from the names, the destructuring, and the flat control flow and nothing else.

## 11. Verification after frontend changes

Naming and structure refactors must not change observable behavior, so verify with the existing suites: run [unit tests](../frontend/tests/unit/views/detail.test.ts) with `make test-frontend-unit`, run `make check-frontend` for registry freshness, static class validation, and type checking, run `make test-go PACKAGE=./server` for integration, and run the focused Playwright viewer suite as section 15 of [STANDARDS.md](STANDARDS.md) requires.

Run `make check-docs` after this guide or any referenced documentation changes, and run `make docs-state-update` after reviewing the dependents listed in [DOC-STATE.md](DOC-STATE.md).
