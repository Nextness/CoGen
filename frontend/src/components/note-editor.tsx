// Immutable review-note editor, draft persistence, history, and bounded comparison.
import { api, mutate, APIError } from "../api.tsx";
import { formatTime, link } from "../state.tsx";
import { h, Fragment, render as renderTree } from "../jsx/jsx-runtime.ts";
import { parseNote, NoteDocument } from "./note-parser.tsx";
import type { NoteLink, ResolvedNoteLink } from "./note-parser.tsx";
import { mountBacklinks } from "./backlinks.tsx";

const diffLineLimit = 200;

/** Builds a browser-local draft key scoped to the opaque corpus and immutable head. */
export function draftKey(corpusID: string, runID: string | number, workRevisionID: string | number, noteID?: any, expectedVersionID?: any): string {
  return ["review-draft", corpusID, runID, workRevisionID, noteID || "new", expectedVersionID || "none"].join(":");
}

/** Reads a draft without assuming browser storage is available. */
export function readDraft(key: string, storage?: Storage): string | null {
  try {
    return (storage || window.localStorage).getItem(key);
  } catch (_) {
    return null;
  }
}

/** Writes a draft and reports storage failure without discarding editor content. */
export function writeDraft(key: string, value: string, storage?: Storage): boolean {
  try {
    (storage || window.localStorage).setItem(key, value);
    return true;
  } catch (_) {
    return false;
  }
}

/** Removes only the exact draft associated with a successful save. */
export function clearDraft(key: string, storage?: Storage): boolean {
  try {
    (storage || window.localStorage).removeItem(key);
    return true;
  } catch (_) {
    return false;
  }
}

/** Returns a bounded line comparison or the complete side-by-side fallback. */
export interface LineDiffResult {
  fallback: boolean;
  before?: string[];
  after?: string[];
  rows?: Array<{ type: string; text: string }>;
}

/** Produces a bounded line comparison or complete side-by-side fallback. */
export function lineDiff(previous: any, current: any, limit?: number): LineDiffResult {
  const before = String(previous || "").split("\n");
  const after = String(current || "").split("\n");
  const bound = limit || diffLineLimit;
  if (before.length > bound || after.length > bound) {
    return {
      fallback: true,
      before: before,
      after: after,
    };
  }
  const matrix = Array.from({ length: before.length + 1 }, () => {
    return new Uint16Array(after.length + 1);
  });
  for (let left = before.length - 1; left >= 0; left -= 1) {
    for (let right = after.length - 1; right >= 0; right -= 1) {
      var cell = matrix[left + 1][right + 1] + 1;
      if (before[left] !== after[right]) cell = Math.max(matrix[left + 1][right], matrix[left][right + 1]);
      matrix[left][right] = cell;
    }
  }

  const rows: Array<{ type: string; text: string }> = [];
  let left = 0;
  let right = 0;
  while (left < before.length || right < after.length) {
    if (left < before.length && right < after.length && before[left] === after[right]) {
      rows.push({
        type: "same",
        text: before[left++],
      });
      right += 1;
    } else if (right < after.length && (left === before.length || matrix[left][right + 1] >= matrix[left + 1][right])) {
      rows.push({
        type: "added",
        text: after[right++],
      });
    } else {
      rows.push({
        type: "removed",
        text: before[left++],
      });
    }
  }
  return {
    fallback: false,
    rows: rows,
  };
}

/** One note record exposed by the review notes API. */
export interface ReviewNoteRecord {
  id: any;
  work_revision_id?: any;
  version: {
    id: any;
    body: string;
    state: string;
    created_at: any;
    reviewer_display: string;
    links?: ResolvedNoteLink[];
    title?: string;
    excerpt?: string;
    body_bytes?: number;
    body_truncated?: boolean;
    link_count?: number;
  };
  inherited_from_context_id?: any;
}

/** One note editor option set. */
export interface NoteEditorOptions {
  corpusID: string;
  runID: string | number;
  workRevisionID: string | number;
  onChanged?: () => void;
  editable?: boolean;
  articleDOI?: string;
}

