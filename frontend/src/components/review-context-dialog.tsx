// Shared review-context initialization dialog and controller.
import { api, mutate, APIError } from "../api.tsx";
import type {
  ReviewContextCandidatesResponse,
  ReviewContextMutationResponse,
} from "../api/types.ts";
import { h, Fragment, render as renderTree, cx, classAdd, classRemove } from "../jsx/jsx-runtime.ts";

/** Typed compound class names used by this module. */
const classNames = {
  uiBasicButton: cx("ui", "basic", "button"),
  uiErrorMessageRwReviewCandidateStatus: cx("ui", "error", "message", "rw-review-candidate-status"),
  uiField: cx("ui", "field"),
  uiFormRwReviewDialogForm: cx("ui", "form", "rw-review-dialog__form"),
  uiIconBasicButtonRwReviewDialogClose: cx("ui", "icon", "basic", "button", "rw-review-dialog__close"),
  uiInfoMessage: cx("ui", "info", "message"),
  uiInfoMessageRwReviewCandidateStatus: cx("ui", "info", "message", "rw-review-candidate-status"),
  uiPrimaryButton: cx("ui", "primary", "button"),
  uiSelectionDropdown: cx("ui", "selection", "dropdown"),
  uiSuccessMessageRwReviewCandidateStatus: cx("ui", "success", "message", "rw-review-candidate-status"),
  uiWarningMessageRwReviewCandidateStatus: cx("ui", "warning", "message", "rw-review-candidate-status"),
};

/** One proposed parent review context returned by the server. */
export interface ProposedParent {
  context_id: number;
  pipeline_run_id: number;
  search_id: any;
  search_revision: any;
  inherited_work_count: number;
}

/** Options used to bind one review-context initialization surface. */
export interface ReviewContextInitializerOptions {
  runID: number;
  proposed: ProposedParent | null;
  onInitialized: () => Promise<void>;
}

/** Summarizes the server-proposed review lineage without hiding the empty-context choice. */
export function reviewContextSummary(proposed: ProposedParent | null): string {
  if (!proposed) {
    return "No earlier compatible review context was proposed. You can start this run with an empty review context.";
  }
  var plural = "s";
  if (proposed.inherited_work_count === 1) plural = "";
  return `Run ${proposed.pipeline_run_id} from ${proposed.search_id} / ${proposed.search_revision} contains ${proposed.inherited_work_count} matching work${plural}.`;
}

/** Renders the shared immutable-lineage selection dialog. */
export function ReviewContextDialog(props: { proposed: ProposedParent | null }): JSX.Element {
  const proposedSummary = reviewContextSummary(props.proposed);
  var recommendedOption: JSX.Element | null = null;
  if (props.proposed) {
    recommendedOption = (
      <option selected value={props.proposed.context_id}>
        {`Recommended: run ${props.proposed.pipeline_run_id} · ${props.proposed.search_id} / ${props.proposed.search_revision} · ${props.proposed.inherited_work_count} matching`}
      </option>
    );
  }
  return (
    <dialog className="rw-review-dialog" data-review-dialog aria-labelledby="review-dialog-title" aria-describedby="review-dialog-description">
      <form className={classNames.uiFormRwReviewDialogForm} data-review-context-form>
        <div className="rw-review-dialog__header">
          <div>
            <p className="rw-review-dialog__eyebrow">Review lineage</p>
            <h3 id="review-dialog-title">Start article review</h3>
            <p id="review-dialog-description">Choose which earlier review context to inherit, or start empty. This choice cannot be changed after initialization.</p>
          </div>
          <button type="button" className={classNames.uiIconBasicButtonRwReviewDialogClose} data-review-close aria-label="Close review setup">{"\u00D7"}</button>
        </div>
        <div className="rw-review-dialog__body">
          <div className={classNames.uiInfoMessage}>
            <span className="header">Recommended starting point</span>
            {`${proposedSummary} Inheritance is frozen when review starts.`}
          </div>
          <div className={classNames.uiField}>
            <label htmlFor="review-parent-context">Parent review context</label>
            <div className={classNames.uiSelectionDropdown}>
              <select id="review-parent-context" data-review-parent>
                <option value="">Start empty with no inherited review evidence</option>
                {recommendedOption}
              </select>
            </div>
            <p className="rw-field-help">Only earlier review contexts are eligible. Matching work heads are copied by immutable version reference.</p>
          </div>
          <section className="rw-review-candidate-panel" aria-labelledby="review-candidate-heading">
            <div>
              <h4 id="review-candidate-heading">Available context scope</h4>
              <p>Same-search contexts load automatically. Expand only when you intentionally need lineage from another search.</p>
            </div>
            <button type="button" className={classNames.uiBasicButton} data-all-review-candidates>Include all earlier searches</button>
          </section>
          <div className={classNames.uiInfoMessageRwReviewCandidateStatus} data-review-candidates aria-live="polite">
            <span className="header">Same-search contexts</span>
            Open this dialog to load eligible alternatives.
          </div>
          <button type="button" className={classNames.uiBasicButton} data-more-review-candidates hidden>Load more eligible contexts</button>
        </div>
        <div className="rw-review-dialog__actions">
          <button type="button" className={classNames.uiBasicButton} data-review-cancel>Cancel</button>
          <button type="submit" className={classNames.uiPrimaryButton} data-confirm-review>Initialize review</button>
        </div>
      </form>
    </dialog>
  );
}

