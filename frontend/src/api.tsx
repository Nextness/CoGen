// API fetch helpers and endpoint builder.
import { state, list } from "./state.tsx";
import type {
  APIQuery,
  Identifier,
  PDFMutationResponse,
  PDFStatus,
  PDFUploadRequest,
  TableInfo,
  TablesResponse,
  WireRecord,
} from "./api/types.ts";

/** Builds an API path and query string from supplied values. */
export function endpoint(path: string, query: APIQuery = {}): string {
  const url = new URL(path, location.origin);
  const queryEntries = Object.entries(query);
  queryEntries.forEach(([key, raw]) => {
    if (raw !== "" && raw !== null && raw !== undefined) url.searchParams.set(key, String(raw));
  });
  return url.pathname + url.search;
}

/** One structured API error detail envelope returned by the server. */
export interface APIErrorEnvelope {
  error?: {
    message?: string;
    code?: string;
    details?: unknown;
  };
}

/** One API request option set forwarded to fetch. */
export interface APIRequestOptions {
  method?: string;
  headers?: Record<string, string>;
  body?: BodyInit | null;
  signal?: AbortSignal | null;
}

/** Represents an HTTP API failure while preserving its status and structured details. */
export class APIError extends Error {
  status: number;
  code: string;
  details: unknown;

  /** Initializes one structured API error returned by a non-successful response. */
  constructor(message: string, status: number, code?: string, details?: unknown) {
    super(message);
    this.name = "APIError";
    this.status = status;

    this.code = code ?? "request_failed";
    this.details = details ?? "No details provided";
  }
}

/** Returns a safe message for an unknown caught value. */
export function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}

/** Decodes one JSON response body or throws the shared invalid-JSON diagnostic. */
async function decodeJSONBody(response: Response, path: string): Promise<unknown> {
  try {
    return await response.json();
  } catch (_) {
    throw new Error(`The server returned invalid JSON for ${path}.`);
  }
}

/** Maps one decoded JSON failure envelope into the shared API error type. */
function responseError(response: Response, body: unknown): APIError {
  var message = `Request failed (${response.status}).`;
  const envelope = body as APIErrorEnvelope;
  if (envelope?.error?.message) message = envelope.error.message;
  return new APIError(message, response.status, envelope?.error?.code, envelope?.error?.details);
}

/** Fetches and decodes one JSON API response. */
export async function api<T>(path: string, query: APIQuery = {}, options: APIRequestOptions = {}): Promise<T> {
  var signal = options.signal;
  if (signal === undefined) signal = state.controller?.signal;

  const method = options.method || "GET";
  const headers = { Accept: "application/json", ...options.headers };
  const response = await fetch(endpoint(path, query), {
    method: method,
    headers: headers,
    body: options.body,
    signal: signal ?? undefined,
  });

  const body = await decodeJSONBody(response, path);
  if (!response.ok) throw responseError(response, body);

  // JSON decoding cannot prove an endpoint-specific contract at runtime. Every
  // call site supplies T from the matching server handler contract, and the
  // fixture shape test guards that boundary against server drift.
  const envelope = body as WireRecord;
  if (envelope?.data !== undefined) return envelope.data as T;
  return body as T;
}

/** Sends a same-origin JSON mutation and returns its decoded response. */
export function mutate<T>(path: string, method: string, body: unknown): Promise<T> {
  return api<T>(path, {}, {
    method: method,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
    signal: null,
  });
}

/** Encodes one digest buffer as lowercase hexadecimal. */
function digestHex(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  const pairs = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0"));
  return pairs.join("");
}

/** Reads the current PDF inventory and mutation precondition for one work. */
export function pdfStatus(workID: Identifier): Promise<PDFStatus> {
  return api<PDFStatus>(`/api/works/${encodeURIComponent(workID)}/pdf-status`);
}

/** Computes one complete PDF upload body identity and captures the current mutation ETag. */
export async function preparePDFUpload(file: File, status: PDFStatus): Promise<PDFUploadRequest> {
  if (!file.size) throw new Error("Select a non-empty PDF file.");
  if (!status.mutation_etag) throw new Error("The PDF selection has no current mutation precondition.");
  const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
  return {
    file: file,
    size: file.size,
    sha256: digestHex(digest),
    mutationETag: status.mutation_etag,
  };
}

/** Uploads or replaces one PDF using the current selection's conditional ETag. */
export function uploadPDF(runID: Identifier, revisionID: Identifier, request: PDFUploadRequest): Promise<PDFMutationResponse> {
  const path = `/api/runs/${encodeURIComponent(runID)}/articles/${encodeURIComponent(revisionID)}/pdf`;
  return api<PDFMutationResponse>(path, {}, {
    method: "PUT",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/pdf",
      "If-Match": request.mutationETag,
      "X-PDF-Size": String(request.size),
      "X-PDF-SHA256": request.sha256,
    },
    body: request.file,
    signal: null,
  });
}

/** Removes one PDF using the current selection's conditional ETag. */
export function removePDF(runID: Identifier, revisionID: Identifier, mutationETag: string): Promise<PDFMutationResponse> {
  const path = `/api/runs/${encodeURIComponent(runID)}/articles/${encodeURIComponent(revisionID)}/pdf`;
  return api<PDFMutationResponse>(path, {}, {
    method: "DELETE",
    headers: {
      Accept: "application/json",
      "If-Match": mutationETag,
    },
    signal: null,
  });
}