/** Renders one active note card with its metadata, content, and actions. */
function noteCardMarkup(note: ReviewNoteRecord, editable: boolean): JSX.Element {
  const noteBody = note.version.body || "";
  var inheritedMarkup: JSX.Element | null = null;
  if (note.inherited_from_context_id) inheritedMarkup = <span className="ui violet label">Inherited</span>;
  const parsedNote = parseNote(noteBody);
  const noteTime = formatTime(note.version.created_at);
  const title = note.version.title || `Note ${note.id}`;
  var contentMarkup: JSX.Element = <p>{note.version.excerpt || "No active note content."}</p>;
  if (!note.version.body_truncated && note.version.state === "active") {
    contentMarkup = <NoteDocument document={parsedNote} resolvedLinks={note.version.links} />;
  }
  var activeActions: JSX.Element | null = null;
  if (note.version.state === "active") {
    activeActions = (
      <Fragment>
        <button type="button" className="ui basic button" data-note-open>Open note</button>
        <button type="button" className="ui basic button" data-note-edit disabled={!editable}>Edit</button>
        <button type="button" className="ui danger button" data-note-delete disabled={!editable}>Remove</button>
      </Fragment>
    );
  }
  return (
    <li data-note-id={note.id}>
      <div className="rw-note-card__meta">
        <div className="rw-note-card__identity">
          <h5>{title}</h5>
          <div className="rw-note-card__badges">
            <span className="ui label">Version {note.version.id}</span>
            {inheritedMarkup}
          </div>
        </div>
        <p>{note.version.reviewer_display} <span aria-hidden="true">·</span> <time datetime={note.version.created_at}>{noteTime}</time></p>
      </div>
      <div className="rw-note-content" data-note-content>{contentMarkup}</div>
      <div className="rw-note-card__actions">
        {activeActions}
        <button type="button" className="ui basic button" data-note-history-open>History</button>
        <button type="button" className="ui basic button" data-note-backlinks>Backlinks</button>
      </div>
      <div data-note-backlink-list></div>
    </li>
  );
}

/** Renders one immutable version comparison row for the note history. */
function versionComparisonMarkup(previous: string, version: any): JSX.Element {
  const comparison = lineDiff(previous, version.body || "");
  var comparisonHTML: JSX.Element;
  if (comparison.fallback) {
    comparisonHTML = (
      <div className="rw-note-comparison">
        <pre>{previous}</pre>
        <pre>{version.body || ""}</pre>
      </div>
    );
  } else {
    const diffRows = (comparison.rows || []).map(({ type, text }) => {
      var prefix = "  ";
      if (type === "added") prefix = "+ ";
      else if (type === "removed") prefix = "- ";
      const rowClass = `rw-note-diff--${type}`;
      return <Fragment><span className={rowClass}>{prefix}{text}</span>{"\n"}</Fragment>;
    });
    comparisonHTML = <pre className="rw-note-diff">{diffRows}</pre>;
  }
  return comparisonHTML;
}

