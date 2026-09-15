"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Container } from "@/components/ui/Container";
import { Button } from "@/components/ui/Button";
import { OrderCard } from "@/components/order/OrderCard";
import { useStaffSession } from "@/lib/useSession";
import { clearStaffSession } from "@/lib/auth";
import { apiFetch } from "@/lib/api";
import type { Order } from "@/lib/types";

export default function StaffDashboardPage() {
  const router = useRouter();
  const session = useStaffSession();
  const [orders, setOrders] = useState<Order[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!session) return;
    apiFetch<{ data: Order[] }>("/api/v1/orders", { auth: "staff" })
      .then((res) => setOrders(res.data ?? []))
      .catch(() => setError("Tidak dapat memuat daftar pesanan."));
  }, [session]);

  if (!session) return null;

  return (
    <div className="min-h-[calc(100vh-4rem)] bg-slate-50 py-12">
      <Container>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 className="font-heading text-2xl font-bold text-navy-900">
              Portal Staf — {session.user.full_name}
            </h1>
            <p className="mt-1 text-sm text-slate-500">
              Peran: {session.user.roles.join(", ")}
            </p>
          </div>
          <Button
            variant="outline"
            className="!border-brand-600 !text-brand-700"
            onClick={() => {
              clearStaffSession();
              router.push("/staff/login");
            }}
          >
            Keluar
          </Button>
        </div>

        <h2 className="mt-10 mb-4 font-heading text-lg font-semibold text-navy-900">
          Semua Pesanan
        </h2>

        {error && <p className="text-sm text-red-600">{error}</p>}
        {orders === null && !error && <p className="text-sm text-slate-500">Memuat…</p>}
        {orders?.length === 0 && <p className="text-sm text-slate-500">Belum ada pesanan.</p>}

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {orders?.map((order) => (
            <OrderCard key={order.id} order={order} href={`/staff/orders/${order.id}`} />
          ))}
        </div>

        <p className="mt-10 text-xs text-slate-400">
          <Link href="/" className="underline">
            Kembali ke situs
          </Link>
        </p>
      </Container>
    </div>
  );
}
