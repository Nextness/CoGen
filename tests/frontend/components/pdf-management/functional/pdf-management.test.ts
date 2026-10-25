// Functional tests for components/pdf-management.tsx — PDF mutation controls and confirmation.
import { describe, it } from "node:test";
import assert from "node:assert/strict";

import "../../../../support/node/setup.ts";
import { mountPDFManagement } from "../../../../../frontend/src/components/pdf-management.tsx";
import type { ArticleRecord, PDFMutationResponse, PDFStatus } from "../../../../../frontend/src/api/types.ts";

/** Builds one minimal article record for PDF management tests. */
function article(): ArticleRecord {
  return {
    id: 11,
    work_id: 10,
    pipeline_run_id: 7,
    title: "A managed article",
    doi: "10.1000/managed",
    year: 2024,
    journal: "Journal",
    source: "crossref",
    publisher: "Publisher",
    abstract: null,
    keywords: null,
    keywords_plus: null,
    citation_count: null,
    reference_count: null,
    validation_status: "valid",
    created_at: "2024-01-01T00:00:00Z",
    payload_hash: "hash",
    producer_stage: "normalize",
    extension_data: null,
  };
}

/** Builds one unavailable PDF status with upload capability. */
function uploadStatus(): PDFStatus {
  return {
    status: "not_available",
    registered: true,
    change_counter: 1,
    mutation_etag: '"pdf-10-1"',
    max_upload_bytes: 1024,
    storage_writable: true,
    storage_reason: "",
    actions: { upload: true, replace: false, remove: false, download: false, reason: "" },
  };
}

/** Builds one available PDF status with replacement and removal capability. */
function availableStatus(): PDFStatus {
  return {
    status: "available",
    content_hash: "a".repeat(64),
    byte_size: 512,
    inventoried_at: "2024-01-02T00:00:00Z",
    registered: true,
    change_counter: 2,
    mutation_etag: '"pdf-10-2"',
    max_upload_bytes: 1024,
    storage_writable: true,
    storage_reason: "",
    actions: { upload: false, replace: true, remove: true, download: true, reason: "" },
  };
}

/** Mounts the PDF management panel into a fresh host. */
function mount(status: PDFStatus): HTMLElement {
  const host = document.createElement("div");
  document.body.append(host);
  mountPDFManagement(host, { record: article(), status: status });
  return host;
}

/** Selects one local file through the visible file input. */
function selectFile(host: HTMLElement, file: File): HTMLInputElement {
  const input = host.querySelector<HTMLInputElement>("[data-pdf-file]")!;
  Object.defineProperty(input, "files", { value: [file], configurable: true });
  input.dispatchEvent(new Event("change"));
  return input;
}

/** Waits for asynchronous mutation and digest work to settle. */
function settle(): Promise<void> {
  return new Promise(function(resolve) {
    setTimeout(resolve, 30);
  });
}

/** Waits for one observable test condition instead of relying on a fixed delay. */
async function waitFor(predicate: () => boolean): Promise<void> {
  for (var attempt = 0; attempt < 100; attempt += 1) {
    if (predicate()) return;
    await settle();
  }
  assert.fail("Timed out waiting for PDF management state");
}

