// API fetch helpers and endpoint builder.
import { state, list } from "./state.tsx";

/** Builds an API path and query string from supplied values. */
export function endpoint(path: string, query: Record<string, any> = {}): string {
  const url = new URL(path, location.origin);
  Object.entries(query).forEach(function([key, raw]) {
    if (raw !== "" && raw !== null && raw !== undefined) {
      url.searchParams.set(key, raw);
    }
  });
  return url.pathname + url.search;
}

/** One structured API error detail envelope returned by the server. */
export interface APIErrorEnvelope {
  error?: {
    message?: string;
    code?: string;
    details?: any;
  };
}

/** One API request option set forwarded to fetch. */
export interface APIRequestOptions {
  method: string;
  headers: Record<string, string>;
  body?: string;
  signal?: AbortSignal;
}

/** Represents an HTTP API failure while preserving its status and structured details. */
export class APIError extends Error {
  status: number;
  code: string;
  details: any;

  /** Initializes one structured API error returned by a non-successful response. */
  constructor(message: string, status: number, code?: string, details?: any) {
    super(message);
    this.name = "APIError";
    this.status = status;

    this.code = code ?? "request_failed";
    this.details = details ?? "No details provided";
  }
}

/** Fetches and decodes one JSON API response. */
export async function api(path: string, query: Record<string, any> = {}, options: APIRequestOptions): Promise<any> {
  const response = await fetch(endpoint(path, query), {
    method: options.method,
    headers: options.headers,
    body: options.body,
    signal: options.signal ?? state.controller?.signal,
  });

  try {
    var body = await response.json();
  } catch (_) {
    throw new Error(`The server returned invalid JSON for ${path}.`);
  }

  if (!response.ok) {
    var message = `Request failed (${response.status}).`;
    if (body?.error?.message) message = body.error.message;
    throw new APIError(message, response.status, body?.error?.code, body?.error?.details);
  }

  if (body?.data !== undefined) return body.data;
  return body;
}

/** Sends a same-origin JSON mutation and returns its decoded response. */
export function mutate(path: string, method: string, body: any): Promise<any> {
  return api(path, {}, {
    method: method,
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** Loads and caches the discovered database table list. */
export async function tables(): Promise<any[]> {
  if (!state.tables.length) {
    const data = await api("/api/tables", {}, { method: "GET", headers: { Accept: "application/json" } });
    state.tables = list(data, ["tables", "items"]);
  }
  return state.tables;
}
