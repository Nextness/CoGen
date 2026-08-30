// API fetch helpers and endpoint builder.
import { state, list } from "./state.tsx";
import type { APIQuery, TableInfo, TablesResponse, WireRecord } from "./api/types.ts";

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
  method: string;
  headers: Record<string, string>;
  body?: string;
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
export async function api<T>(path: string, query: APIQuery = {}, options: APIRequestOptions): Promise<T> {
  var signal = options.signal;
  if (signal === undefined) signal = state.controller?.signal;
  const response = await fetch(endpoint(path, query), {
    method: options.method,
    headers: options.headers,
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

/** Loads and caches the discovered database table list. */
export async function tables(): Promise<TableInfo[]> {
  if (!state.tables.length) {
    const data = await api<TablesResponse>("/api/tables", {}, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    state.tables = list(data, ["tables", "items"]);
  }
  return state.tables;
}
