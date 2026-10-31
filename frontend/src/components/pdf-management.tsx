// Conditional PDF upload, replacement, and removal controls for article detail.
import { APIError, api, downloadExport, errorMessage, preparePDFUpload, removePDF, uploadPDF } from "../api.tsx";
import type { BinaryDownloadResult } from "../api.tsx";
import type { ArticleDetailResponse, ArticleRecord, PDFMutationResponse, PDFStatus } from "../api/types.ts";
import { formatBytes, formatTime, humanLabel } from "../state.tsx";
import { h, Fragment, render as renderTree, cx, classAdd, classRemove } from "../jsx/jsx-runtime.ts";

/** Typed compound class names used by the PDF management controls. */
const classNames = {
  pdfGrid: cx("rw-property-grid", "rw-property-grid--compact"),
  pdfManagement: cx("rw-pdf-management"),
  rwPdfExportAction: cx("rw-pdf-export-action"),
  rwPdfExportActionComments: cx("rw-pdf-export-action__comments"),
  uiBasicButton: cx("ui", "basic", "button"),
  uiDangerButton: cx("ui", "danger", "button"),
  uiErrorMessage: cx("ui", "error", "message"),
  uiErrorMessageRwReviewFeedback: cx("ui", "error", "message", "rw-review-feedback"),
  uiFadedText: cx("ui", "faded", "text"),
  uiField: cx("ui", "field"),
  uiForm: cx("ui", "form"),
  uiFormRwReviewDialogForm: cx("ui", "form", "rw-review-dialog__form"),
  uiIconBasicButtonRwReviewDialogClose: cx("ui", "icon", "basic", "button", "rw-review-dialog__close"),
  uiInfoMessage: cx("ui", "info", "message"),
  uiInfoMessageRwReviewFeedback: cx("ui", "info", "message", "rw-review-feedback"),
  uiPrimaryButton: cx("ui", "primary", "button"),
  uiSuccessMessageRwReviewFeedback: cx("ui", "success", "message", "rw-review-feedback"),
  uiWarningMessage: cx("ui", "warning", "message"),
  uiWarningMessageRwReviewFeedback: cx("ui", "warning", "message", "rw-review-feedback"),
};

/** One user-facing PDF management message. */
export interface PDFManagementFeedback {
  kind: "info" | "success" | "warning" | "error";
  message: string;
}

/** One mounted PDF management controller. */
export interface PDFManagementController {
  destroy: () => void;
}

/** Options for mounting the PDF management controls. */
export interface PDFManagementOptions {
  record: ArticleRecord;
  status: PDFStatus;
  onChanged?: (result: PDFMutationResponse) => Promise<void> | void;
  onRefresh?: () => Promise<void> | void;
}

/** One confirmation target for a destructive PDF mutation. */
type PDFConfirmation = "replace" | "remove";

/** Returns the specific reason PDF mutation controls are unavailable. */
function capabilityReason(status: PDFStatus): string {
  const reason = status.actions?.reason || "";
  const reasons: Record<string, string> = {
    pdf_doi_missing: "This work has no normalized DOI, so PDF inventory is unavailable.",
    run_not_completed: "Only completed runs accept PDF changes.",
    run_trashed: "This run is in the trash, so PDF changes are disabled.",
    pdf_store_unavailable: "The PDF store is not writable.",
    pdf_inventory_missing: "This work has no PDF inventory registration.",
    pdf_status_unavailable: "This PDF state does not support upload or removal.",
  };
  if (reason === "pdf_store_unavailable" && status.storage_reason) return status.storage_reason;
  if (reasons[reason]) return reasons[reason];
  if (status.storage_writable === false) return status.storage_reason || "The PDF store is not writable.";
  return "";
}

