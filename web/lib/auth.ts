"use client";

// Client-side session storage for the two principal types this platform
// issues tokens for (docs/06-database-schema.md §1.9, ADR-013): a customer
// and a staff session are kept under separate keys so switching between
// /login (customer) and /staff/login never mixes them up.

export type StoredCustomer = {
  accessToken: string;
  refreshToken: string;
  customer: { id: string; name: string; customer_code: string };
};

export type StoredStaff = {
  accessToken: string;
  refreshToken: string;
  user: { id: string; full_name: string; roles: string[] };
};

const CUSTOMER_KEY = "laundryku.customer_session";
const STAFF_KEY = "laundryku.staff_session";

// Tiny same-tab pub-sub so useSyncExternalStore (lib/useSession.ts) can
// react to a login/logout that happened in this same tab — the browser's
// own "storage" event only fires for *other* tabs.
type Listener = () => void;
const listeners = new Set<Listener>();
function notifyChange() {
  listeners.forEach((l) => l());
}
export function subscribeToAuthChanges(listener: Listener) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

// useSyncExternalStore requires getSnapshot to return a referentially
// stable value when the underlying data hasn't changed (it compares with
// Object.is) — re-parsing the raw string on every call would return a new
// object each time and trigger React's "getSnapshot should be cached"
// infinite-loop guard. Cache the parsed result per key, keyed by the raw
// string, and only re-parse when the raw string actually differs.
const parseCache = new Map<string, { raw: string; parsed: unknown }>();

function readJSON<T>(key: string): T | null {
  let raw: string | null;
  try {
    raw = localStorage.getItem(key);
  } catch {
    // Private browsing / storage disabled — treat as "not logged in"
    // rather than throwing (see artifact/browser-storage conventions:
    // never let storage failures break the page).
    return null;
  }
  if (raw === null) {
    parseCache.delete(key);
    return null;
  }

  const cached = parseCache.get(key);
  if (cached && cached.raw === raw) return cached.parsed as T;

  try {
    const parsed = JSON.parse(raw) as T;
    parseCache.set(key, { raw, parsed });
    return parsed;
  } catch {
    return null;
  }
}

function writeJSON(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Ignore — the session just won't persist across reloads.
  }
}

export function getCustomerSession(): StoredCustomer | null {
  return readJSON<StoredCustomer>(CUSTOMER_KEY);
}

export function setCustomerSession(session: StoredCustomer) {
  writeJSON(CUSTOMER_KEY, session);
  notifyChange();
}

export function clearCustomerSession() {
  try {
    localStorage.removeItem(CUSTOMER_KEY);
  } catch {
    /* ignore */
  }
  notifyChange();
}

export function getStaffSession(): StoredStaff | null {
  return readJSON<StoredStaff>(STAFF_KEY);
}

export function setStaffSession(session: StoredStaff) {
  writeJSON(STAFF_KEY, session);
  notifyChange();
}

export function clearStaffSession() {
  try {
    localStorage.removeItem(STAFF_KEY);
  } catch {
    /* ignore */
  }
  notifyChange();
}
