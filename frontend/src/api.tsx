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

  var body: unknown;
  try {
    body = await response.json();
  } catch (_) {
    throw new Error(`The server returned invalid JSON for ${path}.`);
  }

  if (!response.ok) {
    var message = `Request failed (${response.status}).`;
    const envelope = body as APIErrorEnvelope;
    if (envelope?.error?.message) message = envelope.error.message;
    throw new APIError(message, response.status, envelope?.error?.code, envelope?.error?.details);
  }

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

/** Loads and caches the discovered database table list. */
export async function tables(): Promise<TableInfo[]> {
  if (!state.tables.length) {
    const data = await api<TablesResponse>("/api/tables");
    state.tables = list(data, ["tables", "items"]);
  }
  return state.tables;
}
