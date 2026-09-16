"use client";

import { use, useEffect, useState } from "react";
import { Container } from "@/components/ui/Container";
import { StatusBadge, PaymentBadge } from "@/components/order/StatusBadge";
import { StatusTimeline } from "@/components/order/StatusTimeline";
import { PaymentNotice } from "@/components/order/PaymentNotice";
import { PaymentActions } from "@/components/order/PaymentActions";
import { useCustomerSession } from "@/lib/useSession";
import { apiFetch } from "@/lib/api";
import { formatIDR, type Order } from "@/lib/types";

export default function CustomerOrderDetailPage({
  params,
}: PageProps<"/orders/[id]">) {
  const { id } = use(params);
  const session = useCustomerSession();
  const [order, setOrder] = useState<Order | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!session) return;
    apiFetch<Order>(`/api/v1/orders/${id}`)
      .then(setOrder)
      .catch(() => setError("Pesanan tidak ditemukan atau Anda tidak berhak melihatnya."));
  }, [session, id]);

  if (!session) return null;

  return (
    <div className="bg-slate-50 py-12">
      <Container className="max-w-2xl">
        {error && <p className="text-sm text-red-600">{error}</p>}
        {!order && !error && <p className="text-sm text-slate-500">Memuat pesanan…</p>}

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

            <div className="mt-8 space-y-3 border-t border-slate-100 pt-6">
              {order.items.map((item) => (
                <div key={item.id} className="flex items-center justify-between text-sm">
                  <div>
                    <p className="font-medium text-navy-900">{item.service_name_snapshot}</p>
                    <p className="text-xs text-slate-500">
                      {item.actual_weight_kg ?? item.estimated_weight_kg ?? "—"} {item.unit}
                      {!item.actual_weight_kg && " (estimasi)"}
                    </p>
                  </div>
                  <span className="text-slate-600">{formatIDR(item.subtotal)}</span>
                </div>
              ))}
            </div>

            <div className="mt-6 space-y-2 border-t border-slate-100 pt-6 text-sm">
              <div className="flex justify-between text-slate-500">
                <span>Subtotal</span>
                <span>{formatIDR(order.subtotal_amount)}</span>
              </div>
              {order.tax_amount > 0 && (
                <div className="flex justify-between text-slate-500">
                  <span>Pajak ({order.tax_rate_snapshot}%)</span>
                  <span>{formatIDR(order.tax_amount)}</span>
                </div>
              )}
              <div className="flex justify-between border-t border-slate-100 pt-2 font-semibold text-navy-900">
                <span>Total</span>
                <span>{formatIDR(order.total_amount ?? order.estimated_total_amount)}</span>
              </div>
            </div>

            <PaymentNotice status={order.payment_status} />
            {order.payment_status === "UNPAID" && order.total_amount !== undefined && (
              <PaymentActions orderId={order.id} amount={order.total_amount} />
            )}
          </div>
        )}
      </Container>
    </div>
  );
}
