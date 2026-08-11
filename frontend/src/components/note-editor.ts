// Immutable review-note editor, draft persistence, history, and bounded comparison.
import { api, mutate, APIError } from "../api.ts";
import { esc, formatTime, link } from "../state.ts";
import { parseNote, renderNote } from "./note-parser.ts";
import type { NoteLink, ResolvedNoteLink } from "./note-parser.ts";

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
  if (before.length > bound || after.length > bound) return { fallback: true, before: before, after: after };
  const matrix = Array.from({ length: before.length + 1 }, function() { return new Uint16Array(after.length + 1); });
  for (let left = before.length - 1; left >= 0; left -= 1) {
    for (let right = after.length - 1; right >= 0; right -= 1) {
      matrix[left][right] = before[left] === after[right] ? matrix[left + 1][right + 1] + 1 : Math.max(matrix[left + 1][right], matrix[left][right + 1]);
    }
  }
  const rows: Array<{ type: string; text: string }> = [];
  let left = 0;
  let right = 0;
  while (left < before.length || right < after.length) {
    if (left < before.length && right < after.length && before[left] === after[right]) rows.push({ type: "same", text: before[left++] }), right += 1;
    else if (right < after.length && (left === before.length || matrix[left][right + 1] >= matrix[left + 1][right])) rows.push({ type: "added", text: after[right++] });
    else rows.push({ type: "removed", text: before[left++] });
  }
  return { fallback: false, rows: rows };
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

