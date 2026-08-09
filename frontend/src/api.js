// API fetch helpers and endpoint builder.
import { state, list } from './state.js';

/** Builds an API path and query string from supplied values. */
export function endpoint(path, query) {
  if (!query) {
    query = {};
  }
  const url = new URL(path, location.origin);
  Object.entries(query).forEach(function([key, raw]) {
    if (raw !== '' && raw !== null && raw !== undefined) {
      url.searchParams.set(key, raw);
    }
  });
  return url.pathname + url.search;
}

/** Represents an HTTP API failure while preserving its status and structured details. */
export class APIError extends Error {
  /** Initializes one structured API error returned by a non-successful response. */
  constructor(message, status, code, details) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.code = code || 'request_failed';
    this.details = details;
  }
}

/** Fetches and decodes one JSON API response. */
export async function api(path, query, options) {
  const request = options || {};
  const headers = { Accept: 'application/json', ...(request.headers || {}) };
  const response = await fetch(endpoint(path, query), {
    ...request,
    headers: headers,
    signal: request.signal || state.controller?.signal,
  });
  var body;
  try {
    body = await response.json();
  } catch (_) {
    throw new Error('The server returned invalid JSON for ' + path + '.');
  }
  if (!response.ok) {
    var message;
    if (body?.error?.message) {
      message = body.error.message;
    } else {
      message = 'Request failed (' + response.status + ').';
    }
    throw new APIError(message, response.status, body?.error?.code, body?.error?.details);
  }
  if (body?.data !== undefined) {
    return body.data;
  }
  return body;
}

/** Sends a same-origin JSON mutation and returns its decoded response. */
export function mutate(path, method, body) {
  return api(path, null, {
    method: method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

/** Loads and caches the discovered database table list. */
export async function tables() {
  if (!state.tables.length) {
    const data = await api('/api/tables');
    state.tables = list(data, ['tables', 'items']);
  }
  return state.tables;
}
