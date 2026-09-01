// Paged inbound note-link evidence shared by review targets.
import { api, errorMessage } from "../api.tsx";
import type { ReviewBacklinksResponse, ReviewNote } from "../api/types.ts";
import { link, stateFor } from "../state.tsx";
import { h, Fragment, render as renderTree, cx, classAdd } from "../jsx/jsx-runtime.ts";

/** Typed compound class names used by this module. */
const classNames = {
  uiBasicButton: cx("ui", "basic", "button"),
  uiErrorMessage: cx("ui", "error", "message"),
  uiFadedText: cx("ui", "faded", "text"),
};

/** One paged backlink target and presentation contract. */
export interface BacklinkOptions {
  runID: number;
  targetType: "note" | "article" | "pdf_page" | "anchor" | "ext";
  targetID: string;
  workRevisionID?: number;
  heading?: string;
}

/** Loads and renders every requested backlink page without discarding prior rows. */
export async function mountBacklinks(host: HTMLElement, options: BacklinkOptions): Promise<void> {
  let cursor = "";
  let hasMore = false;
  let loading = false;
  const loaded: ReviewNote[] = [];

  /** Renders loaded source-note summaries and an explicit continuation control. */
  function render(): void {
    const items = loaded.map((source) => {
      const updates = {
        view: "article",
        article_id: source.work_revision_id,
        note_id: source.id,
        anchor_id: "",
        pdf_page: "",
      };
      const href = link(updates);
      const state = stateFor(updates);
      const title = source.version?.title || `Note ${source.id}`;
      return <li><a href={href} data-state={JSON.stringify(state)}>{title}</a></li>;
    });
    var content: JSX.Element = <p className={classNames.uiFadedText}>No current note links point here.</p>;
    if (items.length) {
      var more: JSX.Element | null = null;
      if (hasMore) more = <button type="button" className={classNames.uiBasicButton} data-backlinks-more>Load more inbound links</button>;
      content = (
        <Fragment>
          <h6>{options.heading || "Inbound note links"}</h6>
          <ul>{items}</ul>
          {more}
        </Fragment>
      );
    }
    renderTree(content, host);
    host.querySelector<HTMLButtonElement>("[data-backlinks-more]")?.addEventListener("click", () => {
      void loadPage();
    });
  }

  /** Appends one opaque-cursor backlink page and retains visible evidence on failure. */
  async function loadPage(): Promise<void> {
    if (loading) return;
    loading = true;
    const existingButton = host.querySelector<HTMLButtonElement>("[data-backlinks-more]");
    if (existingButton) {
      existingButton.disabled = true;
      classAdd(existingButton, ["loading"]);
    }
    try {
      const data = await api<ReviewBacklinksResponse>(`/api/runs/${options.runID}/links/backlinks`, {
        target_type: options.targetType,
        target_id: options.targetID,
        work_revision_id: options.workRevisionID,
        cursor: cursor,
        limit: 25,
      }, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      const known = new Set(loaded.map((source) => String(source.id)));
      for (const source of data.items || data.backlinks || []) {
        if (!known.has(String(source.id))) loaded.push(source);
      }
      cursor = data.next_cursor || "";
      hasMore = Boolean(data.has_more);
      render();
    } catch (error) {
      var prior: JSX.Element | null = null;
      if (loaded.length) {
        const retained = loaded.map((source) => {
          const updates = {
            view: "article",
            article_id: source.work_revision_id,
            note_id: source.id,
            anchor_id: "",
            pdf_page: "",
          };
          const href = link(updates);
          const state = stateFor(updates);
          return <li><a href={href} data-state={JSON.stringify(state)}>{source.version?.title || `Note ${source.id}`}</a></li>;
        });
        prior = <ul>{retained}</ul>;
      }
      const errorMarkup = (
        <Fragment>
          {prior}
          <p className={classNames.uiErrorMessage}>Inbound links could not be loaded: {errorMessage(error, "Unknown error")}</p>
          <button type="button" className={classNames.uiBasicButton} data-backlinks-retry>Retry inbound links</button>
        </Fragment>
      );
      renderTree(errorMarkup, host);
      host.querySelector<HTMLButtonElement>("[data-backlinks-retry]")?.addEventListener("click", () => {
        void loadPage();
      });
    } finally {
      loading = false;
    }
  }

  await loadPage();
}
