// Immutable review-note editor, draft persistence, history, and bounded comparison.
import { api, mutate, APIError } from "../api.tsx";
import { formatTime, link } from "../state.tsx";
import { h, Fragment, render as renderTree } from "../jsx/jsx-runtime.ts";
import { parseNote, NoteDocument } from "./note-parser.tsx";
import type { NoteLink, ResolvedNoteLink } from "./note-parser.tsx";

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
  version: {
    id: any;
    body: string;
    state: string;
    created_at: any;
    reviewer_display: string;
    links?: ResolvedNoteLink[];
  };
  inherited_from_context_id?: any;
}

/** One note editor option set. */
export interface NoteEditorOptions {
  corpusID: string;
  runID: string | number;
  workRevisionID: string | number;
  onChanged?: () => void;
}

/** Renders one active note card with its metadata, content, and actions. */
function noteCardMarkup(note: ReviewNoteRecord): JSX.Element {
  const noteBody = note.version.body || "";
  var inheritedMarkup: JSX.Element | null = null;
  if (note.inherited_from_context_id) inheritedMarkup = <span className="ui violet label">Inherited</span>;
  const parsedNote = parseNote(noteBody);
  const noteTime = formatTime(note.version.created_at);
  return (
    <li data-note-id={note.id}>
      <div className="rw-note-card__meta">
        <div>
          <span className="ui note label">Note {note.id}</span>
          <span className="ui label">Version {note.version.id}</span>
          {inheritedMarkup}
        </div>
        <p>{note.version.reviewer_display} · {noteTime}</p>
      </div>
      <div className="rw-note-content"><NoteDocument document={parsedNote} resolvedLinks={note.version.links} /></div>
      <div className="rw-note-card__actions">
        <button type="button" className="ui basic button" data-note-edit>Edit</button>
        <button type="button" className="ui basic button" data-note-history-open>History</button>
        <button type="button" className="ui danger button" data-note-delete>Remove</button>
      </div>
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
  const editorMarkup = (
    <section className="rw-note-editor">
      <div className="rw-review-section__heading">
        <div>
          <h4 id="review-notes-heading">Review notes</h4>
          <p>Notes keep immutable version history. Unsaved drafts remain only in this browser.</p>
        </div>
      </div>
      <div data-note-list></div>
      <div className="rw-note-workspace">
        <form className="ui form rw-note-form" data-note-form>
          <div className="rw-note-form__heading">
            <div>
              <h5 data-note-editor-title>New note</h5>
              <p>Use the project note syntax for headings, lists, quotes, code, tables, and evidence links.</p>
            </div>
          </div>
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
                  <dd><code>[[anchor:methods-1|Methods excerpt]]</code></dd>
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
          <div className="ui field">
            <label htmlFor="review-note-body">Note body</label>
            <textarea id="review-note-body" rows={10} data-note-body></textarea>
          </div>
          <p className="rw-draft-status ui faded text" data-draft-status aria-live="polite"></p>
          <div className="rw-note-diagnostics" data-note-diagnostics role="alert"></div>
          <div className="rw-review-actions">
            <button type="submit" className="ui primary button" data-note-save>Save note</button>
            <button type="button" className="ui basic button" data-note-cancel>Clear editor</button>
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
      <div data-note-history></div>
    </section>
  );
  renderTree(editorMarkup, host);
  const body = host.querySelector("[data-note-body]") as HTMLTextAreaElement;
  const diagnostics = host.querySelector("[data-note-diagnostics]") as HTMLElement;
  const preview = host.querySelector("[data-note-preview]") as HTMLElement;
  const draftStatus = host.querySelector("[data-draft-status]") as HTMLElement;
  const editorTitle = host.querySelector("[data-note-editor-title]") as HTMLElement;

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
  /** Returns the editor to new-note mode and restores only its matching draft. */
  function resetEditor(): void {
    currentNote = null;
    editorTitle.textContent = "New note";
    body.value = readDraft(key()) || "";
    renderPreview();
    draftStatus.className = "rw-draft-status ui faded text";
    draftStatus.textContent = "";
  }
  /** Loads bounded active note heads and binds their edit, history, and tombstone actions. */
  async function loadNotes(): Promise<void> {
    const data = await api(`/api/runs/${options.runID}/articles/${options.workRevisionID}/notes`, { limit: 100 }, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    const notes = data.notes || [];
    var notesMarkup: JSX.Element = <p className="ui faded text">No active notes.</p>;
    if (notes.length) {
      const noteItems = notes.map((note: ReviewNoteRecord) => {
        return noteCardMarkup(note);
      });
      notesMarkup = <ul className="rw-note-list">{noteItems}</ul>;
    }
    renderTree(notesMarkup, host.querySelector("[data-note-list]")!);
    for (const note of notes) {
      const row = host.querySelector(`[data-note-id="${note.id}"]`) as HTMLElement;
      const editButton = row.querySelector("[data-note-edit]") as HTMLButtonElement;
      editButton.addEventListener("click", () => {
        currentNote = note;
        editorTitle.textContent = `Editing note ${note.id}`;
        body.value = readDraft(key()) ?? note.version.body ?? "";
        renderPreview();
        body.focus();
      });
      const historyButton = row.querySelector("[data-note-history-open]") as HTMLButtonElement;
      historyButton.addEventListener("click", () => { void showHistory(note); });
      const deleteButton = row.querySelector("[data-note-delete]") as HTMLButtonElement;
      deleteButton.addEventListener("click", async () => {
        await mutate(`/api/runs/${options.runID}/notes/${note.id}/versions`, "POST", {
          expected_version_id: note.version.id,
          state: "deleted",
          body: "",
        });
        await loadNotes();
        options.onChanged?.();
      });
    }
  }
  /** Displays one selected head's immutable ancestry and optional restoration control. */
  async function showHistory(note: ReviewNoteRecord): Promise<void> {
    const data = await api(`/api/runs/${options.runID}/notes/${note.id}/versions`, { limit: 100 }, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    const versions: any[] = data.versions || [];
    const latestActive = versions.find((version) => {
      return version.state === "active";
    });
    var restoreMarkup: JSX.Element | null = null;
    if (note.version.state === "deleted") {
      restoreMarkup = <button type="button" className="ui primary button" data-note-restore disabled={!latestActive}>Restore previous content</button>;
    }
    const versionItems = (versions as any[]).map((version, index) => {
      const previous = versions[index + 1]?.body || "";
      const comparisonHTML = versionComparisonMarkup(previous, version);
      const versionTime = formatTime(version.created_at);
      return (
        <details>
          <summary>Version {version.id} · {version.state} · {versionTime}</summary>
          {comparisonHTML}
        </details>
      );
    });
    const historyMarkup = (
      <section className="rw-note-history">
        <div className="rw-review-section__heading">
          <div>
            <h4>Note {note.id} history</h4>
            <p>Compare immutable snapshots from newest to oldest.</p>
          </div>
          {restoreMarkup}
        </div>
        {versionItems}
      </section>
    );
    renderTree(historyMarkup, host.querySelector("[data-note-history]")!);
    const restoreButton = host.querySelector("[data-note-restore]");
    restoreButton?.addEventListener("click", async () => {
      await mutate(`/api/runs/${options.runID}/notes/${note.id}/versions`, "POST", {
        expected_version_id: note.version.id,
        state: "active",
        body: latestActive.body,
      });
      history.replaceState({}, "", link({ note_id: note.id }));
      await loadNotes();
      await focusNote(note.id);
      options.onChanged?.();
    });
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
    renderPreview();
    const persisted = writeDraft(key(), body.value);
    var statusClass = "rw-draft-status ui warning message";
    var statusText = "Draft could not be persisted. Keep a separate copy.";
    if (persisted) {
      statusClass = "rw-draft-status ui success message";
      statusText = "Draft saved in this browser.";
    }
    draftStatus.className = statusClass;
    draftStatus.textContent = statusText;
  });
  const cancelButton = host.querySelector("[data-note-cancel]") as HTMLButtonElement;
  cancelButton.addEventListener("click", resetEditor);
  const noteForm = host.querySelector("[data-note-form]") as HTMLFormElement;
  noteForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!renderPreview()) return;
    const savedKey = key();
    const saveButton = host.querySelector("[data-note-save]") as HTMLButtonElement;
    saveButton.disabled = true;
    saveButton.classList.add("loading");
    try {
      if (currentNote) {
        await mutate(`/api/runs/${options.runID}/notes/${currentNote.id}/versions`, "POST", {
          expected_version_id: currentNote.version.id,
          state: "active",
          body: body.value,
        });
      } else {
        await mutate(`/api/runs/${options.runID}/articles/${options.workRevisionID}/notes`, "POST", {
          body: body.value,
        });
      }
      clearDraft(savedKey);
      resetEditor();
      await loadNotes();
      options.onChanged?.();
    } catch (error: any) {
      var errorMessage = `Save failed. Your draft was kept: ${error.message}`;
      if (error instanceof APIError && error.status === 409) {
        errorMessage = "This note changed elsewhere. Your draft was kept; reload history before retrying.";
      }
      draftStatus.className = "rw-draft-status ui error message";
      draftStatus.textContent = errorMessage;
    } finally {
      if (saveButton.isConnected) {
        saveButton.disabled = false;
        saveButton.classList.remove("loading");
      }
    }
  });
  resetEditor();
  await loadNotes();
  const focused = new URLSearchParams(location.search).get("note_id");
  if (/^[1-9]\d*$/.test(focused || "")) await focusNote(focused);
}
