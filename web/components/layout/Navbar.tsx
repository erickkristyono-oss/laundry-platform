"use client";

import { useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Container } from "@/components/ui/Container";
import { Button, LinkButton } from "@/components/ui/Button";
import { Logo } from "@/components/ui/Logo";
import { getCustomerSession, clearCustomerSession, subscribeToAuthChanges } from "@/lib/auth";

const getServerSnapshot = () => null;

const links = [
  { href: "/#layanan", label: "Layanan" },
  { href: "/#cara-kerja", label: "Cara Kerja" },
  { href: "/#kenapa-kami", label: "Kenapa Kami" },
  { href: "/#outlet", label: "Outlet" },
];

export function Navbar() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const customer = useSyncExternalStore(subscribeToAuthChanges, getCustomerSession, getServerSnapshot);

  function logout() {
    clearCustomerSession();
    setOpen(false);
    router.push("/");
  }

  return (
    <header className="sticky top-0 z-50 border-b border-slate-100 bg-white/90 backdrop-blur">
      <Container className="flex h-16 items-center justify-between py-3">
        <Link href="/" onClick={() => setOpen(false)}>
          <Logo />
        </Link>

        <nav className="hidden items-center gap-8 md:flex">
          {links.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className="text-sm font-medium text-slate-600 transition-colors hover:text-brand-700"
            >
              {link.label}
            </Link>
          ))}
        </nav>

        <div className="hidden items-center gap-3 md:flex">
          {customer ? (
            <>
              <LinkButton href="/dashboard" variant="ghost" className="px-4 py-2 text-sm">
                {customer.customer.name || "Dashboard"}
              </LinkButton>
              <Button variant="outline" onClick={logout} className="!border-brand-600 !text-brand-700 px-5 py-2.5 text-sm">
                Keluar
              </Button>
            </>
          ) : (
            <>
              <LinkButton href="/login" variant="ghost" className="px-4 py-2 text-sm">
                Masuk
              </LinkButton>
              <LinkButton href="/register" className="px-5 py-2.5 text-sm">
                Daftar
              </LinkButton>
            </>
          )}
        </div>

        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="inline-flex h-10 w-10 items-center justify-center rounded-lg text-slate-700 md:hidden"
          aria-label="Buka menu"
          aria-expanded={open}
        >
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            {open ? (
              <path strokeLinecap="round" d="M6 6l12 12M18 6L6 18" />
            ) : (
              <path strokeLinecap="round" d="M4 7h16M4 12h16M4 17h16" />
            )}
          </svg>
        </button>
      </Container>

      {open && (
        <div className="border-t border-slate-100 bg-white md:hidden">
          <Container className="flex flex-col gap-4 py-5">
            {links.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                onClick={() => setOpen(false)}
                className="text-sm font-medium text-slate-600"
              >
                {link.label}
              </Link>
            ))}
            <div className="mt-2 flex flex-col gap-3">
              {customer ? (
                <>
                  <LinkButton href="/dashboard" variant="outline" className="!border-brand-600 !text-brand-700 w-full" onClick={() => setOpen(false)}>
                    {customer.customer.name || "Dashboard"}
                  </LinkButton>
                  <Button onClick={logout} className="w-full">
                    Keluar
                  </Button>
                </>
              ) : (
                <>
                  <LinkButton href="/login" variant="outline" className="!border-brand-600 !text-brand-700 w-full">
                    Masuk
                  </LinkButton>
                  <LinkButton href="/register" className="w-full">
                    Daftar
                  </LinkButton>
                </>
              )}
            </div>
          </Container>
        </div>
      )}
    </header>
  );
}
