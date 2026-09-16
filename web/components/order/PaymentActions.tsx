"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/Button";
import { apiFetch, ApiError } from "@/lib/api";
import type { Payment } from "@/lib/types";

const methods: { value: "QRIS" | "TRANSFER" | "CARD"; label: string }[] = [
  { value: "QRIS", label: "QRIS" },
  { value: "TRANSFER", label: "Transfer Bank" },
  { value: "CARD", label: "Kartu Debit/Kredit" },
];

export function PaymentActions({ orderId, amount }: { orderId: string; amount: number }) {
  const router = useRouter();
  const [method, setMethod] = useState<"QRIS" | "TRANSFER" | "CARD">("QRIS");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function payNow() {
    setLoading(true);
    setError(null);
    try {
      const payment = await apiFetch<Payment>("/api/v1/payments", {
        method: "POST",
        body: JSON.stringify({ order_id: orderId, amount, method }),
      });
      if (payment.checkout_url) {
        const url = new URL(payment.checkout_url);
        router.push(url.pathname);
      }
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : "Tidak dapat memulai pembayaran, coba lagi.",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="mt-4 rounded-xl border border-slate-100 bg-slate-50 p-4">
      <p className="text-sm font-medium text-navy-900">Bayar online sekarang</p>
      <div className="mt-3 flex flex-wrap gap-2">
        {methods.map((m) => (
          <button
            key={m.value}
            type="button"
            onClick={() => setMethod(m.value)}
            className={`rounded-full px-4 py-1.5 text-xs font-semibold transition-colors ${
              method === m.value
                ? "bg-brand-600 text-white"
                : "bg-white text-slate-600 ring-1 ring-slate-200 hover:bg-slate-100"
            }`}
          >
            {m.label}
          </button>
        ))}
      </div>
      {error && <p className="mt-3 text-sm text-red-600">{error}</p>}
      <Button
        type="button"
        onClick={payNow}
        disabled={loading}
        className="mt-4 w-full"
      >
        {loading ? "Memproses…" : "Bayar Sekarang"}
      </Button>
    </div>
  );
}
