// Shared fetch-response fixtures for frontend unit tests.

/** Builds one successful API response envelope. */
export function apiResponse(data: unknown): Promise<Response> {
  return Promise.resolve({
    ok: true,
    status: 200,
    json: () => {
      return Promise.resolve({ data: data });
    },
  } as unknown as Response);
}