/** Mounts the note editor and current immutable note list for one article. */
export async function mountNoteEditor(host: HTMLElement, options: NoteEditorOptions): Promise<void> {
  let currentNote: ReviewNoteRecord | null = null;
  host.innerHTML = `<section class="rw-note-editor"><div class="rw-review-section__heading"><div><h4 id="review-notes-heading">Review notes</h4><p>Notes keep immutable version history. Unsaved drafts remain only in this browser.</p></div></div>`
    + `<div data-note-list></div><div class="rw-note-workspace"><form class="ui form rw-note-form" data-note-form><div class="rw-note-form__heading"><div><h5 data-note-editor-title>New note</h5><p>Use the project note syntax for headings, lists, quotes, code, tables, and evidence links.</p></div></div>`
    + `<details class="rw-disclosure rw-note-syntax"><summary>Note syntax and link examples</summary><div class="rw-disclosure__content"><p>Use <code>#</code> through <code>####</code> for headings, <code>-</code> for bullets, <code>&gt;</code> for quotes, fenced code blocks, and simple Markdown-style tables.</p><dl><div><dt>Article DOI</dt><dd><code>[[article:10.1000/example|Article title]]</code></dd></div><div><dt>PDF page</dt><dd><code>[[pdf:page=5|Methods page]]</code></dd></div><div><dt>Saved anchor</dt><dd><code>[[anchor:methods-1|Methods excerpt]]</code></dd></div><div><dt>Review note</dt><dd><code>[[note:123|Related note]]</code></dd></div><div><dt>External URL</dt><dd><code>[[ext:https://example.test|Source]]</code></dd></div></dl></div></details>`
    + `<div class="ui field"><label for="review-note-body">Note body</label><textarea id="review-note-body" rows="10" data-note-body></textarea></div>`
    + `<p class="rw-draft-status ui faded text" data-draft-status aria-live="polite"></p><div class="rw-note-diagnostics" data-note-diagnostics role="alert"></div>`
    + `<div class="rw-review-actions"><button type="submit" class="ui primary button" data-note-save>Save note</button><button type="button" class="ui basic button" data-note-cancel>Clear editor</button></div></form>`
    + `<aside class="rw-note-preview" aria-labelledby="note-preview-heading"><div class="rw-note-preview__heading"><h5 id="note-preview-heading">Preview</h5><span class="ui label">Safe rendering</span></div><div data-note-preview></div></aside></div>`
    + "<div data-note-history></div></section>";
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
    const document = parseNote(body.value);
    diagnostics.innerHTML = document.errors.length ? "<ul>" + document.errors.map(function(error) { return "<li>" + esc(error.message) + " at position " + error.position + ".</li>"; }).join("") + "</ul>" : "";
    preview.innerHTML = renderNote(document, currentNote?.version?.links || null);
    return document.errors.length === 0;
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
    const data = await api(`/api/runs/${options.runID}/articles/${options.workRevisionID}/notes`, { limit: 100 }, { method: "GET", headers: { Accept: "application/json" } });
    const notes = data.notes || [];
    host.querySelector("[data-note-list]")!.innerHTML = notes.length ? `<ul class="rw-note-list">` + notes.map(function(note: ReviewNoteRecord) {
      const noteBody = note.version.body || "";
      return `<li data-note-id="` + note.id + `"><div class="rw-note-card__meta"><div><span class="ui note label">Note ` + note.id + `</span><span class="ui label">Version ` + note.version.id + `</span>` + (note.inherited_from_context_id ? `<span class="ui violet label">Inherited</span>` : "") + "</div><p>" + esc(note.version.reviewer_display) + " · " + esc(formatTime(note.version.created_at)) + "</p></div>"
        + `<div class="rw-note-content">` + renderNote(parseNote(noteBody), note.version.links) + "</div>"
        + `<div class="rw-note-card__actions"><button type="button" class="ui basic button" data-note-edit>Edit</button><button type="button" class="ui basic button" data-note-history-open>History</button><button type="button" class="ui danger button" data-note-delete>Remove</button></div></li>`;
    }).join("") + "</ul>" : `<p class="ui faded text">No active notes.</p>`;
    for (const note of notes) {
      const row = host.querySelector(`[data-note-id="${note.id}"]`) as HTMLElement;
      (row.querySelector("[data-note-edit]") as HTMLButtonElement).addEventListener("click", function() {
        currentNote = note;
        editorTitle.textContent = "Editing note " + note.id;
        body.value = readDraft(key()) ?? note.version.body ?? "";
        renderPreview();
        body.focus();
      });
      (row.querySelector("[data-note-history-open]") as HTMLButtonElement).addEventListener("click", function() { void showHistory(note); });
      (row.querySelector("[data-note-delete]") as HTMLButtonElement).addEventListener("click", async function() {
        await mutate(`/api/runs/${options.runID}/notes/${note.id}/versions`, "POST", { expected_version_id: note.version.id, state: "deleted", body: "" });
        await loadNotes();
        options.onChanged?.();
      });
    }
  }
  /** Displays one selected head's immutable ancestry and optional restoration control. */
  async function showHistory(note: ReviewNoteRecord): Promise<void> {
    const data = await api(`/api/runs/${options.runID}/notes/${note.id}/versions`, { limit: 100 }, { method: "GET", headers: { Accept: "application/json" } });
    const versions: any[] = data.versions || [];
    const latestActive = versions.find(function(version) { return version.state === "active"; });
    host.querySelector("[data-note-history]")!.innerHTML = `<section class="rw-note-history"><div class="rw-review-section__heading"><div><h4>Note ` + note.id + ` history</h4><p>Compare immutable snapshots from newest to oldest.</p></div>` + (note.version.state === "deleted" ? `<button type="button" class="ui primary button" data-note-restore` + (latestActive ? "" : " disabled") + ">Restore previous content</button>" : "") + "</div>" + (versions as any[]).map(function(version, index) {
      const previous = versions[index + 1]?.body || "";
      const comparison = lineDiff(previous, version.body || "");
      const comparisonHTML = comparison.fallback
        ? `<div class="rw-note-comparison"><pre>` + esc(previous) + "</pre><pre>" + esc(version.body || "") + "</pre></div>"
        : `<pre class="rw-note-diff">` + (comparison.rows || []).map(function(row) { return `<span class="rw-note-diff--` + row.type + `">` + esc((row.type === "added" ? "+ " : row.type === "removed" ? "- " : "  ") + row.text) + "</span>"; }).join("\n") + "</pre>";
      return "<details><summary>Version " + version.id + " · " + esc(version.state) + " · " + esc(formatTime(version.created_at)) + "</summary>" + comparisonHTML + "</details>";
    }).join("") + "</section>";
    host.querySelector("[data-note-restore]")?.addEventListener("click", async function() {
      await mutate(`/api/runs/${options.runID}/notes/${note.id}/versions`, "POST", { expected_version_id: note.version.id, state: "active", body: latestActive.body });
      history.replaceState({}, "", link({ note_id: note.id }));
      await loadNotes();
      await focusNote(note.id);
      options.onChanged?.();
    });
  }
  /** Resolves a URL-focused active or deleted note and exposes its history. */
  async function focusNote(noteID: any): Promise<void> {
    const data = await api(`/api/runs/${options.runID}/notes/${encodeURIComponent(noteID)}`, {}, { method: "GET", headers: { Accept: "application/json" } });
    if (!data.note) return;
    await showHistory(data.note);
    const row = host.querySelector<HTMLElement>(`[data-note-id="${CSS.escape(String(noteID))}"]`);
    row?.scrollIntoView?.({ block: "nearest" });
    row?.querySelector("button")?.focus();
  }
  body.addEventListener("input", function() {
    renderPreview();
    const persisted = writeDraft(key(), body.value);
    draftStatus.className = persisted ? "rw-draft-status ui success message" : "rw-draft-status ui warning message";
    draftStatus.textContent = persisted ? "Draft saved in this browser." : "Draft could not be persisted. Keep a separate copy.";
  });
  (host.querySelector("[data-note-cancel]") as HTMLButtonElement).addEventListener("click", resetEditor);
  (host.querySelector("[data-note-form]") as HTMLFormElement).addEventListener("submit", async function(event) {
    event.preventDefault();
    if (!renderPreview()) return;
    const savedKey = key();
    const saveButton = host.querySelector("[data-note-save]") as HTMLButtonElement;
    saveButton.disabled = true;
    saveButton.classList.add("loading");
    try {
      if (currentNote) {
        await mutate(`/api/runs/${options.runID}/notes/${currentNote.id}/versions`, "POST", { expected_version_id: currentNote.version.id, state: "active", body: body.value });
      } else {
        await mutate(`/api/runs/${options.runID}/articles/${options.workRevisionID}/notes`, "POST", { body: body.value });
      }
      clearDraft(savedKey);
      resetEditor();
      await loadNotes();
      options.onChanged?.();
    } catch (error: any) {
      draftStatus.className = "rw-draft-status ui error message";
      draftStatus.textContent = error instanceof APIError && error.status === 409 ? "This note changed elsewhere. Your draft was kept; reload history before retrying." : `Save failed. Your draft was kept: ${error.message}`;
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