/** Replaces one feedback element's semantic class with the supplied result kind. */
function applyFeedbackClass(node: HTMLElement, kind: PDFManagementFeedback["kind"]): void {
  classRemove(node, "ui");
  classRemove(node, "info");
  classRemove(node, "success");
  classRemove(node, "warning");
  classRemove(node, "error");
  classRemove(node, "message");
  classRemove(node, "rw-review-feedback");
  classAdd(node, ["ui", kind, "message", "rw-review-feedback"]);
}

/** Returns one concise outcome message for a prepared PDF download. */
function pdfDownloadMessage(result: BinaryDownloadResult, includeComments: boolean): string {
  const base = `Download prepared: ${result.filename} (${formatBytes(result.bytes)}).`;
  if (!includeComments) return base;
  if (result.commentsStatus === "included") {
    const count = result.comments || 0;
    return `${base} ${count} saved comment${count === 1 ? "" : "s"} included.`;
  }
  if (result.commentsStatus === "no_review_context") return `${base} No review context is initialized for this run, so the original PDF was downloaded.`;
  if (result.commentsStatus === "no_matching_comments") return `${base} No matching saved comments were found, so the original PDF was downloaded.`;
  return base;
}

/** Returns one concise outcome message for a committed PDF mutation. */
function mutationMessage(result: PDFMutationResponse): PDFManagementFeedback {
  if (!result.changed) return { kind: "info", message: "The stored PDF already matches the selected file. Nothing changed." };
  const labels: Record<string, string> = {
    uploaded: "PDF uploaded.",
    replaced: "PDF replaced.",
    removed: "PDF removed.",
  };
  const label = labels[result.operation] || "PDF saved.";
  if (result.audit_delivery === "pending") {
    return { kind: "success", message: `${label} Audit delivery is pending.` };
  }
  return { kind: "success", message: label };
}

/** One PDF management panel's current record, status, and local selection state. */
export interface PDFManagementPanelProps {
  record: ArticleRecord;
  status: PDFStatus;
  selectedFile?: File | null;
  includeComments?: boolean;
  feedback?: PDFManagementFeedback | null;
}