/** Mounts the note editor and current immutable note list for one article. */
export async function mountNoteEditor(host: HTMLElement, options: NoteEditorOptions): Promise<void> {
  let currentNote: ReviewNoteRecord | null = null;
  const editable = options.editable !== false;
  let noteCursor = "";
  let noteHasMore = false;
  let noteState = "active";
  let noteQuery = "";
  let loadedNotes: ReviewNoteRecord[] = [];
  const initialArticleTarget = options.articleDOI || "";
  const editorMarkup = (
    <section className="rw-note-editor">
      <div className="rw-review-section__heading">
        <div>
          <h4 id="review-notes-heading">Notes</h4>
          <p>Notes keep immutable version history. Unsaved drafts remain only in this browser.</p>
        </div>
      </div>
      <div className="rw-note-workspace">
        <form className="ui form rw-note-form" data-note-form>
          <div className="rw-note-form__heading">
            <div>
              <h5 data-note-editor-title>New note</h5>
              <p>Use the project note syntax for headings, lists, quotes, code, tables, and evidence links.</p>
            </div>
          </div>
          <div className="rw-note-editor-tools">
            <details className="rw-disclosure rw-note-link-tools">
              <summary>Insert evidence link</summary>
              <div className="rw-disclosure__content">
                <div className="rw-note-link-insert">
                  <p>Links use <code>[[type:target|display]]</code>. Saved links resolve only inside the selected run context.</p>
                  <label>
                    Type
                    <select data-note-link-type>
                      <option value="article">Article DOI</option>
                      <option value="pdf">PDF page</option>
                      <option value="anchor">Saved anchor ID</option>
                      <option value="note">Review note ID</option>
                      <option value="ext">External HTTP(S) URL</option>
                    </select>
                  </label>
                  <label>
                    Target
                    <input data-note-link-target value={initialArticleTarget} list="note-anchor-options" />
                    <datalist id="note-anchor-options" data-note-anchor-options></datalist>
                  </label>
                  <label>
                    Display text <span className="rw-optional">Optional</span>
                    <input data-note-link-display />
                  </label>
                  <div className="rw-inline-group">
                    <button type="button" className="ui basic button" data-note-link-insert>Insert link</button>
                    <button type="button" className="ui basic button" data-note-link-anchors-more hidden>Load more saved anchors</button>
                  </div>
                </div>
              </div>
            </details>
            <details className="rw-disclosure rw-note-syntax">
              <summary>Note syntax and link examples</summary>
              <div className="rw-disclosure__content">
                <p>Use <code>#</code> through <code>####</code> for headings, <code>-</code> for bullets, <code>{">"}</code> for quotes, fenced code blocks, and simple Markdown-style tables.</p>
                <dl>
                  <div>
                    <dt>Article DOI</dt>
                    <dd><code>[[article:10.1000/example|Article title]]</code></dd>
                  </div>
                  <div>
                    <dt>PDF page</dt>
                    <dd><code>[[pdf:page=5|Methods page]]</code></dd>
                  </div>
                  <div>
                    <dt>Saved anchor</dt>
                    <dd><code>[[anchor:a0123456789abcdef0123456789abcdef|Methods excerpt]]</code></dd>
                  </div>
                  <div>
                    <dt>Review note</dt>
                    <dd><code>[[note:123|Related note]]</code></dd>
                  </div>
                  <div>
                    <dt>External URL</dt>
                    <dd><code>[[ext:https://example.test|Source]]</code></dd>
                  </div>
                </dl>
              </div>
            </details>
          </div>
          <div className="ui field">
            <label htmlFor="review-note-body">Note body</label>
            <textarea id="review-note-body" rows={10} data-note-body disabled={!editable}></textarea>
          </div>
          <p className="ui faded text" data-note-byte-count>0 of 262144 bytes</p>
          <p className="rw-draft-status ui faded text" data-draft-status aria-live="polite"></p>
          <div className="rw-note-diagnostics" data-note-diagnostics role="alert"></div>
          <div className="rw-review-actions">
            <button type="submit" className="ui primary button" data-note-save disabled={!editable}>Save note</button>
            <button type="button" className="ui basic button" data-note-cancel disabled={!editable}>Discard draft</button>
          </div>
        </form>
        <aside className="rw-note-preview" aria-labelledby="note-preview-heading">
          <div className="rw-note-preview__heading">
            <h5 id="note-preview-heading">Preview</h5>
            <span className="ui label">Safe rendering</span>
          </div>
          <div data-note-preview></div>
        </aside>
      </div>
      <section className="rw-note-saved" aria-labelledby="saved-notes-heading">
        <div className="rw-note-saved__heading">
          <div>
            <h5 id="saved-notes-heading">Saved notes</h5>
            <p>Search current note heads or inspect removed-note history.</p>
          </div>
        </div>
        <form className="ui form rw-filter-bar rw-note-filters" data-note-filter-form>
          <label className="rw-filter-bar__search">
            <span>Search Notes</span>
            <input type="search" data-note-query placeholder="Title or note text" />
          </label>
          <label>
            <span>Note state</span>
            <select data-note-state>
              <option value="active">Active notes</option>
              <option value="removed">Removed notes</option>
              <option value="all">All notes</option>
            </select>
          </label>
          <button type="submit" className="ui primary button">Apply note filters</button>
        </form>
        <div data-note-list></div>
      </section>
      <div data-note-history></div>
    </section>
  );
  renderTree(editorMarkup, host);
  const body = host.querySelector("[data-note-body]") as HTMLTextAreaElement;
  const diagnostics = host.querySelector("[data-note-diagnostics]") as HTMLElement;
  const preview = host.querySelector("[data-note-preview]") as HTMLElement;
  const draftStatus = host.querySelector("[data-draft-status]") as HTMLElement;
  const editorTitle = host.querySelector("[data-note-editor-title]") as HTMLElement;
  const byteCount = host.querySelector("[data-note-byte-count]") as HTMLElement;
  let draftTimer = 0;
  let savedEditorBody = "";
  /** Reports whether the current in-memory body differs from its saved or deliberately cleared baseline. */
  function isDirty(): boolean {
    return body.value !== savedEditorBody;
  }
  /** Protects browser and SPA navigation while preserving a user-controlled discard path. */
  function protectDraft(event: Event): void {
    if (!host.isConnected) {
      document.removeEventListener("rw-before-navigate", protectDraft);
      window.removeEventListener("beforeunload", protectDraft);
      return;
    }
    if (!isDirty()) return;
    if (event.type === "beforeunload") {
      event.preventDefault();
      (event as BeforeUnloadEvent).returnValue = "";
      return;
    }
    if (!window.confirm("Leave this article and discard the unsaved note draft?")) event.preventDefault();
  }
  document.addEventListener("rw-before-navigate", protectDraft);
  window.addEventListener("beforeunload", protectDraft);

  /** Returns the draft key for the current new-note or immutable note head. */
  function key(): string {
    return draftKey(options.corpusID, options.runID, options.workRevisionID, currentNote?.id, currentNote?.version?.id);
  }
  /** Parses and safely renders current textarea content while displaying diagnostics. */
  function renderPreview(): boolean {
    const noteDocument = parseNote(body.value);
    var diagnosticsMarkup: JSX.Element | null = null;
    if (noteDocument.errors.length) {
      const errorItems = noteDocument.errors.map((error) => {
        return <li>{error.message} at position {error.position}.</li>;
      });
      diagnosticsMarkup = <ul>{errorItems}</ul>;
    }
    renderTree(diagnosticsMarkup, diagnostics);
    const previewMarkup = <NoteDocument document={noteDocument} resolvedLinks={currentNote?.version?.links || null} />;
    renderTree(previewMarkup, preview);
    return noteDocument.errors.length === 0;
  }
  /** Updates the UTF-8 evidence limit without parsing the document. */
  function updateByteCount(): void {
    const count = new TextEncoder().encode(body.value).length;
    byteCount.textContent = `${count} of 262144 bytes`;
    byteCount.className = "ui faded text";
    if (count > 262144) byteCount.className = "ui error text";
  }
  /** Persists and renders the in-memory draft after the typing debounce or a forced flush. */
  function flushDraft(): void {
    if (draftTimer) window.clearTimeout(draftTimer);
    draftTimer = 0;
    renderPreview();
    const persisted = writeDraft(key(), body.value);
    draftStatus.className = "rw-draft-status ui warning message";
    draftStatus.textContent = "Browser storage failed. The draft remains in this tab but is not database evidence.";
    if (persisted) {
      draftStatus.className = "rw-draft-status ui success message";
      draftStatus.textContent = "Browser draft saved. This is not database evidence until Save note succeeds.";
    }
  }
  /** Returns the editor to new-note mode and restores only its matching draft. */
  function resetEditor(): void {
    currentNote = null;
    editorTitle.textContent = "New note";
    body.value = readDraft(key()) || "";
    savedEditorBody = "";
    renderPreview();
    updateByteCount();
    draftStatus.className = "rw-draft-status ui faded text";
    draftStatus.textContent = "";
    (host.querySelector("[data-note-cancel]") as HTMLButtonElement).textContent = "Discard draft";
  }
  /** Loads one complete current note head only when an action needs its body and resolved links. */
  async function loadFullNote(note: ReviewNoteRecord): Promise<ReviewNoteRecord> {
    if (!note.version.body_truncated && note.version.links?.length === note.version.link_count) return note;
    const data = await api(`/api/runs/${options.runID}/notes/${note.id}`, {}, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    return data.note;
  }
  /** Renders loaded note pages and binds body, edit, history, removal, and backlink controls. */
  function renderNoteList(): void {
    var notesMarkup: JSX.Element = <p className="ui faded text">No notes match the selected state and search.</p>;
    if (loadedNotes.length) {
      const noteItems = loadedNotes.map((note) => {
        return noteCardMarkup(note, editable);
      });
      notesMarkup = <ul className="rw-note-list">{noteItems}</ul>;
    }
    var loadMoreMarkup: JSX.Element | null = null;
    if (noteHasMore) loadMoreMarkup = <button type="button" className="ui basic button" data-note-load-more>Load older notes</button>;
    const listMarkup = <Fragment><p className="ui error message" data-note-list-message role="alert" hidden></p>{notesMarkup}{loadMoreMarkup}</Fragment>;
    renderTree(listMarkup, host.querySelector("[data-note-list]")!);
    host.querySelector<HTMLButtonElement>("[data-note-load-more]")?.addEventListener("click", async (event) => {
      const button = event.currentTarget as HTMLButtonElement;
      button.disabled = true;
      button.classList.add("loading");
      await loadNotes(false);
    });
    for (const note of loadedNotes) {
      const row = host.querySelector(`[data-note-id="${CSS.escape(String(note.id))}"]`) as HTMLElement;
      row.querySelector<HTMLButtonElement>("[data-note-open]")?.addEventListener("click", async (event) => {
        const button = event.currentTarget as HTMLButtonElement;
        button.disabled = true;
        button.classList.add("loading");
        try {
          const full = await loadFullNote(note);
          const content = row.querySelector("[data-note-content]") as HTMLElement;
          const fullDocument = parseNote(full.version.body || "");
          const fullMarkup = <NoteDocument document={fullDocument} resolvedLinks={full.version.links} />;
          renderTree(fullMarkup, content);
          button.remove();
        } catch (error: any) {
          showNoteListError(error.message || "The complete note could not be loaded.");
          button.disabled = false;
          button.classList.remove("loading");
        }
      });
      row.querySelector<HTMLButtonElement>("[data-note-edit]")?.addEventListener("click", async () => {
        const full = await loadFullNote(note);
        currentNote = full;
        editorTitle.textContent = `Editing ${full.version.title || `note ${full.id}`}`;
        body.value = readDraft(key()) ?? full.version.body ?? "";
        savedEditorBody = full.version.body || "";
        (host.querySelector("[data-note-cancel]") as HTMLButtonElement).textContent = "Cancel edit";
        renderPreview();
        updateByteCount();
        body.focus();
      });
      row.querySelector<HTMLButtonElement>("[data-note-history-open]")!.addEventListener("click", () => { void showHistory(note); });
      row.querySelector<HTMLButtonElement>("[data-note-backlinks]")!.addEventListener("click", () => { void showBacklinks(note, row); });
      row.querySelector<HTMLButtonElement>("[data-note-delete]")?.addEventListener("click", async (event) => {
        const button = event.currentTarget as HTMLButtonElement;
        if (!window.confirm(`Remove ${note.version.title || `note ${note.id}`}? Its immutable history will remain available.`)) return;
        button.disabled = true;
        button.classList.add("loading");
        try {
          const result = await mutate(`/api/runs/${options.runID}/notes/${note.id}/versions`, "POST", {
            expected_version_id: note.version.id,
            state: "deleted",
            body: "",
          });
          const message = host.querySelector("[data-note-list-message]") as HTMLElement;
          message.className = "ui success message";
          message.textContent = "Note removed. Its immutable history remains available below.";
          message.hidden = false;
          button.classList.remove("loading");
          try {
            await loadNotes(true);
            await showHistory(result.note);
            await options.onChanged?.();
          } catch (refreshError: any) {
            message.className = "ui warning message";
            message.textContent = `Note removed, refresh failed: ${refreshError.message}`;
          }
        } catch (error: any) {
          showNoteListError(error.message || "The note could not be removed.");
          button.disabled = false;
          button.classList.remove("loading");
        }
      });
    }
  }
  /** Displays one local list failure without discarding already loaded notes. */
  function showNoteListError(messageText: string): void {
    const message = host.querySelector("[data-note-list-message]") as HTMLElement;
    message.textContent = messageText;
    message.hidden = false;
  }
  /** Loads one bounded active, removed, or combined note page and preserves prior rows on continuation. */
  async function loadNotes(reset = true): Promise<void> {
    if (reset) {
      noteCursor = "";
      noteHasMore = false;
      loadedNotes = [];
    }
    const data = await api(`/api/runs/${options.runID}/articles/${options.workRevisionID}/notes`, {
      limit: 25,
      cursor: noteCursor,
      state: noteState,
      q: noteQuery,
    }, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    const known = new Set(loadedNotes.map((note) => String(note.id)));
    for (const note of data.items || data.notes || []) {
      if (!known.has(String(note.id))) loadedNotes.push(note);
    }
    noteCursor = data.next_cursor || "";
    noteHasMore = Boolean(data.has_more);
    renderNoteList();
  }
  /** Loads paged inbound links for one note target without replacing its card. */
  async function showBacklinks(note: ReviewNoteRecord, row: HTMLElement): Promise<void> {
    const target = row.querySelector("[data-note-backlink-list]") as HTMLElement;
    await mountBacklinks(target, {
      runID: Number(options.runID),
      targetType: "note",
      targetID: String(note.id),
    });
  }
  /** Displays one selected head's paged immutable ancestry and optional restoration control. */
  async function showHistory(note: ReviewNoteRecord): Promise<void> {
    let versions: any[] = [];
    let cursor = "";
    let hasMore = false;
    const historyHost = host.querySelector("[data-note-history]") as HTMLElement;
    /** Renders loaded summaries and loads complete bodies only when a disclosure opens. */
    function renderHistory(): void {
      const latestActive = versions.find((version) => {
        return version.state === "active";
      });
      var restoreMarkup: JSX.Element | null = null;
      if (note.version.state === "deleted") {
        restoreMarkup = <button type="button" className="ui primary button" data-note-restore disabled={!editable || !latestActive}>Restore previous content</button>;
      }
      const versionItems = versions.map((version) => {
        const versionTime = formatTime(version.created_at);
        return (
          <details data-note-version={version.id}>
            <summary>Version {version.id} · {version.state} · {versionTime}</summary>
            <div data-note-version-content><p className="ui faded text">Open to load the complete immutable body.</p></div>
          </details>
        );
      });
      var olderMarkup: JSX.Element | null = null;
      if (hasMore) olderMarkup = <button type="button" className="ui basic button" data-note-history-more>Load older versions</button>;
      const historyMarkup = (
        <section className="rw-note-history">
          <div className="rw-review-section__heading">
            <div>
              <h4>{note.version.title || `Note ${note.id}`} history</h4>
              <p>Open a version to load and compare its complete immutable body.</p>
            </div>
            {restoreMarkup}
          </div>
          {versionItems}
          {olderMarkup}
          <p className="ui error message" data-note-history-error role="alert" hidden></p>
        </section>
      );
      renderTree(historyMarkup, historyHost);
      for (const disclosure of Array.from(historyHost.querySelectorAll<HTMLDetailsElement>("[data-note-version]"))) {
        disclosure.addEventListener("toggle", async () => {
          if (!disclosure.open || disclosure.dataset.loaded === "true") return;
          const versionID = disclosure.dataset.noteVersion as string;
          const content = disclosure.querySelector("[data-note-version-content]") as HTMLElement;
          try {
            const data = await api(`/api/runs/${options.runID}/notes/${note.id}/versions/${versionID}`, {}, {
              method: "GET",
              headers: { Accept: "application/json" },
            });
            const position = versions.findIndex((item) => String(item.id) === versionID);
            var previousBody = "";
            const previous = versions[position + 1];
            if (previous) {
              const previousData = await api(`/api/runs/${options.runID}/notes/${note.id}/versions/${previous.id}`, {}, {
                method: "GET",
                headers: { Accept: "application/json" },
              });
              previousBody = previousData.version?.body || "";
            }
            renderTree(versionComparisonMarkup(previousBody, data.version), content);
            disclosure.dataset.loaded = "true";
          } catch (error: any) {
            const errorMarkup = <p className="ui error message">{error.message}</p>;
            renderTree(errorMarkup, content);
          }
        });
      }
      historyHost.querySelector<HTMLButtonElement>("[data-note-history-more]")?.addEventListener("click", async (event) => {
        const button = event.currentTarget as HTMLButtonElement;
        button.disabled = true;
        button.classList.add("loading");
        await loadHistoryPage();
      });
      historyHost.querySelector<HTMLButtonElement>("[data-note-restore]")?.addEventListener("click", async (event) => {
        if (!latestActive) return;
        const button = event.currentTarget as HTMLButtonElement;
        button.disabled = true;
        button.classList.add("loading");
        try {
          const data = await api(`/api/runs/${options.runID}/notes/${note.id}/versions/${latestActive.id}`, {}, {
            method: "GET",
            headers: { Accept: "application/json" },
          });
          await mutate(`/api/runs/${options.runID}/notes/${note.id}/versions`, "POST", {
            expected_version_id: note.version.id,
            state: "active",
            body: data.version.body,
          });
          history.replaceState({}, "", link({ note_id: note.id }));
          noteState = "active";
          (host.querySelector("[data-note-state]") as HTMLSelectElement).value = noteState;
          const message = historyHost.querySelector("[data-note-history-error]") as HTMLElement;
          message.className = "ui success message";
          message.textContent = "Note restored as a new immutable version.";
          message.hidden = false;
          button.classList.remove("loading");
          try {
            await loadNotes(true);
            await focusNote(note.id);
            await options.onChanged?.();
          } catch (refreshError: any) {
            message.className = "ui warning message";
            message.textContent = `Note restored, refresh failed: ${refreshError.message}`;
          }
        } catch (error: any) {
          const message = historyHost.querySelector("[data-note-history-error]") as HTMLElement;
          message.textContent = error.message || "The note could not be restored.";
          message.hidden = false;
          button.disabled = false;
          button.classList.remove("loading");
        }
      });
    }
    /** Appends one version-summary page while preserving already loaded ancestry. */
    async function loadHistoryPage(): Promise<void> {
      const data = await api(`/api/runs/${options.runID}/notes/${note.id}/versions`, { limit: 25, cursor: cursor }, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      const known = new Set(versions.map((version) => String(version.id)));
      for (const version of data.items || data.versions || []) {
        if (!known.has(String(version.id))) versions.push(version);
      }
      cursor = data.next_cursor || "";
      hasMore = Boolean(data.has_more);
      renderHistory();
    }
    await loadHistoryPage();
  }
  /** Resolves a URL-focused active or deleted note and exposes its history. */
  async function focusNote(noteID: any): Promise<void> {
    const data = await api(`/api/runs/${options.runID}/notes/${encodeURIComponent(noteID)}`, {}, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    if (!data.note) return;
    await showHistory(data.note);
    const row = host.querySelector<HTMLElement>(`[data-note-id="${CSS.escape(String(noteID))}"]`);
    row?.scrollIntoView?.({ block: "nearest" });
    const focusButton = row?.querySelector("button");
    focusButton?.focus();
  }
  body.addEventListener("input", () => {
    updateByteCount();
    draftStatus.className = "rw-draft-status ui warning message";
    draftStatus.textContent = "Browser draft pending. This is not database evidence.";
    if (draftTimer) window.clearTimeout(draftTimer);
    draftTimer = window.setTimeout(flushDraft, 250);
  });
  body.addEventListener("blur", flushDraft);
  window.addEventListener("pagehide", flushDraft);
  const cancelButton = host.querySelector("[data-note-cancel]") as HTMLButtonElement;
  cancelButton.addEventListener("click", () => {
    if (currentNote) {
      clearDraft(key());
      resetEditor();
      return;
    }
    if (body.value && !window.confirm("Discard this unsaved browser draft?")) return;
    clearDraft(key());
    resetEditor();
  });
  const filterForm = host.querySelector("[data-note-filter-form]") as HTMLFormElement;
  filterForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    noteQuery = (host.querySelector("[data-note-query]") as HTMLInputElement).value.trim();
    noteState = (host.querySelector("[data-note-state]") as HTMLSelectElement).value;
    await loadNotes(true);
  });
  const linkType = host.querySelector("[data-note-link-type]") as HTMLSelectElement;
  const linkTarget = host.querySelector("[data-note-link-target]") as HTMLInputElement;
  const linkDisplay = host.querySelector("[data-note-link-display]") as HTMLInputElement;
  const anchorOptions = host.querySelector("[data-note-anchor-options]") as HTMLDataListElement;
  const moreAnchors = host.querySelector("[data-note-link-anchors-more]") as HTMLButtonElement;
  const anchorChoices = new Map<string, string>();
  let anchorChoiceCursor = "";
  let anchorChoiceHasMore = false;
  /** Appends one page of stable anchor identities and human labels for link insertion. */
  async function loadAnchorChoices(reset: boolean): Promise<void> {
    if (reset) {
      anchorChoiceCursor = "";
      anchorChoiceHasMore = false;
      anchorChoices.clear();
      anchorOptions.textContent = "";
    }
    const data = await api(`/api/runs/${options.runID}/articles/${options.workRevisionID}/anchors`, {
      cursor: anchorChoiceCursor,
      limit: 25,
    }, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    for (const anchor of data.items || data.anchors || []) {
      if (anchorChoices.has(String(anchor.id))) continue;
      anchorChoices.set(String(anchor.id), anchor.label || String(anchor.id));
      const option = document.createElement("option");
      option.value = String(anchor.id);
      option.label = anchor.label || String(anchor.id);
      anchorOptions.append(option);
    }
    anchorChoiceCursor = data.next_cursor || "";
    anchorChoiceHasMore = Boolean(data.has_more);
    moreAnchors.hidden = linkType.value !== "anchor" || !anchorChoiceHasMore;
  }
  /** Loads anchor choices with a local diagnostic while keeping the note editor usable. */
  async function loadAnchorChoicesSafely(reset: boolean): Promise<void> {
    try {
      await loadAnchorChoices(reset);
    } catch (error: any) {
      draftStatus.className = "rw-draft-status ui warning message";
      draftStatus.textContent = `Saved anchors could not be loaded: ${error.message}`;
    }
  }
  linkType.addEventListener("change", () => {
    if (linkType.value === "article") linkTarget.value = options.articleDOI || "";
    else {
      linkTarget.value = "";
      if (linkType.value === "anchor" && anchorChoices.size === 0) void loadAnchorChoicesSafely(true);
    }
    moreAnchors.hidden = linkType.value !== "anchor" || !anchorChoiceHasMore;
  });
  linkTarget.addEventListener("input", () => {
    if (linkType.value !== "anchor") return;
    const label = anchorChoices.get(linkTarget.value);
    if (label) linkDisplay.value = label;
  });
  moreAnchors.addEventListener("click", () => {
    void loadAnchorChoicesSafely(false);
  });
  host.querySelector("[data-note-link-insert]")!.addEventListener("click", () => {
    const targetValue = linkTarget.value.trim();
    if (!targetValue) {
      linkTarget.focus();
      return;
    }
    const display = linkDisplay.value.trim();
    var displaySuffix = "";
    if (display) displaySuffix = `|${display}`;
    const token = `[[${linkType.value}:${targetValue}${displaySuffix}]]`;
    body.setRangeText(token, body.selectionStart, body.selectionEnd, "end");
    body.dispatchEvent(new Event("input", { bubbles: true }));
    body.focus();
  });
  const noteForm = host.querySelector("[data-note-form]") as HTMLFormElement;
  noteForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!renderPreview()) return;
    const savedKey = key();
    const saveButton = host.querySelector("[data-note-save]") as HTMLButtonElement;
    saveButton.disabled = true;
    saveButton.classList.add("loading");
    try {
      let saved: any;
      if (currentNote) {
        saved = await mutate(`/api/runs/${options.runID}/notes/${currentNote.id}/versions`, "POST", {
          expected_version_id: currentNote.version.id,
          state: "active",
          body: body.value,
        });
      } else {
        saved = await mutate(`/api/runs/${options.runID}/articles/${options.workRevisionID}/notes`, "POST", {
          body: body.value,
        });
      }
      clearDraft(savedKey);
      resetEditor();
      draftStatus.className = "rw-draft-status ui success message";
      draftStatus.textContent = "Note saved as immutable database evidence.";
      try {
        await loadNotes(true);
        await options.onChanged?.();
      } catch (refreshError: any) {
        draftStatus.className = "rw-draft-status ui warning message";
        draftStatus.textContent = `Note saved, but the local display refresh failed: ${refreshError.message}. Reload to see version ${saved.note?.version?.id || "the new version"}.`;
      }
    } catch (error: any) {
      var errorMessage = `Save failed. Your draft was kept: ${error.message}`;
      if (error instanceof APIError && error.status === 409) {
        errorMessage = "This note changed elsewhere. Your draft was kept.";
      }
      draftStatus.className = "rw-draft-status ui error message";
      var conflictAction: JSX.Element | null = null;
      if (error instanceof APIError && error.status === 409) {
        conflictAction = <button type="button" className="ui basic button" data-note-load-latest>Load latest while keeping my input</button>;
      }
      const errorMarkup = (
        <Fragment>
          <span>{errorMessage}</span>
          {conflictAction}
        </Fragment>
      );
      renderTree(errorMarkup, draftStatus);
      host.querySelector<HTMLButtonElement>("[data-note-load-latest]")?.addEventListener("click", async () => {
        if (!currentNote) return;
        const localDraft = body.value;
        const latest = await api(`/api/runs/${options.runID}/notes/${currentNote.id}`, {}, {
          method: "GET",
          headers: { Accept: "application/json" },
        });
        currentNote = latest.note;
        savedEditorBody = currentNote?.version.body || "";
        body.value = localDraft;
        draftStatus.className = "rw-draft-status ui warning message";
        draftStatus.textContent = `Latest saved version ${currentNote?.version.id} loaded for comparison. Your local input remains in the editor; save again to reapply it.`;
        await showHistory(currentNote!);
      });
    } finally {
      if (saveButton.isConnected) {
        saveButton.disabled = false;
        saveButton.classList.remove("loading");
      }
    }
  });
  resetEditor();
  await loadNotes(true);
  const focused = new URLSearchParams(location.search).get("note_id");
  if (/^[1-9]\d*$/.test(focused || "")) await focusNote(focused);
}
