// Run-scoped review context, complete status versions, notes, and PDF anchors.
import { api, mutate, APIError } from "../api.tsx";
import type {
  AnchorRectangle,
  ArticleDetailResponse,
  ArticleRecord,
  ArticleReviewResponse,
  HealthResponse,
  ReviewAnchor,
  ReviewAnchorCreateResponse,
  ReviewAnchorMutationResponse,
  ReviewAnchorsResponse,
  ReviewAnchorVersionsResponse,
  ReviewContextResponse,
  WorkReviewMutationResponse,
  WorkReviewVersionResponse,
  WorkReviewVersionsResponse,
} from "../api/types.ts";
import { formatTime, humanLabel, link } from "../state.tsx";
import { h, Fragment, render as renderTree, cx, classToggle, classAdd, classRemove } from "../jsx/jsx-runtime.ts";
import { mountNoteEditor } from "./note-editor.tsx";
import { mountPDFViewer } from "./pdf-viewer.tsx";
import { bindReviewContextInitializer, ReviewContextDialog, reviewContextSummary } from "./review-context-dialog.tsx";
import type { ProposedParent } from "./review-context-dialog.tsx";
import { mountBacklinks } from "./backlinks.tsx";

/** Typed compound class names used by this module. */
const classNames = {
  itemActive: cx("item", "active"),
  rwReviewSectionRwAnchorPanel: cx("rw-review-section", "rw-anchor-panel"),
  uiBasicButton: cx("ui", "basic", "button"),
  uiBlueLabel: cx("ui", "blue", "label"),
  uiDangerButton: cx("ui", "danger", "button"),
  uiErrorMessage: cx("ui", "error", "message"),
  uiErrorMessageRwReviewFeedback: cx("ui", "error", "message", "rw-review-feedback"),
  uiFadedText: cx("ui", "faded", "text"),
  uiField: cx("ui", "field"),
  uiFormRwAnchorCandidate: cx("ui", "form", "rw-anchor-candidate"),
  uiFormRwReviewForm: cx("ui", "form", "rw-review-form"),
  uiGreenLabel: cx("ui", "green", "label"),
  uiInfoMessage: cx("ui", "info", "message"),
  uiInfoMessageRwReviewFeedback: cx("ui", "info", "message", "rw-review-feedback"),
  uiLabel: cx("ui", "label"),
  uiNegativeMessage: cx("ui", "negative", "message"),
  uiNeutralLabel: cx("ui", "neutral", "label"),
  uiPrimaryButton: cx("ui", "primary", "button"),
  uiRedLabel: cx("ui", "red", "label"),
  uiSegmentRwReviewPanel: cx("ui", "segment", "rw-review-panel"),
  uiSegmentRwReviewPanelRwReviewPanelEmpty: cx("ui", "segment", "rw-review-panel", "rw-review-panel--empty"),
  uiSelectionDropdown: cx("ui", "selection", "dropdown"),
  uiSuccessMessage: cx("ui", "success", "message"),
  uiSuccessMessageRwReviewFeedback: cx("ui", "success", "message", "rw-review-feedback"),
  uiTabularMenuRwReviewNav: cx("ui", "tabular", "menu", "rw-review-nav"),
  uiTopAttachedHeader: cx("ui", "top", "attached", "header"),
  uiVioletLabel: cx("ui", "violet", "label"),
  uiWarningMessage: cx("ui", "warning", "message"),
  uiWarningMessageRwReviewFeedback: cx("ui", "warning", "message", "rw-review-feedback"),
};

const statuses = ["not_evaluated", "in_progress", "approved", "not_approved", "removed"];
const substatuses = ["redacted", "unrelated", "out_of_scope", "duplicate", "retracted", "withdrawn", "superseded", "predatory_low_quality", "copyright_licensing", "not_peer_reviewed"];
let reviewHealthPromise: Promise<HealthResponse> | null = null;

