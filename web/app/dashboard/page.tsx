"use client";

import { useEffect, useState } from "react";
import { Container } from "@/components/ui/Container";
import { LinkButton } from "@/components/ui/Button";
import { OrderCard } from "@/components/order/OrderCard";
import { useCustomerSession } from "@/lib/useSession";
import { apiFetch } from "@/lib/api";
import type { Order } from "@/lib/types";

export default function DashboardPage() {
  const session = useCustomerSession();
  const [orders, setOrders] = useState<Order[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!session) return;
    apiFetch<{ data: Order[] }>(`/api/v1/orders?customer_id=${session.customer.id}`)
      .then((res) => setOrders(res.data ?? []))
      .catch(() => setError("Tidak dapat memuat daftar pesanan."));
  }, [session]);

  if (!session) return null;

  return (
    <div className="bg-slate-50 py-12">
      <Container>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 className="font-heading text-2xl font-bold text-navy-900">
              Halo, {session.customer.name || "Pelanggan"} 👋
            </h1>
            <p className="mt-1 text-sm text-slate-500">
              Kode pelanggan: {session.customer.customer_code}
            </p>
          </div>
          <LinkButton href="/order/new">+ Buat Pesanan Baru</LinkButton>
        </div>

        <h2 className="mt-10 mb-4 font-heading text-lg font-semibold text-navy-900">
          Pesanan Anda
        </h2>

        {error && <p className="text-sm text-red-600">{error}</p>}

        {orders === null && !error && (
          <p className="text-sm text-slate-500">Memuat pesanan…</p>
        )}

        {orders?.length === 0 && (
          <div className="rounded-2xl border border-dashed border-slate-200 bg-white p-10 text-center">
            <p className="text-slate-500">Belum ada pesanan.</p>
            <LinkButton href="/order/new" className="mt-4 inline-flex">
              Buat Pesanan Pertama
            </LinkButton>
          </div>
        )}

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {orders?.map((order) => (
            <OrderCard key={order.id} order={order} href={`/orders/${order.id}`} />
          ))}
        </div>
      </Container>
    </div>
  );
}
