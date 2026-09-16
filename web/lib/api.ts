import { apiBaseUrl } from "./site";
import {
  getCustomerSession,
  getStaffSession,
  setCustomerSession,
  setStaffSession,
} from "./auth";

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
  const session = auth === "staff" ? getStaffSession() : getCustomerSession();
  return session?.accessToken ?? null;
}

// Access tokens expire after 15 minutes (docs/07-api-contract.md §1); rather
// than force a re-login on every expiry, apiFetch silently exchanges the
// stored refresh_token for a new pair and retries once. inFlightRefresh
// dedupes concurrent 401s (e.g. a page firing several requests at once)
// into a single refresh call instead of one per request.
const inFlightRefresh: Record<"customer" | "staff", Promise<string | null> | null> = {
  customer: null,
  staff: null,
};

async function refreshAccessToken(auth: "customer" | "staff"): Promise<string | null> {
  if (inFlightRefresh[auth]) return inFlightRefresh[auth];

  const attempt = (async () => {
    const path = auth === "staff" ? "/api/v1/auth/refresh" : "/api/v1/customer-auth/refresh";
    const session = auth === "staff" ? getStaffSession() : getCustomerSession();
    if (!session?.refreshToken) return null;

    try {
      const res = await fetch(`${apiBaseUrl}${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: session.refreshToken }),
      });
      if (!res.ok) return null;
      const body = await res.json();

      if (auth === "staff" && "user" in session) {
        setStaffSession({ ...session, accessToken: body.access_token, refreshToken: body.refresh_token });
      } else if ("customer" in session) {
        setCustomerSession({ ...session, accessToken: body.access_token, refreshToken: body.refresh_token });
      }
      return body.access_token as string;
    } catch {
      return null;
    }
  })();

  inFlightRefresh[auth] = attempt;
  try {
    return await attempt;
  } finally {
    inFlightRefresh[auth] = null;
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

  const hadExplicitAuthHeader = !!finalHeaders["Authorization"];
  if (auth !== "none" && !hadExplicitAuthHeader) {
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

  // A 15-minute-old access token (docs/07-api-contract.md §1) shouldn't force
  // a re-login mid-session — silently refresh once and replay the request.
  if (res.status === 401 && auth !== "none" && !hadExplicitAuthHeader) {
    const newToken = await refreshAccessToken(auth);
    if (newToken) {
      finalHeaders["Authorization"] = `Bearer ${newToken}`;
      try {
        res = await fetch(`${apiBaseUrl}${path}`, { ...rest, headers: finalHeaders });
      } catch {
        throw new Error(
          "Tidak dapat terhubung ke server. Coba lagi beberapa saat lagi.",
        );
      }
    }
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
