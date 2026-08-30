// Searchable run-scoped Notes index for the Evaluation queue.
import { api, errorMessage } from "../api.tsx";
import type { ReviewNote, ReviewNotesResponse } from "../api/types.ts";
import { currentDetailOrigin, link } from "../state.tsx";
import { h, Fragment, render as renderTree, cx } from "../jsx/jsx-runtime.ts";

/** Typed compound class names used by this module. */
const classNames = {
  uiBasicButton: cx("ui", "basic", "button"),
  uiErrorMessage: cx("ui", "error", "message"),
  uiFadedText: cx("ui", "faded", "text"),
  uiFormRwFilterBar: cx("ui", "form", "rw-filter-bar"),
  uiNeutralLabel: cx("ui", "neutral", "label"),
  uiPrimaryButton: cx("ui", "primary", "button"),
};

/** Mounts a bounded run Notes index with state search and cursor continuation. */
export async function mountRunNotesIndex(host: HTMLElement, runID: number): Promise<void> {
  let cursor = "";
  let hasMore = false;
  let notes: ReviewNote[] = [];
  let query = "";
  let state = "all";

  /** Renders the current filters, note summaries, and continuation state. */
  function render(): void {
    const noteItems = notes.map((note) => {
      const href = link({
        view: "article",
        article_id: note.work_revision_id,
        note_id: note.id,
        origin: currentDetailOrigin(),
      });
      const title = note.version?.title || `Note ${note.id}`;
      const excerpt = note.version?.excerpt || "No excerpt recorded.";
      return (
        <li>
          <a href={href}>{title}</a>
          <p>{excerpt}</p>
          <span className={classNames.uiNeutralLabel}>{note.version?.state || "unknown"}</span>
        </li>
      );
    });
    var noteList: JSX.Element = <p className={classNames.uiFadedText}>No Notes match this run-scoped search.</p>;
    if (noteItems.length) noteList = <ol className="rw-note-list">{noteItems}</ol>;
    var more: JSX.Element | null = null;
    if (hasMore) more = <button type="button" className={classNames.uiBasicButton} data-run-notes-more>Load more Notes</button>;
    const indexMarkup = (
      <Fragment>
        <form className={classNames.uiFormRwFilterBar} data-run-notes-filter>
          <label>
            <span>Search Notes</span>
            <input name="q" type="search" value={query} />
          </label>
          <label>
            <span>State</span>
            <select name="state">
              <option value="all" selected={state === "all"}>Active and removed</option>
              <option value="active" selected={state === "active"}>Active</option>
              <option value="removed" selected={state === "removed"}>Removed</option>
            </select>
          </label>
          <button type="submit" className={classNames.uiPrimaryButton}>Search Notes</button>
        </form>
        <p className={classNames.uiErrorMessage} data-run-notes-error role="alert" hidden></p>
        {noteList}
        {more}
      </Fragment>
    );
    renderTree(indexMarkup, host);
    host.querySelector<HTMLFormElement>("[data-run-notes-filter]")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = new FormData(event.currentTarget as HTMLFormElement);
      query = String(form.get("q") || "");
      state = String(form.get("state") || "all");
      await loadPage(true);
    });
    host.querySelector<HTMLButtonElement>("[data-run-notes-more]")?.addEventListener("click", () => {
      void loadPage(false);
    });
  }

  /** Loads one bounded note-summary page and keeps prior rows on continuation failure. */
  async function loadPage(reset: boolean): Promise<void> {
    if (reset) {
      cursor = "";
      hasMore = false;
      notes = [];
    }
    try {
      const data = await api<ReviewNotesResponse>(`/api/runs/${runID}/notes`, {
        cursor: cursor,
        limit: 25,
        state: state,
        q: query,
      }, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      const known = new Set(notes.map((note) => String(note.id)));
      for (const note of data.items || data.notes || []) {
        if (!known.has(String(note.id))) notes.push(note);
      }
      cursor = data.next_cursor || "";
      hasMore = Boolean(data.has_more);
      render();
    } catch (error) {
      if (!host.childElementCount) render();
      const message = host.querySelector<HTMLElement>("[data-run-notes-error]");
      if (message) {
        message.textContent = errorMessage(error, "Notes could not be loaded.");
        message.hidden = false;
      }
    }
  }

  await loadPage(true);
}
