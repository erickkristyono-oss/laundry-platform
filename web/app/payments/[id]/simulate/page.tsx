"use client";

import { use, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Container } from "@/components/ui/Container";
import { Button } from "@/components/ui/Button";
import { useCustomerSession } from "@/lib/useSession";
import { apiFetch, ApiError } from "@/lib/api";
import { formatIDR, type Payment } from "@/lib/types";

export default function SimulatePaymentPage({
  params,
}: PageProps<"/payments/[id]/simulate">) {
  const { id } = use(params);
  const router = useRouter();
  const session = useCustomerSession();
  const [payment, setPayment] = useState<Payment | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!session) return;
    apiFetch<Payment>(`/api/v1/payments/${id}`)
      .then(setPayment)
      .catch(() => setError("Pembayaran tidak ditemukan."));
  }, [session, id]);

  if (!session) return null;

  async function resolve(outcome: "PAID" | "FAILED") {
    setLoading(true);
    setError(null);
    try {
      await apiFetch(`/api/v1/payments/${id}/simulate`, {
        method: "POST",
        body: JSON.stringify({ outcome }),
      });
      if (payment) router.push(`/orders/${payment.order_id}`);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : "Gagal memproses simulasi.",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="bg-slate-50 py-12">
      <Container className="max-w-md">
        <div className="rounded-2xl border border-slate-100 bg-white p-6 shadow-sm sm:p-8">
          <p className="text-xs font-semibold uppercase tracking-wide text-amber-600">
            Simulasi Pembayaran (belum terhubung ke payment gateway asli)
          </p>
          <h1 className="mt-2 font-heading text-xl font-bold text-navy-900">
            Halaman Simulasi Checkout
          </h1>
          <p className="mt-2 text-sm text-slate-500">
            Karena belum ada akun payment gateway resmi yang terdaftar, halaman ini
            menggantikan sementara alur pembayaran online (QRIS/transfer/kartu) supaya
            fiturnya bisa dicoba dari sekarang. Begitu akun gateway asli tersedia,
            halaman ini akan diganti dengan redirect ke provider sungguhan.
          </p>

          {error && <p className="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
          {!payment && !error && <p className="mt-4 text-sm text-slate-500">Memuat…</p>}

          {payment && (
            <>
              <div className="mt-6 space-y-2 rounded-xl bg-slate-50 p-4 text-sm">
                <div className="flex justify-between">
                  <span className="text-slate-500">Kode Pembayaran</span>
                  <span className="font-medium text-navy-900">{payment.payment_code}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Metode</span>
                  <span className="font-medium text-navy-900">{payment.method}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Jumlah</span>
                  <span className="font-semibold text-navy-900">{formatIDR(payment.amount)}</span>
                </div>
                {payment.qr_string && (
                  <div className="border-t border-slate-200 pt-2">
                    <p className="text-slate-500">Kode QR (dummy, bukan QRIS asli)</p>
                    <p className="mt-1 break-all font-mono text-xs text-slate-400">{payment.qr_string}</p>
                  </div>
                )}
              </div>

              {payment.status === "PENDING" ? (
                <div className="mt-6 flex gap-3">
                  <Button onClick={() => resolve("PAID")} disabled={loading} className="w-full">
                    Simulasikan Berhasil
                  </Button>
                  <Button
                    variant="outline"
                    className="w-full !border-red-300 !text-red-600 hover:!bg-red-50"
                    onClick={() => resolve("FAILED")}
                    disabled={loading}
                  >
                    Simulasikan Gagal
                  </Button>
                </div>
              ) : (
                <p className="mt-6 text-sm font-medium text-slate-600">
                  Pembayaran ini sudah berstatus {payment.status}.
                </p>
              )}
            </>
          )}
        </div>
      </Container>
    </div>
  );
}