/** One bounded binary download policy. */
export interface BinaryDownloadOptions {
  contentType: string;
  maxBytes: number;
  signal?: AbortSignal | null;
}

/** One completed browser download preparation result. */
export interface BinaryDownloadResult {
  filename: string;
  bytes: number;
  included: number | null;
  skipped: number | null;
  annotated: number | null;
  comments: number | null;
  commentsStatus: string;
  sourceHash: string;
  outputHash: string;
  correlationID: string;
}

/** Reads a bounded response body and rejects truncation or excessive output. */
async function readBoundedResponse(response: Response, path: string, limit: number): Promise<Blob> {
  const declared = Number(response.headers.get("Content-Length") || 0);
  if (declared > limit) {
    try {
      await response.body?.cancel();
    } catch (_) {
      // The rejected response is already complete; no cleanup remains.
    }
    throw new Error(`The server response for ${path} exceeds the ${limit}-byte download limit.`);
  }
  if (!response.body) {
    const buffer = await response.arrayBuffer();
    if (buffer.byteLength > limit) throw new Error(`The server response for ${path} exceeds the ${limit}-byte download limit.`);
    return new Blob([buffer], { type: response.headers.get("Content-Type") || "application/octet-stream" });
  }
  const reader = response.body.getReader();
  const chunks: Uint8Array<ArrayBuffer>[] = [];
  var total = 0;
  while (true) {
    const result = await reader.read();
    if (result.done) break;
    const chunk = result.value;
    total += chunk.byteLength;
    if (total > limit) {
      try {
        await reader.cancel();
      } catch (_) {
        // The over-limit response is discarded regardless of cancellation outcome.
      }
      throw new Error(`The server response for ${path} exceeds the ${limit}-byte download limit.`);
    }
    chunks.push(new Uint8Array(chunk));
  }
  if (declared > 0 && total !== declared) throw new Error(`The server response for ${path} ended before the complete file was received.`);
  return new Blob(chunks, { type: response.headers.get("Content-Type") || "application/octet-stream" });
}

/** Extracts one safe browser filename from a Content-Disposition header. */
function downloadFilename(disposition: string, fallback: string): string {
  const extended = /filename\*=utf-8''([^;]+)/i.exec(disposition);
  if (extended) {
    try {
      return decodeURIComponent(extended[1]);
    } catch (_) {
      return fallback;
    }
  }
  const quoted = /filename="([^"]+)"/i.exec(disposition);
  if (quoted) return quoted[1];
  const plain = /filename=([^;]+)/i.exec(disposition);
  if (plain) return plain[1].trim();
  return fallback;
}

/** Hands one completed blob to the browser and revokes its object URL. */
function saveDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.rel = "noopener";
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  setTimeout(function() {
    URL.revokeObjectURL(url);
  }, 0);
}

/** Prepares one explicit PDF or ZIP download with shared JSON failure decoding. */
export async function downloadExport(path: string, body: unknown, options: BinaryDownloadOptions): Promise<BinaryDownloadResult> {
  const response = await fetch(path, {
    method: "POST",
    headers: {
      Accept: options.contentType,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
    signal: options.signal ?? null,
  });
  if (!response.ok) {
    const failure = await decodeJSONBody(response, path);
    throw responseError(response, failure);
  }
  const actualType = response.headers.get("Content-Type") || "";
  if (!actualType.toLowerCase().startsWith(options.contentType.toLowerCase())) {
    try {
      await response.body?.cancel();
    } catch (_) {
      // No successful download is created from an unexpected media type.
    }
    throw new Error(`The server returned ${actualType || "an unknown media type"} instead of ${options.contentType}.`);
  }
  const blob = await readBoundedResponse(response, path, options.maxBytes);
  const disposition = response.headers.get("Content-Disposition") || "";
  const fallback = options.contentType === "application/zip" ? "pdf-export.zip" : "download.pdf";
  const filename = downloadFilename(disposition, fallback);
  const includedHeader = response.headers.get("X-PDF-Export-Included");
  const skippedHeader = response.headers.get("X-PDF-Export-Skipped");
  const annotatedHeader = response.headers.get("X-PDF-Export-Annotated");
  const commentsHeader = response.headers.get("X-PDF-Export-Comments");
  const result: BinaryDownloadResult = {
    filename: filename,
    bytes: blob.size,
    included: includedHeader === null ? null : Number(includedHeader),
    skipped: skippedHeader === null ? null : Number(skippedHeader),
    annotated: annotatedHeader === null ? null : Number(annotatedHeader),
    comments: commentsHeader === null ? null : Number(commentsHeader),
    commentsStatus: response.headers.get("X-PDF-Export-Comments-Status") || "",
    sourceHash: response.headers.get("X-PDF-Export-Content-Hash") || "",
    outputHash: response.headers.get("X-PDF-Export-Output-Hash") || "",
    correlationID: response.headers.get("X-PDF-Export-Correlation-Id") || "",
  };
  saveDownload(blob, filename);
  return result;
}

/** Loads and caches the discovered database table list. */
export async function tables(): Promise<TableInfo[]> {
  if (!state.tables.length) {
    const data = await api<TablesResponse>("/api/tables");
    state.tables = list(data, ["tables", "items"]);
  }
  return state.tables;
}
