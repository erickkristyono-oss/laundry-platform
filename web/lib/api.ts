import { apiBaseUrl } from "./site";

// Mirrors the standard error envelope in docs/11-error-handling.md §1, so
// every form can surface `error.message` consistently.
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

type ApiFetchOptions = RequestInit & {
  /** Which stored session's bearer token to attach, if any. Defaults to
   * "customer" since most calls from this frontend are customer-facing;
   * pass "staff" from /staff/* pages, or "none" for a PUBLIC endpoint. */
  auth?: "customer" | "staff" | "none";
};

function currentAccessToken(auth: "customer" | "staff"): string | null {
  if (typeof window === "undefined") return null;
  try {
    const key =
      auth === "staff"
        ? "laundryku.staff_session"
        : "laundryku.customer_session";
    const raw = localStorage.getItem(key);
    if (!raw) return null;
    return (JSON.parse(raw) as { accessToken?: string }).accessToken ?? null;
  } catch {
    return null;
  }
}

/**
 * Thin wrapper around fetch() targeting the Gateway (docs/03-system-architecture.md §1
 * — the Gateway is the only entry point clients should call). Throws
 * ApiError for the standard error envelope, or a generic Error for
 * transport-level failures. Attaches an Idempotency-Key
 * (docs/11-error-handling.md §4) to every mutating request that doesn't
 * already carry one — required by the endpoints listed in
 * docs/07-api-contract.md §12 (order/payment/registration creation).
 */
export async function apiFetch<T>(
  path: string,
  options: ApiFetchOptions = {},
): Promise<T> {
  const { auth = "customer", headers, ...rest } = options;
  const method = (rest.method ?? "GET").toUpperCase();

  const finalHeaders: Record<string, string> = {
    "Content-Type": "application/json",
    ...(headers as Record<string, string> | undefined),
  };

  if (
    ["POST", "PUT", "PATCH"].includes(method) &&
    !finalHeaders["Idempotency-Key"]
  ) {
    finalHeaders["Idempotency-Key"] = crypto.randomUUID();
  }

  if (auth !== "none" && !finalHeaders["Authorization"]) {
    const token = currentAccessToken(auth);
    if (token) finalHeaders["Authorization"] = `Bearer ${token}`;
  }

  let res: Response;
  try {
    res = await fetch(`${apiBaseUrl}${path}`, { ...rest, headers: finalHeaders });
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
      // non-JSON error body — fall through to a generic error.
    }
    if (body?.error) throw new ApiError(body);
    throw new Error(`Permintaan gagal (${res.status}).`);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}