/** Binds one shared review-context dialog and returns an explicit opener. */
export function bindReviewContextInitializer(host: HTMLElement, options: ReviewContextInitializerOptions): { open: () => Promise<void> } {
  const dialog = host.querySelector<HTMLDialogElement>("[data-review-dialog]")!;
  const candidateCursors: Record<string, string> = { same_search: "", all: "" };
  let sameSearchLoaded = false;
  let allSearchesLoaded = false;
  let candidateScope = "same_search";
  let initializingContext = false;
  let opener: HTMLElement | null = null;

  /** Closes the setup dialog and returns focus to the exact opener. */
  function closeDialog(): void {
    if (initializingContext) return;
    if (typeof dialog.close === "function") dialog.close();
    else dialog.removeAttribute("open");
    opener?.focus();
  }

  /** Adds one bounded page of eligible parents for the requested search scope. */
  async function appendCandidates(scope: string): Promise<void> {
    const status = host.querySelector<HTMLElement>("[data-review-candidates]")!;
    const expandButton = host.querySelector<HTMLButtonElement>("[data-all-review-candidates]")!;
    const moreButton = host.querySelector<HTMLButtonElement>("[data-more-review-candidates]")!;
    candidateScope = scope;
    status.className = classNames.uiInfoMessageRwReviewCandidateStatus;
    var scopeLabel = "same-search";
    if (scope === "all") scopeLabel = "cross-search";
    const loadingMarkup = (
      <Fragment>
        <span className="header">Searching review history</span>
        Loading eligible {scopeLabel} contexts.
      </Fragment>
    );
    renderTree(loadingMarkup, status);
    if (scope === "all") {
      expandButton.disabled = true;
      classAdd(expandButton, ["loading"]);
    }
    try {
      const candidates = await api<ReviewContextCandidatesResponse>(`/api/runs/${options.runID}/review-context-candidates`, {
        scope: scope,
        limit: 25,
        cursor: candidateCursors[scope],
      }, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      const select = host.querySelector<HTMLSelectElement>("[data-review-parent]")!;
      candidateCursors[scope] = candidates.next_cursor || "";
      moreButton.hidden = !candidates.has_more;
      let added = 0;
      for (const candidate of candidates.rows || []) {
        const exists = Array.from(select.options).some((option) => {
          return option.value === String(candidate.context_id);
        });
        if (exists) continue;
        const option = document.createElement("option");
        option.value = String(candidate.context_id);
        option.textContent = `${candidate.search_id} / ${candidate.search_revision} / run ${candidate.pipeline_run_id} · ${candidate.inherited_work_count} matching`;
        select.append(option);
        added += 1;
      }
      const total = Math.max(0, select.options.length - 1);
      if (scope === "all") {
        allSearchesLoaded = true;
        expandButton.textContent = "All earlier searches included";
        var addedSummary = "No additional cross-search contexts were found. ";
        if (added) {
          var addedVerb = "s were";
          if (added === 1) addedVerb = " was";
          addedSummary = `${added} additional eligible context${addedVerb} added. `;
        }
        var totalVerb = "s are";
        if (total === 1) totalVerb = " is";
        const allMarkup = (
          <Fragment>
            <span className="header">All earlier searches checked</span>
            {addedSummary}{total} total parent option{totalVerb} available.
          </Fragment>
        );
        renderTree(allMarkup, status);
      } else {
        sameSearchLoaded = true;
        var sameSummary = "No same-search parent is available. Start empty or deliberately include all earlier searches.";
        if (total) {
          var sameVerb = "s are";
          if (total === 1) sameVerb = " is";
          sameSummary = `${total} eligible parent option${sameVerb} available, including the recommendation when present.`;
        }
        const sameMarkup = (
          <Fragment>
            <span className="header">Same-search contexts ready</span>
            {sameSummary}
          </Fragment>
        );
        renderTree(sameMarkup, status);
      }
    } catch (error: any) {
      status.className = classNames.uiErrorMessageRwReviewCandidateStatus;
      const errorMarkup = (
        <Fragment>
          <span className="header">Context search failed</span>
          {error.message}
        </Fragment>
      );
      renderTree(errorMarkup, status);
      if (scope === "all") expandButton.disabled = false;
    } finally {
      classRemove(expandButton, "loading");
    }
  }

  /** Opens the modal and loads the first eligible-parent page once. */
  async function open(): Promise<void> {
    opener = document.activeElement as HTMLElement | null;
    dialog.showModal?.();
    if (!dialog.open) dialog.setAttribute("open", "");
    if (!sameSearchLoaded) await appendCandidates("same_search");
  }

  host.querySelector<HTMLElement>("[data-start-review]")?.addEventListener("click", () => {
    void open();
  });
  host.querySelector<HTMLElement>("[data-all-review-candidates]")!.addEventListener("click", () => {
    if (!allSearchesLoaded) void appendCandidates("all");
  });
  host.querySelector<HTMLElement>("[data-more-review-candidates]")!.addEventListener("click", () => {
    void appendCandidates(candidateScope);
  });
  host.querySelector<HTMLElement>("[data-review-close]")!.addEventListener("click", closeDialog);
  host.querySelector<HTMLElement>("[data-review-cancel]")!.addEventListener("click", closeDialog);
  dialog.addEventListener("click", (event) => {
    if (event.target === dialog) closeDialog();
  });
  dialog.addEventListener("cancel", (event) => {
    if (initializingContext) event.preventDefault();
  });
  host.querySelector<HTMLFormElement>("[data-review-context-form]")!.addEventListener("submit", async (event) => {
    event.preventDefault();
    const raw = host.querySelector<HTMLSelectElement>("[data-review-parent]")!.value;
    const button = host.querySelector<HTMLButtonElement>("[data-confirm-review]")!;
    const status = host.querySelector<HTMLElement>("[data-review-candidates]")!;
    const cancelButton = host.querySelector<HTMLButtonElement>("[data-review-cancel]")!;
    const closeButton = host.querySelector<HTMLButtonElement>("[data-review-close]")!;
    var parentContextID: number | null = null;
    if (raw) parentContextID = Number(raw);
    initializingContext = true;
    button.disabled = true;
    classAdd(button, ["loading"]);
    cancelButton.disabled = true;
    closeButton.disabled = true;
    try {
      await mutate<ReviewContextMutationResponse>(`/api/runs/${options.runID}/review-context`, "POST", { parent_context_id: parentContextID });
    } catch (error: any) {
      initializingContext = false;
      status.className = classNames.uiErrorMessageRwReviewCandidateStatus;
      var title = "Review could not be initialized";
      if (error instanceof APIError && error.code === "context_parent_conflict") title = "Review was initialized elsewhere";
      const errorMarkup = (
        <Fragment>
          <span className="header">{title}</span>
          {error.message}
        </Fragment>
      );
      renderTree(errorMarkup, status);
      button.disabled = false;
      classRemove(button, "loading");
      cancelButton.disabled = false;
      closeButton.disabled = false;
      return;
    }
    initializingContext = false;
    closeDialog();
    status.className = classNames.uiSuccessMessageRwReviewCandidateStatus;
    const savedMarkup = (
      <Fragment>
        <span className="header">Review context initialized</span>
        The immutable lineage choice is saved. Loading review tools.
      </Fragment>
    );
    renderTree(savedMarkup, status);
    try {
      await options.onInitialized();
    } catch (error: any) {
      status.className = classNames.uiWarningMessageRwReviewCandidateStatus;
      const refreshMarkup = (
        <Fragment>
          <span className="header">Review context saved, refresh failed</span>
          {error.message}
          <button type="button" className={classNames.uiBasicButton} data-review-refresh>Retry review tools</button>
        </Fragment>
      );
      renderTree(refreshMarkup, status);
      status.querySelector<HTMLButtonElement>("[data-review-refresh]")?.addEventListener("click", () => {
        void options.onInitialized();
      });
    }
  });

  return { open: open };
}
