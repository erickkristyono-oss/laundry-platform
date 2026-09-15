"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { AuthLayout } from "@/components/layout/AuthLayout";
import { Field, Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { apiFetch, ApiError } from "@/lib/api";
import { setCustomerSession } from "@/lib/auth";

// POST /api/v1/customer-auth/login — docs/07-api-contract.md §2 (UQ-03 resolution).
type LoginResponse = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  customer: { id: string; name: string; customer_code: string };
};

export default function CustomerLoginPage() {
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
      const res = await apiFetch<LoginResponse>("/api/v1/customer-auth/login", {
        method: "POST",
        auth: "none",
        body: JSON.stringify({ identifier, password }),
      });
      setCustomerSession({
        accessToken: res.access_token,
        refreshToken: res.refresh_token,
        customer: res.customer,
      });
      router.push("/dashboard");
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
      title="Masuk ke Akun Anda"
      subtitle="Pantau pesanan dan riwayat laundry Anda."
      footer={
        <p>
          Belum punya akun?{" "}
          <Link href="/register" className="font-semibold text-brand-700 hover:underline">
            Daftar sekarang
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
            placeholder="anda@email.com"
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
