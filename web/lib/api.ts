import { apiBaseUrl } from "./site";

// Mirrors the standard error envelope in docs/11-error-handling.md §1, so
// every form can surface `error.message` consistently once the backend
// handlers (Phase 2) exist.
export type ApiErrorBody = {
  error: {
    code: string;
    message: string;
    details?: { field: string; issue: string }[];
    request_id: string;
    correlation_id?: string;
  };
};

export class ApiError extends Error {
  code: string;
  details?: { field: string; issue: string }[];

  constructor(body: ApiErrorBody) {
    super(body.error.message);
    this.code = body.error.code;
    this.details = body.error.details;
  }
}

/**
 * Thin wrapper around fetch() targeting the Gateway (docs/03-system-architecture.md §1
 * — the Gateway is the only entry point clients should call). Throws
 * ApiError for the standard error envelope, or a generic Error for
 * transport-level failures (e.g. the backend not being reachable yet,
 * since Phase 2 handlers are not implemented at this stage).
 */
export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${apiBaseUrl}${path}`, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...options.headers,
      },
    });
  } catch {
    throw new Error(
      "Tidak dapat terhubung ke server. Coba lagi beberapa saat lagi.",
    );
  }

  if (!res.ok) {
    let body: ApiErrorBody | null = null;
    try {
      body = await res.json();
    } catch {
      // non-JSON error body (e.g. a raw 404 from an unmounted route while
      // Phase 2 handlers don't exist yet) — fall through to generic error.
    }
    if (body?.error) throw new ApiError(body);
    throw new Error(`Permintaan gagal (${res.status}).`);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}
