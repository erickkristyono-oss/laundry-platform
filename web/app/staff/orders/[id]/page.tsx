"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { Container } from "@/components/ui/Container";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { StatusBadge, PaymentBadge } from "@/components/order/StatusBadge";
import { StatusTimeline } from "@/components/order/StatusTimeline";
import { useStaffSession } from "@/lib/useSession";
import { apiFetch, ApiError } from "@/lib/api";
import { formatIDR, type Order } from "@/lib/types";

const NEXT_STATUS: Partial<Record<Order["status"], { to: Order["status"]; label: string }[]>> = {
  WASHING: [{ to: "DRYING", label: "Lanjut ke Pengeringan" }],
  DRYING: [{ to: "IRONING", label: "Lanjut ke Setrika" }],
  IRONING: [{ to: "PACKING", label: "Lanjut ke Pengemasan" }],
  PACKING: [{ to: "READY", label: "Tandai Siap Diambil" }],
  READY: [
    { to: "PICKED_UP", label: "Diambil Pelanggan" },
    { to: "DELIVERED", label: "Diantar ke Pelanggan" },
  ],
  PICKED_UP: [{ to: "COMPLETED", label: "Selesaikan Pesanan" }],
  DELIVERED: [{ to: "COMPLETED", label: "Selesaikan Pesanan" }],
};