describe("pdf-management.tsx", function() {

  it("renders one labeled file input, size guidance, and explicit upload action", function() {
    const host = mount(uploadStatus());

    const label = host.querySelector<HTMLLabelElement>('label[for="pdf-upload-file"]');
    const input = host.querySelector<HTMLInputElement>("[data-pdf-file]");
    assert.equal(label?.textContent, "PDF file");
    assert.equal(input?.type, "file");
    assert.equal(input?.accept, "application/pdf,.pdf");
    assert.match(host.textContent || "", /Maximum upload size: 1 KB/);
    assert.match(host.textContent || "", /Upload PDF/);
    assert.equal(host.querySelector("[data-pdf-remove]"), null);

    host.remove();
  });

  it("shows the selected filename and size locally", function() {
    const host = mount(uploadStatus());
    selectFile(host, new File([new Uint8Array(4)], "paper.pdf", { type: "application/pdf" }));

    const selected = host.querySelector<HTMLElement>("[data-pdf-selected]")!;
    assert.equal(selected.hidden, false);
    assert.match(selected.textContent || "", /paper\.pdf/);
    assert.match(selected.textContent || "", /4 B/);

    host.remove();
  });

  it("shows the specific unavailable capability and omits mutation controls", function() {
    const status = uploadStatus();
    status.actions = { upload: false, replace: false, remove: false, download: false, reason: "run_trashed" };
    const host = mount(status);

    assert.match(host.textContent || "", /This run is in the trash, so PDF changes are disabled\./);
    assert.equal(host.querySelector("[data-pdf-form]"), null);
    assert.equal(host.querySelector("[data-pdf-remove]"), null);

    host.remove();
  });

  it("confirms replacement with article identity, global scope, and permanent deletion", function() {
    const host = mount(availableStatus());
    const input = selectFile(host, new File([new Uint8Array(4)], "replacement.pdf", { type: "application/pdf" }));
    const form = host.querySelector<HTMLFormElement>("[data-pdf-form]")!;
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

    const dialog = host.querySelector<HTMLDialogElement>("[data-pdf-confirm]")!;
    assert.equal(dialog.open || dialog.hasAttribute("open"), true);
    assert.match(dialog.textContent || "", /A managed article/);
    assert.match(dialog.textContent || "", /10\.1000\/managed/);
    assert.match(dialog.textContent || "", /work 10 in every run, including historical runs/);
    assert.match(dialog.textContent || "", /permanently deleted and cannot be restored/);

    host.querySelector<HTMLButtonElement>("[data-pdf-confirm-cancel]")!.click();
    assert.equal(dialog.open || dialog.hasAttribute("open"), false);
    assert.equal(input.files?.[0]?.name, "replacement.pdf");
    assert.match(host.querySelector("[data-pdf-selected]")?.textContent || "", /replacement\.pdf/);

    host.remove();
  });

  it("confirms removal and keeps the current selection when cancelled", function() {
    const host = mount(availableStatus());
    host.querySelector<HTMLButtonElement>("[data-pdf-remove]")!.click();

    const dialog = host.querySelector<HTMLDialogElement>("[data-pdf-confirm]")!;
    assert.equal(dialog.open || dialog.hasAttribute("open"), true);
    assert.match(dialog.textContent || "", /Remove stored PDF/);
    assert.match(dialog.textContent || "", /current PDF selection will be cleared/);
    host.querySelector<HTMLButtonElement>("[data-pdf-confirm-cancel]")!.click();
    assert.equal(dialog.open || dialog.hasAttribute("open"), false);
    assert.match(host.textContent || "", /Available/);

    host.remove();
  });

  it("preserves the selected file and reports a stale precondition", async function() {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = function() {
      return Promise.resolve({
        ok: false,
        status: 412,
        json: function() {
          return Promise.resolve({ error: { message: "The PDF selection changed; refresh and retry", code: "pdf_state_conflict" } });
        },
      } as unknown as Response);
    } as typeof fetch;
    const host = mount(availableStatus());
    selectFile(host, new File([new Uint8Array(4)], "replacement.pdf", { type: "application/pdf" }));
    const form = host.querySelector<HTMLFormElement>("[data-pdf-form]")!;
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    host.querySelector<HTMLFormElement>("[data-pdf-confirm-form]")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await waitFor(() => (host.querySelector("[data-pdf-feedback]")?.textContent || "").includes("selection changed"));

    assert.match(host.querySelector("[data-pdf-feedback]")?.textContent || "", /selection changed/);
    assert.match(host.querySelector("[data-pdf-selected]")?.textContent || "", /replacement\.pdf/);
    assert.equal(host.querySelector<HTMLButtonElement>("[data-pdf-submit]")?.disabled, false);

    globalThis.fetch = originalFetch;
    host.remove();
  });

  it("reports an uncertain lost response and refreshes current state explicitly", async function() {
    const originalFetch = globalThis.fetch;
    const changed = availableStatus();
    changed.content_hash = "c".repeat(64);
    changed.mutation_etag = '"pdf-10-3"';
    var calls = 0;
    var refreshes = 0;
    globalThis.fetch = function(_input: RequestInfo | URL, init?: RequestInit) {
      calls += 1;
      if (init?.method === "PUT") return Promise.reject(new TypeError("connection lost"));
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve(changed); } } as unknown as Response);
    } as typeof fetch;
    const host = document.createElement("div");
    document.body.append(host);
    mountPDFManagement(host, {
      record: article(),
      status: availableStatus(),
      onRefresh: () => {
        refreshes += 1;
      },
    });
    selectFile(host, new File([new Uint8Array(4)], "replacement.pdf", { type: "application/pdf" }));
    host.querySelector<HTMLFormElement>("[data-pdf-form]")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    host.querySelector<HTMLFormElement>("[data-pdf-confirm-form]")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await waitFor(() => (host.querySelector("[data-pdf-feedback]")?.textContent || "").includes("may have committed"));

    assert.match(host.querySelector("[data-pdf-feedback]")?.textContent || "", /may have committed/);
    assert.match(host.querySelector("[data-pdf-selected]")?.textContent || "", /replacement\.pdf/);
    assert.equal(calls, 2);
    host.querySelector<HTMLButtonElement>("[data-pdf-refresh]")!.click();
    await waitFor(() => refreshes === 1);

    assert.equal(refreshes, 1);
    assert.match(host.querySelector("[data-pdf-feedback]")?.textContent || "", /Current PDF state refreshed/);
    assert.match(host.textContent || "", new RegExp(changed.content_hash || ""));
    assert.match(host.querySelector("[data-pdf-selected]")?.textContent || "", /replacement\.pdf/);

    globalThis.fetch = originalFetch;
    host.remove();
  });

  it("reports an unchanged save without claiming a mutation", async function() {
    const originalFetch = globalThis.fetch;
    const response: PDFMutationResponse = {
      changed: false,
      operation: "unchanged",
      pdf_status: uploadStatus(),
      event_key: null,
      audit_delivery: "not_applicable",
    };
    globalThis.fetch = function() {
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve(response); } } as unknown as Response);
    } as typeof fetch;
    const host = mount(uploadStatus());
    selectFile(host, new File([new Uint8Array(4)], "same.pdf", { type: "application/pdf" }));
    host.querySelector<HTMLFormElement>("[data-pdf-form]")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await waitFor(() => (host.querySelector("[data-pdf-feedback]")?.textContent || "").includes("already matches"));

    assert.match(host.querySelector("[data-pdf-feedback]")?.textContent || "", /already matches the selected file/);
    assert.equal(host.querySelector<HTMLElement>("[data-pdf-feedback]")?.getAttribute("role"), "status");

    globalThis.fetch = originalFetch;
    host.remove();
  });

  it("warns when a committed PDF change cannot refresh the article display", async function() {
    const originalFetch = globalThis.fetch;
    const response: PDFMutationResponse = {
      changed: true,
      operation: "uploaded",
      pdf_status: availableStatus(),
      event_key: "event-3",
      audit_delivery: "pending",
    };
    globalThis.fetch = function() {
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve(response); } } as unknown as Response);
    } as typeof fetch;
    const host = document.createElement("div");
    document.body.append(host);
    mountPDFManagement(host, {
      record: article(),
      status: uploadStatus(),
      onChanged: () => {
        throw new Error("refresh failed");
      },
    });
    selectFile(host, new File([new Uint8Array(4)], "upload.pdf", { type: "application/pdf" }));
    host.querySelector<HTMLFormElement>("[data-pdf-form]")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await waitFor(() => (host.querySelector("[data-pdf-feedback]")?.textContent || "").includes("saved, but the article display"));

    assert.match(host.querySelector("[data-pdf-feedback]")?.textContent || "", /saved, but the article display could not be refreshed/);
    assert.equal(host.querySelector<HTMLElement>("[data-pdf-feedback]")?.getAttribute("role"), "status");

    globalThis.fetch = originalFetch;
    host.remove();
  });

  it("updates the visible status after a successful replacement", async function() {
    const originalFetch = globalThis.fetch;
    const response: PDFMutationResponse = {
      changed: true,
      operation: "replaced",
      pdf_status: availableStatus(),
      event_key: "event-1",
      audit_delivery: "pending",
    };
    response.pdf_status.content_hash = "b".repeat(64);
    var changed: PDFMutationResponse | null = null;
    globalThis.fetch = function() {
      return Promise.resolve({ ok: true, status: 200, json: function() { return Promise.resolve(response); } } as unknown as Response);
    } as typeof fetch;
    const host = document.createElement("div");
    document.body.append(host);
    mountPDFManagement(host, {
      record: article(),
      status: availableStatus(),
      onChanged: (result) => {
        changed = result;
      },
    });
    selectFile(host, new File([new Uint8Array(4)], "replacement.pdf", { type: "application/pdf" }));
    host.querySelector<HTMLFormElement>("[data-pdf-form]")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    host.querySelector<HTMLFormElement>("[data-pdf-confirm-form]")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await waitFor(() => changed === response);

    assert.equal(changed, response);
    assert.match(host.querySelector("[data-pdf-feedback]")?.textContent || "", /PDF replaced.*Audit delivery is pending/);
    const fileInput = host.querySelector<HTMLInputElement>("[data-pdf-file]");
    assert.equal(fileInput?.files?.length, 0);
    assert.equal(host.querySelector<HTMLElement>("[data-pdf-selected]")?.hidden, true);
    assert.match(host.textContent || "", new RegExp(response.pdf_status.content_hash || ""));

    globalThis.fetch = originalFetch;
    host.remove();
  });

});
