"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { AuthLayout } from "@/components/layout/AuthLayout";
import { Field, Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { apiFetch, ApiError } from "@/lib/api";

// POST /api/v1/customer-auth/register — docs/07-api-contract.md §2 (UQ-03 resolution).
type RegisterResponse = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  customer: { id: string; name: string; customer_code: string };
};

export default function CustomerRegisterPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await apiFetch<RegisterResponse>("/api/v1/customer-auth/register", {
        method: "POST",
        body: JSON.stringify({ name, phone, email: email || undefined, password }),
      });
      router.push("/");
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
      title="Buat Akun Baru"
      subtitle="Gratis, dan pesanan pertama Anda hanya butuh beberapa menit."
      footer={
        <p>
          Sudah punya akun?{" "}
          <Link href="/login" className="font-semibold text-brand-700 hover:underline">
            Masuk di sini
          </Link>
        </p>
      }
    >
      <form className="space-y-5" onSubmit={onSubmit}>
        <Field label="Nama Lengkap">
          <Input
            type="text"
            required
            autoComplete="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Nama Anda"
          />
        </Field>

        <Field label="Nomor HP">
          <Input
            type="tel"
            required
            autoComplete="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder="08xxxxxxxxxx"
          />
        </Field>

        <Field label="Email (opsional)">
          <Input
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="anda@email.com"
          />
        </Field>

        <Field label="Kata Sandi">
          <Input
            type="password"
            required
            minLength={8}
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Minimal 8 karakter"
          />
        </Field>

        {error && (
          <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {error}
          </p>
        )}

        <Button type="submit" disabled={loading} className="w-full">
          {loading ? "Memproses…" : "Daftar"}
        </Button>
      </form>
    </AuthLayout>
  );
}