/** Renders one PDF status strip and its available management controls. */
export function PDFManagementPanel(props: PDFManagementPanelProps): JSX.Element {
  const pdfLabels: Record<string, string> = {
    available: "Available",
    unavailable: "Unavailable without a DOI",
    not_available: "Not Available",
  };
  const actions = props.status.actions;
  const canUpload = actions?.upload === true;
  const canReplace = actions?.replace === true;
  const canRemove = actions?.remove === true;
  var canDownload = props.status.status === "available";
  if (actions) canDownload = actions.download;
  const reason = capabilityReason(props.status);

  var pdfAction: JSX.Element = <span className={classNames.uiFadedText}>No stored PDF is available.</span>;
  if (canDownload) {
    pdfAction = (
      <div className={classNames.rwPdfExportAction}>
        <button type="button" className={classNames.uiPrimaryButton} data-pdf-download>Download PDF</button>
        <label className={classNames.rwPdfExportActionComments}>
          <input type="checkbox" data-pdf-include-comments checked={props.includeComments === true} />
          <span>Include comments</span>
        </label>
        <small className={classNames.uiFadedText}>Matching saved highlights and comments are embedded in a separate copy. Off downloads the original bytes.</small>
      </div>
    );
  }

  var sizeValue: string | null = null;
  if (props.status.byte_size) sizeValue = formatBytes(props.status.byte_size);
  var inventoriedValue: string | null = null;
  if (props.status.inventoried_at) inventoriedValue = formatTime(props.status.inventoried_at);

  const pdfGrid = (
    <div className={classNames.pdfGrid}>
      <div>
        <span className={classNames.uiFadedText}>Status</span>
        <strong>{pdfLabels[props.status.status] || humanLabel(props.status.status)}</strong>
      </div>
      <div>
        <span className={classNames.uiFadedText}>Size</span>
        <strong>{sizeValue || "Not recorded"}</strong>
      </div>
      <div>
        <span className={classNames.uiFadedText}>Inventoried at</span>
        <strong>{inventoriedValue || "Not recorded"}</strong>
      </div>
      <div>
        <span className={classNames.uiFadedText}>Content hash</span>
        <strong>{props.status.content_hash || "Not recorded"}</strong>
      </div>
    </div>
  );

  var capabilityMarkup: JSX.Element | null = null;
  if (reason) {
    capabilityMarkup = <p className={classNames.uiWarningMessage} data-pdf-capability>{reason}</p>;
  }

  var selectedText = "";
  if (props.selectedFile) selectedText = `${props.selectedFile.name} (${formatBytes(props.selectedFile.size)})`;

  var limitText = "";
  if (actions?.upload || actions?.replace) {
    if (props.status.max_upload_bytes) limitText = `Maximum upload size: ${formatBytes(props.status.max_upload_bytes)}. `;
    limitText += "The selected filename stays in this browser and is not sent to the server.";
  }

  var feedbackClassValue = classNames.uiInfoMessageRwReviewFeedback;
  var feedbackMessage = "";
  if (props.feedback) {
    if (props.feedback.kind === "success") feedbackClassValue = classNames.uiSuccessMessageRwReviewFeedback;
    else if (props.feedback.kind === "warning") feedbackClassValue = classNames.uiWarningMessageRwReviewFeedback;
    else if (props.feedback.kind === "error") feedbackClassValue = classNames.uiErrorMessageRwReviewFeedback;
    feedbackMessage = props.feedback.message;
  }

  var fileForm: JSX.Element | null = null;
  if (canUpload || canReplace) {
    var submitLabel = "Upload PDF";
    if (canReplace) submitLabel = "Replace PDF";
    var removeButton: JSX.Element | null = null;
    if (canRemove) {
      removeButton = <button type="button" className={classNames.uiDangerButton} data-pdf-remove>Remove PDF</button>;
    }
    fileForm = (
      <form className={classNames.uiForm} data-pdf-form>
        <div className={classNames.uiField}>
          <label htmlFor="pdf-upload-file">PDF file</label>
          <input id="pdf-upload-file" type="file" accept="application/pdf,.pdf" aria-describedby="pdf-upload-help pdf-selected-file" data-pdf-file />
          <p className="rw-field-help" id="pdf-upload-help">{limitText}</p>
        </div>
        <p className="rw-field-help" id="pdf-selected-file" data-pdf-selected aria-live="polite" hidden={!props.selectedFile}>{selectedText}</p>
        <div className="rw-review-actions">
          <button type="submit" className={classNames.uiPrimaryButton} data-pdf-submit>{submitLabel}</button>
          {removeButton}
        </div>
      </form>
    );
  } else if (canRemove) {
    fileForm = (
      <div className="rw-review-actions">
        <button type="button" className={classNames.uiDangerButton} data-pdf-remove>Remove PDF</button>
      </div>
    );
  }

  const title = props.record.title || props.record.doi || `Work ${props.record.work_id}`;
  const identity = `${title}${props.record.doi ? ` (${props.record.doi})` : ""}`;

  return (
    <Fragment>
      <div className="rw-pdf-status-strip">
        {pdfGrid}
        <div className="rw-pdf-status-strip__action">{pdfAction}</div>
      </div>
      <div className={classNames.pdfManagement}>
        {capabilityMarkup}
        {fileForm}
        <p className={feedbackClassValue} data-pdf-feedback role="status" aria-live="polite" tabIndex={-1} hidden={!props.feedback}>{feedbackMessage}</p>
        <div className="rw-review-actions">
          <button type="button" className={classNames.uiBasicButton} data-pdf-refresh>Refresh PDF state</button>
        </div>
      </div>
      <dialog className="rw-review-dialog" data-pdf-confirm aria-labelledby="pdf-confirm-title" aria-describedby="pdf-confirm-description">
        <form className={classNames.uiFormRwReviewDialogForm} data-pdf-confirm-form>
          <div className="rw-review-dialog__header">
            <div>
              <p className="rw-review-dialog__eyebrow">PDF availability</p>
              <h3 id="pdf-confirm-title" data-pdf-confirm-title>Confirm PDF change</h3>
              <p id="pdf-confirm-description" data-pdf-confirm-description></p>
            </div>
            <button type="button" className={classNames.uiIconBasicButtonRwReviewDialogClose} data-pdf-confirm-cancel aria-label="Close PDF confirmation dialog">Close</button>
          </div>
          <div className="rw-review-dialog__body">
            <p><strong>{identity}</strong></p>
            <p>This changes PDF availability for work {String(props.record.work_id)} in every run, including historical runs.</p>
            <p>If no other work selects the old bytes, they are permanently deleted and cannot be restored through this application.</p>
          </div>
          <div className="rw-review-dialog__actions">
            <button type="button" className={classNames.uiPrimaryButton} data-pdf-confirm-cancel>Cancel</button>
            <button type="submit" className={classNames.uiDangerButton} data-pdf-confirm-submit>Confirm</button>
          </div>
        </form>
      </dialog>
    </Fragment>
  );
}