export default function StaffOrderDetailPage({
  params,
}: PageProps<"/staff/orders/[id]">) {
  const { id } = use(params);
  const session = useStaffSession();
  const [order, setOrder] = useState<Order | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [syncingPayment, setSyncingPayment] = useState(false);
  const [weights, setWeights] = useState<Record<string, string>>({});

  function refresh() {
    return apiFetch<Order>(`/api/v1/orders/${id}`, { auth: "staff" })
      .then((o) => {
        setOrder(o);
        setError(null);
      })
      .catch(() => {
        setError("Pesanan tidak ditemukan atau Anda tidak berhak melihatnya.");
      });
  }

  useEffect(() => {
    if (!session) return;
    apiFetch<Order>(`/api/v1/orders/${id}`, { auth: "staff" })
      .then((o) => {
        setOrder(o);
        setError(null);
      })
      .catch(() => {
        setError("Pesanan tidak ditemukan atau Anda tidak berhak melihatnya.");
      });
  }, [session, id]);

  if (!session) return null;

  async function runAction(action: () => Promise<unknown>) {
    setBusy(true);
    setError(null);
    try {
      await action();
      await refresh();
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : "Terjadi kesalahan, coba lagi.",
      );
    } finally {
      setBusy(false);
    }
  }

  const changeStatus = (to: Order["status"]) =>
    runAction(() =>
      apiFetch(`/api/v1/orders/${id}/status`, {
        method: "POST",
        auth: "staff",
        body: JSON.stringify({ to_status: to }),
      }),
    );

  const saveWeight = (itemId: string) =>
    runAction(() =>
      apiFetch(`/api/v1/orders/${id}/items/${itemId}/weigh`, {
        method: "PUT",
        auth: "staff",
        body: JSON.stringify({ actual_weight_kg: Number(weights[itemId]) }),
      }),
    );

  const finalizeWeighing = () =>
    runAction(() =>
      apiFetch(`/api/v1/orders/${id}/finalize-weighing`, { method: "POST", auth: "staff" }),
    );

  async function acceptCashPayment() {
    await runAction(() =>
      apiFetch("/api/v1/payments", {
        method: "POST",
        auth: "staff",
        body: JSON.stringify({ order_id: id, amount: order?.total_amount, method: "CASH" }),
      }),
    );
    // The payment itself settles instantly, but Core's copy of
    // payment_status (and the WEIGHING -> WASHING transition, which
    // unlocks the next-status buttons below) updates via an async event
    // — typically ~2-4s behind (shared/outbox's relay polls every 2s).
    // Without this, the page looks "stuck" on WEIGHING right after a
    // successful payment until someone thinks to reload it manually.
    setSyncingPayment(true);
    for (let attempt = 0; attempt < 6; attempt++) {
      await new Promise((r) => setTimeout(r, 1500));
      const updated = await apiFetch<Order>(`/api/v1/orders/${id}`, { auth: "staff" }).catch(() => null);
      if (updated) {
        setOrder(updated);
        if (updated.payment_status === "PAID") break;
      }
    }
    setSyncingPayment(false);
  }

  const allWeighed = order?.items.every((i) => i.actual_weight_kg != null) ?? false;
  const isFinalized = order?.total_amount != null;

  return (
    <div className="min-h-[calc(100vh-4rem)] bg-slate-50 py-12">
      <Container className="max-w-2xl">
        <Link href="/staff" className="mb-4 inline-block text-sm text-slate-500 underline hover:text-slate-700">
          ← Kembali ke Portal Staf
        </Link>

        {error && <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
        {!order && !error && <p className="text-sm text-slate-500">Memuat…</p>}

        {order && (
          <div className="rounded-2xl border border-slate-100 bg-white p-6 shadow-sm sm:p-8">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="text-xs text-slate-400">Order</p>
                <h1 className="font-heading text-xl font-bold text-navy-900">{order.order_code}</h1>
              </div>
              <div className="flex gap-2">
                <StatusBadge status={order.status} />
                <PaymentBadge status={order.payment_status} />
              </div>
            </div>

            <div className="mt-8 overflow-x-auto">
              <StatusTimeline status={order.status} />
            </div>

            {/* --- Items / weighing --- */}
            <div className="mt-8 space-y-3 border-t border-slate-100 pt-6">
              {order.items.map((item) => (
                <div key={item.id} className="flex items-center justify-between gap-3 text-sm">
                  <div className="flex-1">
                    <p className="font-medium text-navy-900">{item.service_name_snapshot}</p>
                    <p className="text-xs text-slate-500">
                      Estimasi: {item.estimated_weight_kg ?? "—"} {item.unit}
                    </p>
                  </div>
                  {(order.status === "RECEIVED" || order.status === "WEIGHING") && !item.is_locked ? (
                    <div className="flex items-center gap-2">
                      <Input
                        type="number"
                        step="0.1"
                        placeholder={item.actual_weight_kg ?? "kg"}
                        value={weights[item.id] ?? ""}
                        onChange={(e) => setWeights((w) => ({ ...w, [item.id]: e.target.value }))}
                        className="w-24"
                      />
                      <Button
                        variant="ghost"
                        disabled={busy || !weights[item.id]}
                        onClick={() => saveWeight(item.id)}
                        className="px-3 py-2 text-xs"
                      >
                        Simpan
                      </Button>
                    </div>
                  ) : (
                    <span className="text-slate-600">{item.actual_weight_kg ?? "—"} {item.unit}</span>
                  )}
                </div>
              ))}
            </div>

            <div className="mt-6 flex justify-between border-t border-slate-100 pt-6 text-sm font-semibold text-navy-900">
              <span>Total</span>
              <span>{formatIDR(order.total_amount ?? order.estimated_total_amount)}</span>
            </div>

            {/* --- Actions --- */}
            <div className="mt-6 flex flex-wrap gap-3 border-t border-slate-100 pt-6">
              {order.status === "CREATED" && (
                <Button disabled={busy} onClick={() => changeStatus("RECEIVED")}>
                  Terima Pesanan
                </Button>
              )}

              {(order.status === "RECEIVED" || order.status === "WEIGHING") &&
                allWeighed &&
                !isFinalized && (
                  <Button disabled={busy} onClick={finalizeWeighing}>
                    Finalisasi Penimbangan
                  </Button>
                )}

              {order.status === "WEIGHING" && isFinalized && order.payment_status !== "PAID" && (
                <div className="flex items-center gap-3">
                  <Button disabled={busy || syncingPayment} onClick={acceptCashPayment}>
                    Terima Pembayaran Tunai
                  </Button>
                  {syncingPayment && (
                    <span className="text-xs text-slate-500">
                      Menyinkronkan status pembayaran…
                    </span>
                  )}
                </div>
              )}

              {NEXT_STATUS[order.status]?.map((next) => (
                <Button
                  key={next.to}
                  disabled={busy || (next.to === "COMPLETED" && order.payment_status !== "PAID")}
                  onClick={() => changeStatus(next.to)}
                >
                  {next.label}
                </Button>
              ))}

              {order.status === "COMPLETED" && (
                <p className="text-sm font-medium text-green-700">Pesanan selesai. 🎉</p>
              )}
            </div>
          </div>
        )}
      </Container>
    </div>
  );
}
