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

/** Fetches and decodes one JSON API response. */
export async function api(path, query) {
  const response = await fetch(endpoint(path, query), {
    headers: { Accept: 'application/json' },
    signal: state.controller?.signal,
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
    throw new Error(message);
  }
  if (body?.data !== undefined) {
    return body.data;
  }
  return body;
}

/** Loads and caches the discovered database table list. */
export async function tables() {
  if (!state.tables.length) {
    const data = await api('/api/tables');
    state.tables = list(data, ['tables', 'items']);
  }
  return state.tables;
}