/** Mounts conditional PDF management controls and returns an explicit teardown. */
export function mountPDFManagement(host: HTMLElement, options: PDFManagementOptions): PDFManagementController {
  let currentStatus = options.status;
  let selectedFile: File | null = null;
  let feedback: PDFManagementFeedback | null = null;
  let busy = false;
  let downloading = false;
  let includeComments = false;
  let confirmation: PDFConfirmation | null = null;
  let confirmationOpener: HTMLElement | null = null;
  let destroyed = false;

  renderPanel();

  /** Rebuilds the panel from current status and clears a committed file selection. */
  function renderPanel(): void {
    if (destroyed) return;
    const panel = <PDFManagementPanel record={options.record} status={currentStatus} selectedFile={selectedFile} includeComments={includeComments} feedback={feedback} />;
    renderTree(panel, host);
    bind();
  }

  /** Returns the currently selected file's display text. */
  function selectedText(): string {
    if (!selectedFile) return "";
    return `${selectedFile.name} (${formatBytes(selectedFile.size)})`;
  }

  /** Returns whether the selected file exceeds the advertised upload limit. */
  function fileTooLarge(): boolean {
    if (!selectedFile) return false;
    const limit = currentStatus.max_upload_bytes || 0;
    return limit > 0 && selectedFile.size > limit;
  }

  /** Updates selected-file text and submit availability without replacing the input. */
  function updateSelectedFile(): void {
    const selected = host.querySelector<HTMLElement>("[data-pdf-selected]");
    if (selected) {
      selected.hidden = !selectedFile;
      selected.textContent = selectedText();
    }
    const submit = host.querySelector<HTMLButtonElement>("[data-pdf-submit]");
    if (submit) submit.disabled = busy || !selectedFile || fileTooLarge();
  }

  /** Applies one management feedback message without replacing the file input. */
  function updateFeedback(): void {
    const node = host.querySelector<HTMLElement>("[data-pdf-feedback]");
    if (!node) return;
    node.hidden = !feedback;
    node.textContent = feedback?.message || "";
    node.setAttribute("role", feedback?.kind === "error" ? "alert" : "status");
    if (feedback) applyFeedbackClass(node, feedback.kind);
  }

  /** Disables duplicate PDF mutations while one request is pending. */
  function updateBusy(): void {
    const form = host.querySelector<HTMLFormElement>("[data-pdf-form]");
    if (form) form.setAttribute("aria-busy", String(busy));
    const input = host.querySelector<HTMLInputElement>("[data-pdf-file]");
    if (input) input.disabled = busy;
    const remove = host.querySelector<HTMLButtonElement>("[data-pdf-remove]");
    if (remove) remove.disabled = busy;
    const download = host.querySelector<HTMLButtonElement>("[data-pdf-download]");
    if (download) download.disabled = busy || downloading || !currentStatus.content_hash;
    const comments = host.querySelector<HTMLInputElement>("[data-pdf-include-comments]");
    if (comments) comments.disabled = busy || downloading;
    updateSelectedFile();
  }

  /** Opens the confirmation dialog for one destructive PDF mutation. */
  function openConfirmation(action: PDFConfirmation, opener: HTMLElement): void {
    if (busy) return;
    if (action === "replace" && !selectedFile) {
      feedback = { kind: "warning", message: "Select a PDF file before replacing the stored document." };
      updateFeedback();
      return;
    }
    confirmation = action;
    confirmationOpener = opener;
    const dialog = host.querySelector<HTMLDialogElement>("[data-pdf-confirm]");
    if (!dialog) return;
    const title = host.querySelector<HTMLElement>("[data-pdf-confirm-title]");
    const description = host.querySelector<HTMLElement>("[data-pdf-confirm-description]");
    if (title) title.textContent = action === "replace" ? "Replace stored PDF" : "Remove stored PDF";
    if (description) {
      description.textContent = action === "replace"
        ? "The uploaded file will replace the current PDF for this work after validation."
        : "The current PDF selection will be cleared for this work.";
    }
    if (typeof dialog.showModal === "function") dialog.showModal();
    if (!dialog.open) dialog.setAttribute("open", "");
    host.querySelector<HTMLButtonElement>(".rw-review-dialog__actions [data-pdf-confirm-cancel]")?.focus();
  }

  /** Closes the confirmation dialog and optionally restores focus to its opener. */
  function closeConfirmation(restoreFocus: boolean): void {
    const dialog = host.querySelector<HTMLDialogElement>("[data-pdf-confirm]");
    if (dialog) {
      if (typeof dialog.close === "function") dialog.close();
      else dialog.removeAttribute("open");
    }
    confirmation = null;
    if (restoreFocus && confirmationOpener) confirmationOpener.focus();
    confirmationOpener = null;
  }

  /** Reports one recoverable mutation failure without clearing the selected file. */
  async function handleFailure(error: unknown): Promise<void> {
    if (error instanceof APIError && error.code === "pdf_state_conflict") {
      feedback = {
        kind: "warning",
        message: "The PDF selection changed. Your selected file and review inputs are preserved; refresh the current PDF state before retrying.",
      };
      return;
    }
    if (error instanceof APIError) {
      feedback = { kind: "error", message: errorMessage(error, "The PDF change failed.") };
      return;
    }
    if (error instanceof TypeError) {
      await handleUncertain();
      return;
    }
    feedback = { kind: "error", message: errorMessage(error, "The PDF change failed.") };
  }

  /** Focuses the visible management feedback after a result or failure. */
  function focusFeedback(): void {
    host.querySelector<HTMLElement>("[data-pdf-feedback]")?.focus();
  }

  /** Reconciles an ambiguous transport failure with the current authoritative state. */
  async function handleUncertain(): Promise<void> {
    feedback = { kind: "warning", message: "The response was lost. Checking the current PDF state before retrying." };
    updateFeedback();
    try {
      const current = await api<PDFStatus>(`/api/works/${encodeURIComponent(options.record.work_id)}/pdf-status`, {}, { signal: null });
      const changed = current.status !== currentStatus.status
        || (current.content_hash || "") !== (currentStatus.content_hash || "")
        || current.mutation_etag !== currentStatus.mutation_etag;
      if (changed) {
        feedback = { kind: "warning", message: "The response was lost after the request was sent. The current PDF state differs from the state you started with, so the change may have committed. Refresh the PDF state and review it before retrying." };
      } else {
        feedback = { kind: "warning", message: "The response was lost before confirmation. The current PDF state is unchanged; review it before retrying." };
      }
    } catch (_) {
      feedback = { kind: "warning", message: "The response was lost, so the PDF change may or may not have committed. Refresh the PDF state before retrying." };
    }
  }

  /** Reloads the current PDF state without repeating the previous mutation. */
  async function refreshCurrentStatus(): Promise<void> {
    if (busy) return;
    busy = true;
    updateBusy();
    feedback = { kind: "info", message: "Refreshing the current PDF state." };
    updateFeedback();
    try {
      const detail = await api<ArticleDetailResponse>(`/api/articles/${encodeURIComponent(options.record.id)}`, { run_id: options.record.pipeline_run_id }, { signal: null });
      currentStatus = detail.pdf_status || { status: "not_available" };
      feedback = { kind: "info", message: "Current PDF state refreshed. Review the available action before retrying." };
      busy = false;
      renderPanel();
      if (options.onRefresh) await options.onRefresh();
    } catch (error) {
      feedback = { kind: "error", message: errorMessage(error, "The current PDF state could not be refreshed.") };
    } finally {
      if (!destroyed) {
        busy = false;
        updateBusy();
        updateFeedback();
        focusFeedback();
      }
    }
  }

  /** Prepares one explicit original-byte PDF download without changing selection. */
  async function executeDownload(): Promise<void> {
    if (downloading || busy) return;
    const contentHash = currentStatus.content_hash || "";
    if (!contentHash) {
      feedback = { kind: "warning", message: "The current PDF state has no content hash. Refresh the PDF state before downloading." };
      updateFeedback();
      return;
    }
    downloading = true;
    updateBusy();
    feedback = { kind: "info", message: "Preparing the PDF download." };
    updateFeedback();
    try {
      const path = `/api/runs/${encodeURIComponent(options.record.pipeline_run_id)}/articles/${encodeURIComponent(options.record.id)}/pdf-export`;
      const result = await downloadExport(path, { content_hash: contentHash, include_comments: includeComments }, {
        contentType: "application/pdf",
        maxBytes: 26_000_000,
        signal: null,
      });
      feedback = { kind: "success", message: pdfDownloadMessage(result, includeComments) };
    } catch (error) {
      if (error instanceof APIError && error.code === "pdf_changed") {
        feedback = { kind: "warning", message: "The PDF selection changed after this page loaded. Refresh the PDF state before downloading." };
      } else if (error instanceof APIError && (error.code === "pdf_protected" || error.code === "pdf_signed" || error.code === "pdf_annotation_failed" || error.code === "pdf_export_limit")) {
        feedback = { kind: "warning", message: `${errorMessage(error, "The saved comments could not be embedded.")} Retry with Include comments off to download the original PDF.` };
      } else {
        feedback = { kind: "error", message: errorMessage(error, "The PDF download could not be prepared.") };
      }
    } finally {
      downloading = false;
      if (!destroyed) {
        updateBusy();
        updateFeedback();
        focusFeedback();
      }
    }
  }

  /** Applies one committed mutation response and refreshes the local panel. */
  async function applyResult(result: PDFMutationResponse): Promise<void> {
    currentStatus = result.pdf_status;
    selectedFile = null;
    feedback = mutationMessage(result);
    busy = false;
    renderPanel();
    focusFeedback();
    if (options.onChanged) {
      try {
        await options.onChanged(result);
      } catch (_) {
        feedback = { kind: "warning", message: "The PDF change was saved, but the article display could not be refreshed." };
        updateFeedback();
      }
    }
  }

  /** Uploads or replaces the selected file after local validation. */
  async function executeUpload(): Promise<void> {
    if (busy || !selectedFile) return;
    if (fileTooLarge()) {
      feedback = { kind: "warning", message: `The selected file exceeds the ${formatBytes(currentStatus.max_upload_bytes || 0)} upload limit.` };
      updateFeedback();
      return;
    }
    busy = true;
    updateBusy();
    try {
      const request = await preparePDFUpload(selectedFile, currentStatus);
      const result = await uploadPDF(options.record.pipeline_run_id, options.record.id, request);
      await applyResult(result);
    } catch (error) {
      await handleFailure(error);
    } finally {
      if (!destroyed) {
        busy = false;
        updateBusy();
        updateFeedback();
        focusFeedback();
      }
    }
  }

  /** Removes the current PDF selection after explicit confirmation. */
  async function executeRemove(): Promise<void> {
    if (busy) return;
    const etag = currentStatus.mutation_etag || "";
    if (!etag) {
      feedback = { kind: "warning", message: "The current PDF state has no mutation precondition. Reload the article before retrying." };
      updateFeedback();
      return;
    }
    busy = true;
    updateBusy();
    try {
      const result = await removePDF(options.record.pipeline_run_id, options.record.id, etag);
      await applyResult(result);
    } catch (error) {
      await handleFailure(error);
    } finally {
      if (!destroyed) {
        busy = false;
        updateBusy();
        updateFeedback();
        focusFeedback();
      }
    }
  }

  /** Binds one rendered panel's file, confirmation, and mutation controls. */
  function bind(): void {
    const input = host.querySelector<HTMLInputElement>("[data-pdf-file]");
    input?.addEventListener("change", () => {
      selectedFile = input.files && input.files.length > 0 ? input.files[0] : null;
      feedback = null;
      if (fileTooLarge()) {
        feedback = { kind: "warning", message: `The selected file exceeds the ${formatBytes(currentStatus.max_upload_bytes || 0)} upload limit.` };
      }
      updateSelectedFile();
      updateFeedback();
    });

    const form = host.querySelector<HTMLFormElement>("[data-pdf-form]");
    form?.addEventListener("submit", (event) => {
      event.preventDefault();
      if (busy) return;
      if (!selectedFile) {
        feedback = { kind: "warning", message: "Select a PDF file before uploading." };
        updateFeedback();
        return;
      }
      if (fileTooLarge()) {
        feedback = { kind: "warning", message: `The selected file exceeds the ${formatBytes(currentStatus.max_upload_bytes || 0)} upload limit.` };
        updateFeedback();
        return;
      }
      if (currentStatus.actions?.replace) {
        const submit = host.querySelector<HTMLButtonElement>("[data-pdf-submit]");
        if (submit) openConfirmation("replace", submit);
        return;
      }
      void executeUpload();
    });

    const remove = host.querySelector<HTMLButtonElement>("[data-pdf-remove]");
    remove?.addEventListener("click", () => openConfirmation("remove", remove));

    const refresh = host.querySelector<HTMLButtonElement>("[data-pdf-refresh]");
    refresh?.addEventListener("click", () => {
      void refreshCurrentStatus();
    });

    const download = host.querySelector<HTMLButtonElement>("[data-pdf-download]");
    download?.addEventListener("click", () => {
      void executeDownload();
    });

    const comments = host.querySelector<HTMLInputElement>("[data-pdf-include-comments]");
    comments?.addEventListener("change", () => {
      includeComments = comments.checked;
    });

    const confirmForm = host.querySelector<HTMLFormElement>("[data-pdf-confirm-form]");
    confirmForm?.addEventListener("submit", (event) => {
      event.preventDefault();
      const action = confirmation;
      closeConfirmation(false);
      if (action === "replace") void executeUpload();
      else if (action === "remove") void executeRemove();
    });

    host.querySelectorAll<HTMLButtonElement>("[data-pdf-confirm-cancel]").forEach((button) => {
      button.addEventListener("click", () => closeConfirmation(true));
    });

    const dialog = host.querySelector<HTMLDialogElement>("[data-pdf-confirm]");
    dialog?.addEventListener("click", (event) => {
      if (event.target === dialog) closeConfirmation(true);
    });
    dialog?.addEventListener("cancel", (event) => {
      event.preventDefault();
      closeConfirmation(true);
    });
  }

  return {
    destroy: () => {
      destroyed = true;
      closeConfirmation(false);
      host.replaceChildren();
    },
  };
}
