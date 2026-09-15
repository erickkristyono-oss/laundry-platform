"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { AuthLayout } from "@/components/layout/AuthLayout";
import { Field, Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { apiFetch, ApiError } from "@/lib/api";
import { setStaffSession } from "@/lib/auth";

// POST /api/v1/auth/login — docs/07-api-contract.md §1 (staff/admin, distinct
// from customer-auth per ADR-013 in docs/14-architecture-decisions.md).
type LoginResponse = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user: { id: string; full_name: string; roles: string[] };
};

export default function StaffLoginPage() {
  const router = useRouter();
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const res = await apiFetch<LoginResponse>("/api/v1/auth/login", {
        method: "POST",
        auth: "none",
        body: JSON.stringify({ identifier, password }),
      });
      setStaffSession({
        accessToken: res.access_token,
        refreshToken: res.refresh_token,
        user: res.user,
      });
      router.push("/staff");
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : "Terjadi kesalahan, coba lagi.",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthLayout
      title="Portal Staf"
      subtitle="Khusus untuk pegawai outlet dan admin bisnis."
      footer={
        <p>
          Bukan staf?{" "}
          <Link href="/login" className="font-semibold text-brand-700 hover:underline">
            Masuk sebagai pelanggan
          </Link>
        </p>
      }
    >
      <form className="space-y-5" onSubmit={onSubmit}>
        <Field label="Email atau Nomor HP">
          <Input
            type="text"
            required
            autoComplete="username"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            placeholder="staf@perusahaan.com"
          />
        </Field>

        <Field label="Kata Sandi">
          <Input
            type="password"
            required
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="••••••••"
          />
        </Field>

        {error && (
          <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {error}
          </p>
        )}

        <Button type="submit" disabled={loading} className="w-full">
          {loading ? "Memproses…" : "Masuk"}
        </Button>
      </form>
    </AuthLayout>
  );
}
