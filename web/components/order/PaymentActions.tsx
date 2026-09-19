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

type Choice = "PAY_NOW" | "PAY_LATER" | null;

export function PaymentActions({
  orderId,
  amount,
  fulfillmentType,
}: {
  orderId: string;
  amount?: number;
  fulfillmentType: string;
}) {
  const router = useRouter();
  const [choice, setChoice] = useState<Choice>(null);
  const [method, setMethod] = useState<"QRIS" | "TRANSFER" | "CARD">("QRIS");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Not payable yet: total_amount only exists after staff finalizes
  // weighing (docs/07-api-contract.md §11 — CreatePayment requires it).
  // Shown disabled rather than hidden entirely, so the option is visibly
  // "coming, not missing" — same pattern as the outlet/service picks.
  const payable = amount !== undefined;
  const isDelivery = fulfillmentType === "DELIVERY" || fulfillmentType === "PICKUP_AND_DELIVERY";

  async function payNow() {
    if (!payable) return;
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
    <div className={`mt-4 rounded-xl border border-slate-100 bg-slate-50 p-4 ${!payable ? "opacity-60" : ""}`}>
      <p className="text-sm font-medium text-navy-900">Kapan mau bayar?</p>
      {!payable && (
        <p className="mt-1 text-xs text-slate-500">
          Pilihan ini tersedia setelah staf outlet selesai menimbang cucian Anda.
        </p>
      )}

      {choice === null && (
        <div className="mt-3 grid gap-2 sm:grid-cols-2">
          <button
            type="button"
            disabled={!payable}
            onClick={() => setChoice("PAY_NOW")}
            className="rounded-xl border border-slate-200 bg-white p-3 text-left text-sm transition-colors enabled:hover:border-brand-300 enabled:hover:bg-brand-50 disabled:cursor-not-allowed"
          >
            <span className="font-semibold text-navy-900">💳 Bayar Sekarang</span>
            <span className="mt-1 block text-xs text-slate-500">QRIS, transfer bank, atau kartu — online, langsung.</span>
          </button>
          <button
            type="button"
            disabled={!payable}
            onClick={() => setChoice("PAY_LATER")}
            className="rounded-xl border border-slate-200 bg-white p-3 text-left text-sm transition-colors enabled:hover:border-brand-300 enabled:hover:bg-brand-50 disabled:cursor-not-allowed"
          >
            <span className="font-semibold text-navy-900">🏪 Bayar Saat {isDelivery ? "Pengantaran" : "Pengambilan"}</span>
            <span className="mt-1 block text-xs text-slate-500">Bayar tunai ke staf {isDelivery ? "saat cucian diantar" : "di outlet"}.</span>
          </button>
        </div>
      )}

      {choice === "PAY_LATER" && (
        <div className="mt-3 rounded-xl bg-brand-50 p-3 text-sm text-brand-800">
          <p>
            Baik! Siapkan pembayaran tunai saat {isDelivery ? "cucian diantar ke Anda" : "Anda mengambil cucian di outlet"}.
            Masih bisa berubah pikiran dan bayar online kapan saja sebelum itu.
          </p>
          <button
            type="button"
            onClick={() => setChoice(null)}
            className="mt-2 text-xs font-semibold text-brand-700 underline"
          >
            ← Ubah pilihan
          </button>
        </div>
      )}

      {choice === "PAY_NOW" && (
        <div className="mt-3">
          <div className="flex flex-wrap gap-2">
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
          <Button type="button" onClick={payNow} disabled={loading} className="mt-3 w-full">
            {loading ? "Memproses…" : "Bayar Sekarang"}
          </Button>
          <button
            type="button"
            onClick={() => setChoice(null)}
            className="mt-2 text-xs font-semibold text-slate-500 underline"
          >
            ← Ubah pilihan
          </button>
        </div>
      )}
    </div>
  );
}
