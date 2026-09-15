"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  getCustomerSession,
  getStaffSession,
  type StoredCustomer,
  type StoredStaff,
} from "./auth";

// Deliberately NOT useSyncExternalStore here (unlike Navbar's read-only
// display, which uses it safely): its SSR snapshot is always null, and an
// effect that redirects the instant it sees `session === null` can fire
// on that transient SSR value before React's client-side correction
// lands — verified empirically (a logged-in visit to /dashboard bounced
// to /login even though the Navbar, rendered in the same commit, showed
// the correct logged-in state). Reading the real session directly inside
// this effect avoids the race, at the cost of the setState-in-effect
// pattern below (justified: localStorage cannot be read at render time).

/** Redirects to /login if no customer session exists; otherwise returns it.
 * `undefined` while the check is in flight (avoids a flash of redirect on
 * first paint), `null` only ever momentarily before the redirect fires. */
export function useCustomerSession(): StoredCustomer | null | undefined {
  const router = useRouter();
  const [session, setSession] = useState<StoredCustomer | null | undefined>(undefined);

  useEffect(() => {
    const s = getCustomerSession();
    if (!s) {
      router.replace("/login");
      return;
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect -- see file-level note above
    setSession(s);
  }, [router]);

  return session;
}

/** Same as useCustomerSession, for the staff principal type. */
export function useStaffSession(): StoredStaff | null | undefined {
  const router = useRouter();
  const [session, setSession] = useState<StoredStaff | null | undefined>(undefined);

  useEffect(() => {
    const s = getStaffSession();
    if (!s) {
      router.replace("/staff/login");
      return;
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect -- see file-level note above
    setSession(s);
  }, [router]);

  return session;
}