/** Loads immutable viewer capability data once per page and retries after a failed request. */
async function reviewHealth(): Promise<HealthResponse> {
  if (!reviewHealthPromise) {
    reviewHealthPromise = api<HealthResponse>("/api/health", {}, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
  }
  try {
    return await reviewHealthPromise;
  } catch (error) {
    reviewHealthPromise = null;
    throw error;
  }
}

/** One PDF text selection captured by the reader. */
export interface PDFSelection {
  page: number;
  selectedText: string;
  rectangles: AnchorRectangle[];
}

/** One anchor head returned by the review anchors API. */
export type AnchorHead = ReviewAnchor;

/** Mounts all editable review controls for one immutable run article revision. */
export async function mountArticleReview(host: HTMLElement, pdfHost: HTMLElement | null, record: any, detailData: any, onAuditChange?: () => Promise<void>): Promise<{ destroy: () => any }> {
  const runID = Number(record.pipeline_run_id);
  const revisionID = Number(record.id);
  const workID = Number(record.work_id);
  let pdfController: any = null;
  let pendingSelection: PDFSelection | null = null;
  let reviewEditable = false;
  let notesEditable = false;
  let anchorsEditable = false;
  let loadedAnchors: AnchorHead[] = [];
  let anchorCursor = "";
  let anchorHasMore = false;
  let setReviewSection: (name: string) => void = () => {};
  let activateReviewSection: (name: string) => Promise<void> = async (name) => {
    setReviewSection(name);
  };

  if (detailData.pdf_status?.status === "available" && pdfHost) {
    pdfController = await mountPDFViewer(pdfHost, {
      url: `/api/pdf/${workID}`,
      page: Number(new URLSearchParams(location.search).get("pdf_page") || 1),
      onPageChange: (page: number) => {
        history.replaceState({}, "", link({ pdf_page: page }));
      },
      onSelection: (selection: PDFSelection) => {
        pendingSelection = selection;
        host.closest(".rw-reading-workspace")?.dispatchEvent(new CustomEvent("rw-pdf-selection"));
        void activateReviewSection("anchors").then(() => {
          renderAnchorCandidate();
          const anchorLabel = host.querySelector<HTMLInputElement>("[data-anchor-label]");
          anchorLabel?.focus();
          const candidate = host.querySelector<HTMLElement>("[data-anchor-candidate]");
          candidate?.scrollIntoView({ block: "nearest" });
        });
      },
    }).catch((error: any) => {
      const errorMarkup = <p className={classNames.uiNegativeMessage}>The embedded PDF could not be rendered. The original PDF remains available through the download endpoint: {error.message}</p>;
      renderTree(errorMarkup, pdfHost!);
      return null;
    });
  }

  var context: ReviewContextResponse;
  try {
    context = await api<ReviewContextResponse>(`/api/runs/${runID}/review-context`, {}, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
  } catch (error: any) {
    const contextErrorMarkup = (
      <section className={classNames.uiErrorMessage} role="alert">
        <span className="header">Article review is unavailable</span>
        <p>{error.message}</p>
        <p>The immutable article and document reader remain available.</p>
        <button type="button" className={classNames.uiBasicButton} data-review-context-retry>Retry Article review</button>
      </section>
    );
    renderTree(contextErrorMarkup, host);
    host.querySelector<HTMLButtonElement>("[data-review-context-retry]")?.addEventListener("click", () => {
      void mountArticleReview(host, null, record, detailData, onAuditChange);
    });
    return { destroy: () => { return pdfController?.destroy(); } };
  }

  if (!context.context_initialized) {
    renderStartReview(context.proposed_parent ?? null);
    return { destroy: () => { return pdfController?.destroy(); } };
  }
  await renderReview();
  return { destroy: () => { return pdfController?.destroy(); } };

  /** Renders explicit context initialization with safe parent confirmation. */
  function renderStartReview(proposed: ProposedParent | null): void {
    const proposedSummary = reviewContextSummary(proposed);
    const startReviewMarkup = (
      <section className={classNames.uiSegmentRwReviewPanelRwReviewPanelEmpty}>
        <div className={classNames.uiTopAttachedHeader}>
          <div>
            <h3>Article review</h3>
            <p>Record a run-scoped decision, notes, and PDF anchors without changing pipeline evidence.</p>
          </div>
          <span className={classNames.uiLabel}>Not started</span>
        </div>
        <div className="content">
          <div className="rw-review-onboarding">
            <div>
              <h4>Start a review context for this run</h4>
              <p>Starting review freezes any inherited article decisions, notes, and anchors so later changes remain independent.</p>
            </div>
            <button type="button" className={classNames.uiPrimaryButton} data-start-review>Start review</button>
          </div>
          <p className={classNames.uiInfoMessage}><span className="header">Suggested lineage</span>{proposedSummary}</p>
        </div>
        <ReviewContextDialog proposed={proposed} />
      </section>
    );
    renderTree(startReviewMarkup, host);
    bindReviewContextInitializer(host, {
      runID: runID,
      proposed: proposed,
      onInitialized: async () => {
        await renderReview();
        try {
          await onAuditChange?.();
        } catch (_) {
          const message = host.querySelector<HTMLElement>("[data-review-message]");
          if (message) {
            message.className = classNames.uiWarningMessageRwReviewFeedback;
            message.textContent = "Review context was saved, but the article audit display could not be refreshed.";
          }
        }
      },
    });
  }

  /** Loads and binds complete status state, history, notes, PDF, and anchors. */
  async function renderReview(): Promise<void> {
    const data = await api<ArticleReviewResponse>(`/api/runs/${runID}/articles/${revisionID}/review`, {}, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    reviewEditable = data.editability?.decision ?? data.editable;
    notesEditable = data.editability?.notes ?? data.editable;
    anchorsEditable = data.editability?.anchors ?? (data.editable && detailData.pdf_status?.status === "available");
    const state = data.review;
    const version = state?.version;
    const selectedStatus = version?.status || "not_evaluated";
    const selectedSubstatuses = new Set(version?.sub_statuses || []);
    var contextLabel: JSX.Element = <span className={classNames.uiNeutralLabel}>This context</span>;
    if (state?.inherited_from_context_id) {
      contextLabel = <span className={classNames.uiVioletLabel}>Inherited from context {state.inherited_from_context_id}</span>;
    }
    const statusOptions = statuses.map((status) => {
      return <option value={status} selected={selectedStatus === status}>{humanLabel(status)}</option>;
    });
    const substatusOptions = substatuses.map((status) => {
      return (
        <label className="rw-review-check">
          <input type="checkbox" value={status} checked={selectedSubstatuses.has(status)} disabled={!reviewEditable} />
          <span>{humanLabel(status)}</span>
        </label>
      );
    });
    var feedbackMarkup: JSX.Element;
    if (version) {
      feedbackMarkup = (
        <>
          <span className="header">Current saved version</span>
          Version {version.id} by {version.reviewer_display} at {formatTime(version.created_at)}.
        </>
      );
    } else {
      feedbackMarkup = (
        <>
          <span className="header">No saved decision yet</span>
          The current context defaults to Not Evaluated until you save a complete review state.
        </>
      );
    }
    const reviewMarkup = (
      <section className={classNames.uiSegmentRwReviewPanel}>
        <div className={classNames.uiTopAttachedHeader}>
          <div>
            <h3>Article review</h3>
            <p>Run-scoped decisions, notes, and anchors append immutable versions.</p>
          </div>
          {contextLabel}
        </div>
        <div className="content">
          <nav className={classNames.uiTabularMenuRwReviewNav} aria-label="Article review sections" role="tablist">
            <button id="review-tab-decision" type="button" className={classNames.itemActive} role="tab" data-review-section="decision" aria-selected="true" aria-controls="review-panel-decision" tabIndex={0}>Decision</button>
            <button id="review-tab-notes" type="button" className="item" role="tab" data-review-section="notes" aria-selected="false" aria-controls="review-panel-notes" tabIndex={-1}>Notes</button>
            <button id="review-tab-anchors" type="button" className="item" role="tab" data-review-section="anchors" aria-selected="false" aria-controls="review-panel-anchors" tabIndex={-1}>PDF anchors</button>
          </nav>
          <section id="review-panel-decision" className="rw-review-section" role="tabpanel" data-review-section-panel="decision" aria-labelledby="review-tab-decision">
            <div className="rw-review-section__heading">
              <div>
                <h4 id="review-decision-heading">Review decision</h4>
                <p>Save the complete state for this article in the selected run context.</p>
              </div>
            </div>
            <form className={classNames.uiFormRwReviewForm} data-review-form>
              <div className="rw-review-form__primary">
                <div className={classNames.uiField}>
                  <label htmlFor="article-review-status">Decision status</label>
                  <div className={classNames.uiSelectionDropdown}>
                    <select id="article-review-status" data-review-status disabled={!reviewEditable}>{statusOptions}</select>
                  </div>
                </div>
                <div className={classNames.uiField}>
                  <label htmlFor="article-review-reason">Reason or review summary <span className="rw-optional">Optional</span></label>
                  <textarea id="article-review-reason" rows={4} data-review-reason maxLength={32768} disabled={!reviewEditable}>{version?.reason || ""}</textarea>
                  <p className="rw-field-help">The saved reason is included in the append-only audit change for this decision.</p>
                </div>
              </div>
              <fieldset className="rw-review-substatuses" data-review-substatuses>
                <legend>Decision qualifiers</legend>
                <p>Select qualifiers only when the status is Not Approved or Removed.</p>
                <div className="rw-review-option-grid">{substatusOptions}</div>
              </fieldset>
              <div className={classNames.uiInfoMessageRwReviewFeedback} data-review-message aria-live="polite">{feedbackMarkup}</div>
              <div className="rw-review-actions">
                <button type="submit" className={classNames.uiPrimaryButton} data-review-save disabled={!reviewEditable}>Save review decision</button>
                <button type="button" className={classNames.uiBasicButton} data-review-history aria-expanded="false">Show version history</button>
              </div>
            </form>
            <div className="rw-review-history-panel" data-review-history-list hidden></div>
          </section>
          <section id="review-panel-notes" className="rw-review-section" role="tabpanel" data-review-section-panel="notes" aria-labelledby="review-tab-notes" hidden>
            <div data-note-host></div>
            <details className="rw-review-history">
              <summary>Inbound links to this article</summary>
              <div>
                <button type="button" className={classNames.uiBasicButton} data-article-backlinks disabled={!record.doi}>Load article backlinks</button>
                <div data-article-backlink-list></div>
              </div>
            </details>
          </section>
          <section id="review-panel-anchors" className={classNames.rwReviewSectionRwAnchorPanel} role="tabpanel" data-review-section-panel="anchors" aria-labelledby="review-tab-anchors" hidden>
            <div className="rw-review-section__heading">
              <div>
                <h4 id="review-anchors-heading">PDF anchors</h4>
                <p>Select text in the document reader, then save a named anchor for this review context.</p>
              </div>
            </div>
            <div data-anchor-candidate></div>
            <div data-anchor-list></div>
            <details className="rw-review-history">
              <summary>Inbound links to the current PDF page</summary>
              <div>
                <button type="button" className={classNames.uiBasicButton} data-page-backlinks>Load page backlinks</button>
                <div data-page-backlink-list></div>
              </div>
            </details>
          </section>
        </div>
      </section>
    );
    renderTree(reviewMarkup, host);
    const sectionButtons = Array.from(host.querySelectorAll<HTMLButtonElement>("[data-review-section]"));
    const sectionPanels = Array.from(host.querySelectorAll<HTMLElement>("[data-review-section-panel]"));
    let notesLoaded = false;
    let anchorsLoaded = false;
    /** Switches visible review content without hiding its section identity or state. */
    setReviewSection = (name: string) => {
      sectionButtons.forEach((button) => {
        const active = button.dataset.reviewSection === name;
        classToggle(button, "active", active);
        button.setAttribute("aria-selected", String(active));
        var tabIndex = -1;
        if (active) tabIndex = 0;
        button.tabIndex = tabIndex;
      });
      sectionPanels.forEach((panel) => {
        panel.hidden = panel.dataset.reviewSectionPanel !== name;
      });
    };
    /** Activates one review tab and isolates optional panel loading failures. */
    activateReviewSection = async (name: string): Promise<void> => {
      setReviewSection(name);
      if (name === "notes" && !notesLoaded) {
        const noteHost = host.querySelector("[data-note-host]") as HTMLElement;
        const loadingMarkup = <p className={classNames.uiInfoMessage}>Loading Notes.</p>;
        renderTree(loadingMarkup, noteHost);
        try {
          const health = await reviewHealth();
          await mountNoteEditor(noteHost, {
            corpusID: health.corpus_id,
            runID: runID,
            workRevisionID: revisionID,
            articleDOI: record.doi || "",
            editable: notesEditable,
            onChanged: onAuditChange,
          });
          notesLoaded = true;
        } catch (error: any) {
          const errorMarkup = (
            <p className={classNames.uiErrorMessage} role="alert">
              <span className="header">Notes could not be loaded</span>
              {error.message}
              <button type="button" className={classNames.uiBasicButton} data-notes-retry>Retry Notes</button>
            </p>
          );
          renderTree(errorMarkup, noteHost);
          noteHost.querySelector<HTMLButtonElement>("[data-notes-retry]")?.addEventListener("click", () => {
            void activateReviewSection("notes");
          });
        }
      }
      if (name === "anchors" && !anchorsLoaded) {
        const anchorHost = host.querySelector("[data-anchor-list]") as HTMLElement;
        const loadingMarkup = <p className={classNames.uiInfoMessage}>Loading PDF anchors.</p>;
        renderTree(loadingMarkup, anchorHost);
        try {
          await loadAnchors(true);
          renderAnchorCandidate();
          anchorsLoaded = true;
        } catch (error: any) {
          const errorMarkup = (
            <p className={classNames.uiErrorMessage} role="alert">
              <span className="header">PDF anchors could not be loaded</span>
              {error.message}
              <button type="button" className={classNames.uiBasicButton} data-anchors-retry>Retry PDF anchors</button>
            </p>
          );
          renderTree(errorMarkup, anchorHost);
          anchorHost.querySelector<HTMLButtonElement>("[data-anchors-retry]")?.addEventListener("click", () => {
            void activateReviewSection("anchors");
          });
        }
      }
    };
    sectionButtons.forEach((button) => {
      button.addEventListener("click", () => {
        void activateReviewSection(button.dataset.reviewSection as string);
      });
    });
    host.querySelector<HTMLButtonElement>("[data-article-backlinks]")?.addEventListener("click", async (event) => {
      const button = event.currentTarget as HTMLButtonElement;
      const target = host.querySelector<HTMLElement>("[data-article-backlink-list]")!;
      button.disabled = true;
      await mountBacklinks(target, {
        runID: runID,
        targetType: "article",
        targetID: String(record.doi),
        heading: "Notes linking to this article",
      });
    });
    host.querySelector<HTMLButtonElement>("[data-page-backlinks]")?.addEventListener("click", async (event) => {
      const button = event.currentTarget as HTMLButtonElement;
      const target = host.querySelector<HTMLElement>("[data-page-backlink-list]")!;
      const currentPage = new URLSearchParams(location.search).get("pdf_page") || "1";
      button.disabled = true;
      await mountBacklinks(target, {
        runID: runID,
        targetType: "pdf_page",
        targetID: currentPage,
        workRevisionID: revisionID,
        heading: `Notes linking to PDF page ${currentPage}`,
      });
    });
    host.querySelector<HTMLElement>(".rw-review-nav")!.addEventListener("keydown", (event: KeyboardEvent) => {
      const current = sectionButtons.indexOf(document.activeElement as HTMLButtonElement);
      if (current < 0) return;
      let target = current;
      if (event.key === "ArrowRight" || event.key === "ArrowDown") target = (current + 1) % sectionButtons.length;
      else if (event.key === "ArrowLeft" || event.key === "ArrowUp") target = (current - 1 + sectionButtons.length) % sectionButtons.length;
      else if (event.key === "Home") target = 0;
      else if (event.key === "End") target = sectionButtons.length - 1;
      else return;
      event.preventDefault();
      void activateReviewSection(sectionButtons[target].dataset.reviewSection as string);
      sectionButtons[target].focus();
    });
    const statusSelect = host.querySelector("[data-review-status]") as HTMLSelectElement;
    const substatusField = host.querySelector("[data-review-substatuses]") as HTMLFieldSetElement;
    const reviewForm = host.querySelector("[data-review-form]") as HTMLFormElement;
    const reasonInput = host.querySelector("[data-review-reason]") as HTMLTextAreaElement;
    let expectedVersionID = version?.id || null;
    /** Serializes only user-editable decision input for dirty-state comparison. */
    function decisionDraft(): string {
      const checked = Array.from(substatusField.querySelectorAll<HTMLInputElement>("input:checked")).map((input) => input.value).sort();
      return JSON.stringify({ status: statusSelect.value, reason: reasonInput.value, qualifiers: checked });
    }
    let savedDecisionDraft = decisionDraft();
    /** Prevents route changes from silently discarding a local decision draft. */
    function protectDecision(event: Event): void {
      if (!reviewForm.isConnected) {
        document.removeEventListener("rw-before-navigate", protectDecision);
        window.removeEventListener("beforeunload", protectDecision);
        return;
      }
      if (decisionDraft() === savedDecisionDraft) return;
      if (event.type === "beforeunload") {
        event.preventDefault();
        (event as BeforeUnloadEvent).returnValue = "";
        return;
      }
      if (!window.confirm("Leave this article and discard the unsaved review decision?")) event.preventDefault();
    }
    document.addEventListener("rw-before-navigate", protectDecision);
    window.addEventListener("beforeunload", protectDecision);
    /** Enables sub-status choices only for the two compatible terminal statuses. */
    function updateSubstatuses(): void {
      const compatible = statusSelect.value === "not_approved" || statusSelect.value === "removed";
      substatusField.disabled = !reviewEditable || !compatible;
    }
    statusSelect.addEventListener("change", updateSubstatuses);
    updateSubstatuses();
    reviewForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const message = host.querySelector("[data-review-message]") as HTMLElement;
      const saveButton = host.querySelector("[data-review-save]") as HTMLButtonElement;
      const reasonText = reasonInput.value.trim();
      saveButton.disabled = true;
      classAdd(saveButton, ["loading"]);
      const compatible = statusSelect.value === "not_approved" || statusSelect.value === "removed";
      var checkedInputs: HTMLInputElement[] = [];
      if (compatible) checkedInputs = Array.from(substatusField.querySelectorAll<HTMLInputElement>("input:checked"));
      var saved: any;
      try {
        saved = await mutate<WorkReviewMutationResponse>(`/api/runs/${runID}/articles/${revisionID}/review`, "PUT", {
          expected_version_id: expectedVersionID,
          status: statusSelect.value,
          sub_statuses: checkedInputs.map((input) => {
            return input.value;
          }),
          reason: reasonText || null,
        });
      } catch (error: any) {
        message.className = classNames.uiErrorMessageRwReviewFeedback;
        var errorMessage = error.message;
        var rebaseAction: JSX.Element | null = null;
        if (error instanceof APIError && error.code === "version_conflict") {
          rebaseAction = <button type="button" className={classNames.uiBasicButton} data-review-load-latest>Load latest while keeping my input</button>;
        }
        if (rebaseAction) errorMessage = "A newer version exists. Your input is preserved.";
        const errorMarkup = (
          <>
            <span className="header">Review was not saved</span>
            {errorMessage}
            {rebaseAction}
          </>
        );
        renderTree(errorMarkup, message);
        saveButton.disabled = false;
        classRemove(saveButton, "loading");
        host.querySelector<HTMLButtonElement>("[data-review-load-latest]")?.addEventListener("click", async () => {
          const latest = await api<ArticleReviewResponse>(`/api/runs/${runID}/articles/${revisionID}/review`, {}, {
            method: "GET",
            headers: { Accept: "application/json" },
          });
          expectedVersionID = latest.review?.version?.id || null;
          message.className = classNames.uiWarningMessageRwReviewFeedback;
          const latestStatus = humanLabel(latest.review?.version?.status || "not_evaluated");
          const latestMarkup = (
            <Fragment>
              <span className="header">Latest saved decision loaded</span>
              <p>Version {expectedVersionID || "none"}: {latestStatus}. Your local status, reason, and qualifiers remain unchanged; save again to reapply them.</p>
              <button type="button" className={classNames.uiBasicButton} data-review-history-after-conflict>Inspect version history</button>
            </Fragment>
          );
          renderTree(latestMarkup, message);
          host.querySelector<HTMLButtonElement>("[data-review-history-after-conflict]")?.addEventListener("click", () => {
            (host.querySelector("[data-review-history]") as HTMLButtonElement).click();
          });
        });
        return;
      }
      expectedVersionID = saved.review?.version?.id || expectedVersionID;
      savedDecisionDraft = decisionDraft();
      message.className = classNames.uiSuccessMessageRwReviewFeedback;
      const savedMarkup = (
        <Fragment>
          <span className="header">Decision saved</span>
          Immutable version {expectedVersionID} is now database evidence.
        </Fragment>
      );
      renderTree(savedMarkup, message);
      saveButton.disabled = false;
      classRemove(saveButton, "loading");
      try {
        await renderReview();
      } catch (refreshError: any) {
        message.className = classNames.uiWarningMessageRwReviewFeedback;
        const refreshMarkup = (
          <Fragment>
            <span className="header">Decision saved, refresh failed</span>
            <p>{refreshError.message}</p>
            <button type="button" className={classNames.uiBasicButton} data-review-refresh>Retry refresh</button>
          </Fragment>
        );
        renderTree(refreshMarkup, message);
        message.querySelector<HTMLButtonElement>("[data-review-refresh]")?.addEventListener("click", () => { void renderReview(); });
        return;
      }
      if (saved.changed && onAuditChange) {
        try {
          await onAuditChange();
        } catch (_) {
          const currentMessage = host.querySelector("[data-review-message]") as HTMLElement;
          currentMessage.className = classNames.uiWarningMessageRwReviewFeedback;
          const auditWarningMarkup = (
            <Fragment>
              <span className="header">Decision saved</span>
              The audit display could not be refreshed. Reload the article to see the persisted event.
            </Fragment>
          );
          renderTree(auditWarningMarkup, currentMessage);
        }
      }
    });
    let decisionVersions: any[] = [];
    let decisionCursor = "";
    let decisionHasMore = false;
    /** Renders every loaded decision-summary page and lazy full-reason controls. */
    function renderDecisionHistory(): void {
      const target = host.querySelector("[data-review-history-list]") as HTMLElement;
      const historyItems = decisionVersions.map((item: any) => {
        var reasonMarkup: JSX.Element | null = null;
        if (item.reason) {
          var reasonSuffix = "";
          if (item.reason_truncated) reasonSuffix = "…";
          reasonMarkup = <blockquote data-review-reason-version={item.id}>{item.reason}{reasonSuffix}</blockquote>;
        }
        var fullReasonMarkup: JSX.Element | null = null;
        if (item.reason_truncated) fullReasonMarkup = <button type="button" className={classNames.uiBasicButton} data-review-full-version={item.id}>Load full reason</button>;
        var qualifiersMarkup: JSX.Element | null = null;
        if (item.sub_statuses?.length) {
          const qualifierLabels = item.sub_statuses.map(humanLabel);
          qualifiersMarkup = <p className="rw-review-qualifiers">{qualifierLabels.join(" · ")}</p>;
        }
        return <li><div><strong>Version {item.id} · {humanLabel(item.status)}</strong><p>{item.reviewer_display} · {formatTime(item.created_at)}</p></div>{reasonMarkup}{fullReasonMarkup}{qualifiersMarkup}</li>;
      });
      var olderMarkup: JSX.Element | null = null;
      if (decisionHasMore) olderMarkup = <button type="button" className={classNames.uiBasicButton} data-review-history-more>Load older decisions</button>;
      const historyMarkup = (
        <Fragment>
          <div className="rw-review-section__heading">
            <div>
              <h4>Version history</h4>
              <p>The newest immutable decision appears first.</p>
            </div>
          </div>
          <ol className="rw-review-history">{historyItems}</ol>
          {olderMarkup}
        </Fragment>
      );
      renderTree(historyMarkup, target);
      target.querySelector<HTMLButtonElement>("[data-review-history-more]")?.addEventListener("click", async (event) => {
        const button = event.currentTarget as HTMLButtonElement;
        button.disabled = true;
        classAdd(button, ["loading"]);
        await loadDecisionHistoryPage();
      });
      for (const button of Array.from(target.querySelectorAll<HTMLButtonElement>("[data-review-full-version]"))) {
        button.addEventListener("click", async () => {
          const versionID = button.dataset.reviewFullVersion as string;
          const data = await api<WorkReviewVersionResponse>(`/api/runs/${runID}/articles/${revisionID}/review/versions/${versionID}`, {}, {
            method: "GET",
            headers: { Accept: "application/json" },
          });
          const quote = target.querySelector(`[data-review-reason-version="${CSS.escape(versionID)}"]`) as HTMLElement;
          quote.textContent = data.version.reason || "";
          button.remove();
        });
      }
    }
    /** Appends one opaque decision-history page without replacing prior rows. */
    async function loadDecisionHistoryPage(): Promise<void> {
      const historyData = await api<WorkReviewVersionsResponse>(`/api/runs/${runID}/articles/${revisionID}/review/versions`, { limit: 25, cursor: decisionCursor }, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      const known = new Set(decisionVersions.map((item) => String(item.id)));
      for (const item of historyData.items || historyData.versions || []) {
        if (!known.has(String(item.id))) decisionVersions.push(item);
      }
      decisionCursor = historyData.next_cursor || "";
      decisionHasMore = Boolean(historyData.has_more);
      renderDecisionHistory();
    }
    host.querySelector("[data-review-history]")!.addEventListener("click", async () => {
      const button = host.querySelector("[data-review-history]") as HTMLButtonElement;
      const target = host.querySelector("[data-review-history-list]") as HTMLElement;
      if (!target.hidden) {
        target.hidden = true;
        button.setAttribute("aria-expanded", "false");
        button.textContent = "Show version history";
        return;
      }
      button.disabled = true;
      classAdd(button, ["loading"]);
      try {
        if (!decisionVersions.length) await loadDecisionHistoryPage();
        target.hidden = false;
        button.setAttribute("aria-expanded", "true");
        button.textContent = "Hide version history";
      } catch (error: any) {
        const errorMarkup = (
          <p className={classNames.uiErrorMessage}>
            <span className="header">History could not be loaded</span>
            {error.message}
          </p>
        );
        renderTree(errorMarkup, target);
        target.hidden = false;
      } finally {
        button.disabled = false;
        classRemove(button, "loading");
      }
    });
    var initialSection = "decision";
    if (pendingSelection || new URLSearchParams(location.search).get("anchor_id")) initialSection = "anchors";
    else if (new URLSearchParams(location.search).get("note_id")) initialSection = "notes";
    await activateReviewSection(initialSection);
  }

  /** Converts one current PDF text selection into an accessible anchor creation form. */
  function renderAnchorCandidate(): void {
    const targetElement = host.querySelector<HTMLElement>("[data-anchor-candidate]");
    if (!targetElement || !pendingSelection || !anchorsEditable) return;
    const selection = pendingSelection;
    const anchorFormMarkup = (
      <form className={classNames.uiFormRwAnchorCandidate} data-anchor-form>
        <div>
          <span className={classNames.uiBlueLabel}>Selection from page {selection.page}</span>
          <blockquote>{"\u201C"}{selection.selectedText}{"\u201D"}</blockquote>
        </div>
        <div className={classNames.uiField}>
          <label htmlFor="review-anchor-label">Anchor label</label>
          <input id="review-anchor-label" required pattern="[A-Za-z][A-Za-z0-9._-]{0,63}" placeholder="methods-sample" data-anchor-label />
          <p className="rw-field-help">Begin with a letter, then use letters, numbers, periods, underscores, or hyphens.</p>
        </div>
        <div className="rw-review-actions">
          <button type="submit" className={classNames.uiPrimaryButton} data-anchor-save>Save anchor</button>
          <button type="button" className={classNames.uiBasicButton} data-anchor-discard>Discard selection</button>
        </div>
        <p data-anchor-message aria-live="polite"></p>
      </form>
    );
    renderTree(anchorFormMarkup, targetElement);
    const discardButton = targetElement.querySelector("[data-anchor-discard]");
    discardButton!.addEventListener("click", () => {
      pendingSelection = null;
      targetElement.textContent = "";
      window.getSelection?.()?.removeAllRanges?.();
    });
    const anchorForm = targetElement.querySelector("form");
    anchorForm!.addEventListener("submit", async (event) => {
      event.preventDefault();
      const button = targetElement.querySelector("[data-anchor-save]") as HTMLButtonElement;
      const message = targetElement.querySelector("[data-anchor-message]") as HTMLElement;
      button.disabled = true;
      classAdd(button, ["loading"]);
      try {
        await mutate<ReviewAnchorCreateResponse>(`/api/runs/${runID}/articles/${revisionID}/anchors`, "POST", {
          label: (targetElement.querySelector("[data-anchor-label]") as HTMLInputElement).value,
          page: selection.page,
          selected_text: selection.selectedText,
          rectangles: selection.rectangles,
        });
        pendingSelection = null;
        window.getSelection?.()?.removeAllRanges?.();
        message.className = classNames.uiSuccessMessage;
        message.textContent = "PDF anchor saved as immutable review evidence.";
        classRemove(button, "loading");
        try {
          await loadAnchors();
          await onAuditChange?.();
          targetElement.textContent = "";
        } catch (refreshError: any) {
          message.className = classNames.uiWarningMessage;
          message.textContent = `PDF anchor saved, refresh failed: ${refreshError.message}`;
        }
      } catch (error: any) {
        message.className = classNames.uiErrorMessage;
        const errorMarkup = (
          <>
            <span className="header">Anchor was not saved</span>
            {error.message}
          </>
        );
        renderTree(errorMarkup, message);
        button.disabled = false;
        classRemove(button, "loading");
      }
    });
  }

  /** Loads bounded active anchor heads, textual controls, and content-matched highlights. */
  async function loadAnchors(reset = true): Promise<void> {
    const target = host.querySelector("[data-anchor-list]") as HTMLElement | null;
    if (!target) return;
    if (reset) {
      loadedAnchors = [];
      anchorCursor = "";
      anchorHasMore = false;
    }
    const data = await api<ReviewAnchorsResponse>(`/api/runs/${runID}/articles/${revisionID}/anchors`, { limit: 25, cursor: anchorCursor }, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    const known = new Set(loadedAnchors.map((anchor) => anchor.id));
    for (const anchor of data.items || data.anchors || []) {
      if (!known.has(anchor.id)) loadedAnchors.push(anchor);
    }
    anchorCursor = data.next_cursor || "";
    anchorHasMore = Boolean(data.has_more);
    const activeAnchors = loadedAnchors;
    var anchorsMarkup: JSX.Element = <p className={classNames.uiFadedText}>No active anchors. Select PDF text to add one, or use this keyboard-operable list to revisit existing anchors.</p>;
    if (activeAnchors.length) {
      const anchorItems = activeAnchors.map((anchor) => {
        const mismatch = anchor.version.pdf_content_hash !== detailData.pdf_status?.content_hash;
        const anchorLabel = anchor.label || anchor.id;
        var contextLabel: JSX.Element = <span className={classNames.uiNeutralLabel}>This context</span>;
        if (anchor.inherited_from_context_id) {
          contextLabel = <span className={classNames.uiVioletLabel}>Inherited</span>;
        }
        var statusClass = classNames.uiGreenLabel;
        var statusText = "Available";
        if (mismatch) {
          statusClass = classNames.uiRedLabel;
          statusText = "PDF changed";
        }
        var pageButtonText = `Open page ${anchor.version.page}`;
        var pageButtonLabel = `Open anchor ${anchorLabel} on PDF page ${anchor.version.page}`;
        if (mismatch) {
          pageButtonText = "Page unavailable for changed PDF";
          pageButtonLabel = `Anchor ${anchorLabel} belongs to different PDF content`;
        }
        return (
          <li data-anchor-id={anchor.id}>
            <div className="rw-anchor-card__meta">
              <div>
                <span className={classNames.uiLabel}>{anchorLabel}</span>
                <span className={classNames.uiLabel}>Page {anchor.version.page}</span>
                {contextLabel}
              </div>
              <span className={statusClass}>{statusText}</span>
            </div>
            <blockquote>{anchor.version.selected_text || ""}</blockquote>
            <div className="rw-anchor-card__actions">
              <button type="button" className={classNames.uiPrimaryButton} data-anchor-page={anchor.version.page} disabled={mismatch} aria-label={pageButtonLabel}>{pageButtonText}</button>
              <button type="button" className={classNames.uiBasicButton} data-anchor-history>History</button>
              <button type="button" className={classNames.uiDangerButton} data-anchor-delete disabled={!anchorsEditable}>Remove</button>
            </div>
          </li>
        );
      });
      anchorsMarkup = <ul className="rw-anchor-list">{anchorItems}</ul>;
    }
    var loadMoreMarkup: JSX.Element | null = null;
    if (anchorHasMore) loadMoreMarkup = <button type="button" className={classNames.uiBasicButton} data-anchor-load-more>Load more anchors</button>;
    anchorsMarkup = <Fragment><p className={classNames.uiErrorMessage} data-anchor-list-message role="alert" hidden></p>{anchorsMarkup}{loadMoreMarkup}</Fragment>;
    renderTree(anchorsMarkup, target);
    const matchedAnchors = activeAnchors.filter((anchor) => {
      return anchor.version.pdf_content_hash === detailData.pdf_status?.content_hash;
    });
    pdfController?.setAnchors(matchedAnchors);
    target.querySelector<HTMLButtonElement>("[data-anchor-load-more]")?.addEventListener("click", async (event) => {
      const button = event.currentTarget as HTMLButtonElement;
      button.disabled = true;
      classAdd(button, ["loading"]);
      await loadAnchors(false);
    });
    for (const anchor of activeAnchors) {
      const row = target.querySelector(`[data-anchor-id="${CSS.escape(anchor.id)}"]`) as HTMLElement;
      const pageButton = row.querySelector("[data-anchor-page]") as HTMLButtonElement;
      pageButton.addEventListener("click", () => {
        history.replaceState({}, "", link({
          anchor_id: anchor.id,
          pdf_page: anchor.version.page,
        }));
        pdfController?.goToPage(Number(anchor.version.page));
      });
      const historyButton = row.querySelector("[data-anchor-history]") as HTMLButtonElement;
      historyButton.addEventListener("click", () => { void showAnchorHistory(anchor.id, anchor.label); });
      const deleteButton = row.querySelector("[data-anchor-delete]") as HTMLButtonElement;
      deleteButton.addEventListener("click", async () => {
        if (!anchorsEditable || !window.confirm(`Remove anchor ${anchor.label || anchor.id}? Its immutable history will remain available.`)) return;
        deleteButton.disabled = true;
        classAdd(deleteButton, ["loading"]);
        try {
          await mutate<ReviewAnchorMutationResponse>(`/api/runs/${runID}/anchors/${encodeURIComponent(anchor.id)}/versions`, "POST", {
            expected_version_id: anchor.version.id,
            state: "deleted",
            page: 0,
            selected_text: "",
            rectangles: [],
          });
          history.replaceState({}, "", link({ anchor_id: anchor.id }));
          const message = target.querySelector("[data-anchor-list-message]") as HTMLElement;
          message.className = classNames.uiSuccessMessage;
          message.textContent = "Anchor removed. Its immutable history remains available.";
          message.hidden = false;
          classRemove(deleteButton, "loading");
          try {
            await loadAnchors();
            await showAnchorHistory(anchor.id, anchor.label);
            await onAuditChange?.();
          } catch (refreshError: any) {
            message.className = classNames.uiWarningMessage;
            message.textContent = `Anchor removed, refresh failed: ${refreshError.message}`;
          }
        } catch (error: any) {
          const message = target.querySelector("[data-anchor-list-message]") as HTMLElement;
          message.textContent = error.message || "Anchor could not be removed.";
          message.hidden = false;
          deleteButton.disabled = false;
          classRemove(deleteButton, "loading");
        }
      });
    }
    const focused = new URLSearchParams(location.search).get("anchor_id");
    if (focused && !activeAnchors.some((anchor) => {
      return anchor.id === focused;
    })) await showAnchorHistory(focused);
  }

  /** Displays bounded immutable active and tombstone ancestry for a focused anchor. */
  async function showAnchorHistory(anchorID: string, anchorLabel?: string): Promise<void> {
    const target = host.querySelector("[data-anchor-list]") as HTMLElement;
    let versions: any[] = [];
    let cursor = "";
    let hasMore = false;
    /** Renders all loaded immutable anchor summaries and their continuation controls. */
    function renderHistory(): void {
      const newest = versions[0];
      const restorable = versions.find((version: any) => {
        return version.state === "active";
      });
      const restorableOnCurrentPDF = restorable && restorable.pdf_content_hash === detailData.pdf_status?.content_hash;
      var restoreMarkup: JSX.Element | null = null;
      if (newest?.state === "deleted") {
        var restoreText = "Load older versions to find restorable geometry";
        if (restorable) {
          restoreText = "Cannot restore after PDF change";
          if (restorableOnCurrentPDF) restoreText = "Restore anchor";
        }
        restoreMarkup = <button type="button" className={classNames.uiPrimaryButton} data-anchor-restore disabled={!anchorsEditable || !restorableOnCurrentPDF}>{restoreText}</button>;
      }
      const historyItems = versions.map((version: any) => {
        var pageSummary = " · tombstone";
        if (version.state === "active") pageSummary = ` · page ${version.page}`;
        var quoteMarkup: JSX.Element | null = null;
        if (version.state === "active") {
          var quoteSuffix = "";
          if (version.selected_text_truncated) quoteSuffix = "…";
          quoteMarkup = <blockquote>{version.selected_text || ""}{quoteSuffix}</blockquote>;
        }
        return (
          <li>
            <div>
              <strong>Version {version.id} · {version.state}</strong>
              <p>{version.reviewer_display} · {formatTime(version.created_at)}{pageSummary}</p>
            </div>
            {quoteMarkup}
          </li>
        );
      });
      var olderMarkup: JSX.Element | null = null;
      if (hasMore) olderMarkup = <button type="button" className={classNames.uiBasicButton} data-anchor-history-more>Load older versions</button>;
      const historyNode = (
        <section className="rw-anchor-history">
          <div className="rw-review-section__heading">
            <div>
              <h4>Anchor {anchorLabel || anchorID} history</h4>
              <p>The newest immutable anchor version appears first.</p>
            </div>
            <div className="rw-inline-group">{restoreMarkup}<button type="button" className={classNames.uiBasicButton} data-anchor-backlinks>Backlinks</button></div>
          </div>
          <ol>{historyItems}</ol>
          {olderMarkup}
          <div data-anchor-backlink-list></div>
          <p className={classNames.uiErrorMessage} data-anchor-history-message role="alert" hidden></p>
        </section>
      );
      target.querySelector(".rw-anchor-history")?.remove();
      target.appendChild(historyNode);
      target.querySelector<HTMLButtonElement>("[data-anchor-history-more]")?.addEventListener("click", async (event) => {
        const button = event.currentTarget as HTMLButtonElement;
        button.disabled = true;
        classAdd(button, ["loading"]);
        await loadHistoryPage();
      });
      target.querySelector<HTMLButtonElement>("[data-anchor-backlinks]")?.addEventListener("click", async () => {
        const backlinkTarget = target.querySelector("[data-anchor-backlink-list]") as HTMLElement;
        await mountBacklinks(backlinkTarget, {
          runID: runID,
          targetType: "anchor",
          targetID: anchorID,
        });
      });
      const restoreButton = target.querySelector<HTMLButtonElement>("[data-anchor-restore]");
      restoreButton?.addEventListener("click", async () => {
        if (!restorable || !restorableOnCurrentPDF) return;
        restoreButton.disabled = true;
        classAdd(restoreButton, ["loading"]);
        try {
          await mutate<ReviewAnchorMutationResponse>(`/api/runs/${runID}/anchors/${encodeURIComponent(anchorID)}/versions`, "POST", {
            expected_version_id: newest.id,
            state: "active",
            restore_from_version_id: restorable.id,
            page: 0,
            selected_text: "",
            rectangles: [],
          });
          const message = target.querySelector("[data-anchor-history-message]") as HTMLElement;
          message.className = classNames.uiSuccessMessage;
          message.textContent = "Anchor restored as a new immutable version.";
          message.hidden = false;
          classRemove(restoreButton, "loading");
          try {
            await loadAnchors(true);
            await onAuditChange?.();
          } catch (refreshError: any) {
            message.className = classNames.uiWarningMessage;
            message.textContent = `Anchor restored, refresh failed: ${refreshError.message}`;
          }
        } catch (error: any) {
          const message = target.querySelector("[data-anchor-history-message]") as HTMLElement;
          message.textContent = error.message || "Anchor could not be restored.";
          message.hidden = false;
          restoreButton.disabled = false;
          classRemove(restoreButton, "loading");
        }
      });
    }
    /** Appends one anchor-version cursor page without duplicating existing history. */
    async function loadHistoryPage(): Promise<void> {
      const data = await api<ReviewAnchorVersionsResponse>(`/api/runs/${runID}/anchors/${encodeURIComponent(anchorID)}/versions`, { limit: 25, cursor: cursor }, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      anchorLabel ||= data.anchor?.label;
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
}
